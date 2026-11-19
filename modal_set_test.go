package schemaui

import (
	"testing"
)

// v0.2：modal 原语与 action.set 的解析与纯逻辑语义（不画 UI）。

func TestModalAndActionSetParse(t *testing.T) {
	doc := `{
		"type": "modal",
		"props": {"stateKey": "detailOpen", "width": 960},
		"children": [
			{"type": "button", "props": {"text": "close"},
			 "action": {"set": {"detailOpen": false}}}
		]
	}`
	n, err := ParseSchema([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Type != TypeModal {
		t.Fatalf("type = %q, want modal", n.Type)
	}
	if n.Props.StateKey != "detailOpen" {
		t.Fatalf("stateKey = %q, want detailOpen", n.Props.StateKey)
	}
	if len(n.Children) != 1 {
		t.Fatalf("children = %d, want 1", len(n.Children))
	}
	set := n.Children[0].Action.Set
	if v, ok := set["detailOpen"].(bool); !ok || v {
		t.Fatalf("set[detailOpen] = %#v, want false", set["detailOpen"])
	}
}

func TestFireAppliesSetWithoutHandler(t *testing.T) {
	r := NewRenderer(nil)
	// 纯声明式 action：无 id、无 toast —— c 不会被使用。
	r.fire(nil, &Action{Set: map[string]any{
		"detailOpen":  true,
		"detailIndex": "3",
	}})
	if !r.State().Bool("detailOpen") {
		t.Fatal("set must write the bool state")
	}
	if r.State().String("detailIndex") != "3" {
		t.Fatalf("set must write the string state, got %q", r.State().String("detailIndex"))
	}

	// 再次 fire 覆盖旧值
	r.fire(nil, &Action{Set: map[string]any{"detailOpen": false}})
	if r.State().Bool("detailOpen") {
		t.Fatal("set must overwrite")
	}
}

func TestFireActionExported(t *testing.T) {
	r := NewRenderer(nil)
	called := false
	r.Registry().RegisterAction("ping", func(_ *Renderer, _ map[string]any) { called = true })
	r.FireAction(nil, &Action{ID: "ping", Set: map[string]any{"ok": true}})
	if !called {
		t.Fatal("FireAction must dispatch the registered handler")
	}
	if !r.State().Bool("ok") {
		t.Fatal("FireAction must apply the set side effect")
	}
}

func TestTextTokenResolution(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterText("hello", func() string { return "你好" })
	r := NewRenderer(reg)

	if got := r.text("{{loc:hello}}"); got != "你好" {
		t.Fatalf("registered token = %q, want 你好", got)
	}
	if got := r.text("{{loc:missing}}"); got != "{{loc:missing}}" {
		t.Fatalf("unregistered token must pass through, got %q", got)
	}
	if got := r.text("plain text"); got != "plain text" {
		t.Fatalf("plain literal must not change, got %q", got)
	}
	if got := r.text("{{loc:}}"); got != "{{loc:}}" {
		t.Fatalf("empty key must pass through, got %q", got)
	}
}

func TestResolveActionItemPath(t *testing.T) {
	r := NewRenderer(nil)
	r.Registry().RegisterAction("noop", func(_ *Renderer, _ map[string]any) {})

	it := &iteration{value: map[string]any{"RiotID": "玩家#54192"}, index: 2}
	a := r.resolveAction(&Action{
		ID:   "noop",
		Args: map[string]any{"name": "$item.RiotID", "index": "$index", "keep": "plain"},
	}, it)

	if a.Args["name"] != "玩家#54192" {
		t.Fatalf("name = %v, want resolved riot id", a.Args["name"])
	}
	if a.Args["index"] != 2 {
		t.Fatalf("index = %v, want 2", a.Args["index"])
	}
	if a.Args["keep"] != "plain" {
		t.Fatalf("plain arg must pass through, got %v", a.Args["keep"])
	}

	// 无迭代上下文：原样返回
	if got := r.resolveAction(a, nil); got != a {
		t.Fatal("nil iteration must return the action untouched")
	}
}

func TestVisibleIfParse(t *testing.T) {
	doc := `{
		"type": "row",
		"children": [
			{"type": "text", "props": {"text": "badge"},
			 "visibleIf": {"source": "home", "path": "Summoner.InGame"}}
		]
	}`
	n, err := ParseSchema([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Children[0].VisibleIf == nil || n.Children[0].VisibleIf.Source != "home" {
		t.Fatal("visibleIf must parse as a bind spec")
	}
}

func TestFireUnknownIDStillAppliesSet(t *testing.T) {
	r := NewRegistry() // 空 registry：未注册 id 降级跳过，set 照常
	rd := NewRenderer(r)
	rd.fire(nil, &Action{ID: "nope", Set: map[string]any{"ok": true}})
	if !rd.State().Bool("ok") {
		t.Fatal("unknown action id must not block set")
	}
}
