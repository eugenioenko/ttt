package e2e

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
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

func rectText(h *testHarness, r ui.Rect) string {
	cells, w, _ := h.screen.GetContents()
	var lines []string
	for y := r.Y; y < r.Y+r.H; y++ {
		lines = append(lines, rowString(cells[y*w+r.X:y*w+r.X+r.W]))
	}
	return strings.Join(lines, "\n")
}

func assertSearchInputInside(t *testing.T, bar ui.Widget, bounds ui.Rect) {
	t.Helper()
	holder, ok := bar.(ui.InputHolder)
	if !ok || holder.FocusedInput() == nil {
		t.Fatalf("search bar %T has no focused input", bar)
	}
	inRect := holder.FocusedInput().GetRect()
	if inRect.X < bounds.X || inRect.X+inRect.W > bounds.X+bounds.W ||
		inRect.Y < bounds.Y || inRect.Y >= bounds.Y+bounds.H {
		t.Fatalf("input rect %+v is outside active group content rect %+v", inRect, bounds)
	}
}

func TestFindAndReplaceAnchorToActiveSplitGroup(t *testing.T) {
	cases := []struct {
		name      string
		splitCmd  string
		pickFirst bool
	}{
		{name: "side-by-side left active", splitCmd: "editor.split", pickFirst: true},
		{name: "side-by-side right active", splitCmd: "editor.split", pickFirst: false},
		{name: "top-bottom bottom active", splitCmd: "editor.splitDown", pickFirst: false},
	}

	for _, tc := range cases {
		for _, command := range []string{"search.find", "search.replace"} {
			t.Run(tc.name+"/"+command, func(t *testing.T) {
				h := newTestHarness(t, 120, 30)
				defer h.stop()

				first := h.app.EditorGroup
				h.typeText("alpha first")
				h.exec(tc.splitCmd)
				second := h.app.EditorGroup
				h.typeText("beta second")

				active, inactive, query := second, first, "beta"
				if tc.pickFirst {
					r := first.Editor.GetRect()
					h.click(r.X+4, r.Y+1)
					active, inactive, query = first, second, "alpha"
				}
				if h.app.EditorGroup != active {
					t.Fatal("expected target split group to be active")
				}

				h.exec(command)
				pasteText(h, pasteKeys(query))
				activeRect := active.ContentRect()
				inactiveRect := inactive.ContentRect()
				assertSearchInputInside(t, h.app.Root.TopOverlayWidget(), activeRect)
				if !strings.Contains(rectText(h, activeRect), "1/1") {
					t.Fatalf("expected active group to contain find bar match counter 1/1")
				}
				if strings.Contains(rectText(h, inactiveRect), "1/1") {
					t.Fatalf("inactive group unexpectedly contains find bar match counter 1/1")
				}

				if command == "search.find" {
					h.exec("search.replace")
				} else {
					h.exec("search.find")
				}
				assertSearchInputInside(t, h.app.Root.TopOverlayWidget(), activeRect)
			})
		}
	}
}
