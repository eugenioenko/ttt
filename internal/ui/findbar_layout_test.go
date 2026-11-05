package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestFindBarButtonsUseRenderedOrigin(t *testing.T) {
	for _, expanded := range []bool{false, true} {
		rect := Rect{X: 7, Y: 3, W: 60, H: 20}
		surface := NewRenderSurface(makeGrid(80, 30), rect)
		var bar Widget
		var layout *findBarLayout
		toggled, dismissed := false, false
		if expanded {
			replace := NewReplaceBarWidget()
			replace.OnToggleFind = func() { toggled = true }
			replace.OnDismiss = func() { dismissed = true }
			bar, layout = replace, &replace.findBarLayout
		} else {
			find := NewFindBarWidget()
			find.OnToggleReplace = func() { toggled = true }
			find.OnDismiss = func() { dismissed = true }
			bar, layout = find, &find.findBarLayout
		}
		bar.SetRect(rect)
		bar.Render(surface)
		for _, hit := range []HitRegion{layout.btnToggle, layout.btnClose} {
			if result := bar.HandleEvent(tcell.NewEventMouse(hit.X, hit.Y, tcell.Button1, tcell.ModNone)); result != EventConsumed {
				t.Fatal("rendered button ignored click")
			}
		}
		if !toggled || !dismissed {
			t.Fatalf("expanded=%v: toggled=%v dismissed=%v", expanded, toggled, dismissed)
		}
	}
}

func TestFindBarNarrowInputRemainsReachable(t *testing.T) {
	for _, width := range []int{8, 12, 16, 20, 32} {
		for _, expanded := range []bool{false, true} {
			var bar Widget
			var input *InputWidget
			var layout *findBarLayout
			if expanded {
				replace := NewReplaceBarWidget()
				replace.SearchInput.Text = "find"
				replace.Matches = []FindMatch{{Len: 4}}
				replace.OnToggleFind = func() {}
				bar, input, layout = replace, replace.SearchInput, &replace.findBarLayout
			} else {
				find := NewFindBarWidget()
				find.Input.Text = "find"
				find.Matches = []FindMatch{{Len: 4}}
				find.OnToggleReplace = func() {}
				bar, input, layout = find, find.Input, &find.findBarLayout
			}
			input.CursorPos = 4
			rect := Rect{W: width, H: 16}
			bar.SetRect(rect)
			bar.Render(NewRenderSurface(makeGrid(width, 16), rect))
			if layout.inputW < 1 {
				t.Fatalf("width=%d expanded=%v: no input region", width, expanded)
			}
			for _, hit := range input.ActionHits {
				if hit.X < layout.inputX || hit.X+hit.W > layout.inputX+layout.inputW {
					t.Fatalf("width=%d: action overlaps input bounds: %+v", width, hit)
				}
			}
			bar.HandleEvent(tcell.NewEventMouse(0, 12, tcell.Button1, tcell.ModNone))
			if bar.(InputHolder).FocusedInput() != nil {
				t.Fatal("outside click did not release focus")
			}
			bar.HandleEvent(tcell.NewEventMouse(layout.inputX, 5, tcell.Button1, tcell.ModNone))
			if bar.(InputHolder).FocusedInput() != input {
				t.Fatalf("width=%d expanded=%v: input click did not restore focus", width, expanded)
			}
			bar.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone))
			if len(input.Text) != len("find")+len("x") {
				t.Fatalf("width=%d expanded=%v: typing after click = %q", width, expanded, input.Text)
			}
		}
	}
}
