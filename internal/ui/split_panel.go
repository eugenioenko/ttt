package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"log/slog"

	"github.com/gdamore/tcell/v2"
)

const DefaultSidebarWidth = 30

// MinSidebarWidth is the minimum width below which the sidebar resets to
// DefaultSidebarWidth when toggled back on. This prevents the sidebar from
// reopening at an unusably small width after being dragged nearly closed.
const MinSidebarWidth = 10

type SplitPanelWidget struct {
	BaseWidget
	Left              Widget
	Right             Widget
	DividerPos        int
	Borders           *term.BorderSet
	ShowLeft          bool
	RightBorderStartY int
	OnResize          func(width int)
	OnLeftClick       func()
	OnRightClick      func()
	dragging          bool
	wasPressed        bool
	capturedChild     Widget
}

func NewSplitPanelWidget() *SplitPanelWidget {
	return &SplitPanelWidget{
		DividerPos: DefaultSidebarWidth,
		ShowLeft:   true,
	}
}

func (s *SplitPanelWidget) Focusable() bool { return false }

func (s *SplitPanelWidget) Render(surface *RenderSurface) {
	w, h := surface.Size()
	if w < 4 || h < 3 {
		return
	}

	b := term.SingleBorderSet()
	if s.Borders != nil {
		b = *s.Borders
	}
	bs := term.StyleBorder
	r := s.GetRect()

	if !s.ShowLeft {
		s.renderSinglePanel(surface, w, h, b, bs)
		return
	}

	divX := s.DividerPos + 1
	if divX < 2 {
		divX = 2
	}
	if divX >= w-2 {
		divX = w - 3
	}

	// Top border — left side only: ┌───┐
	surface.SetCell(0, 0, term.Cell{Ch: b.TopLeft, Style: bs})
	for x := 1; x < divX; x++ {
		surface.SetCell(x, 0, term.Cell{Ch: b.Horizontal, Style: bs})
	}
	surface.SetCell(divX, 0, term.Cell{Ch: b.TopRight, Style: bs})

	// Bottom border — full width: └───┴───┘
	surface.SetCell(0, h-1, term.Cell{Ch: b.BottomLeft, Style: bs})
	for x := 1; x < divX; x++ {
		surface.SetCell(x, h-1, term.Cell{Ch: b.Horizontal, Style: bs})
	}
	surface.SetCell(divX, h-1, term.Cell{Ch: b.BottomTee, Style: bs})
	for x := divX + 1; x < w-1; x++ {
		surface.SetCell(x, h-1, term.Cell{Ch: b.Horizontal, Style: bs})
	}
	surface.SetCell(w-1, h-1, term.Cell{Ch: b.BottomRight, Style: bs})

	// Left border
	for y := 1; y < h-1; y++ {
		surface.SetCell(0, y, term.Cell{Ch: b.Vertical, Style: bs})
	}

	// Right border — starts at RightBorderStartY to skip tab bar area
	rbStart := s.RightBorderStartY
	if rbStart == 0 {
		surface.SetCell(w-1, 0, term.Cell{Ch: b.TopRight, Style: bs})
		rbStart = 1
	} else if rbStart > 0 && rbStart < h-1 {
		surface.SetCell(w-1, rbStart, term.Cell{Ch: b.TopRight, Style: bs})
		rbStart++
	}
	for y := rbStart; y < h-1; y++ {
		surface.SetCell(w-1, y, term.Cell{Ch: b.Vertical, Style: bs})
	}

	// Divider
	for y := 1; y < h-1; y++ {
		surface.SetCell(divX, y, term.Cell{Ch: b.Vertical, Style: bs})
	}

	// Left content — inside left border, below top border, above bottom border
	leftW := divX - 1
	leftH := h - 2
	if s.Left != nil && leftW > 0 && leftH > 0 {
		s.Left.SetRect(Rect{X: r.X + 1, Y: r.Y + 1, W: leftW, H: leftH})
		leftSurface := surface.Sub(Rect{X: 1, Y: 1, W: leftW, H: leftH})
		s.Left.Render(leftSurface)
	}

	// Right content — right of divider, full height (no top border), above bottom border
	rightX := divX + 1
	rightW := w - 1 - rightX
	rightH := h - 1
	if s.Right != nil && rightW > 0 && rightH > 0 {
		s.Right.SetRect(Rect{X: r.X + rightX, Y: r.Y, W: rightW, H: rightH})
		rightSurface := surface.Sub(Rect{X: rightX, Y: 0, W: rightW, H: rightH})
		s.Right.Render(rightSurface)
	}
}

func (s *SplitPanelWidget) renderSinglePanel(surface *RenderSurface, w, h int, b term.BorderSet, bs term.Style) {
	r := s.GetRect()

	// Bottom border
	surface.SetCell(0, h-1, term.Cell{Ch: b.BottomLeft, Style: bs})
	for x := 1; x < w-1; x++ {
		surface.SetCell(x, h-1, term.Cell{Ch: b.Horizontal, Style: bs})
	}
	surface.SetCell(w-1, h-1, term.Cell{Ch: b.BottomRight, Style: bs})

	// Left border — starts at RightBorderStartY
	lbStart := s.RightBorderStartY
	if lbStart == 0 {
		surface.SetCell(0, 0, term.Cell{Ch: b.TopLeft, Style: bs})
		lbStart = 1
	} else if lbStart > 0 && lbStart < h-1 {
		surface.SetCell(0, lbStart, term.Cell{Ch: b.TopLeft, Style: bs})
		lbStart++
	}
	for y := lbStart; y < h-1; y++ {
		surface.SetCell(0, y, term.Cell{Ch: b.Vertical, Style: bs})
	}

	// Right border — starts at RightBorderStartY
	rbStart := s.RightBorderStartY
	if rbStart == 0 {
		surface.SetCell(w-1, 0, term.Cell{Ch: b.TopRight, Style: bs})
		rbStart = 1
	} else if rbStart > 0 && rbStart < h-1 {
		surface.SetCell(w-1, rbStart, term.Cell{Ch: b.TopRight, Style: bs})
		rbStart++
	}
	for y := rbStart; y < h-1; y++ {
		surface.SetCell(w-1, y, term.Cell{Ch: b.Vertical, Style: bs})
	}

	// Content — no top border, inside side borders, above bottom border
	cw := w - 2
	ch := h - 1
	if s.Right != nil && cw > 0 && ch > 0 {
		s.Right.SetRect(Rect{X: r.X + 1, Y: r.Y, W: cw, H: ch})
		sub := surface.Sub(Rect{X: 1, Y: 0, W: cw, H: ch})
		s.Right.Render(sub)
	}
}

func (s *SplitPanelWidget) HandleEvent(ev tcell.Event) EventResult {
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return EventIgnored
	}

	r := s.GetRect()
	mx, my := mev.Position()
	btn := mev.Buttons()
	pressed := btn&tcell.Button1 != 0
	freshClick := pressed && !s.wasPressed
	s.wasPressed = pressed
	inBounds := my >= r.Y && my < r.Y+r.H && mx >= r.X && mx < r.X+r.W

	if s.dragging {
		if pressed {
			if s.OnResize != nil {
				newWidth := mx - r.X - 1
				s.OnResize(newWidth)
			}
			return EventCaptured
		}
		s.dragging = false
		return EventIgnored
	}

	if s.capturedChild != nil {
		if btn == tcell.ButtonNone {
			s.capturedChild.HandleEvent(ev)
			s.capturedChild = nil
			return EventConsumed
		}
		result := s.capturedChild.HandleEvent(ev)
		if result == EventCaptured {
			return EventCaptured
		}
		return EventConsumed
	}

	if !inBounds {
		return EventIgnored
	}

	isClick := pressed

	if s.ShowLeft {
		divX := s.DividerScreenX()
		slog.Debug("splitPanel", "action", "route", "mx", mx, "divX", divX, "showLeft", true)
		// divX to divX+1: grab zone extends right only to avoid overlapping the scrollbar
		if freshClick && mx >= divX && mx <= divX+1 && s.OnResize != nil {
			s.dragging = true
			return EventCaptured
		}
		if mx < divX {
			if s.Left != nil {
				result := s.Left.HandleEvent(ev)
				slog.Debug("splitPanel", "action", "leftChild", "result", result)
				if result == EventCaptured {
					s.capturedChild = s.Left
					if s.OnLeftClick != nil {
						s.OnLeftClick()
					}
					return EventCaptured
				}
				if result == EventConsumed && isClick && s.OnLeftClick != nil {
					s.OnLeftClick()
				}
				return result
			}
		} else {
			if s.Right != nil {
				result := s.Right.HandleEvent(ev)
				slog.Debug("splitPanel", "action", "rightChild", "result", result)
				if result == EventCaptured {
					s.capturedChild = s.Right
					if s.OnRightClick != nil {
						s.OnRightClick()
					}
					return EventCaptured
				}
				if result == EventConsumed && isClick && s.OnRightClick != nil {
					s.OnRightClick()
				}
				return result
			}
		}
	} else {
		if freshClick && mx == r.X {
			s.dragging = true
			return EventCaptured
		}
		if s.Right != nil {
			result := s.Right.HandleEvent(ev)
			if result == EventCaptured {
				s.capturedChild = s.Right
				if s.OnRightClick != nil {
					s.OnRightClick()
				}
				return EventCaptured
			}
			if result == EventConsumed && isClick && s.OnRightClick != nil {
				s.OnRightClick()
			}
			return result
		}
	}

	return EventIgnored
}

func (s *SplitPanelWidget) DividerScreenX() int {
	r := s.GetRect()
	return r.X + s.DividerPos + 1
}
