package schemaui_test

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	schemaui "github.com/leoheung/mygo-schema-interpreter"
)

// The host side of a schema-driven page: register the tables once, fetch the
// schema (here from a constant; over HTTP or an argot frame elsewhere),
// parse it, hand it to the renderer, and call Draw in the page's view
// function. The renderer only sees JSON — transports are the caller's
// business.
func Example() {
	reg := schemaui.NewRegistry()

	// A data source: binds like {"source":"home","path":"summoner.name"}
	// resolve against what it returns. In the yuumi app this wraps a flow
	// snapshot.
	reg.RegisterProvider("home", func() any {
		return map[string]any{
			"summoner": map[string]any{"name": "Leo", "level": 214},
		}
	})

	// A composite component: native code for a widget too rich to describe
	// in primitives. The schema references it by id and passes props.
	reg.RegisterComponent("playerCard", func(c *ui.Context, r *schemaui.Renderer, node *schemaui.Node) {
		ui.Box(c).Padding(12).Radius(8).Children(func() {
			ui.Text(c, node.Props.Label).Bold()
		})
	})

	// An action: what a schema button fires. It can read the state store.
	reg.RegisterAction("search", func(r *schemaui.Renderer, args map[string]any) {
		fmt.Println("searching for", args["q"], "with input", r.State().String("form.q"))
	})

	schema, err := schemaui.ParseSchema([]byte(`{
		"type": "column",
		"props": {"gap": 12, "padding": [24]},
		"children": [
			{"type": "text", "bind": {"source": "home", "path": "summoner.name"}, "props": {"fontSize": 18, "bold": true}},
			{"type": "component", "componentId": "playerCard", "props": {"label": "Lane: Support"}},
			{"type": "textInput", "key": "form.q", "props": {"placeholder": "Summoner name"}},
			{"type": "button", "props": {"text": "Search"}, "action": {"id": "search", "args": {"q": "leo"}}}
		]
	}`))
	if err != nil {
		fmt.Println("bad schema:", err)
		return
	}

	r := schemaui.NewRenderer(reg)
	r.SetSchema(schema)

	// In the page's view function, every frame:
	//	r.Draw(c)
}

// A container iterating a bound list with a per-item template: the row
// binds to source "home" path "team", and the template's text binds to
// "$item.name" for each element. Unknown types and failed binds degrade to
// placeholders rather than panicking, so a newer schema runs on an older
// host.
func Example_iterate() {
	reg := schemaui.NewRegistry()
	reg.RegisterProvider("home", func() any {
		return map[string]any{
			"team": []any{
				map[string]any{"name": "leo", "champ": "Yuumi"},
				map[string]any{"name": "zoe", "champ": "Ekko"},
			},
		}
	})
	r := schemaui.NewRenderer(reg)
	r.SetSchema(&schemaui.Node{
		Type:  schemaui.TypeColumn,
		Props: schemaui.Props{Gap: ptr(8)},
		Bind:  &schemaui.Bind{Source: "home", Path: "team"},
		Items: []schemaui.Node{{
			Type: schemaui.TypeRow,
			Props: schemaui.Props{
				Gap: ptr(6),
			},
			Children: []schemaui.Node{
				{Type: schemaui.TypeText, Bind: &schemaui.Bind{Source: schemaui.SourceItem, Path: "champ"}, Props: schemaui.Props{Bold: true}},
				{Type: schemaui.TypeText, Bind: &schemaui.Bind{Source: schemaui.SourceItem, Path: "name"}},
			},
		}},
	})
	// r.Draw(c) in the page's view.
}

func ptr(f float32) *float32 { return &f }
