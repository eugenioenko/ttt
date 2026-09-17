package ui

import (
	"slices"
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
	if got := split.TopContentHeight(); got != top.GetRect().H {
		t.Fatalf("20-row ratio top height = %d, rendered %d", got, top.GetRect().H)
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

// The divider is draggable but looks inert; the hover callback is what lets the
// host switch the mouse pointer, and it must fire on the edge, not on every
// motion event.
func TestDividerHoverFiresOnEdges(t *testing.T) {
	cs := NewContentSplitWidget()
	cs.Top = &BaseWidget{}
	cs.Bottom = &BaseWidget{}
	cs.ShowBottom = true
	cs.BottomH = 20
	cs.SetRect(Rect{X: 0, Y: 0, W: 100, H: 100})

	var calls []bool
	cs.OnDividerHover = func(over bool) { calls = append(calls, over) }

	move := func(x, y int) {
		cs.HandleEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone))
	}

	divY := 100 - 20 - 1
	move(5, divY)
	move(5, divY) // staying put must not fire again
	move(5, 0)

	want := []bool{true, false}
	if !slices.Equal(calls, want) {
		t.Fatalf("hover calls = %v, want %v", calls, want)
	}
}
