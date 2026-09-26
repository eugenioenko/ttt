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

func TestContentSplitOverDivider(t *testing.T) {
	cs := NewContentSplitWidget()
	cs.Top = &BaseWidget{}
	cs.Bottom = &BaseWidget{}
	cs.ShowBottom = true
	cs.BottomH = 20
	cs.SetRect(Rect{X: 10, Y: 0, W: 100, H: 100})

	divY := 100 - 20 - 1
	for _, tt := range []struct {
		x, y int
		want bool
	}{
		{15, divY, true},
		{15, divY - 1, false},
		{9, divY, false},
		{109, divY, false},
	} {
		if got := cs.OverDivider(tt.x, tt.y); got != tt.want {
			t.Errorf("OverDivider(%d, %d) = %v, want %v", tt.x, tt.y, got, tt.want)
		}
	}

	cs.ShowBottom = false
	if cs.OverDivider(15, divY) {
		t.Error("hidden panel should have no divider")
	}
}

func TestSplitPanelOverDivider(t *testing.T) {
	s := NewSplitPanelWidget()
	s.ShowLeft = true
	s.SetRect(Rect{X: 0, Y: 0, W: 100, H: 30})
	divX := s.DividerScreenX()

	for _, tt := range []struct {
		x, y int
		want bool
	}{
		{divX, 5, true},
		{divX + 1, 5, true},
		{divX - 1, 5, false},
		{divX, 30, false},
	} {
		if got := s.OverDivider(tt.x, tt.y); got != tt.want {
			t.Errorf("OverDivider(%d, %d) = %v, want %v", tt.x, tt.y, got, tt.want)
		}
	}
}

// Docked right, the divider is a column and the pointer resizes sideways.
func TestContentSplitOverDividerDockedRight(t *testing.T) {
	cs := NewContentSplitWidget()
	cs.Top = &BaseWidget{}
	cs.Bottom = &BaseWidget{}
	cs.ShowBottom = true
	cs.Position = SplitRight
	cs.SetRect(Rect{X: 10, Y: 0, W: 100, H: 40})

	divX := 10 + 100 - cs.constrainedRightWidth(100, cs.requestedRightWidth(100)) - 1
	for _, tt := range []struct {
		x, y int
		want bool
	}{
		{divX, 20, true},
		{divX, 0, true},
		{divX - 1, 20, false},
		{divX + 1, 20, false},
		{divX, 40, false},
	} {
		if got := cs.OverDivider(tt.x, tt.y); got != tt.want {
			t.Errorf("OverDivider(%d, %d) = %v, want %v", tt.x, tt.y, got, tt.want)
		}
	}
	if got := cs.ResizeShape(); got != "ew-resize" {
		t.Errorf("ResizeShape() = %q docked right, want ew-resize", got)
	}
	cs.Position = SplitBottom
	if got := cs.ResizeShape(); got != "ns-resize" {
		t.Errorf("ResizeShape() = %q docked bottom, want ns-resize", got)
	}
}
