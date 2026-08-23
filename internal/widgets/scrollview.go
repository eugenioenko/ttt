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
	hbar                      horizontalScrollbar
	focused                   bool
	lastContentW              int
	lastContentH              int
	lastViewW                 int
	lastViewH                 int
	viewport                  Rect
	layoutValid               bool
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
	if sv.Child == nil {
		return
	}
	r := sv.rect
	contentW, contentH := sv.Child.ScrollSize()
	viewW, viewH := sv.viewportSize(r.W-sv.BoxOverheadW(), r.H-sv.BoxOverheadH(), contentW, contentH)

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
	sv.clamp(contentW, contentH, viewW, viewH)
}

func (sv *ScrollViewWidget) viewportSize(w, h, contentW, contentH int) (int, int) {
	w = max(w, 0)
	h = max(h, 0)
	vw, vh := w, h
	for {
		nextW := w
		nextH := h
		if contentH > vh {
			nextW--
		}
		if contentW > vw {
			nextH--
		}
		nextW = max(nextW, 0)
		nextH = max(nextH, 0)
		if nextW == vw && nextH == vh {
			break
		}
		vw, vh = nextW, nextH
	}
	return vw, vh
}

// viewportOrigin returns the screen position of the inner content area.
func (sv *ScrollViewWidget) viewportOrigin() (int, int) {
	return sv.contentOrigin()
}

func (sv *ScrollViewWidget) Render(surface Surface) {
	surface = sv.RenderBox(surface)
	w, h := surface.Size()
	surface.Fill(term.Cell{Ch: ' '})
	if w <= 0 || h <= 0 || sv.Child == nil {
		sv.invalidateRenderState(surface)
		return
	}

	contentW, contentH := sv.Child.ScrollSize()
	contentW = max(contentW, 0)
	contentH = max(contentH, 0)
	viewW, viewH := sv.viewportSize(w, h, contentW, contentH)

	sv.lastContentW = contentW
	sv.lastContentH = contentH
	sv.lastViewW = viewW
	sv.lastViewH = viewH
	sv.clamp(contentW, contentH, viewW, viewH)

	virt := newVirtualSurface(contentW, contentH)
	// Children keep screen-space rects: content origin = viewport origin minus scroll offset.
	ox, oy := sv.viewportOrigin()
	sv.viewport = Rect{X: ox, Y: oy, W: viewW, H: viewH}
	sv.layoutValid = true
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

	vGeometry := scrollbarGeometry{
		localTrack: Rect{X: viewW, Y: 0, W: 1, H: viewH},
		hitTrack:   Rect{X: ox + viewW, Y: oy, W: 1, H: viewH},
	}
	_, vInvalidated := sv.vbar.Render(surface, vGeometry, newScrollRange(viewH, contentH, sv.scrollY))
	hGeometry := scrollbarGeometry{
		localTrack: Rect{X: 0, Y: viewH, W: viewW, H: 1},
		hitTrack:   Rect{X: ox, Y: oy + viewH, W: viewW, H: 1},
	}
	_, hInvalidated := sv.hbar.Render(surface, hGeometry, newScrollRange(viewW, contentW, sv.scrollX))
	if viewW < w && viewH < h {
		surface.SetCell(viewW, viewH, term.Cell{Ch: ' '})
	}
	sv.notifyPointerCaptureInvalidated(vInvalidated || hInvalidated)
}

func (sv *ScrollViewWidget) HandleEvent(ev tcell.Event) EventResult {
	if newLeft, result := sv.hbar.HandleEvent(ev); result != EventIgnored {
		sv.scrollX = newLeft
		return result
	}
	if newTop, result := sv.vbar.HandleEvent(ev); result != EventIgnored {
		sv.scrollY = newTop
		return result
	}

	switch e := ev.(type) {
	case *tcell.EventMouse:
		btn := e.Buttons()
		mx, my := e.Position()

		mod := e.Modifiers()
		if btn&tcell.WheelLeft != 0 || (btn&tcell.WheelUp != 0 && mod&tcell.ModShift != 0) {
			horizontal, _ := sv.currentScrollRanges()
			sv.scrollX = horizontal.clampOffset(sv.scrollX - 3)
			return EventConsumed
		}
		if btn&tcell.WheelRight != 0 || (btn&tcell.WheelDown != 0 && mod&tcell.ModShift != 0) {
			sv.scrollHRight(3)
			return EventConsumed
		}
		if btn&tcell.WheelUp != 0 {
			_, vertical := sv.currentScrollRanges()
			sv.scrollY = vertical.clampOffset(sv.scrollY - 3)
			return EventConsumed
		}
		if btn&tcell.WheelDown != 0 {
			_, vertical := sv.currentScrollRanges()
			sv.scrollY = vertical.clampOffset(sv.scrollY + 3)
			return EventConsumed
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
	canceled := sv.vbar.cancel()
	if sv.hbar.cancel() {
		canceled = true
	}
	sv.cancelingPointerCapture = true
	if sv.Child != nil {
		if CancelPointerCapture(sv.Child) {
			canceled = true
		}
	}
	sv.cancelingPointerCapture = false
	sv.notifyPointerCaptureInvalidated(canceled)
	return canceled
}

func (sv *ScrollViewWidget) OwnsPointerCapture() bool {
	if sv.vbar.isDragging() || sv.hbar.isDragging() {
		return true
	}
	owner, ok := sv.Child.(PointerCaptureOwner)
	return ok && owner.OwnsPointerCapture()
}

func (sv *ScrollViewWidget) InvalidatePointerInteraction() bool {
	invalidated := sv.vbar.cancel()
	if sv.hbar.cancel() {
		invalidated = true
	}
	sv.cancelingPointerCapture = true
	if sv.Child != nil {
		if InvalidatePointerInteraction(sv.Child) {
			invalidated = true
		}
	}
	sv.cancelingPointerCapture = false
	sv.notifyPointerCaptureInvalidated(invalidated)
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
	if !sv.layoutValid {
		return false
	}
	r := sv.viewport
	return mx >= r.X && mx < r.X+r.W && my >= r.Y && my < r.Y+r.H
}

func (sv *ScrollViewWidget) scrollHRight(amount int) {
	horizontal, _ := sv.currentScrollRanges()
	sv.scrollX = horizontal.clampOffset(sv.scrollX + amount)
}

func (sv *ScrollViewWidget) Focusable() bool         { return true }
func (sv *ScrollViewWidget) SetFocused(focused bool) { sv.focused = focused }
func (sv *ScrollViewWidget) IsFocused() bool         { return sv.focused }

func (sv *ScrollViewWidget) clamp(contentW, contentH, viewW, viewH int) {
	sv.scrollY = newScrollRange(viewH, contentH, sv.scrollY).offset
	sv.scrollX = newScrollRange(viewW, contentW, sv.scrollX).offset
}

func (sv *ScrollViewWidget) currentScrollRanges() (horizontal, vertical scrollRange) {
	if sv.layoutValid {
		return newScrollRange(sv.lastViewW, sv.lastContentW, sv.scrollX), newScrollRange(sv.lastViewH, sv.lastContentH, sv.scrollY)
	}
	if sv.Child == nil {
		return newScrollRange(0, 0, sv.scrollX), newScrollRange(0, 0, sv.scrollY)
	}
	contentW, contentH := sv.Child.ScrollSize()
	viewW, viewH := sv.viewportSize(sv.rect.W-sv.BoxOverheadW(), sv.rect.H-sv.BoxOverheadH(), contentW, contentH)
	return newScrollRange(viewW, contentW, sv.scrollX), newScrollRange(viewH, contentH, sv.scrollY)
}

func (sv *ScrollViewWidget) invalidateRenderState(surface Surface) {
	sv.layoutValid = false
	sv.viewport = Rect{}
	_, vInvalidated := sv.vbar.Render(surface, scrollbarGeometry{}, newScrollRange(0, 0, sv.scrollY))
	_, hInvalidated := sv.hbar.Render(surface, scrollbarGeometry{}, newScrollRange(0, 0, sv.scrollX))
	sv.cancelingPointerCapture = true
	childInvalidated := false
	if sv.Child != nil {
		childInvalidated = InvalidatePointerInteraction(sv.Child)
	}
	sv.cancelingPointerCapture = false
	sv.notifyPointerCaptureInvalidated(vInvalidated || hInvalidated || childInvalidated)
}

func (sv *ScrollViewWidget) notifyPointerCaptureInvalidated(invalidated bool) {
	if invalidated && sv.pointerCaptureInvalidated != nil {
		sv.pointerCaptureInvalidated()
	}
}
