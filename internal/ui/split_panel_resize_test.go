package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func newResizeTestSplit() (*SplitPanelWidget, *mockWidget, *[]int) {
	right := &mockWidget{}
	split := NewSplitPanelWidget()
	split.Left = &mockWidget{}
	split.Right = right
	split.DividerPos = 20
	split.ShowLeft = true
	split.SetRect(Rect{W: 60, H: 10})
	var resizes []int
	split.OnResize = func(w int) { resizes = append(resizes, w) }
	return split, right, &resizes
}

func TestSplitPanelStationaryClickOnRightEdgeReachesRight(t *testing.T) {
	split, right, resizes := newResizeTestSplit()
	x := split.DividerScreenX() + 1

	split.HandleEvent(tcell.NewEventMouse(x, 3, tcell.Button1, 0))
	split.HandleEvent(tcell.NewEventMouse(x, 3, tcell.ButtonNone, 0))

	if right.eventCount != 2 {
		t.Fatalf("right panel events = %d, want replayed press and release", right.eventCount)
	}
	if len(*resizes) != 0 {
		t.Fatalf("stationary click resized: %v", *resizes)
	}
}

func TestSplitPanelVerticalMoveOnRightEdgeIsNotAClick(t *testing.T) {
	split, right, resizes := newResizeTestSplit()
	x := split.DividerScreenX() + 1

	split.HandleEvent(tcell.NewEventMouse(x, 3, tcell.Button1, 0))
	split.HandleEvent(tcell.NewEventMouse(x, 6, tcell.Button1, 0))
	split.HandleEvent(tcell.NewEventMouse(x, 6, tcell.ButtonNone, 0))

	if right.eventCount != 0 {
		t.Fatalf("right panel received %d events from a vertical drag", right.eventCount)
	}
	if len(*resizes) != 0 {
		t.Fatalf("vertical-only drag resized: %v", *resizes)
	}
	if split.Dragging() || split.OwnsPointerCapture() {
		t.Fatal("drag state survived release")
	}
}

func TestSplitPanelHorizontalDragFromRightEdgeResizes(t *testing.T) {
	split, right, resizes := newResizeTestSplit()
	x := split.DividerScreenX() + 1

	split.HandleEvent(tcell.NewEventMouse(x, 3, tcell.Button1, 0))
	split.HandleEvent(tcell.NewEventMouse(x+4, 3, tcell.Button1, 0))
	split.HandleEvent(tcell.NewEventMouse(x+4, 3, tcell.ButtonNone, 0))

	if right.eventCount != 0 {
		t.Fatalf("right panel received %d events from a resize drag", right.eventCount)
	}
	if len(*resizes) == 0 || (*resizes)[len(*resizes)-1] != x+4-1 {
		t.Fatalf("resizes = %v, want final width %d", *resizes, x+3)
	}
}
