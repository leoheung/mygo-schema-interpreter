package schemaui

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestParseSchema(t *testing.T) {
	doc := `{
		"type": "column",
		"props": {"gap": 12, "padding": [16], "background": "#1e1e2e"},
		"children": [
			{"type": "text", "props": {"text": "hello", "fontSize": 14, "bold": true}},
			{"type": "button", "props": {"text": "go"}, "action": {"id": "search", "args": {"q": "yuumi"}}},
			{"type": "component", "componentId": "playerCard", "props": {"label": "ally"}},
			{
				"type": "column",
				"bind": {"source": "home", "path": "team"},
				"items": [{"type": "text", "bind": {"source": "$item", "path": "name"}}]
			}
		]
	}`
	n, err := ParseSchema([]byte(doc))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if n.Type != TypeColumn {
		t.Fatalf("root type = %q, want column", n.Type)
	}
	p := n.Props
	if p.Gap == nil || *p.Gap != 12 {
		t.Errorf("gap = %v, want 12", p.Gap)
	}
	if len(p.Padding) != 1 || p.Padding[0] != 16 {
		t.Errorf("padding = %v, want [16]", p.Padding)
	}
	if len(n.Children) != 4 {
		t.Fatalf("children = %d, want 4", len(n.Children))
	}
	btn := n.Children[1]
	if btn.Action == nil || btn.Action.ID != "search" || btn.Action.Args["q"] != "yuumi" {
		t.Errorf("button action = %+v", btn.Action)
	}
	comp := n.Children[2]
	if comp.Type != TypeComponent || comp.ComponentID != "playerCard" || comp.Props.Label != "ally" {
		t.Errorf("component node = %+v", comp)
	}
	iter := n.Children[3]
	if iter.Bind == nil || iter.Bind.Source != "home" || iter.Bind.Path != "team" {
		t.Errorf("iter bind = %+v", iter.Bind)
	}
	if len(iter.Items) != 1 || iter.Items[0].Bind.Source != SourceItem {
		t.Errorf("items = %+v", iter.Items)
	}
}

func TestParseSchemaErrors(t *testing.T) {
	if _, err := ParseSchema([]byte(`{"type":""}`)); !errors.Is(err, ErrEmptyType) {
		t.Errorf("empty type: err = %v, want ErrEmptyType", err)
	}
	if _, err := ParseSchema([]byte(`not json`)); err == nil {
		t.Error("invalid json: want error")
	}
}

type stepTeam struct {
	Name  string
	Champ string
}

type stepHome struct {
	Team  []stepTeam
	Meta  map[string]any
	Ptr   *stepTeam
	Score float64
}

func TestStep(t *testing.T) {
	home := stepHome{
		Team:  []stepTeam{{Name: "leo", Champ: "yuumi"}},
		Meta:  map[string]any{"region": "euw"},
		Ptr:   &stepTeam{Name: "viaPtr"},
		Score: 12.5,
	}
	cases := []struct {
		v    any
		path string
		want any
	}{
		{home, "Team.0.Name", "leo"},
		{home, "Team.0.Champ", "yuumi"},
		{home, "Team.5.Name", nil},
		{home, "Team.x.Name", nil},
		{home, "Meta.region", "euw"},
		{home, "Meta.missing", nil},
		{home, "Ptr.Name", "viaPtr"},
		{home, "Score", 12.5},
		{map[string]any{"list": []any{"a", "b"}}, "list.1", "b"},
		{nil, "anything", nil},
	}
	for _, tc := range cases {
		v := tc.v
		var got any = v
		ok := true
		for _, seg := range splitPath(tc.path) {
			got = step(got, seg)
			if got == nil {
				ok = false
				break
			}
		}
		if !ok {
			got = nil
		}
		if got != tc.want {
			t.Errorf("step(%T, %q) = %v, want %v", tc.v, tc.path, got, tc.want)
		}
	}
}

func TestLookup(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterProvider("home", func() any {
		return map[string]any{"team": []any{map[string]any{"name": "leo"}}}
	})
	r := NewRenderer(reg)

	v, ok := r.lookup(&Bind{Source: "home", Path: "team.0.name"}, nil)
	if !ok || v != "leo" {
		t.Errorf("lookup home.team.0.name = %v, %v", v, ok)
	}
	if _, ok := r.lookup(&Bind{Source: "nowhere"}, nil); ok {
		t.Error("missing source resolved")
	}
	// $item with and without an enclosing iteration.
	it := &iteration{value: map[string]any{"name": "item0"}}
	v, ok = r.lookup(&Bind{Source: SourceItem, Path: "name"}, it)
	if !ok || v != "item0" {
		t.Errorf("$item.name = %v, %v", v, ok)
	}
	if _, ok := r.lookup(&Bind{Source: SourceItem}, nil); ok {
		t.Error("$item without iteration resolved")
	}
	// Nested iterations resolve inside-out.
	nested := &iteration{parent: it, value: map[string]any{"name": "inner"}}
	v, ok = r.lookup(&Bind{Source: SourceItem, Path: "name"}, nested)
	if !ok || v != "inner" {
		t.Errorf("nested $item.name = %v, %v", v, ok)
	}
}

func TestLookupSlice(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterProvider("list", func() any { return []string{"a", "b", "c"} })
	reg.RegisterProvider("one", func() any { return 7 })
	r := NewRenderer(reg)

	got := r.lookupSlice(&Bind{Source: "list"}, nil)
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Errorf("slice = %v", got)
	}
	got = r.lookupSlice(&Bind{Source: "one"}, nil)
	if len(got) != 1 || got[0] != 7 {
		t.Errorf("scalar as one-element slice = %v", got)
	}
	if s := r.lookupSlice(&Bind{Source: "missing"}, nil); s != nil {
		t.Errorf("missing source = %v, want nil", s)
	}
}

func TestToText(t *testing.T) {
	cases := []struct {
		v    any
		want string
	}{
		{nil, ""},
		{"plain", "plain"},
		{float64(5), "5"},
		{float64(12.5), "12.5"},
		{float32(1.25), "1.25"},
		{42, "42"},
		{true, "true"},
	}
	for _, tc := range cases {
		if got := toText(tc.v); got != tc.want {
			t.Errorf("toText(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestStateStore(t *testing.T) {
	s := newStateStore()
	p1 := s.StringPtr("email", "init@x")
	if *p1 != "init@x" {
		t.Fatalf("init value = %q", *p1)
	}
	*p1 = "changed"
	p2 := s.StringPtr("email", "other")
	if p2 != p1 || *p2 != "changed" {
		t.Error("StringPtr not stable across calls")
	}
	if s.String("email") != "changed" {
		t.Errorf("String(email) = %q", s.String("email"))
	}
	b1 := s.BoolPtr("tos")
	if *b1 {
		t.Error("new bool not false")
	}
	*b1 = true
	b2 := s.BoolPtr("tos")
	if b2 != b1 || !s.Bool("tos") {
		t.Error("BoolPtr not stable")
	}
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterComponent("playerCard", func(c *ui.Context, r *Renderer, n *Node) {})
	if reg.component("playerCard") == nil {
		t.Error("component not found after register")
	}
	reg.RegisterProvider("home", func() any { return 1 })
	if reg.provider("home") == nil {
		t.Error("provider not found")
	}
	reg.RegisterAction("search", func(r *Renderer, args map[string]any) {})
	if reg.action("search") == nil {
		t.Error("action not found")
	}
	if reg.component("nope") != nil || reg.provider("nope") != nil || reg.action("nope") != nil {
		t.Error("unregistered names resolved")
	}
}

func TestRendererSchemaSwap(t *testing.T) {
	r := NewRenderer(nil) // nil registry must not panic
	if r.Schema() != nil {
		t.Error("fresh renderer has a schema")
	}
	if r.State() == nil || r.Registry() == nil {
		t.Error("nil state or registry")
	}
	n := &Node{Type: TypeText}
	r.SetSchema(n)
	if r.Schema() != n {
		t.Error("SetSchema/Schema round trip")
	}
	r.SetSchema(nil)
	if r.Schema() != nil {
		t.Error("SetSchema(nil) did not clear")
	}
}

func TestEdges(t *testing.T) {
	cases := []struct {
		in   []float32
		want []float32
	}{
		{[]float32{8}, []float32{8, 8, 8, 8}},
		{[]float32{6, 10}, []float32{6, 10, 6, 10}},
		{[]float32{1, 2, 3}, []float32{1, 2, 3, 2}},
		{[]float32{1, 2, 3, 4}, []float32{1, 2, 3, 4}},
		{[]float32{1, 2, 3, 4, 5}, []float32{1, 2, 3, 4}},
		{nil, nil},
	}
	for _, tc := range cases {
		got := edges(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("edges(%v) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("edges(%v) = %v, want %v", tc.in, got, tc.want)
				break
			}
		}
	}
}

func TestDecodeDataURI(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte("img"))
	if b, ok := decodeDataURI("data:image/png;base64," + payload); !ok || string(b) != "img" {
		t.Errorf("standard base64 = %v, %v", b, ok)
	}
	if _, ok := decodeDataURI("https://example.com/a.png"); ok {
		t.Error("non-data URI decoded")
	}
	if _, ok := decodeDataURI("data:image/png;base64,!!!not-base64!!!"); ok {
		t.Error("invalid base64 decoded")
	}
}

func TestSplitPath(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"name", 1},
		{"team.0.name", 3},
		{"a..b", 2},
	}
	for _, tc := range cases {
		if got := splitPath(tc.in); len(got) != tc.want {
			t.Errorf("splitPath(%q) = %v, want %d segments", tc.in, got, tc.want)
		}
	}
}
