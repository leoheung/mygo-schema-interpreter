package schemaui

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// iteration is one level of an iterating container: the current item, its
// 0-based position (for "$index" action substitution), and the enclosing
// iteration, so nested "$item" binds resolve inside-out.
type iteration struct {
	parent *iteration
	value  any
	index  int
}

// lookup resolves a bind to a value: the "$item" source walks the current
// iteration, any other source walks the registered provider's value. Path
// segments step into maps by key, slices and arrays by numeric index, and
// structs by field name. It returns ok=false when the source is missing or
// the walk falls off; callers degrade instead of failing.
func (r *Renderer) lookup(b *Bind, it *iteration) (any, bool) {
	if b == nil {
		return nil, false
	}
	var v any
	if b.Source == SourceItem {
		if it == nil {
			return nil, false
		}
		v = it.value
	} else {
		// 帧内同 source 的解析值只取一次 provider()：Draw 是同步的单帧
		// 快照，state 不在帧中途变（fire 写 state 时会清本缓存）。
		if cv, ok := r.frameProviders[b.Source]; ok {
			v = cv
		} else {
			p := r.reg.provider(b.Source)
			if p == nil {
				return nil, false // 未注册不缓存，每次照查 registry
			}
			v = p()
			r.frameProviders[b.Source] = v // nil 返回值也缓存：行为一致
		}
	}
	for _, seg := range splitPath(b.Path) {
		v = step(v, seg)
		if v == nil {
			return nil, false
		}
	}
	return v, true
}

// lookupSlice resolves a bind to a slice of values for iteration. Anything
// slice- or array-like counts; a single value counts as a one-element slice
// so a schema iterating a scalar still draws its template once.
func (r *Renderer) lookupSlice(b *Bind, it *iteration) []any {
	v, ok := r.lookup(b, it)
	if !ok || v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]any, rv.Len())
		for i := range out {
			out[i] = rv.Index(i).Interface()
		}
		return out
	default:
		return []any{v}
	}
}

// splitPath splits a bind path on ".", dropping empty segments.
func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	parts := strings.Split(path, ".")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// step walks one path segment into v: a map by key, a slice or array by
// numeric index, a struct by field name, and through a non-nil pointer. It
// returns nil when the segment does not apply.
func step(v any, seg string) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Map:
		k := reflect.ValueOf(seg).Convert(rv.Type().Key())
		x := rv.MapIndex(k)
		if !x.IsValid() {
			return nil
		}
		return x.Interface()
	case reflect.Slice, reflect.Array:
		i, err := strconv.Atoi(seg)
		if err != nil || i < 0 || i >= rv.Len() {
			return nil
		}
		return rv.Index(i).Interface()
	case reflect.Pointer:
		if rv.IsNil() {
			return nil
		}
		return step(rv.Elem().Interface(), seg)
	case reflect.Struct:
		f := rv.FieldByName(seg)
		if !f.IsValid() || !f.CanInterface() {
			return nil
		}
		return f.Interface()
	}
	return nil
}

// toText renders a bound value as display text.
func toText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	default:
		return fmt.Sprint(v)
	}
}

// toImageSource turns a bound value into something ui.Image can show: a
// *ui.Bitmap or a []byte of encoded image data pass through, a data-URI
// string decodes, and anything else is absent.
func toImageSource(v any) ui.ImageSource {
	switch t := v.(type) {
	case *ui.Bitmap:
		if t == nil {
			return nil
		}
		return t
	case []byte:
		if bmp, err := ui.DecodeBitmap(t); err == nil {
			return bmp
		}
		return nil
	case string:
		if b, ok := decodeDataURI(t); ok {
			if bmp, err := ui.DecodeBitmap(b); err == nil {
				return bmp
			}
		}
		return nil
	default:
		return nil
	}
}

// decodeDataURI decodes a "data:<mime>;base64,<payload>" string. It accepts
// the base64 in both standard and URL-safe alphabets, padded or not.
func decodeDataURI(s string) ([]byte, bool) {
	const prefix = "base64,"
	i := strings.Index(s, prefix)
	if i < 0 || !strings.HasPrefix(s, "data:") {
		return nil, false
	}
	b, err := base64.StdEncoding.DecodeString(s[i+len(prefix):])
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(s[i+len(prefix):])
	}
	if err != nil {
		b, err = base64.URLEncoding.DecodeString(s[i+len(prefix):])
	}
	if err != nil {
		b, err = base64.RawURLEncoding.DecodeString(s[i+len(prefix):])
	}
	if err != nil {
		return nil, false
	}
	return b, true
}
