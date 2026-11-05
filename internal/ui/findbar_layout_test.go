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
