package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/eugenioenko/ttt/internal/widgets"
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
			var input *widgets.InputWidget
			var layout *findBarLayout
			if expanded {
				replace := NewReplaceBarWidget()
				seedInputText(replace.SearchInput, "find")
				replace.Matches = []FindMatch{{Len: 4}}
				replace.OnToggleFind = func() {}
				bar, input, layout = replace, replace.SearchInput, &replace.findBarLayout
			} else {
				find := NewFindBarWidget()
				seedInputText(find.Input, "find")
				find.Matches = []FindMatch{{Len: 4}}
				find.OnToggleReplace = func() {}
				bar, input, layout = find, find.Input, &find.findBarLayout
			}
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
			if len(input.Text()) != len("find")+len("x") {
				t.Fatalf("width=%d expanded=%v: typing after click = %q", width, expanded, input.Text())
			}
		}
	}
}

func TestReplaceBarConsumesMatchCounterClick(t *testing.T) {
	bar := NewReplaceBarWidget()
	seedInputText(bar.SearchInput, "find")
	bar.Matches = []FindMatch{{Len: 4}}
	rect := Rect{X: 7, Y: 3, W: 60, H: 20}
	bar.SetRect(rect)
	bar.Render(NewRenderSurface(makeGrid(80, 30), rect))
	bar.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
	inputRect := bar.SearchInput.GetRect()
	x, y := inputRect.X+inputRect.W+1, inputRect.Y
	if result := bar.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone)); result != EventConsumed {
		t.Fatalf("match counter click leaked through the bar: %v", result)
	}
	if bar.FocusedInput() != bar.SearchInput {
		t.Fatal("match counter click did not focus the search row")
	}
}

func TestFindBarAnchorsToRect(t *testing.T) {
	cases := []struct {
		name     string
		surface  Rect
		anchor   Rect
		expanded bool
		want     Rect
	}{
		{
			name:    "left split",
			surface: Rect{W: 120, H: 30},
			anchor:  Rect{X: 32, Y: 4, W: 43, H: 24},
			want:    Rect{X: 35, Y: 4, W: 40, H: 3},
		},
		{
			name:     "bottom split replace",
			surface:  Rect{W: 120, H: 30},
			anchor:   Rect{X: 32, Y: 18, W: 87, H: 10},
			expanded: true,
			want:     Rect{X: 79, Y: 18, W: 40, H: 4},
		},
		{
			name:    "narrow split fits inside group",
			surface: Rect{W: 120, H: 30},
			anchor:  Rect{X: 10, Y: 4, W: 28, H: 20},
			want:    Rect{X: 10, Y: 4, W: 28, H: 3},
		},
		{
			name:    "very narrow split keeps minimum width",
			surface: Rect{W: 80, H: 24},
			anchor:  Rect{X: 70, Y: 4, W: 8, H: 18},
			want:    Rect{X: 54, Y: 4, W: 24, H: 3},
		},
		{
			name:    "very narrow left-edge split clamps to surface",
			surface: Rect{W: 80, H: 24},
			anchor:  Rect{X: 2, Y: 4, W: 8, H: 18},
			want:    Rect{X: 0, Y: 4, W: 24, H: 3},
		},
		{
			name:     "cramped bottom clamps to surface height",
			surface:  Rect{W: 120, H: 30},
			anchor:   Rect{X: 32, Y: 28, W: 50, H: 2},
			expanded: true,
			want:     Rect{X: 42, Y: 26, W: 40, H: 4},
		},
		{
			name:    "non-zero surface origin",
			surface: Rect{X: 7, Y: 3, W: 80, H: 24},
			anchor:  Rect{X: 12, Y: 7, W: 43, H: 18},
			want:    Rect{X: 8, Y: 4, W: 40, H: 3},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var bar Widget
			var layout *findBarLayout
			toggled, dismissed := false, false
			anchor := tc.anchor
			if tc.expanded {
				replace := NewReplaceBarWidget()
				replace.AnchorRect = func() Rect { return anchor }
				replace.OnToggleFind = func() { toggled = true }
				replace.OnDismiss = func() { dismissed = true }
				bar, layout = replace, &replace.findBarLayout
			} else {
				find := NewFindBarWidget()
				find.AnchorRect = func() Rect { return anchor }
				find.OnToggleReplace = func() { toggled = true }
				find.OnDismiss = func() { dismissed = true }
				bar, layout = find, &find.findBarLayout
			}
			bar.SetRect(tc.surface)
			gridW := tc.surface.X + tc.surface.W
			gridH := tc.surface.Y + tc.surface.H
			bar.Render(NewRenderSurface(makeGrid(gridW, gridH), tc.surface))
			if layout.barRect != tc.want {
				t.Fatalf("barRect = %+v, want %+v", layout.barRect, tc.want)
			}
			for _, hit := range []HitRegion{layout.btnToggle, layout.btnClose} {
				if result := bar.HandleEvent(tcell.NewEventMouse(hit.X, hit.Y, tcell.Button1, tcell.ModNone)); result != EventConsumed {
					t.Fatalf("button hit %+v was ignored", hit)
				}
			}
			if !toggled || !dismissed {
				t.Fatalf("toggled=%v dismissed=%v, want both true", toggled, dismissed)
			}
		})
	}
}
