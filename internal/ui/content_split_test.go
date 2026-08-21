package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestContentSplitDragRespectsChildMinimumHeights(t *testing.T) {
	top := &mockWidget{}
	bottom := &mockWidget{}
	cs := NewContentSplitWidget()
	cs.Top = top
	cs.Bottom = bottom
	cs.ShowBottom = true
	cs.BottomH = 8
	cs.MinTopH = 5
	cs.MinBottomH = 4
	cs.SetRect(Rect{X: 3, Y: 2, W: 30, H: 20})
	cs.OnResize = func(height int) { cs.BottomH = height }

	render := func() {
		cs.Render(NewRenderSurface(makeGrid(30, 20), Rect{X: 3, Y: 2, W: 30, H: 20}))
	}
	drag := func(fromY, toY int) {
		t.Helper()
		if got := cs.HandleEvent(tcell.NewEventMouse(5, fromY, tcell.Button1, tcell.ModNone)); got != EventCaptured {
			t.Fatalf("drag start result = %v, want captured", got)
		}
		if got := cs.HandleEvent(tcell.NewEventMouse(5, toY, tcell.Button1, tcell.ModNone)); got != EventCaptured {
			t.Fatalf("drag move result = %v, want captured", got)
		}
		cs.HandleEvent(tcell.NewEventMouse(5, toY, tcell.ButtonNone, tcell.ModNone))
		render()
	}

	render()
	drag(cs.DividerScreenY(), 21)
	if cs.BottomH != 4 {
		t.Errorf("bottom height after downward drag = %d, want minimum 4", cs.BottomH)
	}
	if got := bottom.GetRect().H; got != 4 {
		t.Errorf("rendered bottom height = %d, want 4", got)
	}

	drag(cs.DividerScreenY(), 2)
	if cs.BottomH != 14 {
		t.Errorf("bottom height after upward drag = %d, want maximum 14", cs.BottomH)
	}
	if got := top.GetRect().H; got != 5 {
		t.Errorf("rendered top height = %d, want minimum 5", got)
	}
}

func TestContentSplitExposesChildrenInLayoutOrder(t *testing.T) {
	top := &mockWidget{}
	bottom := &mockWidget{}
	cs := NewContentSplitWidget()
	cs.Top = top
	cs.Bottom = bottom

	children := cs.WidgetChildren()
	if len(children) != 2 || children[0] != top || children[1] != bottom {
		t.Fatalf("children = %#v, want top then bottom", children)
	}
}

func TestContentSplitRatioTracksAvailableHeightUntilResized(t *testing.T) {
	top := &mockWidget{}
	bottom := &mockWidget{}
	cs := NewContentSplitWidget()
	cs.Top = top
	cs.Bottom = bottom
	cs.ShowBottom = true
	cs.BottomH = 0
	cs.BottomRatio = 0.5

	render := func(height int) {
		cs.SetRect(Rect{W: 30, H: height})
		cs.Render(NewRenderSurface(makeGrid(30, height), Rect{W: 30, H: height}))
	}
	render(20)
	if bottom.GetRect().H != 10 || top.GetRect().H != 9 {
		t.Fatalf("20-row half split = top %d bottom %d, want 9/10 around divider", top.GetRect().H, bottom.GetRect().H)
	}
	render(30)
	if bottom.GetRect().H != 15 || top.GetRect().H != 14 {
		t.Fatalf("30-row half split = top %d bottom %d, want 14/15 around divider", top.GetRect().H, bottom.GetRect().H)
	}
}
