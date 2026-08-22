package widgets

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type ScrollViewWidget struct {
	BaseWidget
	Child ScrollableWidget

	scrollX                   int
	scrollY                   int
	vbar                      scrollbar
	focused                   bool
	lastContentH              int
	lastViewH                 int
	pointerCaptureInvalidated func()
	cancelingPointerCapture   bool
}

func (s *ScrollViewWidget) WidgetChildren() []Widget {
	if s.Child != nil {
		return []Widget{s.Child}
	}
	return nil
}

func NewScrollViewWidget(child ScrollableWidget) *ScrollViewWidget {
	return &ScrollViewWidget{Child: child}
}

func (sv *ScrollViewWidget) Height() int { return 0 }
func (sv *ScrollViewWidget) Width() int  { return 0 }

// ContentHeight lets an outer scroll view measure a nested one (e.g. the markdown widget).
func (sv *ScrollViewWidget) ContentHeight() int {
	if sv.Child == nil {
		return 0
	}
	_, h := sv.Child.ScrollSize()
	return h + sv.BoxOverheadH()
}

func (sv *ScrollViewWidget) EnsureVisible(x, y int) {
	r := sv.rect
	contentW, contentH := sv.Child.ScrollSize()
	viewW, viewH := sv.viewportSize(r.W, r.H, contentW, contentH)

	if y < sv.scrollY {
		sv.scrollY = y
	}
	if y >= sv.scrollY+viewH {
		sv.scrollY = y - viewH + 1
	}
	if x < sv.scrollX {
		sv.scrollX = x
	}
	if x >= sv.scrollX+viewW {
		sv.scrollX = x - viewW + 1
	}
}

func (sv *ScrollViewWidget) viewportSize(w, h, contentW, contentH int) (int, int) {
	vw, vh := w, h
	if contentH > h {
		vw--
	}
	if contentW > vw {
		vh--
	}
	if contentH > vh && vw == w {
		vw--
	}
	if vw < 0 {
		vw = 0
	}
	if vh < 0 {
		vh = 0
	}
	return vw, vh
}

// viewportOrigin returns the screen position of the inner content area.
func (sv *ScrollViewWidget) viewportOrigin() (int, int) {
	ox := sv.rect.X + sv.Box.MarginLeft + sv.Box.PaddingLeft
	oy := sv.rect.Y + sv.Box.MarginTop + sv.Box.PaddingTop
	if sv.Box.BorderLeft {
		ox++
	}
	if sv.Box.BorderTop {
		oy++
	}
	return ox, oy
}

func (sv *ScrollViewWidget) Render(surface Surface) {
	surface = sv.RenderBox(surface)
	w, h := surface.Size()
	if w <= 0 || h <= 0 || sv.Child == nil {
		sv.InvalidatePointerInteraction()
		return
	}

	contentW, contentH := sv.Child.ScrollSize()
	viewW, viewH := sv.viewportSize(w, h, contentW, contentH)
	hasVBar := contentH > viewH
	hasHBar := contentW > viewW

	sv.lastContentH = contentH
	sv.lastViewH = viewH
	sv.clamp(contentW, contentH, viewW, viewH)

	virt := newVirtualSurface(contentW, contentH)
	// Children keep screen-space rects: content origin = viewport origin minus scroll offset.
	ox, oy := sv.viewportOrigin()
	sv.Child.SetRect(Rect{X: ox - sv.scrollX, Y: oy - sv.scrollY, W: contentW, H: contentH})
	sv.Child.Render(virt)

	for y := range viewH {
		for x := range viewW {
			sx, sy := x+sv.scrollX, y+sv.scrollY
			if sx < contentW && sy < contentH {
				surface.SetCell(x, y, virt.cells[sy][sx])
			}
		}
	}

	if hasVBar {
		sv.vbar.X = sv.rect.X + sv.Box.MarginLeft + sv.Box.PaddingLeft + w - 1
		sv.vbar.Y = sv.rect.Y + sv.Box.MarginTop + sv.Box.PaddingTop
		sv.vbar.Height = viewH
		sv.vbar.TotalItems = contentH
		sv.vbar.TopItem = sv.scrollY
		sv.vbar.Render(surface, w-1, 0)
	}

	if hasHBar {
		sv.renderHBar(surface, viewW, h-1, contentW)
	}
}

func (sv *ScrollViewWidget) renderHBar(surface Surface, barW, y, totalW int) {
	if totalW <= barW || barW <= 0 {
		return
	}
	thumbH := barW * barW / totalW
	if thumbH < 1 {
		thumbH = 1
	}
	scrollable := totalW - barW
	thumbPos := sv.scrollX * (barW - thumbH) / scrollable
	if thumbPos+thumbH > barW {
		thumbPos = barW - thumbH
	}

	for x := range barW {
		style := term.StyleScrollbar
		if x >= thumbPos && x < thumbPos+thumbH {
			style = term.StyleScrollbarThumb
		}
		surface.SetCell(x, y, term.Cell{Ch: '▄', Style: style})
	}
}

func (sv *ScrollViewWidget) HandleEvent(ev tcell.Event) EventResult {
	if newTop, consumed := sv.vbar.HandleEvent(ev); consumed {
		sv.scrollY = newTop
		if sv.vbar.isDragging() {
			return EventCaptured
		}
		return EventConsumed
	}

	switch e := ev.(type) {
	case *tcell.EventMouse:
		btn := e.Buttons()
		mx, my := e.Position()
		r := sv.rect

		mod := e.Modifiers()
		if btn&tcell.WheelLeft != 0 || (btn&tcell.WheelUp != 0 && mod&tcell.ModShift != 0) {
			sv.scrollX -= 3
			if sv.scrollX < 0 {
				sv.scrollX = 0
			}
			return EventConsumed
		}
		if btn&tcell.WheelRight != 0 || (btn&tcell.WheelDown != 0 && mod&tcell.ModShift != 0) {
			sv.scrollHRight(3)
			return EventConsumed
		}
		if btn&tcell.WheelUp != 0 {
			sv.scrollY -= 3
			if sv.scrollY < 0 {
				sv.scrollY = 0
			}
			return EventConsumed
		}
		if btn&tcell.WheelDown != 0 {
			sv.scrollY += 3
			if sv.lastContentH > 0 {
				maxY := sv.lastContentH - sv.lastViewH
				if maxY < 0 {
					maxY = 0
				}
				if sv.scrollY > maxY {
					sv.scrollY = maxY
				}
			}
			return EventConsumed
		}

		if btn&tcell.Button1 != 0 && sv.Child != nil {
			contentW, contentH := sv.Child.ScrollSize()
			_, viewH := sv.viewportSize(r.W, r.H, contentW, contentH)
			hasHBar := contentW > r.W
			if hasHBar && my == r.Y+r.H-1 && mx >= r.X && mx < r.X+r.W {
				sv.scrollHToClick(mx-r.X, r.W, contentW)
				return EventConsumed
			}
			_ = viewH
		}

		// Scrolled-out widgets keep offscreen rects — don't let them catch stray clicks.
		if btn != tcell.ButtonNone && btn&(tcell.WheelUp|tcell.WheelDown|tcell.WheelLeft|tcell.WheelRight) == 0 {
			if !sv.viewportContains(mx, my) {
				return EventIgnored
			}
		}
	}

	if sv.Child != nil {
		return sv.Child.HandleEvent(ev)
	}
	return EventIgnored
}

func (sv *ScrollViewWidget) CancelPointerCapture() bool {
	canceled := sv.vbar.isDragging()
	sv.vbar.dragging = false
	sv.cancelingPointerCapture = true
	if sv.Child != nil {
		canceled = CancelPointerCapture(sv.Child) || canceled
	}
	sv.cancelingPointerCapture = false
	if canceled && sv.pointerCaptureInvalidated != nil {
		sv.pointerCaptureInvalidated()
	}
	return canceled
}

func (sv *ScrollViewWidget) OwnsPointerCapture() bool {
	if sv.vbar.isDragging() {
		return true
	}
	owner, ok := sv.Child.(PointerCaptureOwner)
	return ok && owner.OwnsPointerCapture()
}

func (sv *ScrollViewWidget) InvalidatePointerInteraction() bool {
	invalidated := sv.vbar.isDragging()
	sv.vbar.dragging = false
	sv.cancelingPointerCapture = true
	if sv.Child != nil {
		invalidated = InvalidatePointerInteraction(sv.Child) || invalidated
	}
	sv.cancelingPointerCapture = false
	if invalidated && sv.pointerCaptureInvalidated != nil {
		sv.pointerCaptureInvalidated()
	}
	return invalidated
}

func (sv *ScrollViewWidget) SetPointerCaptureInvalidated(invalidated func()) {
	sv.pointerCaptureInvalidated = invalidated
	if sv.Child != nil {
		SetPointerCaptureInvalidated(sv.Child, func() {
			if !sv.cancelingPointerCapture && sv.pointerCaptureInvalidated != nil {
				sv.pointerCaptureInvalidated()
			}
		})
	}
}

func (sv *ScrollViewWidget) viewportContains(mx, my int) bool {
	if sv.Child == nil {
		return false
	}
	ox, oy := sv.viewportOrigin()
	contentW, contentH := sv.Child.ScrollSize()
	innerW := sv.rect.W - sv.BoxOverheadW()
	innerH := sv.rect.H - sv.BoxOverheadH()
	viewW, viewH := sv.viewportSize(innerW, innerH, contentW, contentH)
	return mx >= ox && mx < ox+viewW && my >= oy && my < oy+viewH
}

func (sv *ScrollViewWidget) scrollHRight(amount int) {
	sv.scrollX += amount
	if sv.Child != nil {
		contentW, _ := sv.Child.ScrollSize()
		r := sv.rect
		maxX := contentW - r.W
		if maxX < 0 {
			maxX = 0
		}
		if sv.scrollX > maxX {
			sv.scrollX = maxX
		}
	}
}

func (sv *ScrollViewWidget) scrollHToClick(clickX, barW, totalW int) {
	if barW <= 0 || totalW <= barW {
		return
	}
	ratio := float64(clickX) / float64(barW)
	sv.scrollX = int(ratio * float64(totalW-barW))
	if sv.scrollX < 0 {
		sv.scrollX = 0
	}
	maxX := totalW - barW
	if sv.scrollX > maxX {
		sv.scrollX = maxX
	}
}

func (sv *ScrollViewWidget) Focusable() bool         { return true }
func (sv *ScrollViewWidget) SetFocused(focused bool) { sv.focused = focused }
func (sv *ScrollViewWidget) IsFocused() bool         { return sv.focused }

func (sv *ScrollViewWidget) clamp(contentW, contentH, viewW, viewH int) {
	maxY := contentH - viewH
	if maxY < 0 {
		maxY = 0
	}
	if sv.scrollY > maxY {
		sv.scrollY = maxY
	}
	if sv.scrollY < 0 {
		sv.scrollY = 0
	}

	maxX := contentW - viewW
	if maxX < 0 {
		maxX = 0
	}
	if sv.scrollX > maxX {
		sv.scrollX = maxX
	}
	if sv.scrollX < 0 {
		sv.scrollX = 0
	}
}
