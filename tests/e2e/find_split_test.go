package e2e

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestFindAndReplaceDismissOnEditorGroupChange(t *testing.T) {
	for _, command := range []string{"search.find", "search.replace"} {
		t.Run(command, func(t *testing.T) {
			h := newTestHarness(t, 140, 32)
			defer h.stop()
			left := h.app.EditorGroup
			h.typeText("left needle")
			h.exec("editor.split")
			right := h.app.EditorGroup
			h.typeText("right needle")

			h.exec(command)
			pasteText(h, pasteKeys("needle"))
			bar := h.app.Root.TopOverlayWidget()
			if bar == nil || len(right.Editor.SearchMatches) != 1 {
				t.Fatal("the search bar did not find the match in the right group")
			}
			r := right.Editor.GetRect()
			h.click(r.X+6, r.Y+6)
			if h.app.Root.TopOverlayWidget() != bar {
				t.Fatal("clicking in the same group closed the search bar")
			}

			r = left.Editor.GetRect()
			h.click(r.X+6, r.Y+6)
			if h.app.EditorGroup != left || h.app.Root.Focused != left {
				t.Fatal("clicking in the left group did not focus it")
			}
			if h.app.Root.HasOverlay() {
				t.Error("the search bar stayed open after switching editor groups")
			}
			if len(right.Editor.SearchMatches) != 0 || len(left.Editor.SearchMatches) != 0 {
				t.Error("the dismissed search still highlights one of the groups")
			}
			h.pressKey(tcell.KeyHome, tcell.ModNone)
			h.pressRune('x')
			if got := left.ActiveBuffer().Lines[0]; got != "xleft needle" {
				t.Errorf("left buffer = %q, want xleft needle", got)
			}
			if got := right.ActiveBuffer().Lines[0]; got != "right needle" {
				t.Errorf("right buffer = %q, want right needle", got)
			}
		})
	}
}
