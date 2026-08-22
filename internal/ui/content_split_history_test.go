package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestContentSplitRatioTracksLayoutUntilDragged(t *testing.T) {
	top := &mockWidget{}
	bottom := &mockWidget{}
	split := NewContentSplitWidget()
	split.Top = top
	split.Bottom = bottom
	split.ShowBottom = true
	split.BottomH = 0
	split.BottomRatio = 0.5
	split.MinTopH = 5
	split.MinBottomH = 4
	split.OnResize = func(height int) { split.BottomH = height }

	render := func(height int) {
		split.SetRect(Rect{X: 2, Y: 3, W: 30, H: height})
		split.Render(NewRenderSurface(makeGrid(30, height), Rect{X: 2, Y: 3, W: 30, H: height}))
	}
	render(20)
	if got := bottom.GetRect().H; got != 10 {
		t.Fatalf("20-row ratio height = %d, want 10", got)
	}
	render(30)
	if got := bottom.GetRect().H; got != 15 {
		t.Fatalf("30-row ratio height = %d, want 15", got)
	}

	divider := split.DividerScreenY()
	split.HandleEvent(tcell.NewEventMouse(4, divider, tcell.Button1, tcell.ModNone))
	split.HandleEvent(tcell.NewEventMouse(4, divider+5, tcell.Button1, tcell.ModNone))
	split.HandleEvent(tcell.NewEventMouse(4, divider+5, tcell.ButtonNone, tcell.ModNone))
	if split.BottomRatio != 0 || split.BottomH != 10 {
		t.Fatalf("manual split = ratio %.1f height %d, want ratio 0 height 10", split.BottomRatio, split.BottomH)
	}
	render(40)
	if got := bottom.GetRect().H; got != 10 {
		t.Fatalf("manual height changed after resize: %d", got)
	}
}

func TestContentSplitRatioRetainsUsableMinimums(t *testing.T) {
	split := NewContentSplitWidget()
	split.Top = &mockWidget{}
	split.Bottom = &mockWidget{}
	split.ShowBottom = true
	split.BottomH = 0
	split.BottomRatio = 0.5
	split.MinTopH = 5
	split.MinBottomH = 4

	if got := split.constrainedBottomHeight(10, split.requestedBottomHeight(10)); got != 4 {
		t.Fatalf("tight layout bottom = %d, want history minimum 4", got)
	}
	if got := split.constrainedBottomHeight(6, split.requestedBottomHeight(6)); got != 0 {
		t.Fatalf("impossible layout bottom = %d, want primary surface minimum to win", got)
	}
}
