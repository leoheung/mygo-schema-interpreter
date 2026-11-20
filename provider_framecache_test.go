package schemaui

import "testing"

// v0.2.10：provider 帧缓存——同帧同 source 只调一次 provider；fire 写
// state 后缓存作废，动作的同帧效果不被缓存滞后。纯逻辑测试，不画 UI。

func TestProviderFrameCache(t *testing.T) {
	calls := 0
	reg := NewRegistry()
	reg.RegisterProvider("home", func() any {
		calls++
		return map[string]any{"name": "玩家"}
	})
	r := NewRenderer(reg)

	b := &Bind{Source: "home", Path: "name"}
	v1, ok1 := r.lookup(b, nil)
	v2, ok2 := r.lookup(b, nil)
	if !ok1 || !ok2 {
		t.Fatalf("both lookups must resolve, got %v/%v", ok1, ok2)
	}
	if v1 != "玩家" || v2 != "玩家" {
		t.Fatalf("values = %q/%q, want 玩家/玩家", v1, v2)
	}
	if calls != 1 {
		t.Fatalf("same frame must call provider once, got %d", calls)
	}

	r.beginFrame()
	r.lookup(b, nil)
	if calls != 2 {
		t.Fatalf("new frame must re-invoke provider, got %d calls", calls)
	}
}

func TestFireInvalidatesFrameCache(t *testing.T) {
	reg := NewRegistry()
	var r *Renderer
	// provider 返回 renderer 的 state 值；闭包捕获变量，lookup 执行时
	// r 已赋值。
	reg.RegisterProvider("home", func() any { return r.State().String("k") })
	r = NewRenderer(reg)

	b := &Bind{Source: "home"}
	*r.State().StringPtr("k", "") = "a"
	if v, _ := r.lookup(b, nil); v != "a" {
		t.Fatalf("first lookup = %v, want a", v)
	}

	// fire 写 state → 缓存作废 → 同帧后续 lookup 读到新值。
	r.fire(nil, &Action{Set: map[string]any{"k": "b"}})
	if v, _ := r.lookup(b, nil); v != "b" {
		t.Fatalf("post-fire lookup = %v, want b (cache must be invalidated)", v)
	}

	// 对照组：不走 fire 的直写不清缓存——lookup 仍返回缓存值，证明
	// 缓存在挡、fire 是唯一失效口。
	*r.State().StringPtr("k", "") = "c"
	if v, _ := r.lookup(b, nil); v != "b" {
		t.Fatalf("direct state write must not bypass the frame cache, got %v", v)
	}
}

func TestItemLookupNotCached(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterProvider("home", func() any { return "named" })
	r := NewRenderer(reg)

	it := &iteration{value: map[string]any{"name": "行内"}}
	if v, ok := r.lookup(&Bind{Source: SourceItem, Path: "name"}, it); !ok || v != "行内" {
		t.Fatalf("$item lookup = %v/%v, want 行内/true", v, ok)
	}
	if len(r.frameProviders) != 0 {
		t.Fatalf("$item must not populate the named-source cache, got %v", r.frameProviders)
	}
}

func TestUnregisteredSourceUncached(t *testing.T) {
	r := NewRenderer(nil)
	for i := 0; i < 2; i++ {
		if v, ok := r.lookup(&Bind{Source: "nowhere"}, nil); ok || v != nil {
			t.Fatalf("unregistered source must degrade to nil/false, got %v/%v", v, ok)
		}
	}
	if len(r.frameProviders) != 0 {
		t.Fatalf("unregistered source must not be cached, got %v", r.frameProviders)
	}
}
