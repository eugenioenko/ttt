package ui

import (
	"github.com/gdamore/tcell/v3"

	"github.com/eugenioenko/ttt/internal/term"
)

type SidebarWidget struct {
	BaseWidget
	TabbedPanel
	Visible       bool
	Borders       *term.BorderSet
	capturedChild Widget
}

func NewSidebarWidget() *SidebarWidget {
	s := &SidebarWidget{
		TabbedPanel: NewTabbedPanel(),
		Visible:     true,
	}
	s.InitTabClick()
	return s
}

func (s *SidebarWidget) Focusable() bool { return true }

func (s *SidebarWidget) Render(surface Surface) {
	w, h := surface.Size()
	r := s.GetRect()

	tabH := 2
	if !s.Visible || w <= 0 || h <= tabH {
		s.Tabs.InvalidatePointerInteraction()
		return
	}

	s.RenderTabs(surface, Rect{X: r.X, Y: r.Y, W: r.W, H: 1})
	s.RenderDivider(surface, 1, w, s.Borders)

	active := s.ActiveWidget()
	if active != nil {
		contentH := h - tabH
		active.SetRect(Rect{X: r.X, Y: r.Y + tabH, W: r.W, H: contentH})
		sub := surface.Sub(Rect{X: 0, Y: tabH, W: w, H: contentH})
		active.Render(sub)
	}
}

func (s *SidebarWidget) CancelPointerCapture() bool {
	return s.Tabs.CancelPointerCapture()
}

func (s *SidebarWidget) OwnsPointerCapture() bool {
	return s.Tabs.OwnsPointerCapture()
}

func (s *SidebarWidget) InvalidatePointerInteraction() bool {
	return s.Tabs.InvalidatePointerInteraction()
}

func (s *SidebarWidget) SetPointerCaptureInvalidated(invalidated func()) {
	s.Tabs.SetPointerCaptureInvalidated(invalidated)
}

func (s *SidebarWidget) HandleEvent(ev tcell.Event) EventResult {
	if s.capturedChild != nil {
		result := s.capturedChild.HandleEvent(ev)
		if tev, ok := ev.(*tcell.EventMouse); ok && tev.Buttons() == tcell.ButtonNone {
			s.capturedChild = nil
		}
		return result
	}
	if tev, ok := ev.(*tcell.EventMouse); ok {
		_, my := tev.Position()
		r := s.GetRect()
		if my == r.Y || s.Tabs.PointerGestureActive() {
			return s.Tabs.HandleEvent(ev)
		}
	}
	active := s.ActiveWidget()
	if active != nil {
		result := active.HandleEvent(ev)
		if result == EventCaptured {
			s.capturedChild = active
		}
		return result
	}
	return EventIgnored
}
