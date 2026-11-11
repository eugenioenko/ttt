package e2e

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestCancelPaletteRestoresOpeningPane(t *testing.T) {
	for _, origin := range []string{"sidebar", "search", "terminal"} {
		for _, fileMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/files=%v", origin, fileMode), func(t *testing.T) {
				h := newTestHarness(t, 100, 24)
				defer h.stop()
				h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
				switch origin {
				case "sidebar":
					h.exec("sidebar.explorer")
				case "search":
					h.exec("sidebar.search")
				case "terminal":
					h.app.Settings.Terminal.Shell = "/bin/cat"
					h.exec("terminal.fullscreen")
					t.Cleanup(h.app.Terminals[0].Term.Close)
				}
				previous := h.app.Root.Focused
				height := h.app.ContentSplit.BottomH
				h.app.OpenCommandPalette(fileMode)
				h.pressKey(tcell.KeyEscape, tcell.ModNone)
				if h.app.Root.Focused != previous {
					t.Fatalf("cancel moved focus from %T to %T", previous, h.app.Root.Focused)
				}
				if h.app.ContentSplit.BottomH != height {
					t.Error("cancel resized the terminal panel")
				}
			})
		}
	}
}

func TestCancelReplaceConfirmationReturnsToReplacement(t *testing.T) {
	for _, escape := range []bool{false, true} {
		t.Run(fmt.Sprint(escape), func(t *testing.T) {
			h := newTestHarness(t, 100, 24)
			defer h.stop()
			h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
			h.exec("sidebar.searchReplace")
			h.pressKey(tcell.KeyTab, tcell.ModNone)
			if h.app.Search.FocusedInput() != h.app.Search.ReplaceInput {
				t.Fatal("replacement input not focused")
			}
			h.app.ApplySearchReplaceAll(map[string][]ui.SearchMatch{filepath.Join(h.dir, "alpha.txt"): {{LineNum: 1}}}, "X", ui.SearchOptions{})
			if escape {
				h.pressKey(tcell.KeyEscape, tcell.ModNone)
			} else {
				h.pressKey(tcell.KeyEnter, tcell.ModNone)
			}
			h.pressRune('Q')
			if got := h.app.Search.ReplaceInput.Text(); got != "Q" {
				t.Errorf("typing after cancellation missed replacement input: %q", got)
			}
			if got := h.app.EditorGroup.ActiveBuffer().Lines[0]; got != "a" {
				t.Errorf("cancellation sent typing to editor: %q", got)
			}
		})
	}
}

func TestNestedDialogRestoresParentThenPane(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()
	h.exec("sidebar.search")
	previous := h.app.Root.Focused
	h.app.ShowInputDialog("parent", "", "", func(string) {})
	parent := h.app.Root.Focused
	h.app.ShowInfoDialog("child", nil)
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	if h.app.Root.Focused != parent {
		t.Fatal("closing child did not restore parent dialog focus")
	}
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	if h.app.Root.Focused != previous {
		t.Fatal("closing parent did not restore search focus")
	}
}

func TestDialogSkipsRemovedParentFocus(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()
	h.exec("sidebar.search")
	previous := h.app.Root.Focused
	h.app.ShowInfoDialog("parent", nil)
	parent := h.app.Root.Focused
	h.app.ShowInfoDialog("child", nil)
	h.app.Root.RemoveOverlay(parent)
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	if h.app.Root.Focused != previous {
		t.Fatal("closing child restored a removed parent")
	}
}

func TestPaletteNavigationStillFocusesEditor(t *testing.T) {
	for _, mode := range []string{"file", "line"} {
		t.Run(mode, func(t *testing.T) {
			h := newTestHarness(t, 100, 24)
			defer h.stop()
			h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
			h.app.Settings.Editor.FocusOnOpen = false
			h.exec("sidebar.search")
			h.app.OpenCommandPalette(true)
			dialog := h.app.Root.TopOverlayWidget().(*ui.SelectDialogWidget)
			if mode == "file" {
				dialog.OnOpenFile(filepath.Join(h.dir, "beta.txt"))
			} else {
				dialog.OnGoToLine(1)
			}
			if h.app.Root.Focused != h.app.EditorGroup {
				t.Fatal("navigation did not focus the editor")
			}
		})
	}
}

func TestCancelDialogDoesNotRefocusClosedTerminalPanel(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()
	panelID := openPanelTerminals(t, h, 1)[0]
	h.app.OpenCommandPalette(false)
	h.app.CloseTerminal(panelID)
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	if h.app.Root.Focused != h.app.EditorGroup || h.app.ContentSplit.ShowBottom {
		t.Fatal("cancel restored focus to the closed terminal panel")
	}
}
