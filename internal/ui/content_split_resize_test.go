package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestContentSplitBottomDragTracksPointerWithAndWithoutDivider(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		split := NewContentSplitWidget()
		split.Top, split.Bottom = &passThroughWidget{}, &passThroughWidget{}
		split.ShowBottom, split.HideBottomDivider = true, hidden
		split.BottomH, split.MinTopH, split.MinBottomH = 10, 5, 4
		split.SetRect(Rect{X: 2, Y: 3, W: 30, H: 24})
		split.OnResize = func(height int) { split.BottomH = height }
		y := split.DividerScreenY()
		if result := split.HandleEvent(tcell.NewEventMouse(4, y, tcell.Button1, 0)); result != EventCaptured {
			t.Fatalf("hidden %v: divider press was not captured", hidden)
		}
		for _, target := range []int{y, y + 3} {
			split.HandleEvent(tcell.NewEventMouse(4, target, tcell.Button1, 0))
			if got := split.DividerScreenY(); got != target {
				t.Fatalf("hidden %v: divider row %d, want pointer row %d", hidden, got, target)
			}
		}
		split.HandleEvent(tcell.NewEventMouse(4, y+3, tcell.ButtonNone, 0))
		if split.OwnsPointerCapture() {
			t.Fatal("divider retained capture after release")
		}
	}
}

func TestContentSplitResizePanelStepsEffectiveSize(t *testing.T) {
	split := NewContentSplitWidget()
	split.ShowBottom = true
	split.Position = SplitRight
	split.SetRect(Rect{W: 50, H: 20})

	split.ResizePanel(-1)
	if split.RightW != 28 {
		t.Fatalf("shorter from oversized RightW = %d, want 28", split.RightW)
	}

	split.RightW = 60
	split.ResizePanel(1)
	if split.RightW != 29 {
		t.Fatalf("taller from oversized RightW = %d, want 29", split.RightW)
	}

	split.Position = SplitBottom
	split.MinTopH = 5
	split.BottomH = 40
	split.ResizePanel(-1)
	if split.BottomH != 13 {
		t.Fatalf("shorter from oversized BottomH = %d, want 13", split.BottomH)
	}
}
