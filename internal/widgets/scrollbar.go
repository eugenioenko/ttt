package widgets

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type scrollRange struct {
	trackLength   int
	contentLength int
	offset        int
}

func newScrollRange(trackLength, contentLength, offset int) scrollRange {
	if trackLength < 0 {
		trackLength = 0
	}
	if contentLength < 0 {
		contentLength = 0
	}
	r := scrollRange{trackLength: trackLength, contentLength: contentLength}
	r.offset = r.clampOffset(offset)
	return r
}

func (r scrollRange) visible() bool {
	return r.trackLength > 0 && r.contentLength > r.trackLength
}

func (r scrollRange) maxOffset() int {
	if r.trackLength <= 0 || r.contentLength <= r.trackLength {
		return 0
	}
	return r.contentLength - r.trackLength
}

func (r scrollRange) clampOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	if maxOffset := r.maxOffset(); offset > maxOffset {
		return maxOffset
	}
	return offset
}

func (r scrollRange) thumb() (start, length int) {
	if !r.visible() {
		return 0, r.trackLength
	}
	length = r.trackLength * r.trackLength / r.contentLength
	if length < 1 {
		length = 1
	}
	maxStart := r.trackLength - length
	maxOffset := r.maxOffset()
	if r.offset >= maxOffset {
		return maxStart, length
	}
	return r.offset * maxStart / maxOffset, length
}

func (r scrollRange) offsetForThumb(start int) int {
	_, length := r.thumb()
	maxStart := r.trackLength - length
	if start <= 0 {
		return 0
	}
	if maxStart <= 0 {
		return r.maxOffset()
	}
	if start >= maxStart {
		return r.maxOffset()
	}
	return start * r.maxOffset() / maxStart
}

func (r scrollRange) offsetForPointer(position, grabOffset int) int {
	return r.offsetForThumb(position - grabOffset)
}

type scrollbarGeometry struct {
	localTrack Rect
	hitTrack   Rect
}

type scrollbarInteraction struct {
	rangeModel scrollRange
	geometry   scrollbarGeometry
	dragging   bool
	grabOffset int
}

func (s *scrollbarInteraction) configure(r scrollRange, geometry scrollbarGeometry) (offset int, invalidated bool) {
	s.rangeModel = r
	if !r.visible() {
		invalidated = s.dragging
		s.dragging = false
		s.grabOffset = 0
		s.geometry = scrollbarGeometry{}
		return r.offset, invalidated
	}
	s.geometry = geometry
	return r.offset, false
}

func (s *scrollbarInteraction) handlePointer(position int, inside bool, buttons tcell.ButtonMask) (int, EventResult) {
	if s.dragging {
		if buttons == tcell.ButtonNone {
			s.dragging = false
			return s.rangeModel.offset, EventConsumed
		}
		if buttons&tcell.Button1 != 0 {
			offset := s.rangeModel.offsetForPointer(position, s.grabOffset)
			s.rangeModel.offset = offset
			return offset, EventCaptured
		}
		return s.rangeModel.offset, EventCaptured
	}

	if !s.rangeModel.visible() || buttons&tcell.Button1 == 0 || !inside {
		return s.rangeModel.offset, EventIgnored
	}

	thumbStart, thumbLength := s.rangeModel.thumb()
	s.dragging = true
	if position >= thumbStart && position < thumbStart+thumbLength {
		s.grabOffset = position - thumbStart
		return s.rangeModel.offset, EventCaptured
	}

	s.grabOffset = thumbLength / 2
	offset := s.rangeModel.offsetForPointer(position, s.grabOffset)
	s.rangeModel.offset = offset
	return offset, EventCaptured
}

func (s *scrollbarInteraction) cancel() bool {
	if !s.dragging {
		return false
	}
	s.dragging = false
	s.grabOffset = 0
	return true
}

func (s *scrollbarInteraction) visible() bool    { return s.rangeModel.visible() }
func (s *scrollbarInteraction) isDragging() bool { return s.dragging }

type scrollbar struct {
	interaction scrollbarInteraction
}

func (s *scrollbar) Render(surface Surface, geometry scrollbarGeometry, r scrollRange) (int, bool) {
	offset, invalidated := s.interaction.configure(r, geometry)
	if !s.visible() {
		return offset, invalidated
	}
	thumbStart, thumbLength := s.interaction.rangeModel.thumb()
	for y := range r.trackLength {
		style := term.StyleScrollbar
		if y >= thumbStart && y < thumbStart+thumbLength {
			style = term.StyleScrollbarThumb
		}
		surface.SetCell(geometry.localTrack.X, geometry.localTrack.Y+y, term.Cell{Ch: '█', Style: style})
	}
	return offset, invalidated
}

func (s *scrollbar) HandleEvent(ev tcell.Event) (int, EventResult) {
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return s.interaction.rangeModel.offset, EventIgnored
	}
	mx, my := mev.Position()
	hit := s.interaction.geometry.hitTrack
	inside := mx >= hit.X && mx < hit.X+hit.W && my >= hit.Y && my < hit.Y+hit.H
	return s.interaction.handlePointer(my-hit.Y, inside, mev.Buttons())
}

func (s *scrollbar) visible() bool    { return s.interaction.visible() }
func (s *scrollbar) isDragging() bool { return s.interaction.isDragging() }
func (s *scrollbar) cancel() bool     { return s.interaction.cancel() }

type horizontalScrollbar struct {
	interaction scrollbarInteraction
}

func (s *horizontalScrollbar) Render(surface Surface, geometry scrollbarGeometry, r scrollRange) (int, bool) {
	offset, invalidated := s.interaction.configure(r, geometry)
	if !s.visible() {
		return offset, invalidated
	}
	thumbStart, thumbLength := s.interaction.rangeModel.thumb()
	for x := range r.trackLength {
		style := term.StyleScrollbar
		if x >= thumbStart && x < thumbStart+thumbLength {
			style = term.StyleScrollbarThumb
		}
		surface.SetCell(geometry.localTrack.X+x, geometry.localTrack.Y, term.Cell{Ch: '▄', Style: style})
	}
	return offset, invalidated
}

func (s *horizontalScrollbar) HandleEvent(ev tcell.Event) (int, EventResult) {
	mev, ok := ev.(*tcell.EventMouse)
	if !ok {
		return s.interaction.rangeModel.offset, EventIgnored
	}
	mx, my := mev.Position()
	hit := s.interaction.geometry.hitTrack
	inside := mx >= hit.X && mx < hit.X+hit.W && my >= hit.Y && my < hit.Y+hit.H
	return s.interaction.handlePointer(mx-hit.X, inside, mev.Buttons())
}

func (s *horizontalScrollbar) visible() bool    { return s.interaction.visible() }
func (s *horizontalScrollbar) isDragging() bool { return s.interaction.isDragging() }
func (s *horizontalScrollbar) cancel() bool     { return s.interaction.cancel() }
