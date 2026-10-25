package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

func openTextTab(t *testing.T, h *testHarness) {
	t.Helper()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	src := filepath.Join(h.dir, "details.lua")
	if err := os.WriteFile(src, []byte(`
		local ttt = require("ttt")
		function show()
			ttt.open_tab({ title = "Details", render = function(panel)
				panel:label("alpha line")
				panel:label("beta line")
			end })
		end
	`), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, src)
	p := h.app.PluginManager.FindPlugin("details")
	if p == nil {
		t.Fatal("plugin not loaded")
	}
	t.Cleanup(p.Destroy)
	if err := p.State.DoString(`show()`); err != nil {
		t.Fatal(err)
	}
	h.redraw()
}

func textAt(t *testing.T, h *testHarness, text string) (int, int) {
	t.Helper()
	for y, line := range strings.Split(h.screenText(), "\n") {
		if i := strings.Index(line, text); i >= 0 {
			return textwidth.String(line[:i]), y
		}
	}
	t.Fatalf("%q not on screen", text)
	return 0, 0
}

func TestPluginTabDragSelectionCopies(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	clipboard.DisableSystem()
	openTextTab(t, h)

	x0, y0 := textAt(t, h, "alpha line")
	x1, y1 := textAt(t, h, "beta line")
	// Drag from the end back to the start: the label's first column is also
	// the sidebar's resize edge, so a press there would resize the sidebar.
	end := x1 + len("beta") - 1
	h.app.Root.HandleEvent(tcell.NewEventMouse(end, y1, tcell.Button1, tcell.ModNone))
	h.app.Root.HandleEvent(tcell.NewEventMouse(x0, y0, tcell.Button1, tcell.ModNone))
	h.app.Root.HandleEvent(tcell.NewEventMouse(x0, y0, tcell.ButtonNone, tcell.ModNone))
	h.redraw()

	h.exec("editor.copy")
	if got := clipboard.Get(); got != "alpha line\nbeta" {
		t.Fatalf("clipboard = %q", got)
	}
}

func TestPluginTabSelectAllAndContextMenu(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	clipboard.DisableSystem()
	openTextTab(t, h)

	x, y := textAt(t, h, "beta line")
	h.rightClick(x, y)
	h.assertContains("Select All")
	h.assertNotContains("Go to Definition")
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	h.exec("editor.selectAll")
	h.exec("editor.copy")
	if got := clipboard.Get(); !strings.Contains(got, "alpha line\nbeta line") {
		t.Fatalf("clipboard = %q, want every drawn line", got)
	}
}
