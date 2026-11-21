package schemaui

import (
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
)

// Renderer interprets a schema tree onto native mygo widgets. It is the
// schema counterpart of what the mygo markdown view does for an AST: every
// frame, Draw walks the current tree and calls the ui constructors, so a
// SetSchema from any goroutine simply takes effect on the next frame.
//
// A host page typically owns one Renderer, feeds it a schema once fetched,
// and calls Draw inside its view function:
//
//	reg := schemaui.NewRegistry()
//	// reg.RegisterProvider("home", ...), reg.RegisterComponent("playerCard", ...), ...
//	r := schemaui.NewRenderer(reg)
//	r.SetSchema(schema) // from ParseSchema, any goroutine
//	// in the page's view:
//	r.Draw(c)
type Renderer struct {
	root  atomic.Pointer[Node]
	reg   *Registry
	state *StateStore
	// frameProviders 具名 source 的当前帧解析值（UI 线程独占，无锁——
	// lookup 只在 Draw 链上被调）。Draw 开帧清空；fire 在帧中途写 state
	// 后也清——后续 lookup 重读 provider，动作的同帧效果（modal 内容、
	// 翻页）不因缓存滞后一帧。
	frameProviders map[string]any
}

// NewRenderer returns a renderer drawing through reg's tables. A nil registry
// is treated as empty: everything without a primitive implementation
// degrades to a placeholder.
func NewRenderer(reg *Registry) *Renderer {
	if reg == nil {
		reg = NewRegistry()
	}
	return &Renderer{reg: reg, state: newStateStore(), frameProviders: map[string]any{}}
}

// SetSchema swaps the tree to draw. Safe from any goroutine; the new tree
// shows on the next frame. A nil schema draws a spinner.
func (r *Renderer) SetSchema(n *Node) { r.root.Store(n) }

// Schema returns the current tree, or nil when none was set.
func (r *Renderer) Schema() *Node { return r.root.Load() }

// State returns the renderer's state store, where input widgets keep their
// values. Actions read and clear inputs through it.
func (r *Renderer) State() *StateStore { return r.state }

// Registry returns the registry the renderer draws through, so a host can
// register components lazily.
func (r *Renderer) Registry() *Registry { return r.reg }

// Draw draws the current schema tree. Call it from the page's view function
// every frame, like any mygo widget. With no schema set it draws a spinner.
func (r *Renderer) Draw(c *ui.Context) {
	// 开帧先清缓存——early-return 的帧（spinner）也不让快照跨帧残留。
	r.beginFrame()
	root := r.root.Load()
	if root == nil {
		ui.Spinner(c)
		return
	}
	r.drawNode(c, root, "0", nil, nil)
}

// hoverState reports whether the pointer is over the enclosing container;
// nil where there is no hoverable parent (the root, a modal overlay).
type hoverState func() bool

// beginFrame 开始新的一帧：清空具名 provider 的帧缓存（delete 复用 map，
// 不重新分配）。fire 在帧中途写 state 后也调用——后续 lookup 必须重读
// provider，动作的同帧效果（modal 内容、翻页）不因缓存滞后一帧。
func (r *Renderer) beginFrame() {
	for k := range r.frameProviders {
		delete(r.frameProviders, k)
	}
}

// drawNode draws one node at a tree path under an enclosing iteration. The
// node's key for cross-frame state is its explicit Key when set, else the
// path, so a stable schema keeps input and scroll state across frames.
// hover is the enclosing container's hover state, needed by hover-only
// nodes (nil at the root, where hover-only makes no sense).
func (r *Renderer) drawNode(c *ui.Context, n *Node, path string, it *iteration, hover hoverState) {
	if n == nil {
		return
	}
	if n.VisibleIf != nil {
		if v, ok := r.lookup(n.VisibleIf, it); !ok || v != true {
			return // the condition is false or unresolvable: skip the subtree
		}
	}
	if n.HoverOnly && (hover == nil || !hover()) {
		return // no hoverable parent, or the pointer is elsewhere
	}
	key := n.Key
	if key == "" {
		key = path
	}
	kc := c.Key(key)

	switch n.Type {
	case TypeColumn:
		el := r.reg.applyProps(c, ui.Column(kc), &n.Props)
		el.Children(func() {
			r.drawContent(c, n, path, it, el.Hovered)
		})
	case TypeRow:
		el := r.reg.applyProps(c, ui.Row(kc), &n.Props)
		el = r.bindPaint(c, el, n, it)
		el = r.hoverPaint(c, el, n, it)
		r.clickable(c, el, n, it)
		el.Children(func() {
			r.drawContent(c, n, path, it, el.Hovered)
		})
	case TypeBox:
		el := r.reg.applyProps(c, ui.Box(kc), &n.Props)
		el = r.bindPaint(c, el, n, it)
		el = r.hoverPaint(c, el, n, it)
		r.clickable(c, el, n, it)
		el.Children(func() {
			r.drawContent(c, n, path, it, el.Hovered)
		})
	case TypeScroll:
		el := r.reg.applyProps(c, ui.Scroll(kc), &n.Props)
		el.Children(func() {
			r.drawContent(c, n, path, it, el.Hovered)
		})
	case TypeText:
		text := r.text(n.Props.Text)
		if v, ok := r.lookup(n.Bind, it); ok {
			text = toText(v)
		}
		el := r.reg.applyProps(c, ui.Text(kc, text), &n.Props)
		if n.Props.TextColorBind != nil {
			if v, ok := r.lookup(n.Props.TextColorBind, it); ok {
				if col, ok := r.reg.resolveColor(c, toText(v)); ok {
					el = el.TextColor(col)
				}
			}
		}
	case TypeImage:
		// 原生对齐：ui.Image(nil) 画空——数据缺席（如某局没有技能资源）
		// 静默跳过，不画调试占位；契约违规的 fallback 只留给
		// component/unknown type。
		if src := r.resolveImage(n, it); src != nil {
			r.reg.applyProps(c, ui.Image(kc, src), &n.Props)
		}
	case TypeButton:
		btn := r.reg.applyProps(c, ui.Button(kc, r.text(n.Props.Text)), &n.Props)
		btn.Children(func() {
			r.drawContent(c, n, path, it, btn.Hovered)
		})
		if btn.Clicked() {
			r.fire(c, n.Action)
		}
	case TypeTextInput:
		r.reg.applyProps(c, ui.TextInput(kc, r.state.StringPtr(key, n.Props.Value)), &n.Props)
	case TypeCheckbox:
		r.reg.applyProps(c, ui.Checkbox(kc, r.state.BoolPtr(key), n.Props.Label), &n.Props)
	case TypeSelect:
		options := make([]string, len(n.Props.Options))
		for i, o := range n.Props.Options {
			options[i] = r.text(o)
		}
		r.reg.applyProps(c, ui.Select(kc, r.state.StringPtr(key, r.text(n.Props.Value)), options), &n.Props)
	case TypeSpinner:
		r.reg.applyProps(c, ui.Spinner(kc), &n.Props)
	case TypeDivider:
		r.reg.applyProps(c, ui.Divider(kc), &n.Props)
	case TypeSpacer:
		r.reg.applyProps(c, ui.Spacer(kc), &n.Props)
	case TypeModal:
		// The overlay follows the bool state at stateKey (the node's own
		// key when unset); buttons open and close it by writing that state.
		openKey := n.Props.StateKey
		if openKey == "" {
			openKey = key
		}
		r.reg.applyProps(c, ui.Modal(kc, r.state.BoolPtr(openKey), func() {
			r.drawContent(c, n, path, it, nil) // modal overlay: no hover parent
		}), &n.Props)
	case TypeComponent:
		n2 := n
		if n.Bind != nil {
			// A component's bind resolves to its label, so a component in an
			// iterating template takes its argument from the item. The copy
			// keeps the schema tree itself immutable.
			if v, ok := r.lookup(n.Bind, it); ok {
				cp := *n
				cp.Props.Label = toText(v)
				n2 = &cp
			}
		}
		if f := r.reg.component(n2.ComponentID); f != nil {
			a := r.resolveAction(n2.Action, it)
			if a != n2.Action {
				cp := *n2
				cp.Action = a
				n2 = &cp
			}
			f(c, r, n2)
			return
		}
		r.drawFallback(c, "component not registered: "+n2.ComponentID)
	default:
		r.drawFallback(c, "unknown type: "+n.Type)
	}
}

// drawContent draws a container's content: with a bind and item templates it
// iterates the bound slice and draws the templates with "$item" resolving to
// each element; otherwise it draws the plain children. hover is the
// container's hover state (for hover-only nodes; nil without a hoverable
// parent).
func (r *Renderer) drawContent(c *ui.Context, n *Node, path string, it *iteration, hover hoverState) {
	if n.Bind != nil && len(n.Items) > 0 {
		for i, item := range r.lookupSlice(n.Bind, it) {
			child := &iteration{parent: it, value: item, index: i}
			for j := range n.Items {
				r.drawNode(c, &n.Items[j], path+"."+itoa(i)+"."+itoa(j), child, hover)
			}
		}
		return
	}
	for i := range n.Children {
		r.drawNode(c, &n.Children[i], path+"."+itoa(i), it, hover)
	}
}

// resolveImage finds an image source for an image node: the bound value
// first, else the data URI in props.
func (r *Renderer) resolveImage(n *Node, it *iteration) ui.ImageSource {
	if v, ok := r.lookup(n.Bind, it); ok {
		if src := toImageSource(v); src != nil {
			return src
		}
	}
	if n.Props.Src != "" {
		return toImageSource(n.Props.Src)
	}
	return nil
}

// fire dispatches a button's action through the registry, applies the
// declarative Set side effect, then fires the optional Toast. An action with
// no handler id still runs Set and Toast — a purely declarative button (a
// modal's close button needs no host code).
func (r *Renderer) fire(c *ui.Context, a *Action) {
	if a == nil {
		return
	}
	// handler 与 Set 都可能写 state（宿主 handler 常绕过 setter 直写
	// StateStore），provider 帧缓存必须先于两者作废——后续同帧 lookup
	// 重读 provider，点击的当帧效果（modal 内容、翻页）不滞后。
	r.beginFrame()
	if a.ID != "" {
		if f := r.reg.action(a.ID); f != nil {
			f(r, a.Args)
		}
	}
	for k, v := range a.Set {
		switch t := v.(type) {
		case bool:
			*r.state.BoolPtr(k) = t
		case string:
			*r.state.StringPtr(k, "") = t
		}
	}
	if a.Toast != nil {
		r.fireToast(c, a.Toast)
	}
}

// fireToast sends a toast: Title and Description first resolve their
// {{loc:key}} tokens (the host's i18n), then {{stateKey}} templates expand
// against the state store — loc first, because the state expander would
// blank an unknown {{...}} key. The fully-resolved toast then goes to the
// host's registered toast renderer when there is one, mygo's built-in
// AddToast otherwise. DurationMs zero means the mygo default (4s); the
// toast ID re-uses the title so repeated clicks overwrite instead of
// stacking.
func (r *Renderer) fireToast(c *ui.Context, t *ActionToast) {
	duration := time.Duration(t.DurationMs) * time.Millisecond
	toast := ui.Toast{
		ID:          t.Title,
		Title:       expandStateTemplate(r.text(t.Title), r.state),
		Description: expandStateTemplate(r.text(t.Description), r.state),
		Type:        t.Type,
		Timeout:     duration,
	}
	if h := r.reg.toastHandler(); h != nil {
		h(toast)
		return
	}
	if c == nil {
		return
	}
	c.AddToast(toast)
}

// FireAction fires a schema action from host code — the entry composite
// components use to run the referencing node's action (handler, set side
// effects, toast) exactly as a button click would.
func (r *Renderer) FireAction(c *ui.Context, a *Action) {
	r.fire(c, a)
}

// text resolves a whole-string {{loc:key}} token through the registry's
// text table. Anything else — plain literals, unregistered keys, data from
// binds — returns untouched, so a missing translation shows as the token
// itself (visible, not silent).
func (r *Renderer) text(s string) string {
	const prefix, suffix = "{{loc:", "}}"
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) || len(s) <= len(prefix)+len(suffix) {
		return s
	}
	key := s[len(prefix) : len(s)-len(suffix)]
	if f := r.reg.text(key); f != nil {
		return f()
	}
	return s
}

// bindPaint applies the data-bound paint overrides: BackgroundBind,
// BorderEdgeBind and BorderColorBind each resolve to a color string (token
// or hex).
func (r *Renderer) bindPaint(c *ui.Context, el ui.Element, n *Node, it *iteration) ui.Element {
	if n.Props.BackgroundBind != nil {
		if v, ok := r.lookup(n.Props.BackgroundBind, it); ok {
			if col, ok := r.reg.resolveColor(c, toText(v)); ok {
				el = el.Background(col)
			}
		}
	}
	if n.Props.BorderEdgeBind != nil {
		if v, ok := r.lookup(n.Props.BorderEdgeBind, it); ok {
			if col, ok := r.reg.resolveColor(c, toText(v)); ok {
				el = el.BorderWidth(0, 0, 0, 4).BorderColor(col)
			}
		}
	}
	if n.Props.BorderColorBind != nil {
		if v, ok := r.lookup(n.Props.BorderColorBind, it); ok {
			if col, ok := r.reg.resolveColor(c, toText(v)); ok {
				w := float32(1)
				if n.Props.Border != nil {
					w = n.Props.Border.Width
				}
				el = el.BorderWidth(w).BorderColor(col)
			}
		}
	}
	return el
}

// hoverPaint applies the data-bound hover shade: while the pointer is over
// the element, HoverBackgroundBind overrides the base background — the
// immediate-mode counterpart of the native "if hovered, repaint" idiom.
func (r *Renderer) hoverPaint(c *ui.Context, el ui.Element, n *Node, it *iteration) ui.Element {
	if n.Props.HoverBackgroundBind != nil && el.Hovered() {
		if v, ok := r.lookup(n.Props.HoverBackgroundBind, it); ok {
			if col, ok := r.reg.resolveColor(c, toText(v)); ok {
				el = el.Background(col)
			}
		}
	}
	return el
}

// resolveAction substitutes iteration-dependent args: "$index" becomes the
// item's position, "$item.<path>" resolves a value off the current item
// (rendered as text). Called before the host sees the action — both from
// clickable rows and from composite components drawn inside an iteration.
func (r *Renderer) resolveAction(a *Action, it *iteration) *Action {
	if a == nil || it == nil || len(a.Args) == 0 {
		return a
	}
	args := make(map[string]any, len(a.Args))
	changed := false
	for k, v := range a.Args {
		if s, ok := v.(string); ok {
			switch {
			case s == "$index":
				v, changed = it.index, true
			case strings.HasPrefix(s, "$item."):
				if val, ok := r.lookup(&Bind{Source: SourceItem, Path: strings.TrimPrefix(s, "$item.")}, it); ok {
					v, changed = toText(val), true
				}
			}
		}
		args[k] = v
	}
	if !changed {
		return a
	}
	return &Action{ID: a.ID, Args: args, Set: a.Set, Toast: a.Toast}
}

// clickable wires a container's optional action: a pointer cursor and the
// click firing it — a row or box acts as a button without button styling.
// Iteration-dependent args are resolved against the current item first.
func (r *Renderer) clickable(c *ui.Context, el ui.Element, n *Node, it *iteration) {
	if n.Action == nil {
		return
	}
	el.Cursor(ui.CursorPointer)
	if el.Clicked() {
		r.fire(c, r.resolveAction(n.Action, it))
	}
}

// drawFallback draws a small muted placeholder box with a note, the one
// visual an unknown type, an unregistered component or a missing image
// source degrades to.
func (r *Renderer) drawFallback(c *ui.Context, note string) {
	ui.Box(c).Padding(4, 8).Radius(4).Children(func() {
		ui.Text(c, note).FontSize(11).Opacity(0.55)
	})
}

func itoa(i int) string { return strconv.Itoa(i) }
