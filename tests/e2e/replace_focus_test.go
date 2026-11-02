package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestReplaceBarEditorFocus(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	h.exec("file.new")
	h.pressCtrl(tcell.KeyCtrlR)
	bar, ok := h.app.Root.TopOverlayWidget().(*ui.ReplaceBarWidget)
	if !ok {
		t.Fatalf("expected ReplaceBarWidget, got %T", h.app.Root.TopOverlayWidget())
	}
	h.pressRune('a')
	h.pressKey(tcell.KeyTab, tcell.ModNone)
	h.pressRune('b')

	h.click(6, 4)
	if bar.FocusedInput() != nil {
		t.Fatal("replace bar kept input focus after clicking the editor")
	}
	if _, _, visible := bar.CursorPosition(); visible {
		t.Fatal("replace bar kept its cursor after clicking the editor")
	}
	h.pressRune('x')
	pasteText(h, pasteKeys("yz"))
	if got := editorText(h); got != "xyz" {
		t.Fatalf("editor text = %q, want xyz", got)
	}
	if bar.SearchInput.Text != "a" || bar.ReplaceInput.Text != "b" {
		t.Fatal("editor input changed the search or replacement")
	}
	h.pressCtrl(tcell.KeyCtrlZ)
	if got := editorText(h); got != "x" {
		t.Fatalf("editor text after undo = %q, want x", got)
	}
	if h.app.Root.TopOverlayWidget() != bar {
		t.Fatal("editing closed the replace bar")
	}

	h.click(84, 5)
	if bar.FocusedInput() != bar.SearchInput {
		t.Fatal("clicking search did not restore input focus")
	}
	h.pressKey(tcell.KeyEnd, tcell.ModNone)
	h.pressRune('c')
	if bar.SearchInput.Text != "ac" {
		t.Fatalf("search input = %q, want ac", bar.SearchInput.Text)
	}
	h.click(84, 6)
	if bar.FocusedInput() != bar.ReplaceInput {
		t.Fatal("clicking replacement did not restore input focus")
	}
	h.pressKey(tcell.KeyEnd, tcell.ModNone)
	pasteText(h, pasteKeys("d"))
	if bar.ReplaceInput.Text != "bd" {
		t.Fatalf("replacement = %q, want bd", bar.ReplaceInput.Text)
	}

	h.click(6, 4)
	h.pressCtrl(tcell.KeyCtrlR)
	if h.app.Root.TopOverlayWidget() != bar || len(h.app.Root.Overlays) != 1 || bar.FocusedInput() != bar.ReplaceInput {
		t.Fatal("Ctrl+R did not refocus the existing replace bar")
	}
	h.pressRune('e')
	if bar.ReplaceInput.Text != "bde" {
		t.Fatalf("replacement after Ctrl+R = %q, want bde", bar.ReplaceInput.Text)
	}
	h.click(6, 4)
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	if h.app.Root.HasOverlay() {
		t.Fatal("Escape in the editor did not dismiss the replace bar")
	}
}
