package ui

import (
	"github.com/gdamore/tcell/v3"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type SidebarWidget struct {
	BaseWidget
	TabbedPanel
	Visible                   bool
	Borders                   *term.BorderSet
	capturedChild             Widget
	pointerCaptureInvalidated func()
	cancelingPointerCapture   bool
	lastSeenBtn               tcell.ButtonMask
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
		s.InvalidatePointerInteraction()
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
	captured := s.capturedChild
	canceled := captured != nil
	s.capturedChild = nil
	s.cancelingPointerCapture = true
	if captured != nil {
		canceled = widgets.CancelPointerCapture(captured) || canceled
	}
	canceled = s.Tabs.CancelPointerCapture() || canceled
	s.cancelingPointerCapture = false
	if canceled && s.pointerCaptureInvalidated != nil {
		s.pointerCaptureInvalidated()
	}
	return canceled
}

func (s *SidebarWidget) OwnsPointerCapture() bool {
	if s.capturedChild != nil {
		owner, ok := s.capturedChild.(widgets.PointerCaptureOwner)
		if !ok || owner.OwnsPointerCapture() {
			return true
		}
		s.capturedChild = nil
	}
	return s.Tabs.OwnsPointerCapture()
}

func (s *SidebarWidget) InvalidatePointerInteraction() bool {
	captured := s.capturedChild
	invalidated := captured != nil
	s.capturedChild = nil
	s.cancelingPointerCapture = true
	if captured != nil {
		invalidated = widgets.InvalidatePointerInteraction(captured) || invalidated
	}
	invalidated = s.Tabs.InvalidatePointerInteraction() || invalidated
	s.cancelingPointerCapture = false
	if invalidated && s.pointerCaptureInvalidated != nil {
		s.pointerCaptureInvalidated()
	}
	return invalidated
}

func (s *SidebarWidget) SetPointerCaptureInvalidated(invalidated func()) {
	s.pointerCaptureInvalidated = invalidated
	s.Tabs.SetPointerCaptureInvalidated(func() {
		if !s.cancelingPointerCapture && s.pointerCaptureInvalidated != nil {
			s.pointerCaptureInvalidated()
		}
	})
}

func (s *SidebarWidget) captureChild(child Widget) {
	s.capturedChild = child
	widgets.SetPointerCaptureInvalidated(child, func() {
		if s.capturedChild != child {
			return
		}
		s.capturedChild = nil
		if !s.cancelingPointerCapture && s.pointerCaptureInvalidated != nil {
			s.pointerCaptureInvalidated()
		}
	})
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
		btn := tev.Buttons()
		prevBtn := s.lastSeenBtn
		s.lastSeenBtn = btn

		if s.Tabs.PointerGestureActive() {
			return s.Tabs.HandleEvent(ev)
		}
		if my == r.Y {
			if btn&tcell.Button1 != 0 && prevBtn&tcell.Button1 != 0 {
				return EventIgnored
			}
			return s.Tabs.HandleEvent(ev)
		}
	}
	active := s.ActiveWidget()
	if active != nil {
		result := active.HandleEvent(ev)
		if result == EventCaptured {
			s.captureChild(active)
		}
		return result
	}
	return EventIgnored
}
