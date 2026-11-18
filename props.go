package schemaui

import (
	"errors"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// ErrEmptyType is returned by ParseSchema for a document whose root node has
// no type.
var ErrEmptyType = errors.New("schemaui: root node has empty type")

// BorderSpec is a node's border: width in DIPs and a color string.
type BorderSpec struct {
	Width float32 `json:"width"`
	Color string  `json:"color,omitempty"`
}

// Props are a node's layout, style and content properties. Zero values mean
// "not set" and are skipped when applying, so a node only overrides what its
// schema says. Colors parse as "#rgb", "#rgba", "#rrggbb" or "#rrggbbaa"; an
// invalid color is ignored (the mygo Hex helper panics, this does not).
type Props struct {
	// Size. Width and Height are DIPs; Fill stretches the node over its
	// parent's main axis.
	Width  *float32 `json:"width,omitempty"`
	Height *float32 `json:"height,omitempty"`
	// WidthPercent sizes against the parent's width in percent (0..100).
	WidthPercent *float32 `json:"widthPercent,omitempty"`
	MinWidth     *float32 `json:"minWidth,omitempty"`
	MaxWidth     *float32 `json:"maxWidth,omitempty"`
	MinHeight    *float32 `json:"minHeight,omitempty"`
	MaxHeight    *float32 `json:"maxHeight,omitempty"`
	Fill         bool     `json:"fill,omitempty"`

	// Spacing. Padding, Margin and Radius take 1..4 values like CSS
	// (all, vertical/horizontal, or top/right/bottom/left).
	Padding []float32 `json:"padding,omitempty"`
	Margin  []float32 `json:"margin,omitempty"`
	Radius  []float32 `json:"radius,omitempty"`
	Gap     *float32  `json:"gap,omitempty"`

	// Flex. Grow, Shrink and Basis map straight onto the mygo properties.
	Grow        *float32 `json:"grow,omitempty"`
	Shrink      *float32 `json:"shrink,omitempty"`
	Basis       *float32 `json:"basis,omitempty"`
	Stretch     bool     `json:"stretch,omitempty"`     // AlignSelf(Stretch)
	AspectRatio *float32 `json:"aspectRatio,omitempty"` // height = width / r

	// Alignment: "start", "center", "end", "stretch", "spaceBetween",
	// "spaceAround", "spaceEvenly" or "baseline". Invalid values are ignored.
	Justify    string `json:"justify,omitempty"`
	AlignItems string `json:"alignItems,omitempty"`
	AlignSelf  string `json:"alignSelf,omitempty"`
	Center     bool   `json:"center,omitempty"` // Center() shorthand

	// Paint.
	Background string `json:"background,omitempty"`
	// BackgroundBind resolves to a color string (a host token name or hex)
	// at draw time — a win/loss row paints itself from its data.
	BackgroundBind *Bind `json:"backgroundBind,omitempty"`
	// BorderEdgeBind resolves to a color string for a 4-wide bottom edge
	// (the row accent), data-driven like BackgroundBind.
	BorderEdgeBind *Bind       `json:"borderEdgeBind,omitempty"`
	Border         *BorderSpec `json:"border,omitempty"`
	// BorderBottom is the row-edge accent: a bottom-only border of the
	// given width and color (win/loss rows carry one).
	BorderBottom *BorderSpec `json:"borderBottom,omitempty"`
	// Cursor is "pointer" for clickable-looking nodes; anything else is
	// ignored.
	Cursor   string   `json:"cursor,omitempty"`
	Opacity  *float32 `json:"opacity,omitempty"`
	Clip     bool     `json:"clip,omitempty"`
	Hidden   bool     `json:"hidden,omitempty"`
	Absolute bool     `json:"absolute,omitempty"`
	Left     *float32 `json:"left,omitempty"`
	Top      *float32 `json:"top,omitempty"`
	Bottom   *float32 `json:"bottom,omitempty"`
	Right    *float32 `json:"right,omitempty"`

	// Text style.
	FontSize  *float32 `json:"fontSize,omitempty"`
	Bold      bool     `json:"bold,omitempty"`
	TextColor string   `json:"textColor,omitempty"` // hex, or a host-registered token name

	// Type-specific content.
	Text        string   `json:"text,omitempty"`        // text, button
	Src         string   `json:"src,omitempty"`         // image: data URI
	Placeholder string   `json:"placeholder,omitempty"` // textInput
	Options     []string `json:"options,omitempty"`     // select
	Label       string   `json:"label,omitempty"`       // checkbox
	Value       string   `json:"value,omitempty"`       // textInput, select initial
	StateKey    string   `json:"stateKey,omitempty"`    // modal: the bool state that opens it (default: the node's key)
}

// alignValues maps schema alignment names onto mygo's Align constants.
var alignValues = map[string]ui.Align{
	"start":        ui.Start,
	"center":       ui.Center,
	"end":          ui.End,
	"stretch":      ui.Stretch,
	"spaceBetween": ui.SpaceBetween,
	"spaceAround":  ui.SpaceAround,
	"spaceEvenly":  ui.SpaceEvenly,
	"baseline":     ui.Baseline,
}

// resolveColor resolves a color spec: a host-registered token name first
// (theme-aware), else "#hex" parsing. Invalid input is skipped, not fatal.
func (g *Registry) resolveColor(c *ui.Context, spec string) (ui.Color, bool) {
	if spec == "" {
		return ui.Color{}, false
	}
	if g != nil {
		if f := g.color(spec); f != nil {
			return f(c), true
		}
	}
	return parseColor(spec)
}

// parseColor converts a schema color string. It returns ok=false on invalid
// input instead of panicking the way ui.Hex does.
func parseColor(s string) (ui.Color, bool) {
	h := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(h) != 3 && len(h) != 4 && len(h) != 6 && len(h) != 8 {
		return ui.Color{}, false
	}
	for i := 0; i < len(h); i++ {
		if _, err := strconv.ParseUint(h[i:i+1], 16, 8); err != nil {
			return ui.Color{}, false
		}
	}
	return ui.Hex(h), true
}

// edges expands 1..4 schema values the way mygo's variadic properties do:
// one value is all sides, two are vertical and horizontal, four are top,
// right, bottom, left. More than four keeps the first four.
func edges(v []float32) []float32 {
	switch len(v) {
	case 0:
		return nil
	case 1:
		return []float32{v[0], v[0], v[0], v[0]}
	case 2:
		return []float32{v[0], v[1], v[0], v[1]}
	case 3:
		return []float32{v[0], v[1], v[2], v[1]}
	default:
		return v[:4]
	}
}

// applyProps chains a node's set properties onto a freshly built element
// and returns the element for further chaining. Color strings resolve
// through the registry's token table first (theme-aware, host-registered),
// falling back to hex parsing.
func (g *Registry) applyProps(c *ui.Context, el ui.Element, p *Props) ui.Element {
	if p == nil {
		return el
	}
	// Fill first, so an explicit Width/WidthPercent/Height after it wins —
	// native code chains Fill().WidthPercent(40) and expects the 40% to
	// stand; replaying Fill last would snap the width back to 100%.
	if p.Fill {
		el = el.Fill()
	}
	if p.Width != nil {
		el = el.Width(*p.Width)
	}
	if p.WidthPercent != nil {
		el = el.WidthPercent(*p.WidthPercent)
	}
	if p.Height != nil {
		el = el.Height(*p.Height)
	}
	if p.MinWidth != nil {
		el = el.MinWidth(*p.MinWidth)
	}
	if p.MaxWidth != nil {
		el = el.MaxWidth(*p.MaxWidth)
	}
	if p.MinHeight != nil {
		el = el.MinHeight(*p.MinHeight)
	}
	if p.MaxHeight != nil {
		el = el.MaxHeight(*p.MaxHeight)
	}
	if len(p.Padding) > 0 {
		el = el.Padding(edges(p.Padding)...)
	}
	if len(p.Margin) > 0 {
		el = el.Margin(edges(p.Margin)...)
	}
	if len(p.Radius) > 0 {
		el = el.Radius(edges(p.Radius)...)
	}
	if p.Gap != nil {
		el = el.Gap(*p.Gap)
	}
	if p.Grow != nil {
		el = el.Grow(*p.Grow)
	}
	if p.Shrink != nil {
		el = el.Shrink(*p.Shrink)
	}
	if p.Basis != nil {
		el = el.Basis(*p.Basis)
	}
	if p.AspectRatio != nil {
		el = el.AspectRatio(*p.AspectRatio)
	}
	if p.Stretch {
		el = el.AlignSelf(ui.Stretch)
	}
	if a, ok := alignValues[p.Justify]; ok {
		el = el.Justify(a)
	}
	if a, ok := alignValues[p.AlignItems]; ok {
		el = el.AlignItems(a)
	}
	if a, ok := alignValues[p.AlignSelf]; ok {
		el = el.AlignSelf(a)
	}
	if p.Center {
		el = el.Center()
	}
	if c2, ok := g.resolveColor(c, p.Background); ok {
		el = el.Background(c2)
	}
	if p.Border != nil {
		if col, ok := g.resolveColor(c, p.Border.Color); ok {
			el = el.Border(p.Border.Width, col)
		}
	}
	if p.BorderBottom != nil {
		if col, ok := g.resolveColor(c, p.BorderBottom.Color); ok {
			el = el.BorderWidth(0, 0, 0, p.BorderBottom.Width).BorderColor(col)
		}
	}
	if p.Cursor == "pointer" {
		el = el.Cursor(ui.CursorPointer)
	}
	if p.Opacity != nil {
		el = el.Opacity(*p.Opacity)
	}
	if p.Clip {
		el = el.Clip()
	}
	if p.Hidden {
		el = el.Hidden(true)
	}
	if p.Absolute {
		el = el.Absolute()
	}
	if p.Left != nil {
		el = el.Left(*p.Left)
	}
	if p.Top != nil {
		el = el.Top(*p.Top)
	}
	if p.Bottom != nil {
		el = el.Bottom(*p.Bottom)
	}
	if p.Right != nil {
		el = el.Right(*p.Right)
	}
	if p.FontSize != nil {
		el = el.FontSize(*p.FontSize)
	}
	if p.Bold {
		el = el.Bold()
	}
	if col, ok := g.resolveColor(c, p.TextColor); ok {
		el = el.TextColor(col)
	}
	if p.Placeholder != "" {
		el = el.Placeholder(p.Placeholder)
	}
	return el
}
