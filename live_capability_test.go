package schemaui

import "testing"

// v0.2.12：hoverOnly 子树、textColorBind/borderColorBind、singleLine/wrap
// 的解析契约（绘制路径需 UI，纯逻辑断言落在解析与字段上）。

func TestLivePageCapabilityParse(t *testing.T) {
	doc := `{
		"type": "box",
		"props": {"border": {"width": 1}, "borderColorBind": {"source": "$item", "path": "BorderToken"}},
		"children": [
			{"type": "component", "componentId": "openButton",
			 "hoverOnly": true,
			 "bind": {"source": "$item", "path": "RiotID"}},
			{"type": "text", "bind": {"source": "$item", "path": "Name"},
			 "props": {"singleLine": true, "textColorBind": {"source": "$item", "path": "NameColor"}}},
			{"type": "text", "props": {"wrap": true, "text": "note"}}
		]
	}`
	n, err := ParseSchema([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	open := n.Children[0]
	if !open.HoverOnly {
		t.Fatal("hoverOnly must parse onto the node")
	}
	if open.Bind == nil || open.Bind.Path != "RiotID" {
		t.Fatal("component bind must parse (label from item)")
	}
	name := n.Children[1]
	if !name.Props.SingleLine {
		t.Fatal("singleLine must parse")
	}
	if name.Props.TextColorBind == nil || name.Props.TextColorBind.Path != "NameColor" {
		t.Fatal("textColorBind must parse")
	}
	if !n.Children[2].Props.Wrap {
		t.Fatal("wrap must parse")
	}
	if n.Props.BorderColorBind == nil || n.Props.BorderColorBind.Path != "BorderToken" {
		t.Fatal("borderColorBind must parse")
	}
	if n.Props.Border == nil || n.Props.Border.Width != 1 {
		t.Fatal("border width must parse alongside borderColorBind")
	}
}

func TestGridParse(t *testing.T) {
	doc := `{
		"type": "grid",
		"props": {"grow": 1, "basis": 0, "columns": 5, "gap": 8},
		"bind": {"source": "live", "path": "Ally"},
		"items": [{"type": "text", "props": {"text": "card"}}]
	}`
	n, err := ParseSchema([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Type != TypeGrid {
		t.Fatalf("type = %q, want grid", n.Type)
	}
	if n.Props.Columns == nil || *n.Props.Columns != 5 {
		t.Fatalf("columns = %v, want 5", n.Props.Columns)
	}
	if n.Bind == nil || n.Bind.Source != "live" {
		t.Fatal("grid bind must parse for items iteration")
	}
}
