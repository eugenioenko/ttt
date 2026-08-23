package widgets

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestScrollRangeEdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		track           int
		content         int
		offset          int
		wantVisible     bool
		wantOffset      int
		wantThumbStart  int
		wantThumbLength int
	}{
		{name: "zero track", track: 0, content: 10, offset: 4, wantOffset: 0},
		{name: "negative dimensions", track: -2, content: -1, offset: 4, wantOffset: 0},
		{name: "content smaller", track: 10, content: 5, offset: 4, wantOffset: 0, wantThumbLength: 10},
		{name: "content equal", track: 10, content: 10, offset: 4, wantOffset: 0, wantThumbLength: 10},
		{name: "one cell overflow", track: 1, content: 2, offset: 1, wantVisible: true, wantOffset: 1, wantThumbLength: 1},
		{name: "negative offset", track: 10, content: 100, offset: -4, wantVisible: true, wantOffset: 0, wantThumbLength: 1},
		{name: "oversized offset", track: 10, content: 100, offset: 500, wantVisible: true, wantOffset: 90, wantThumbStart: 9, wantThumbLength: 1},
		{name: "middle offset", track: 10, content: 20, offset: 5, wantVisible: true, wantOffset: 5, wantThumbStart: 2, wantThumbLength: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newScrollRange(tt.track, tt.content, tt.offset)
			if got := r.visible(); got != tt.wantVisible {
				t.Fatalf("visible = %v, want %v", got, tt.wantVisible)
			}
			if r.offset != tt.wantOffset {
				t.Fatalf("offset = %d, want %d", r.offset, tt.wantOffset)
			}
			start, length := r.thumb()
			if start != tt.wantThumbStart || length != tt.wantThumbLength {
				t.Fatalf("thumb = (%d,%d), want (%d,%d)", start, length, tt.wantThumbStart, tt.wantThumbLength)
			}
		})
	}
}

func TestScrollRangeMappingIsMonotonicWithExactEndpoints(t *testing.T) {
	for _, track := range []int{2, 3, 10, 17} {
		r := newScrollRange(track, 101, 0)
		_, thumbLength := r.thumb()
		maxThumbStart := track - thumbLength
		previous := -1
		for position := -3; position <= maxThumbStart+3; position++ {
			offset := r.offsetForThumb(position)
			if offset < previous {
				t.Fatalf("track %d mapping decreased at %d: %d < %d", track, position, offset, previous)
			}
			previous = offset
		}
		if got := r.offsetForThumb(0); got != 0 {
			t.Fatalf("track %d start offset = %d, want 0", track, got)
		}
		if got := r.offsetForThumb(maxThumbStart); got != r.maxOffset() {
			t.Fatalf("track %d end offset = %d, want %d", track, got, r.maxOffset())
		}
		bottom := newScrollRange(track, 101, r.maxOffset())
		start, length := bottom.thumb()
		if start+length != track {
			t.Fatalf("track %d bottom thumb ends at %d, want %d", track, start+length, track)
		}
	}
}

type scrollbarHarness struct {
	name       string
	glyph      rune
	geometry   scrollbarGeometry
	render     func(Surface, scrollbarGeometry, scrollRange) (int, bool)
	handle     func(tcell.Event) (int, EventResult)
	cancel     func() bool
	isDragging func() bool
	mouse      func(position int, buttons tcell.ButtonMask) *tcell.EventMouse
	cell       func(*testSurface, int) term.Cell
}

func newScrollbarHarness(horizontal bool) scrollbarHarness {
	if horizontal {
		bar := &horizontalScrollbar{}
		geometry := scrollbarGeometry{
			localTrack: Rect{X: 2, Y: 1, W: 10, H: 1},
			hitTrack:   Rect{X: 12, Y: 21, W: 10, H: 1},
		}
		return scrollbarHarness{
			name:       "horizontal",
			glyph:      '▄',
			geometry:   geometry,
			render:     bar.Render,
			handle:     bar.HandleEvent,
			cancel:     bar.cancel,
			isDragging: bar.isDragging,
			mouse: func(position int, buttons tcell.ButtonMask) *tcell.EventMouse {
				return tcell.NewEventMouse(geometry.hitTrack.X+position, geometry.hitTrack.Y, buttons, 0)
			},
			cell: func(surface *testSurface, position int) term.Cell {
				return surface.cells[geometry.localTrack.Y][geometry.localTrack.X+position]
			},
		}
	}

	bar := &scrollbar{}
	geometry := scrollbarGeometry{
		localTrack: Rect{X: 2, Y: 1, W: 1, H: 10},
		hitTrack:   Rect{X: 12, Y: 21, W: 1, H: 10},
	}
	return scrollbarHarness{
		name:       "vertical",
		glyph:      '█',
		geometry:   geometry,
		render:     bar.Render,
		handle:     bar.HandleEvent,
		cancel:     bar.cancel,
		isDragging: bar.isDragging,
		mouse: func(position int, buttons tcell.ButtonMask) *tcell.EventMouse {
			return tcell.NewEventMouse(geometry.hitTrack.X, geometry.hitTrack.Y+position, buttons, 0)
		},
		cell: func(surface *testSurface, position int) term.Cell {
			return surface.cells[geometry.localTrack.Y+position][geometry.localTrack.X]
		},
	}
}

func TestScrollbarOrientationRenderAndInteractionParity(t *testing.T) {
	for _, horizontal := range []bool{false, true} {
		h := newScrollbarHarness(horizontal)
		t.Run(h.name, func(t *testing.T) {
			surface := newTestSurface(20, 20)
			r := newScrollRange(10, 20, 0)
			h.render(surface, h.geometry, r)
			thumbStart, thumbLength := r.thumb()
			for position := range 10 {
				cell := h.cell(surface, position)
				if cell.Ch != h.glyph {
					t.Fatalf("cell %d glyph = %q, want %q", position, cell.Ch, h.glyph)
				}
				wantStyle := term.StyleScrollbar
				if position >= thumbStart && position < thumbStart+thumbLength {
					wantStyle = term.StyleScrollbarThumb
				}
				if cell.Style != wantStyle {
					t.Fatalf("cell %d style = %v, want %v", position, cell.Style, wantStyle)
				}
			}

			if offset, result := h.handle(h.mouse(6, tcell.Button1)); result != EventCaptured || offset != 8 {
				t.Fatalf("centered track press = (%d,%v), want (8,captured)", offset, result)
			}
			if offset, result := h.handle(h.mouse(30, tcell.Button1)); result != EventCaptured || offset != 10 {
				t.Fatalf("outside-track held drag = (%d,%v), want (10,captured)", offset, result)
			}
			if _, result := h.handle(h.mouse(30, tcell.ButtonNone)); result != EventConsumed {
				t.Fatalf("owned release result = %v, want consumed", result)
			}
			if h.isDragging() {
				t.Fatal("owned release retained drag state")
			}
		})
	}
}

func TestScrollbarGrabOffsetPreserved(t *testing.T) {
	for _, horizontal := range []bool{false, true} {
		h := newScrollbarHarness(horizontal)
		t.Run(h.name, func(t *testing.T) {
			h.render(newTestSurface(20, 20), h.geometry, newScrollRange(10, 20, 5))
			if _, result := h.handle(h.mouse(6, tcell.Button1)); result != EventCaptured {
				t.Fatalf("thumb press result = %v, want captured", result)
			}
			if offset, result := h.handle(h.mouse(5, tcell.Button1)); result != EventCaptured || offset != 2 {
				t.Fatalf("grab-offset drag = (%d,%v), want (2,captured)", offset, result)
			}
		})
	}
}

func TestScrollbarCancelAndRangeInvalidationParity(t *testing.T) {
	for _, horizontal := range []bool{false, true} {
		h := newScrollbarHarness(horizontal)
		t.Run(h.name, func(t *testing.T) {
			surface := newTestSurface(20, 20)
			h.render(surface, h.geometry, newScrollRange(10, 20, 0))
			h.handle(h.mouse(0, tcell.Button1))
			if !h.cancel() || h.isDragging() || h.cancel() {
				t.Fatal("explicit cancellation was not stateful and idempotent")
			}

			h.handle(h.mouse(0, tcell.Button1))
			if _, invalidated := h.render(surface, h.geometry, newScrollRange(10, 40, 0)); invalidated || !h.isDragging() {
				t.Fatal("content growth canceled capture")
			}
			if _, invalidated := h.render(surface, h.geometry, newScrollRange(10, 10, 0)); !invalidated || h.isDragging() {
				t.Fatal("shrink-to-fit did not invalidate capture")
			}
		})
	}
}

func TestScrollbarOneCellTrackClampsOutsideDragToEndpoints(t *testing.T) {
	for _, horizontal := range []bool{false, true} {
		h := newScrollbarHarness(horizontal)
		t.Run(h.name, func(t *testing.T) {
			geometry := h.geometry
			if horizontal {
				geometry.localTrack.W = 1
				geometry.hitTrack.W = 1
			} else {
				geometry.localTrack.H = 1
				geometry.hitTrack.H = 1
			}
			h.geometry = geometry
			h.render(newTestSurface(20, 20), geometry, newScrollRange(1, 5, 0))
			if _, result := h.handle(h.mouse(0, tcell.Button1)); result != EventCaptured {
				t.Fatalf("one-cell press result = %v, want captured", result)
			}
			if offset, result := h.handle(h.mouse(2, tcell.Button1)); result != EventCaptured || offset != 4 {
				t.Fatalf("positive outside drag = (%d,%v), want (4,captured)", offset, result)
			}
			if offset, result := h.handle(h.mouse(-2, tcell.Button1)); result != EventCaptured || offset != 0 {
				t.Fatalf("negative outside drag = (%d,%v), want (0,captured)", offset, result)
			}
		})
	}
}

func TestReleaseInvisibleScrollbarDoesNotCapture(t *testing.T) {
	for _, horizontal := range []bool{false, true} {
		h := newScrollbarHarness(horizontal)
		t.Run(h.name, func(t *testing.T) {
			h.render(newTestSurface(20, 20), h.geometry, newScrollRange(10, 2, 0))
			if _, result := h.handle(h.mouse(0, tcell.Button1)); result != EventIgnored || h.isDragging() {
				t.Fatal("invisible scrollbar intercepted a pointer press")
			}

			h.render(newTestSurface(20, 20), h.geometry, newScrollRange(10, 20, 0))
			if _, result := h.handle(h.mouse(-1, tcell.Button1)); result != EventIgnored || h.isDragging() {
				t.Fatal("idle scrollbar captured a press outside its stored hit track")
			}
		})
	}
}
