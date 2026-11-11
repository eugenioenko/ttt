package e2e

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
)

func TestEditorFocusRestoresMaximizedTerminalSplit(t *testing.T) {
	for _, mode := range []string{"quick-open", "new-terminal", "existing-terminal", "focus-command"} {
		t.Run(mode, func(t *testing.T) {
			h := newTestHarness(t, 100, 30)
			defer h.stop()
			h.app.Settings.Terminal.Shell = "/bin/cat"
			var editorID string
			if mode == "existing-terminal" {
				h.exec("terminal.newInEditor")
				editorID = h.app.EditorTerminals[0].ID
				t.Cleanup(h.app.EditorTerminals[0].Term.Close)
			}
			h.exec("terminal.fullscreen")
			t.Cleanup(h.app.Terminals[0].Term.Close)
			cs := h.app.ContentSplit
			if cs.BottomRect().H != cs.GetRect().H-1 {
				t.Fatal("panel was not maximized")
			}
			switch mode {
			case "quick-open":
				h.app.OpenCommandPalette(true)
				dialog := h.app.Root.TopOverlayWidget().(*ui.SelectDialogWidget)
				dialog.OnOpenFile(filepath.Join(h.dir, "alpha.txt"))
			case "new-terminal":
				h.exec("terminal.newInEditor")
				t.Cleanup(h.app.EditorTerminals[0].Term.Close)
			case "existing-terminal":
				if err := h.app.FocusTerminal(editorID); err != nil {
					t.Fatal(err)
				}
			default:
				h.exec("editor.focus")
			}
			h.redraw()
			if h.app.Root.Focused != h.app.EditorGroup || cs.BottomRect().H >= cs.GetRect().H-3 {
				t.Fatalf("editor focus remained invisible: panel=%+v split=%+v", cs.BottomRect(), cs.GetRect())
			}
			if !cs.ShowBottom {
				t.Error("normal-height screen should keep panel visible")
			}
			if mode == "quick-open" {
				h.pressRune('Q')
				h.pressRune('Q')
				h.assertContains("QQ")
			}
		})
	}
}

func TestEditorFocusPreservesUsablePanelSize(t *testing.T) {
	for _, position := range []ui.SplitPosition{ui.SplitBottom, ui.SplitRight} {
		h := newTestHarness(t, 100, 30)
		h.app.ContentSplit.Position = position
		h.app.ContentSplit.ShowBottom = true
		h.app.ContentSplit.BottomH = 8
		h.app.ContentSplit.RightW = 35
		h.app.FocusEditor()
		if h.app.ContentSplit.BottomH != 8 || h.app.ContentSplit.RightW != 35 {
			t.Fatal("usable panel size changed")
		}
		h.stop()
	}
}

func TestFocusOnOpenDisabledPreservesMaximizedPanel(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.app.Settings.Terminal.Shell = "/bin/cat"
	h.exec("terminal.fullscreen")
	t.Cleanup(h.app.Terminals[0].Term.Close)
	h.app.Settings.Editor.FocusOnOpen = false
	h.app.FocusEditorIfEnabled()
	if h.app.Root.Focused != h.app.TerminalPanel || h.app.ContentSplit.BottomRect().H != h.app.ContentSplit.GetRect().H-1 {
		t.Fatal("disabled focus-on-open changed terminal focus or maximized size")
	}
}

func TestEditorFocusHidesMaximizedPanelOnTinyScreen(t *testing.T) {
	for height := 7; height <= 12; height++ {
		t.Run(fmt.Sprint(height), func(t *testing.T) {
			h := newTestHarness(t, 80, height)
			defer h.stop()
			h.app.NewFileFromText("VISIBLE")
			h.app.ContentSplit.ShowBottom = true
			h.app.ContentSplit.BottomH = h.app.ContentSplit.GetRect().H - 1
			h.app.FocusEditor()
			h.redraw()
			h.assertContains("VISIBLE")
		})
	}
}

func TestEditorFocusRestoresAlmostMaximizedPanel(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	h.app.NewFileFromText("VISIBLE")
	h.app.ContentSplit.ShowBottom = true
	for remaining := 1; remaining <= 3; remaining++ {
		h.app.ContentSplit.BottomH = h.app.ContentSplit.GetRect().H - 1 - remaining
		h.app.FocusEditor()
		h.redraw()
		h.assertContains("VISIBLE")
	}
}

func TestPaletteFullscreenCommandSeesMaximizedLayout(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.app.Settings.Terminal.Shell = "/bin/cat"
	h.exec("terminal.fullscreen")
	t.Cleanup(h.app.Terminals[0].Term.Close)
	h.app.OpenCommandPalette(false)
	dialog := h.app.Root.TopOverlayWidget().(*ui.SelectDialogWidget)
	dialog.OnExecute("terminal.fullscreen")
	h.redraw()
	if h.app.ContentSplit.ShowBottom || h.app.Root.Focused != h.app.EditorGroup {
		t.Fatal("palette fullscreen toggle re-maximized the panel instead of hiding it")
	}
}
