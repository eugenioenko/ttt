package e2e

import (
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/gdamore/tcell/v3"
	lua "github.com/yuin/gopher-lua"
)

func openAlphaWithTerminal(t *testing.T, h *testHarness) {
	t.Helper()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.app.Settings.Terminal.Shell = ""
	h.exec("terminal.toggle")
	if len(h.app.Terminals) != 1 {
		t.Fatalf("expected 1 terminal after toggle, got %d", len(h.app.Terminals))
	}
	terminal := h.app.Terminals[0].Term
	t.Cleanup(func() { terminal.Close() })
	if h.app.Root.Focused != h.app.TerminalPanel {
		t.Fatalf("expected the terminal to be focused, got %T", h.app.Root.Focused)
	}
}

func TestClosingLowerDrawerPreservesDialogFocus(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.app.FocusSidebar()
	origin := h.app.Root.Focused
	p := &plugin.Plugin{Name: "focus-test", Enabled: true}
	if err := p.InitFromSource(`function render(panel) panel:label("Drawer") end`); err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	h.app.WirePlugin(p)
	panel := plugin.NewPluginPanelWidget(p, p.State.GetGlobal("render").(*lua.LFunction), nil)
	p.OpenDrawer(panel, 40, 20, "right")
	accepted := false
	h.app.ShowConfirmDialog("Continue?", []string{"OK"}, []func(){func() {
		accepted = true
		h.app.DismissDialog()
	}})
	dialog := h.app.Root.TopOverlayWidget()
	h.redraw()
	p.CloseDrawer()
	h.redraw()
	if h.app.Root.Focused != dialog {
		t.Errorf("focus = %T, want remaining dialog %T", h.app.Root.Focused, dialog)
	}
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if !accepted {
		t.Fatal("Enter did not activate the remaining dialog button")
	}
	if h.app.Root.Focused != origin {
		t.Errorf("focus = %T, want drawer origin %T", h.app.Root.Focused, origin)
	}
}

// Issue #578: keys typed after dismissing an overlay opened from the terminal
// must reach the terminal, not the editor buffer.
func TestDismissedOverlayReturnsFocusToTerminal(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	openAlphaWithTerminal(t, h)

	h.pressCtrl(tcell.KeyCtrlP)
	if !h.app.Root.HasModalOverlay() {
		t.Fatal("expected ctrl+p to open the palette over the terminal")
	}
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	h.typeText("XY")

	if h.app.Root.Focused != h.app.TerminalPanel {
		t.Errorf("expected focus back on the terminal, got %T", h.app.Root.Focused)
	}
	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "a" {
		t.Errorf("editor buffer changed to %q, want %q", got, "a")
	}
}

func TestDismissedOverlayReturnsFocusToSidebar(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.app.FocusSidebar()
	explorer := h.app.Root.Focused

	h.app.ShowConfirmDialog("Continue?", []string{"OK"}, []func(){func() {}})
	h.redraw()
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	if h.app.Root.Focused != explorer {
		t.Errorf("expected focus back on %T, got %T", explorer, h.app.Root.Focused)
	}
}

// The terminal hidden while the palette was open must not get focus back:
// keystrokes would go to a PTY that is no longer on screen.
func TestDismissedOverlayFallsBackToEditorWhenOriginHidden(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	openAlphaWithTerminal(t, h)

	h.pressCtrl(tcell.KeyCtrlP)
	h.app.HideBottomPanel()
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	if h.app.Root.Focused != h.app.EditorGroup {
		t.Errorf("expected focus on the editor, got %T", h.app.Root.Focused)
	}
}
