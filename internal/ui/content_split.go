package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

type ContentSplitWidget struct {
	BaseWidget
	Top                       Widget
	Bottom                    Widget
	ShowBottom                bool
	BottomH                   int
	Borders                   *term.BorderSet
	OnResize                  func(height int)
	OnBottomClick             func()
	OnTopClick                func()
	RightBorderStartY         *int
	dragging                  bool
	wasPressed                bool
	capturedChild             Widget
	pointerCaptureInvalidated func()
}

func NewContentSplitWidget() *ContentSplitWidget {
	return &ContentSplitWidget{
		ShowBottom: false,
		BottomH:    15,
	}
}

func (cs *ContentSplitWidget) Focusable() bool { return false }

func (cs *ContentSplitWidget) Render(surface Surface) {
	w, h := surface.Size()
	r := cs.GetRect()
	if w <= 0 || h <= 0 {
		cs.InvalidatePointerInteraction()
		return
	}

	if !cs.ShowBottom || cs.Bottom == nil {
		widgets.InvalidatePointerInteraction(cs.Bottom)
		if cs.capturedChild == cs.Bottom {
			cs.capturedChild = nil
		}
		if cs.RightBorderStartY != nil {
			*cs.RightBorderStartY = 2
		}
		if cs.Top != nil && w > 0 && h > 0 {
			cs.Top.SetRect(Rect{X: r.X, Y: r.Y, W: r.W, H: r.H})
			cs.Top.Render(surface)
		} else {
			widgets.InvalidatePointerInteraction(cs.Top)
		}
		return
	}

	b := term.SingleBorderSet()
	if cs.Borders != nil {
		b = *cs.Borders
	}
	bs := term.StyleBorder

	// 1 row for divider + bottomH for content
	needed := cs.BottomH + 1
	if needed > h {
		needed = h
	}
	divY := h - needed
	if divY < 0 {
		divY = 0
	}
	topH := divY

	if cs.RightBorderStartY != nil {
		*cs.RightBorderStartY = min(topH, 2)
	}

	// Top content
	if cs.Top != nil && topH > 0 {
		cs.Top.SetRect(Rect{X: r.X, Y: r.Y, W: r.W, H: topH})
		topSurface := surface.Sub(Rect{X: 0, Y: 0, W: w, H: topH})
		cs.Top.Render(topSurface)
	} else {
		widgets.InvalidatePointerInteraction(cs.Top)
		if cs.capturedChild == cs.Top {
			cs.capturedChild = nil
		}
	}

	// Horizontal divider
	for x := 0; x < w; x++ {
		surface.SetCell(x, divY, term.Cell{Ch: b.Horizontal, Style: bs})
	}

	// Bottom content
	bottomContentH := h - divY - 1
	if cs.Bottom != nil && bottomContentH > 0 {
		cs.Bottom.SetRect(Rect{X: r.X, Y: r.Y + divY + 1, W: r.W, H: bottomContentH})
		bottomSurface := surface.Sub(Rect{X: 0, Y: divY + 1, W: w, H: bottomContentH})
		cs.Bottom.Render(bottomSurface)
	} else {
		widgets.InvalidatePointerInteraction(cs.Bottom)
		if cs.capturedChild == cs.Bottom {
			cs.capturedChild = nil
		}
	}
}

func (cs *ContentSplitWidget) HandleEvent(ev tcell.Event) EventResult {
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return EventIgnored
	}

	r := cs.GetRect()
	mx, my := mev.Position()
	btn := mev.Buttons()
	pressed := btn&tcell.Button1 != 0
	freshClick := pressed && !cs.wasPressed
	cs.wasPressed = pressed

	if cs.dragging {
		if pressed {
			newH := r.Y + r.H - my - 1
			if cs.OnResize != nil {
				cs.OnResize(newH)
			}
			return EventCaptured
		}
		cs.dragging = false
		return EventIgnored
	}

	if cs.capturedChild != nil {
		if btn == tcell.ButtonNone {
			cs.capturedChild.HandleEvent(ev)
			cs.capturedChild = nil
			return EventConsumed
		}
		result := cs.capturedChild.HandleEvent(ev)
		if result == EventCaptured {
			return EventCaptured
		}
		return EventConsumed
	}

	if cs.ShowBottom {
		needed := cs.BottomH + 1
		if needed > r.H {
			needed = r.H
		}
		divY := r.Y + r.H - needed
		if divY < r.Y {
			divY = r.Y
		}

		// r.W-1: exclude last column to avoid colliding with editor scrollbar
		// For divY+1 (tab bar row), let the bottom panel handle clicks first
		if freshClick && my == divY+1 && mx < r.X+r.W-1 && cs.Bottom != nil {
			if cs.Bottom.HandleEvent(ev) == EventConsumed {
				if btn&tcell.Button1 != 0 && cs.OnBottomClick != nil {
					cs.OnBottomClick()
				}
				return EventConsumed
			}
			cs.dragging = true
			return EventCaptured
		}

		if freshClick && my == divY && mx < r.X+r.W-1 {
			cs.dragging = true
			return EventCaptured
		}

		if my > divY && cs.Bottom != nil {
			result := cs.Bottom.HandleEvent(ev)
			if result == EventCaptured {
				cs.capturedChild = cs.Bottom
				if cs.OnBottomClick != nil {
					cs.OnBottomClick()
				}
				return EventCaptured
			}
			if result == EventConsumed && btn&tcell.Button1 != 0 && cs.OnBottomClick != nil {
				cs.OnBottomClick()
			}
			return result
		}
	} else {
		if freshClick && my == r.Y+r.H {
			cs.dragging = true
			return EventCaptured
		}
	}

	if cs.Top != nil {
		result := cs.Top.HandleEvent(ev)
		if result == EventCaptured {
			cs.capturedChild = cs.Top
			if cs.OnTopClick != nil {
				cs.OnTopClick()
			}
			return EventCaptured
		}
		if result == EventConsumed && btn&tcell.Button1 != 0 && cs.OnTopClick != nil {
			cs.OnTopClick()
		}
		return result
	}

	return EventIgnored
}

func (cs *ContentSplitWidget) DividerScreenY() int {
	if !cs.ShowBottom || cs.Bottom == nil {
		return -1
	}
	r := cs.GetRect()
	needed := cs.BottomH + 1
	if needed > r.H {
		needed = r.H
	}
	return r.Y + r.H - needed
}

func (cs *ContentSplitWidget) TopContentHeight() int {
	r := cs.GetRect()
	if r.W <= 0 || r.H <= 0 {
		return 0
	}
	if !cs.ShowBottom || cs.Bottom == nil {
		return r.H
	}
	needed := min(cs.BottomH+1, r.H)
	return max(r.H-needed, 0)
}

func (cs *ContentSplitWidget) CancelPointerCapture() bool {
	canceled := cs.dragging || cs.capturedChild != nil
	cs.dragging = false
	cs.wasPressed = false
	cs.capturedChild = nil
	for _, child := range []Widget{cs.Top, cs.Bottom} {
		if child == nil {
			continue
		}
		canceled = widgets.CancelPointerCapture(child) || canceled
	}
	if canceled && cs.pointerCaptureInvalidated != nil {
		cs.pointerCaptureInvalidated()
	}
	return canceled
}

func (cs *ContentSplitWidget) InvalidatePointerInteraction() bool {
	invalidated := cs.dragging || cs.capturedChild != nil
	cs.dragging = false
	cs.wasPressed = false
	cs.capturedChild = nil
	invalidated = widgets.InvalidatePointerInteraction(cs.Top) || invalidated
	invalidated = widgets.InvalidatePointerInteraction(cs.Bottom) || invalidated
	if invalidated && cs.pointerCaptureInvalidated != nil {
		cs.pointerCaptureInvalidated()
	}
	return invalidated
}

func (cs *ContentSplitWidget) SetPointerCaptureInvalidated(invalidated func()) {
	cs.pointerCaptureInvalidated = invalidated
	for _, child := range []Widget{cs.Top, cs.Bottom} {
		capturedChild := child
		widgets.SetPointerCaptureInvalidated(child, func() {
			if cs.capturedChild == capturedChild {
				cs.capturedChild = nil
			}
			cs.wasPressed = false
			if cs.pointerCaptureInvalidated != nil {
				cs.pointerCaptureInvalidated()
			}
		})
	}
}

func (cs *ContentSplitWidget) OwnsPointerCapture() bool {
	if cs.dragging {
		return true
	}
	if cs.capturedChild != nil {
		owner, ok := cs.capturedChild.(widgets.PointerCaptureOwner)
		if !ok || owner.OwnsPointerCapture() {
			return true
		}
		cs.capturedChild = nil
		cs.wasPressed = false
	}
	for _, child := range []Widget{cs.Top, cs.Bottom} {
		if owner, ok := child.(widgets.PointerCaptureOwner); ok && owner.OwnsPointerCapture() {
			return true
		}
	}
	return false
}
