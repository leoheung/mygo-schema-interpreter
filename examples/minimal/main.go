// Minimal demonstrates the schema interpreter end to end: on launch it reads
// a local schema.json, parses it with the interpreter, and renders it as
// native mygo widgets. A clock provider and a one-second invalidate show the
// per-frame re-interpretation; a team list shows bind iteration with the
// tierBadge composite component; the form shows the state store and an
// action.
//
//	go run ./examples/minimal
//	go run ./examples/minimal -schema /path/to/schema.json
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	schemaui "github.com/leoheung/mygo-schema-interpreter"
)

// teamMember is what the team provider returns; the schema binds into these
// fields by name ("$item.champ").
type teamMember struct {
	Name  string
	Champ string
	Tier  string
}

var team = []teamMember{
	{Name: "Leo", Champ: "Yuumi", Tier: "diamond"},
	{Name: "Zoe", Champ: "Ekko", Tier: "master"},
	{Name: "Mika", Champ: "Riven", Tier: "gold"},
	{Name: "Ana", Champ: "RekSai", Tier: "emerald"},
	{Name: "Sam", Champ: "Ezreal", Tier: "platinum"},
}

// tierColors maps a tier to a badge color, the component's own business.
var tierColors = map[string]string{
	"iron": "#8d8d99", "bronze": "#a2764a", "silver": "#9aa4b2",
	"gold": "#e0b040", "platinum": "#3fb9a0", "emerald": "#2fb457",
	"diamond": "#4f8ff7", "master": "#b45cf0", "grandmaster": "#e0483d",
	"challenger": "#f2c200", "unranked": "#6c6f7e",
}

// demo is the app: the renderer plus the schema-load error, if any.
type demo struct {
	renderer *schemaui.Renderer
	loadErr  error
}

func newDemo(schemaPath string) *demo {
	reg := schemaui.NewRegistry()

	reg.RegisterProvider("clock", func() any {
		return map[string]any{"now": time.Now().Format("15:04:05")}
	})
	reg.RegisterProvider("team", func() any { return team })

	reg.RegisterComponent("tierBadge", func(c *ui.Context, r *schemaui.Renderer, node *schemaui.Node) {
		tier := node.Props.Label
		color, ok := tierColors[tier]
		if !ok {
			color = tierColors["unranked"]
		}
		ui.Box(c).Padding(2, 8).Radius(4).Background(ui.Hex(color)).Children(func() {
			ui.Text(c, tier).FontSize(10).TextColor(ui.Hex("#ffffff"))
		})
	})

	reg.RegisterAction("echo", func(r *schemaui.Renderer, args map[string]any) {
		fmt.Printf("action echo: args=%v input=%q checked=%v region=%v\n",
			args, r.State().String("demo.input"), r.State().Bool("demo.tos"), r.State().String("demo.region"))
	})

	d := &demo{renderer: schemaui.NewRenderer(reg)}
	path, err := findSchema(schemaPath)
	if err != nil {
		d.loadErr = err
		return d
	}
	data, err := os.ReadFile(path)
	if err != nil {
		d.loadErr = fmt.Errorf("read %s: %w", path, err)
		return d
	}
	root, err := schemaui.ParseSchema(data)
	if err != nil {
		d.loadErr = fmt.Errorf("parse %s: %w", path, err)
		return d
	}
	d.renderer.SetSchema(root)
	return d
}

// findSchema resolves the schema path: the flag, else schema.json in the
// working directory, else the one next to this example.
func findSchema(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	for _, p := range []string{"schema.json", "examples/minimal/schema.json"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("schema.json not found (use -schema)")
}

// view draws the schema every frame. The AfterFunc keeps one invalidate per
// second pending so the clock's provider value re-renders.
func (d *demo) view(c *ui.Context) {
	if d.loadErr != nil {
		t := c.Theme()
		ui.Column(c).Fill().Center().Gap(8).Children(func() {
			ui.Text(c, "schema load failed").FontSize(16).Bold().TextColor(t.Danger)
			ui.Text(c, d.loadErr.Error()).FontSize(12).TextColor(t.TextMuted)
		})
		return
	}
	time.AfterFunc(time.Second, c.Invalidate)
	d.renderer.Draw(c)
}

func main() {
	schemaPath := flag.String("schema", "", "path to schema.json (default: ./schema.json or examples/minimal/schema.json)")
	flag.Parse()
	app := newDemo(*schemaPath)
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:   "schema-interpreter demo",
			Width:   520,
			Height:  720,
			Content: ui.View(app.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
