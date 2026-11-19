package schemaui

import (
	"sync"

	"github.com/egoist/mygo/ui"
)

// ComponentFunc draws a composite component: a widget the host builds in
// native code (a player card, a tier badge) that a schema references by id.
// It receives the renderer's context, the renderer itself (state, registry)
// and the referencing node, whose Props carry the component's arguments.
type ComponentFunc func(c *ui.Context, r *Renderer, node *Node)

// ProviderFunc is a data source: it returns the current value a bind resolves
// against. A provider typically returns a snapshot of the host's state (a
// flow snapshot, a cached struct, a map).
type ProviderFunc func() any

// ActionFunc is a host action a button can fire. It may read and write the
// renderer's state store and kick off host work.
type ActionFunc func(r *Renderer, args map[string]any)

// ColorFunc resolves a color token at draw time; it may read the context's
// palette, so a token stays correct across theme switches.
type ColorFunc func(c *ui.Context) ui.Color

// TextFunc resolves a text token ({{loc:key}}) at draw time — the host's
// current-locale catalog accessor, so a schema stays correct across
// language switches.
type TextFunc func() string

// ToastFunc receives a fully-resolved toast (loc tokens and state templates
// already expanded) for the host to render its own way — through the app's
// own toast system, for instance. Registering one takes over toast
// rendering; without a registration, toasts go through mygo's built-in
// AddToast.
type ToastFunc func(t ui.Toast)

// Registry holds the host's tables: composite components, data-source
// providers, actions, color tokens and text tokens. A schema references
// them by name; registering the same name overwrites. All methods are safe
// for concurrent use, though the usual pattern is registering everything up
// front and only looking up while drawing.
type Registry struct {
	mu         sync.RWMutex
	components map[string]ComponentFunc
	providers  map[string]ProviderFunc
	actions    map[string]ActionFunc
	colors     map[string]ColorFunc
	texts      map[string]TextFunc
	toast      ToastFunc
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		components: make(map[string]ComponentFunc),
		providers:  make(map[string]ProviderFunc),
		actions:    make(map[string]ActionFunc),
		colors:     make(map[string]ColorFunc),
		texts:      make(map[string]TextFunc),
	}
}

// RegisterComponent registers a composite component under id.
func (g *Registry) RegisterComponent(id string, f ComponentFunc) {
	g.mu.Lock()
	g.components[id] = f
	g.mu.Unlock()
}

// RegisterProvider registers a data source under name.
func (g *Registry) RegisterProvider(name string, f ProviderFunc) {
	g.mu.Lock()
	g.providers[name] = f
	g.mu.Unlock()
}

// RegisterAction registers an action under id.
func (g *Registry) RegisterAction(id string, f ActionFunc) {
	g.mu.Lock()
	g.actions[id] = f
	g.mu.Unlock()
}

// RegisterColor registers a color token schemas may use wherever a color
// string is accepted (background, border, text): "textMuted" and friends
// resolve to the host's current theme palette.
func (g *Registry) RegisterColor(name string, f ColorFunc) {
	g.mu.Lock()
	g.colors[name] = f
	g.mu.Unlock()
}

func (g *Registry) color(name string) ColorFunc {
	g.mu.RLock()
	f := g.colors[name]
	g.mu.RUnlock()
	return f
}

// RegisterText registers a text token: a schema literal "{{loc:key}}" in a
// text, button, select option or toast title resolves through f at draw
// time — the host's own i18n stays the single source of copy.
func (g *Registry) RegisterText(key string, f TextFunc) {
	g.mu.Lock()
	g.texts[key] = f
	g.mu.Unlock()
}

func (g *Registry) text(key string) TextFunc {
	g.mu.RLock()
	f := g.texts[key]
	g.mu.RUnlock()
	return f
}

// RegisterToast registers the host's toast renderer. With one registered,
// every schema toast is handed to it fully resolved; without one, toasts
// render through mygo's built-in AddToast.
func (g *Registry) RegisterToast(f ToastFunc) {
	g.mu.Lock()
	g.toast = f
	g.mu.Unlock()
}

func (g *Registry) toastHandler() ToastFunc {
	g.mu.RLock()
	f := g.toast
	g.mu.RUnlock()
	return f
}

func (g *Registry) component(id string) ComponentFunc {
	g.mu.RLock()
	f := g.components[id]
	g.mu.RUnlock()
	return f
}

func (g *Registry) provider(name string) ProviderFunc {
	g.mu.RLock()
	f := g.providers[name]
	g.mu.RUnlock()
	return f
}

func (g *Registry) action(id string) ActionFunc {
	g.mu.RLock()
	f := g.actions[id]
	g.mu.RUnlock()
	return f
}
