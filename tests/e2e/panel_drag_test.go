package e2e

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestPanelDragWithoutTerminalKeepsDragAndTab(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.app.ShowBottomPanel()
	h.app.BottomPanel.SetActivePanel("output")
	h.redraw()

	cs := h.app.ContentSplit
	y := cs.DividerScreenY()
	startH := cs.BottomH
	mouse := func(y int, btn tcell.ButtonMask) {
		h.app.Root.HandleEvent(tcell.NewEventMouse(60, y, btn, tcell.ModNone))
		h.redraw()
	}

	mouse(y, tcell.Button1)
	mouse(y-3, tcell.Button1)
	mouse(y-6, tcell.Button1)
	if !cs.Dragging() {
		t.Fatal("drag was canceled mid-drag")
	}
	mouse(y-6, tcell.ButtonNone)

	if cs.BottomH != startH+6 {
		t.Errorf("panel height = %d, want %d", cs.BottomH, startH+6)
	}
	if n := len(h.app.Terminals); n != 0 {
		t.Errorf("resizing an open panel started %d terminal(s)", n)
	}
	if got := h.app.BottomPanel.ActivePanel; got != "output" {
		t.Errorf("active panel = %q, want output", got)
	}
}

func TestPanelDraggedOpenStartsTerminalOnRelease(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	cs := h.app.ContentSplit
	r := cs.GetRect()
	edge := r.Y + r.H
	mouse := func(y int, btn tcell.ButtonMask) {
		h.app.Root.HandleEvent(tcell.NewEventMouse(60, y, btn, tcell.ModNone))
		h.redraw()
	}

	mouse(edge, tcell.Button1)
	mouse(edge-5, tcell.Button1)
	mouse(edge-10, tcell.Button1)
	if !cs.Dragging() {
		t.Fatal("drag was canceled mid-drag")
	}
	if n := len(h.app.Terminals); n != 0 {
		t.Fatalf("terminal started mid-drag")
	}
	mouse(edge-10, tcell.ButtonNone)

	if !cs.ShowBottom {
		t.Fatal("panel should be open after dragging it open")
	}
	if n := len(h.app.Terminals); n != 1 {
		t.Errorf("terminals after drag-open = %d, want 1", n)
	}
}
