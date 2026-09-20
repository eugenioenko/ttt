package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

type captureStubWidget struct {
	BaseWidget
	owns   bool
	events int
}

func (s *captureStubWidget) OwnsPointerCapture() bool { return s.owns }

func (s *captureStubWidget) HandleEvent(tcell.Event) EventResult {
	s.events++
	return EventCaptured
}

func TestTerminalPanelDragIntoTabStripDoesNotSwitchTerminal(t *testing.T) {
	tp := NewTerminalPanelWidget()
	first := &captureStubWidget{}
	second := &captureStubWidget{}
	tp.AddTerminal(first)
	tp.AddTerminal(second)
	tp.SetActive(0)
	tp.SetRect(Rect{X: 0, Y: 0, W: 40, H: 10})

	first.owns = true
	drag := tcell.NewEventMouse(2, 1, tcell.Button1, 0)
	tp.HandleEvent(drag)

	if tp.ActiveIndex() != 0 {
		t.Errorf("active = %d, want 0: drag into the tab strip switched terminals", tp.ActiveIndex())
	}
	if first.events != 1 {
		t.Errorf("active terminal got %d events, want 1", first.events)
	}
}
