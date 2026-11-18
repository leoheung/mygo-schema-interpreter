package schemaui

import (
	"strconv"
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
}

// NewRenderer returns a renderer drawing through reg's tables. A nil registry
// is treated as empty: everything without a primitive implementation
// degrades to a placeholder.
func NewRenderer(reg *Registry) *Renderer {
	if reg == nil {
		reg = NewRegistry()
	}
	return &Renderer{reg: reg, state: newStateStore()}
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
	root := r.root.Load()
	if root == nil {
		ui.Spinner(c)
		return
	}
	r.drawNode(c, root, "0", nil)
}

// drawNode draws one node at a tree path under an enclosing iteration. The
// node's key for cross-frame state is its explicit Key when set, else the
// path, so a stable schema keeps input and scroll state across frames.
func (r *Renderer) drawNode(c *ui.Context, n *Node, path string, it *iteration) {
	if n == nil {
		return
	}
	if n.VisibleIf != nil {
		if v, ok := r.lookup(n.VisibleIf, it); !ok || v != true {
			return // the condition is false or unresolvable: skip the subtree
		}
	}
	key := n.Key
	if key == "" {
		key = path
	}
	kc := c.Key(key)

	switch n.Type {
	case TypeColumn:
		r.reg.applyProps(c, ui.Column(kc), &n.Props).Children(func() {
			r.drawContent(c, n, path, it)
		})
	case TypeRow:
		el := r.reg.applyProps(c, ui.Row(kc), &n.Props)
		el = r.bindPaint(c, el, n, it)
		r.clickable(c, el, n, it)
		el.Children(func() {
			r.drawContent(c, n, path, it)
		})
	case TypeBox:
		el := r.reg.applyProps(c, ui.Box(kc), &n.Props)
		el = r.bindPaint(c, el, n, it)
		r.clickable(c, el, n, it)
		el.Children(func() {
			r.drawContent(c, n, path, it)
		})
	case TypeScroll:
		r.reg.applyProps(c, ui.Scroll(kc), &n.Props).Children(func() {
			r.drawContent(c, n, path, it)
		})
	case TypeText:
		text := n.Props.Text
		if v, ok := r.lookup(n.Bind, it); ok {
			text = toText(v)
		}
		r.reg.applyProps(c, ui.Text(kc, text), &n.Props)
	case TypeImage:
		if src := r.resolveImage(n, it); src != nil {
			r.reg.applyProps(c, ui.Image(kc, src), &n.Props)
		} else {
			r.drawFallback(c, "image: no source")
		}
	case TypeButton:
		btn := r.reg.applyProps(c, ui.Button(kc, n.Props.Text), &n.Props)
		btn.Children(func() {
			r.drawContent(c, n, path, it)
		})
		if btn.Clicked() {
			r.fire(c, n.Action)
		}
	case TypeTextInput:
		r.reg.applyProps(c, ui.TextInput(kc, r.state.StringPtr(key, n.Props.Value)), &n.Props)
	case TypeCheckbox:
		r.reg.applyProps(c, ui.Checkbox(kc, r.state.BoolPtr(key), n.Props.Label), &n.Props)
	case TypeSelect:
		r.reg.applyProps(c, ui.Select(kc, r.state.StringPtr(key, n.Props.Value), n.Props.Options), &n.Props)
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
			r.drawContent(c, n, path, it)
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
// each element; otherwise it draws the plain children.
func (r *Renderer) drawContent(c *ui.Context, n *Node, path string, it *iteration) {
	if n.Bind != nil && len(n.Items) > 0 {
		for i, item := range r.lookupSlice(n.Bind, it) {
			child := &iteration{parent: it, value: item, index: i}
			for j := range n.Items {
				r.drawNode(c, &n.Items[j], path+"."+itoa(i)+"."+itoa(j), child)
			}
		}
		return
	}
	for i := range n.Children {
		r.drawNode(c, &n.Children[i], path+"."+itoa(i), it)
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

// fireToast sends a toast through the mygo engine. Title and Description
// have {{stateKey}} templates expanded against the renderer's state store.
// DurationMs zero means the mygo default (4s); the toast ID re-uses the
// title so repeated clicks overwrite instead of stacking.
func (r *Renderer) fireToast(c *ui.Context, t *ActionToast) {
	duration := time.Duration(t.DurationMs) * time.Millisecond
	c.AddToast(ui.Toast{
		ID:          t.Title,
		Title:       expandStateTemplate(t.Title, r.state),
		Description: expandStateTemplate(t.Description, r.state),
		Type:        t.Type,
		Timeout:     duration,
	})
}

// bindPaint applies the data-bound paint overrides: BackgroundBind and
// BorderEdgeBind each resolve to a color string (token or hex).
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
	return el
}

// clickable wires a container's optional action: a pointer cursor and the
// click firing it — a row or box acts as a button without button styling.
// Inside an items iteration, an args value of "$index" becomes the item's
// position, so a row's click can name itself.
func (r *Renderer) clickable(c *ui.Context, el ui.Element, n *Node, it *iteration) {
	if n.Action == nil {
		return
	}
	el.Cursor(ui.CursorPointer)
	if el.Clicked() {
		a := n.Action
		if it != nil && len(a.Args) > 0 {
			args := make(map[string]any, len(a.Args))
			for k, v := range a.Args {
				if s, ok := v.(string); ok && s == "$index" {
					v = it.index
				}
				args[k] = v
			}
			a = &Action{ID: a.ID, Args: args, Set: a.Set, Toast: a.Toast}
		}
		r.fire(c, a)
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
