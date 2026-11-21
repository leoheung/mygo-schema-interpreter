// Package schemaui is a UI-schema interpreter for the mygo framework
// (github.com/egoist/mygo): JSON in, native mygo widgets out.
//
// The package knows nothing about transports. A caller that fetches schemas
// over HTTP, from a file, or through an obfuscated protocol (argot or
// otherwise) decodes the bytes to JSON itself and feeds them to [ParseSchema]
// and [Renderer.SetSchema].
//
// A schema is a tree of [Node]s. Nodes of primitive types map straight onto
// mygo widget constructors; a node of type [TypeComponent] names a composite
// component the host registered up front, which keeps schemas small for rich
// widgets (a player card, a tier badge). Nodes may bind to host-registered
// data sources and dispatch host-registered actions, so a schema describes
// layout while data and behavior stay native.
//
// Unknown types, unregistered components and failed bindings degrade to a
// placeholder instead of panicking, so an older client still runs a newer
// schema.
package schemaui

import "encoding/json"

// Node types. The set is open: render draws a placeholder for an unknown type.
const (
	TypeColumn    = "column"    // ui.Column, children top to bottom
	TypeRow       = "row"       // ui.Row, children left to right
	TypeBox       = "box"       // ui.Box, the generic container
	TypeScroll    = "scroll"    // ui.Scroll, scrollable child
	TypeText      = "text"      // ui.Text
	TypeImage     = "image"     // ui.Image, src from props.src (data URI) or bind
	TypeButton    = "button"    // ui.Button, fires props-declared action on click
	TypeTextInput = "textInput" // ui.TextInput, value kept in the state store
	TypeCheckbox  = "checkbox"  // ui.Checkbox
	TypeSelect    = "select"    // ui.Select
	TypeSpinner   = "spinner"   // ui.Spinner
	TypeDivider   = "divider"   // ui.Divider
	TypeSpacer    = "spacer"    // ui.Spacer
	// TypeModal draws its children inside a mygo modal overlay while the
	// bool state at props.stateKey is true; buttons open and close it by
	// writing that state (an action handler or the declarative action.set).
	TypeModal = "modal"
	// TypeComponent renders a composite component the host registered under
	// node.ComponentID. The component draws with the renderer's context,
	// state and registry, and reads its arguments from node.Props.
	TypeComponent = "component"
)

// Node is one node of a UI schema tree.
type Node struct {
	// Type selects the primitive, or TypeComponent for a registered
	// composite component.
	Type string `json:"type"`
	// Key is the node's stable identity for cross-frame state (text inputs,
	// scroll offsets, focus). When empty the tree position ("0.2.1") is used.
	Key string `json:"key,omitempty"`
	// Props carries layout, style and type-specific content.
	Props Props `json:"props,omitempty"`
	// Bind connects the node to a data source: text shows the value, image
	// shows the bitmap, and a container with Items iterates a slice.
	Bind *Bind `json:"bind,omitempty"`
	// VisibleIf resolves a bool: the node (and its subtree) is skipped
	// while it is false — badges, unranked placeholders, loading states.
	VisibleIf *Bind `json:"visibleIf,omitempty"`
	// HoverOnly draws the node only while the pointer is over its parent
	// container — a card's floating action that appears on hover. Without
	// a hoverable parent (or while not hovered) the node is skipped.
	HoverOnly bool `json:"hoverOnly,omitempty"`
	// Action is what a button fires when clicked.
	Action *Action `json:"action,omitempty"`
	// Children are drawn inside the node.
	Children []Node `json:"children,omitempty"`
	// ComponentID names the registered component for a TypeComponent node.
	ComponentID string `json:"componentId,omitempty"`
	// Items is the per-item template drawn once for every element of the
	// bound slice. Inside Items a bind with source "$item" resolves against
	// the current item.
	Items []Node `json:"items,omitempty"`
}

// Bind resolves a value from a host data source. Path is a "."-separated walk
// where a numeric segment indexes a slice and any other segment keys a map or
// a struct field: "team.0.name".
type Bind struct {
	Source string `json:"source"` // registered provider name, or "$item"
	Path   string `json:"path,omitempty"`
}

// SourceItem is the bind source that refers to the current item of an
// iterating container.
const SourceItem = "$item"

// Action names a host-registered action and carries its arguments. The
// declarative Set side effect writes values into the renderer's state store
// after the handler (if any) ran — a schema can toggle a modal or flip a
// page marker without the host registering anything. The optional Toast side
// effect fires after Set, so a toast can echo the new state.
type Action struct {
	ID    string         `json:"id"`
	Args  map[string]any `json:"args,omitempty"`
	Set   map[string]any `json:"set,omitempty"` // stateKey → bool | string
	Toast *ActionToast   `json:"toast,omitempty"`
}

// ActionToast is an automatic toast the interpreter fires as an action
// side effect. Title and Description support {{stateKey}} templates that
// expand against the renderer's StateStore so a toast can echo the current
// value of any input widget.
//
//	"action": {
//	  "id": "echo",
//	  "toast": { "title": "Echo: {{demo.input}}", "type": "success" }
//	}
type ActionToast struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`        // success / error / warning (default: info)
	DurationMs  int    `json:"durationMs,omitempty"`  // 0 → mygo default (4s)
}

// ParseSchema decodes a schema document into a Node tree. The input is plain
// JSON; where it came from (HTTP, argot frame, local file) is the caller's
// business.
func ParseSchema(data []byte) (*Node, error) {
	n := new(Node)
	if err := json.Unmarshal(data, n); err != nil {
		return nil, err
	}
	if n.Type == "" {
		return nil, ErrEmptyType
	}
	return n, nil
}
