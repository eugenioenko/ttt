package e2e

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func openAlphaWithTerminal(t *testing.T, h *testHarness) {
	t.Helper()
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.app.Settings.Terminal.Shell = "C:/Program Files/Git/usr/bin/cat.exe"
	h.exec("terminal.toggle")
	if len(h.app.Terminals) != 1 {
		t.Fatalf("expected 1 terminal after toggle, got %d", len(h.app.Terminals))
	}
	t.Cleanup(func() { h.app.Terminals[0].Term.Close() })
	if h.app.Root.Focused != h.app.TerminalPanel {
		t.Fatalf("expected the terminal to be focused, got %T", h.app.Root.Focused)
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
