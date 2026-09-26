package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

// The zero value must stay SplitBottom: the changes panel reuses this widget.
type SplitPosition int

const (
	SplitBottom SplitPosition = iota
	SplitRight
)

type ContentSplitWidget struct {
	BaseWidget
	Top                       Widget
	Bottom                    Widget
	Position                  SplitPosition
	ShowBottom                bool
	BottomH                   int
	RightW                    int
	BottomRatio               float64
	MinTopH                   int
	MinBottomH                int
	MinRightW                 int
	Borders                   *term.BorderSet
	OnResize                  func(height int)
	OnBottomClick             func()
	OnTopClick                func()
	RightBorderStartY         *int
	dragging                  bool
	wasPressed                bool
	capturedChild             Widget
	pointerCaptureInvalidated func()
	cancelingPointerCapture   bool
}

func (cs *ContentSplitWidget) WidgetChildren() []Widget {
	children := make([]Widget, 0, 2)
	if cs.Top != nil {
		children = append(children, cs.Top)
	}
	if cs.Bottom != nil {
		children = append(children, cs.Bottom)
	}
	return children
}

func NewContentSplitWidget() *ContentSplitWidget {
	return &ContentSplitWidget{
		ShowBottom: false,
		BottomH:    15,
		RightW:     60,
	}
}

func (cs *ContentSplitWidget) Focusable() bool { return false }

func (cs *ContentSplitWidget) constrainedBottomHeight(totalH, requested int) int {
	if totalH <= 1 {
		return 0
	}
	maxBottom := max(totalH-1-max(cs.MinTopH, 0), 0)
	minBottom := min(max(cs.MinBottomH, 0), maxBottom)
	return min(max(requested, minBottom), maxBottom)
}

const minTopW = 20

func (cs *ContentSplitWidget) constrainedRightWidth(totalW, requested int) int {
	if totalW <= 1 {
		return 0
	}
	maxRight := max(totalW-1-minTopW, 0)
	minRight := min(max(cs.MinRightW, 0), maxRight)
	return min(max(requested, minRight), maxRight)
}

func (cs *ContentSplitWidget) requestedRightWidth(totalW int) int {
	if cs.RightW <= 0 && cs.BottomRatio > 0 {
		return int(float64(totalW) * cs.BottomRatio)
	}
	return cs.RightW
}

func (cs *ContentSplitWidget) requestedBottomHeight(totalH int) int {
	if cs.BottomH <= 0 && cs.BottomRatio > 0 {
		return int(float64(totalH) * cs.BottomRatio)
	}
	return cs.BottomH
}

func (cs *ContentSplitWidget) ResizePanel(delta int) {
	r := cs.GetRect()
	if cs.Position == SplitRight {
		size := cs.requestedRightWidth(r.W)
		if r.W > 1 {
			size = cs.constrainedRightWidth(r.W, cs.constrainedRightWidth(r.W, size)+delta)
		} else {
			size += delta
		}
		cs.RightW = max(size, 1)
		return
	}
	size := cs.requestedBottomHeight(r.H)
	if r.H > 1 {
		size = cs.constrainedBottomHeight(r.H, cs.constrainedBottomHeight(r.H, size)+delta)
	} else {
		size += delta
	}
	cs.BottomH = max(size, 1)
}

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

	if cs.Position == SplitRight {
		cs.renderRight(surface, r, w, h, b, bs)
		return
	}

	bottomH := cs.constrainedBottomHeight(h, cs.requestedBottomHeight(h))
	divY := h - bottomH - 1
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
	bottomContentH := bottomH
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

func (cs *ContentSplitWidget) renderRight(surface Surface, r Rect, w, h int, b term.BorderSet, bs term.Style) {
	rightW := cs.constrainedRightWidth(w, cs.requestedRightWidth(w))
	divX := w - rightW - 1
	topW := divX

	if cs.RightBorderStartY != nil {
		*cs.RightBorderStartY = 0
	}

	if cs.Top != nil && topW > 0 {
		cs.Top.SetRect(Rect{X: r.X, Y: r.Y, W: topW, H: r.H})
		cs.Top.Render(surface.Sub(Rect{X: 0, Y: 0, W: topW, H: h}))
	} else {
		widgets.InvalidatePointerInteraction(cs.Top)
		if cs.capturedChild == cs.Top {
			cs.capturedChild = nil
		}
	}

	surface.SetCell(divX, 0, term.Cell{Ch: b.TopLeft, Style: bs})
	for x := divX + 1; x < w; x++ {
		surface.SetCell(x, 0, term.Cell{Ch: b.Horizontal, Style: bs})
	}
	for y := 1; y < h; y++ {
		surface.SetCell(divX, y, term.Cell{Ch: b.Vertical, Style: bs})
	}

	if cs.Bottom != nil && rightW > 0 && h > 1 {
		cs.Bottom.SetRect(Rect{X: r.X + divX + 1, Y: r.Y + 1, W: rightW, H: r.H - 1})
		cs.Bottom.Render(surface.Sub(Rect{X: divX + 1, Y: 1, W: rightW, H: h - 1}))
		if divider, ok := cs.Bottom.(dividerYProvider); ok {
			y := divider.DividerScreenY() - r.Y
			if y >= 0 && y < h {
				surface.SetCell(divX, y, term.Cell{Ch: b.LeftTee, Style: bs})
			}
		}
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
			var size int
			if cs.Position == SplitRight {
				size = cs.constrainedRightWidth(r.W, r.X+r.W-mx-1)
			} else {
				size = cs.constrainedBottomHeight(r.H, r.Y+r.H-my-1)
			}
			cs.BottomRatio = 0
			if cs.OnResize != nil {
				cs.OnResize(size)
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

	if cs.ShowBottom && cs.Position == SplitRight {
		rightW := cs.constrainedRightWidth(r.W, cs.requestedRightWidth(r.W))
		divX := r.X + r.W - rightW - 1

		if freshClick && mx == divX {
			cs.dragging = true
			return EventCaptured
		}

		if mx > divX && cs.Bottom != nil {
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
	} else if cs.ShowBottom {
		bottomH := cs.constrainedBottomHeight(r.H, cs.requestedBottomHeight(r.H))
		divY := r.Y + r.H - bottomH - 1

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

// OverDivider reports whether the pointer is on the panel's divider: a row
// when the panel is docked at the bottom, a column when it is docked right.
func (cs *ContentSplitWidget) OverDivider(mx, my int) bool {
	r := cs.GetRect()
	if cs.Position == SplitRight {
		if !cs.ShowBottom || cs.Bottom == nil || my < r.Y || my >= r.Y+r.H {
			return false
		}
		return mx == r.X+r.W-cs.constrainedRightWidth(r.W, cs.requestedRightWidth(r.W))-1
	}
	return my == cs.DividerScreenY() && mx >= r.X && mx < r.X+r.W-1
}

// ResizeShape is the pointer shape for dragging the divider.
func (cs *ContentSplitWidget) ResizeShape() string {
	if cs.Position == SplitRight {
		return "ew-resize"
	}
	return "ns-resize"
}

func (cs *ContentSplitWidget) Dragging() bool { return cs.dragging }

func (cs *ContentSplitWidget) DividerScreenY() int {
	if !cs.ShowBottom || cs.Bottom == nil || cs.Position == SplitRight {
		return -1
	}
	r := cs.GetRect()
	bottomH := cs.constrainedBottomHeight(r.H, cs.requestedBottomHeight(r.H))
	return r.Y + r.H - bottomH - 1
}

func (cs *ContentSplitWidget) DividerScreenX() int {
	if !cs.ShowBottom || cs.Bottom == nil || cs.Position != SplitRight {
		return -1
	}
	r := cs.GetRect()
	return r.X + r.W - cs.constrainedRightWidth(r.W, cs.requestedRightWidth(r.W)) - 1
}

func (cs *ContentSplitWidget) BorderJunctions() borderJunctions {
	junctions := borderJunctions{BottomX: [2]int{-1, -1}, RightY: -1}
	if !cs.ShowBottom || cs.Bottom == nil {
		return junctions
	}
	if cs.Position == SplitRight {
		junctions.BottomX[0] = cs.DividerScreenX()
		if divider, ok := cs.Bottom.(dividerYProvider); ok {
			junctions.RightY = divider.DividerScreenY()
		}
	}
	if divider, ok := cs.Bottom.(dividerXProvider); ok {
		x := divider.DividerScreenX()
		if junctions.BottomX[0] < 0 {
			junctions.BottomX[0] = x
		} else {
			junctions.BottomX[1] = x
		}
	}
	return junctions
}

func (cs *ContentSplitWidget) TopContentHeight() int {
	r := cs.GetRect()
	if r.W <= 0 || r.H <= 0 {
		return 0
	}
	if !cs.ShowBottom || cs.Bottom == nil || cs.Position == SplitRight {
		return r.H
	}
	bottomH := cs.constrainedBottomHeight(r.H, cs.requestedBottomHeight(r.H))
	return max(r.H-bottomH-1, 0)
}

func (cs *ContentSplitWidget) CancelPointerCapture() bool {
	canceled := cs.dragging || cs.capturedChild != nil
	cs.dragging = false
	cs.wasPressed = false
	cs.capturedChild = nil
	cs.cancelingPointerCapture = true
	for _, child := range []Widget{cs.Top, cs.Bottom} {
		if child == nil {
			continue
		}
		canceled = widgets.CancelPointerCapture(child) || canceled
	}
	cs.cancelingPointerCapture = false
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
	cs.cancelingPointerCapture = true
	invalidated = widgets.InvalidatePointerInteraction(cs.Top) || invalidated
	invalidated = widgets.InvalidatePointerInteraction(cs.Bottom) || invalidated
	cs.cancelingPointerCapture = false
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
			if cs.capturedChild != capturedChild {
				return
			}
			cs.capturedChild = nil
			cs.wasPressed = false
			if !cs.cancelingPointerCapture && cs.pointerCaptureInvalidated != nil {
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
