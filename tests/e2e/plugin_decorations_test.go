package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/plugin"
	lua "github.com/yuin/gopher-lua"
)

func TestPluginDecorationsTintHighlightAndHide(t *testing.T) {
	h := newTestHarness(t, 60, 12)
	defer h.stop()
	theme := config.DefaultTheme()
	theme.ResolveColors()
	app.ApplyThemeStyles(h.app.Screen, theme)
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	file := filepath.Join(h.dir, "code.txt")
	if err := os.WriteFile(file, []byte("cursor line\nsecond line\nthird line\n"), 0644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(h.dir, "deco.lua")
	if err := os.WriteFile(src, []byte(`
		local deco = require("ttt.decorations")
		function decorate(path)
			deco.set(path, {
				{ line = 2 },
				{ line = 2, col = 8, end_col = 12 },
				{ line = 3, col = 1, end_col = 6, style = "hide" },
				{ line = 3, note = "Read me first." },
			})
		end
	`), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, src)
	p := h.app.PluginManager.FindPlugin("deco")
	if p == nil {
		t.Fatal("plugin not loaded")
	}
	defer p.Destroy()
	if err := p.State.CallByParam(lua.P{Fn: p.State.GetGlobal("decorate"), Protect: true}, lua.LString(file)); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(file)
	h.redraw()

	h.assertContains(" Read me first.")
	cells, w, _ := h.screen.GetContents()
	find := func(text string) (int, int) {
		for y := 0; y*w < len(cells); y++ {
			if x := displayColumnOf(h.screenRow(y), text); x >= 0 {
				return x, y
			}
		}
		t.Fatalf("%q not on screen", text)
		return 0, 0
	}
	bg := func(x, y int) any { return cells[y*w+x].Style.GetBackground() }
	x1, y1 := find("third line")
	x2, y2 := find("second line")
	plain, tinted, span := bg(x1, y1), bg(x2, y2), bg(x2+7, y2)
	if tinted == plain {
		t.Error("tinted line has the plain background")
	}
	if span == tinted || span == plain {
		t.Error("highlighted text has no background of its own")
	}
	if got := bg(x2+6, y2); got != tinted {
		t.Error("highlight starts before its column")
	}
	for x := x1; x < x1+5; x++ {
		style := cells[y1*w+x].Style
		if style.GetForeground() != style.GetBackground() {
			t.Error("hidden text is visible")
		}
	}
	neighbor := cells[y1*w+x1+6].Style
	if neighbor.GetForeground() == neighbor.GetBackground() {
		t.Error("text outside the hidden range is concealed")
	}

	h.app.ClearPluginSources("deco")
	h.redraw()
	h.assertNotContains("Read me first.")
	cells, _, _ = h.screen.GetContents()
	if bg(x2, y2) != plain || bg(x2+7, y2) != plain {
		t.Error("decorations remain after the plugin's sources were cleared")
	}
	if style := cells[y1*w+x1].Style; style.GetForeground() == style.GetBackground() {
		t.Error("text remains concealed after the plugin's sources were cleared")
	}
}
