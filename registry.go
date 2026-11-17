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

// Registry holds the host's three tables: composite components, data-source
// providers and actions. A schema references them by name; registering the
// same name overwrites. All methods are safe for concurrent use, though the
// usual pattern is registering everything up front and only looking up while
// drawing.
type Registry struct {
	mu         sync.RWMutex
	components map[string]ComponentFunc
	providers  map[string]ProviderFunc
	actions    map[string]ActionFunc
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		components: make(map[string]ComponentFunc),
		providers:  make(map[string]ProviderFunc),
		actions:    make(map[string]ActionFunc),
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
