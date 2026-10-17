package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/gdamore/tcell/v3"
)

func TestPluginInputWrapsAndBreaksLines(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	src := filepath.Join(h.dir, "chat.lua")
	if err := os.WriteFile(src, []byte(`
		sent = nil
		require("ttt").register({ bottom = { title = "Chat", render = function(panel)
			panel:label({ text = "above" })
			panel:input({ wrap = true, max_rows = 3, on_submit = function(t) sent = t end })
			panel:label({ text = "below" })
		end } })
	`), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, src)
	p := h.app.PluginManager.FindPlugin("chat")
	if p == nil {
		t.Fatal("plugin not loaded")
	}
	t.Cleanup(p.Destroy)
	h.app.ContentSplit.ShowBottom = true
	h.app.BottomPanel.SetActivePanel("plugin.chat")
	h.redraw()
	h.app.Root.SetFocus(h.app.BottomPanel.ActiveWidget())
	h.redraw()

	long := "alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima mike november"
	for _, r := range long {
		h.pressRune(r)
	}
	h.pressKey(tcell.KeyEnter, tcell.ModAlt)
	for _, r := range "oscar" {
		h.pressRune(r)
	}
	h.redraw()
	h.assertContains("oscar")
	h.assertContains("below")
	_, yBelow := findText(t, h, "below")
	_, yOscar := findText(t, h, "oscar")
	if yBelow != yOscar+1 {
		t.Fatalf("the input did not grow: oscar on row %d, below on row %d", yOscar, yBelow)
	}

	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if got := p.State.GetGlobal("sent").String(); got != long+"\noscar" {
		t.Fatalf("sent %q", got)
	}
}

func findText(t *testing.T, h *testHarness, text string) (int, int) {
	t.Helper()
	for y := range 30 {
		if x := displayColumnOf(h.screenRow(y), text); x >= 0 {
			return x, y
		}
	}
	t.Fatalf("%q not on screen:\n%s", text, h.screenText())
	return 0, 0
}
