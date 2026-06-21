package ui

import (
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v2"
)

type ContentSplitWidget struct {
	BaseWidget
	Top               Widget
	Bottom            Widget
	ShowBottom        bool
	BottomH           int
	Borders           *term.BorderSet
	OnResize          func(height int)
	OnBottomClick     func()
	OnTopClick        func()
	RightBorderStartY *int
	dragging          bool
	wasPressed        bool
	capturedChild     Widget
}

func NewContentSplitWidget() *ContentSplitWidget {
	return &ContentSplitWidget{
		ShowBottom: false,
		BottomH:    10,
	}
}

func (cs *ContentSplitWidget) Focusable() bool { return false }

func (cs *ContentSplitWidget) Render(surface *RenderSurface) {
	w, h := surface.Size()
	r := cs.GetRect()

	if !cs.ShowBottom || cs.Bottom == nil {
		if cs.RightBorderStartY != nil {
			*cs.RightBorderStartY = 2
		}
		if cs.Top != nil && w > 0 && h > 0 {
			cs.Top.SetRect(Rect{X: r.X, Y: r.Y, W: r.W, H: r.H})
			cs.Top.Render(surface)
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
