package widgets

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

// scrollTestChild is a simple scrollable widget for testing.
type scrollTestChild struct {
	BaseWidget
	contentW int
	contentH int
}

func newScrollTestChild(w, h int) *scrollTestChild {
	return &scrollTestChild{contentW: w, contentH: h}
}

func (c *scrollTestChild) Height() int                            { return 0 }
func (c *scrollTestChild) Width() int                             { return 0 }
func (c *scrollTestChild) ScrollSize() (int, int)                 { return c.contentW, c.contentH }
func (c *scrollTestChild) HandleEvent(ev tcell.Event) EventResult { return EventIgnored }

func (c *scrollTestChild) Render(surface Surface) {
	w, h := surface.Size()
	for y := 0; y < h && y < c.contentH; y++ {
		for x := 0; x < w && x < c.contentW; x++ {
			// Fill each row with a letter based on the row number (A, B, C, ...)
			ch := rune('A' + y%26)
			surface.SetCell(x, y, term.Cell{Ch: ch, Style: term.StyleDefault})
		}
	}
}

func TestScrollViewContentSmallerThanViewport(t *testing.T) {
	child := newScrollTestChild(10, 5)
	sv := NewScrollViewWidget(child)

	// Viewport is 20x10 but content is only 10x5: no scrollbar needed
	s := renderWidget(sv, 0, 0, 20, 10)

	// Content should render at top
	if s.cells[0][0].Ch != 'A' {
		t.Errorf("expected 'A' at row 0, got %c", s.cells[0][0].Ch)
	}
	if s.cells[4][0].Ch != 'E' {
		t.Errorf("expected 'E' at row 4, got %c", s.cells[4][0].Ch)
	}

	// No scrollbar in the rightmost column (content fits)
	scrollbarStyle := s.cells[0][19].Style
	if scrollbarStyle == term.StyleScrollbar || scrollbarStyle == term.StyleScrollbarThumb {
		t.Error("should not have scrollbar when content fits in viewport")
	}
}

func TestScrollViewContentTallerThanViewport(t *testing.T) {
	child := newScrollTestChild(15, 30)
	sv := NewScrollViewWidget(child)

	// Viewport is 20x10, content is 15x30: vertical scrollbar needed
	s := renderWidget(sv, 0, 0, 20, 10)

	// Content renders at top
	if s.cells[0][0].Ch != 'A' {
		t.Errorf("expected 'A' at row 0, got %c", s.cells[0][0].Ch)
	}

	// Scrollbar should be visible on the right edge (x=19)
	hasScrollbar := false
	for y := 0; y < 10; y++ {
		style := s.cells[y][19].Style
		if style == term.StyleScrollbar || style == term.StyleScrollbarThumb {
			hasScrollbar = true
			break
		}
	}
	if !hasScrollbar {
		t.Error("should have vertical scrollbar when content is taller than viewport")
	}
}

func TestScrollViewScrollbarCaptureLifecycle(t *testing.T) {
	sv := NewScrollViewWidget(newScrollTestChild(15, 30))
	renderWidget(sv, 0, 0, 20, 10)
	invalidations := 0
	sv.SetPointerCaptureInvalidated(func() { invalidations++ })

	if got := sv.HandleEvent(tcell.NewEventMouse(19, 0, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("scrollbar press result = %v, want captured", got)
	}
	if !sv.OwnsPointerCapture() {
		t.Fatal("scroll view did not report scrollbar capture")
	}
	sv.HandleEvent(tcell.NewEventMouse(30, 30, tcell.ButtonNone, 0))
	if sv.OwnsPointerCapture() || invalidations != 0 {
		t.Fatalf("normal release retained capture or notified: invalidations=%d", invalidations)
	}

	sv.HandleEvent(tcell.NewEventMouse(19, 0, tcell.Button1, 0))
	if !sv.CancelPointerCapture() || sv.OwnsPointerCapture() {
		t.Fatal("scroll view did not cancel scrollbar capture")
	}
	if invalidations != 1 {
		t.Fatalf("capture invalidations = %d, want 1", invalidations)
	}

	sv.HandleEvent(tcell.NewEventMouse(19, 0, tcell.Button1, 0))
	if !sv.InvalidatePointerInteraction() || sv.OwnsPointerCapture() {
		t.Fatal("scroll view did not invalidate scrollbar capture")
	}
	if invalidations != 2 {
		t.Fatalf("capture invalidations = %d, want 2", invalidations)
	}
}

func TestScrollViewWheelDown(t *testing.T) {
	child := newScrollTestChild(15, 30)
	sv := NewScrollViewWidget(child)
	renderWidget(sv, 0, 0, 20, 10)

	if sv.scrollY != 0 {
		t.Fatalf("initial scrollY should be 0, got %d", sv.scrollY)
	}

	// Scroll down
	wheelDown := tcell.NewEventMouse(5, 5, tcell.WheelDown, tcell.ModNone)
	result := sv.HandleEvent(wheelDown)
	if result != EventConsumed {
		t.Error("wheel down should be consumed")
	}
	if sv.scrollY != 3 {
		t.Errorf("expected scrollY=3 after wheel down, got %d", sv.scrollY)
	}

	// Scroll down again
	sv.HandleEvent(wheelDown)
	if sv.scrollY != 6 {
		t.Errorf("expected scrollY=6 after second wheel down, got %d", sv.scrollY)
	}
}

func TestScrollViewWheelUp(t *testing.T) {
	child := newScrollTestChild(15, 30)
	sv := NewScrollViewWidget(child)
	renderWidget(sv, 0, 0, 20, 10)

	// Scroll down first
	wheelDown := tcell.NewEventMouse(5, 5, tcell.WheelDown, tcell.ModNone)
	sv.HandleEvent(wheelDown)
	sv.HandleEvent(wheelDown)
	if sv.scrollY != 6 {
		t.Fatalf("scrollY should be 6, got %d", sv.scrollY)
	}

	// Scroll up
	wheelUp := tcell.NewEventMouse(5, 5, tcell.WheelUp, tcell.ModNone)
	result := sv.HandleEvent(wheelUp)
	if result != EventConsumed {
		t.Error("wheel up should be consumed")
	}
	if sv.scrollY != 3 {
		t.Errorf("expected scrollY=3 after wheel up, got %d", sv.scrollY)
	}
}

func TestScrollViewClampsAtTop(t *testing.T) {
	child := newScrollTestChild(15, 30)
	sv := NewScrollViewWidget(child)
	renderWidget(sv, 0, 0, 20, 10)

	// Try to scroll up from 0
	wheelUp := tcell.NewEventMouse(5, 5, tcell.WheelUp, tcell.ModNone)
	sv.HandleEvent(wheelUp)
	if sv.scrollY != 0 {
		t.Errorf("scrollY should clamp at 0, got %d", sv.scrollY)
	}

	// Scroll up multiple times
	for i := 0; i < 10; i++ {
		sv.HandleEvent(wheelUp)
	}
	if sv.scrollY != 0 {
		t.Errorf("scrollY should remain 0 after many wheel ups, got %d", sv.scrollY)
	}
}

func TestScrollViewClampsAtBottom(t *testing.T) {
	child := newScrollTestChild(15, 30)
	sv := NewScrollViewWidget(child)
	renderWidget(sv, 0, 0, 20, 10)

	// Scroll down many times to exceed content
	wheelDown := tcell.NewEventMouse(5, 5, tcell.WheelDown, tcell.ModNone)
	for i := 0; i < 20; i++ {
		sv.HandleEvent(wheelDown)
	}

	// Content height = 30, viewport height = 10, max scrollY = 20
	maxScroll := 30 - 10
	if sv.scrollY > maxScroll {
		t.Errorf("scrollY should clamp at %d, got %d", maxScroll, sv.scrollY)
	}
	if sv.scrollY < 0 {
		t.Errorf("scrollY should not be negative, got %d", sv.scrollY)
	}
}

func TestScrollViewContentRendersWithScroll(t *testing.T) {
	child := newScrollTestChild(15, 30)
	sv := NewScrollViewWidget(child)

	// First render at top
	s := renderWidget(sv, 0, 0, 20, 10)
	if s.cells[0][0].Ch != 'A' {
		t.Errorf("expected 'A' at row 0 before scroll, got %c", s.cells[0][0].Ch)
	}

	// Scroll down 3 rows
	wheelDown := tcell.NewEventMouse(5, 5, tcell.WheelDown, tcell.ModNone)
	sv.HandleEvent(wheelDown)

	// Re-render after scroll
	s2 := renderWidget(sv, 0, 0, 20, 10)
	// After scrolling 3, row 0 of viewport shows content row 3 = 'D'
	if s2.cells[0][0].Ch != 'D' {
		t.Errorf("expected 'D' at row 0 after scrolling 3, got %c", s2.cells[0][0].Ch)
	}
}

func TestScrollViewFocusable(t *testing.T) {
	child := newScrollTestChild(10, 10)
	sv := NewScrollViewWidget(child)

	if !sv.Focusable() {
		t.Error("scroll view should be focusable")
	}
	sv.SetFocused(true)
	if !sv.IsFocused() {
		t.Error("scroll view should report focused after SetFocused(true)")
	}
	sv.SetFocused(false)
	if sv.IsFocused() {
		t.Error("scroll view should not report focused after SetFocused(false)")
	}
}

func TestScrollViewWidgetChildren(t *testing.T) {
	child := newScrollTestChild(10, 10)
	sv := NewScrollViewWidget(child)

	children := sv.WidgetChildren()
	if len(children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(children))
	}

	sv2 := &ScrollViewWidget{}
	children2 := sv2.WidgetChildren()
	if children2 != nil {
		t.Error("WidgetChildren should return nil when no child")
	}
}

func TestScrollViewHeightWidth(t *testing.T) {
	child := newScrollTestChild(10, 10)
	sv := NewScrollViewWidget(child)

	// ScrollView returns 0 for both (flexible sizing)
	if sv.Height() != 0 {
		t.Errorf("expected Height()=0, got %d", sv.Height())
	}
	if sv.Width() != 0 {
		t.Errorf("expected Width()=0, got %d", sv.Width())
	}
}

func TestScrollViewBoxModelApplied(t *testing.T) {
	child := newScrollTestChild(15, 30)
	sv := NewScrollViewWidget(child)
	sv.SetBoxModel(BoxModel{
		PaddingTop:  1,
		PaddingLeft: 2,
	})

	s := renderWidget(sv, 0, 0, 25, 15)

	// With padding top=1, left=2, the content should start at (2,1)
	// The first content row starts at y=1 (after padding top)
	// and at x=2 (after padding left)
	if s.cells[1][2].Ch != 'A' {
		t.Errorf("expected 'A' at (2,1) after padding, got %c", s.cells[1][2].Ch)
	}
}

func TestScrollViewEnsureVisible(t *testing.T) {
	child := newScrollTestChild(15, 30)
	sv := NewScrollViewWidget(child)
	sv.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})

	// Initially scrollY=0, viewport shows rows 0-9
	// Ensure row 15 is visible: should scroll down
	sv.EnsureVisible(0, 15)
	if sv.scrollY <= 0 {
		t.Error("EnsureVisible to row 15 should scroll down")
	}
	// Row 15 should be within [scrollY, scrollY+viewH)
	// viewH is 10 (no scrollbar on x since 15 < 20)
	if sv.scrollY > 15 {
		t.Errorf("scrollY should be <= 15 to show row 15, got %d", sv.scrollY)
	}

	// Ensure row 0 is visible: should scroll back up
	sv.EnsureVisible(0, 0)
	if sv.scrollY != 0 {
		t.Errorf("EnsureVisible to row 0 should set scrollY=0, got %d", sv.scrollY)
	}
}

func TestScrollViewNoChildRender(t *testing.T) {
	sv := &ScrollViewWidget{}

	// Should not panic when rendering with nil child
	s := renderWidget(sv, 0, 0, 20, 10)
	_ = s
}

func TestScrollViewHorizontalWheelRight(t *testing.T) {
	child := newScrollTestChild(60, 5)
	sv := NewScrollViewWidget(child)
	renderWidget(sv, 0, 0, 20, 10)

	ev := tcell.NewEventMouse(5, 5, tcell.WheelRight, tcell.ModNone)
	result := sv.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("WheelRight should be consumed")
	}
	if sv.scrollX != 3 {
		t.Errorf("expected scrollX=3 after WheelRight, got %d", sv.scrollX)
	}
}

func TestScrollViewHorizontalWheelLeft(t *testing.T) {
	child := newScrollTestChild(60, 5)
	sv := NewScrollViewWidget(child)
	sv.scrollX = 10
	renderWidget(sv, 0, 0, 20, 10)

	ev := tcell.NewEventMouse(5, 5, tcell.WheelLeft, tcell.ModNone)
	result := sv.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("WheelLeft should be consumed")
	}
	if sv.scrollX != 7 {
		t.Errorf("expected scrollX=7 after WheelLeft, got %d", sv.scrollX)
	}
}

func TestScrollViewHorizontalWheelLeftClampsAtZero(t *testing.T) {
	child := newScrollTestChild(60, 5)
	sv := NewScrollViewWidget(child)
	sv.scrollX = 1
	renderWidget(sv, 0, 0, 20, 10)

	ev := tcell.NewEventMouse(5, 5, tcell.WheelLeft, tcell.ModNone)
	sv.HandleEvent(ev)
	if sv.scrollX != 0 {
		t.Errorf("expected scrollX=0 after WheelLeft clamp, got %d", sv.scrollX)
	}
}

func TestScrollViewHorizontalWheelRightClampsAtMax(t *testing.T) {
	child := newScrollTestChild(60, 5)
	sv := NewScrollViewWidget(child)
	sv.scrollX = 38
	renderWidget(sv, 0, 0, 20, 10)

	ev := tcell.NewEventMouse(5, 5, tcell.WheelRight, tcell.ModNone)
	sv.HandleEvent(ev)
	if sv.scrollX > 40 {
		t.Errorf("scrollX should be clamped, got %d", sv.scrollX)
	}
}

func TestScrollViewHorizontalBarClick(t *testing.T) {
	child := newScrollTestChild(60, 15)
	sv := NewScrollViewWidget(child)
	renderWidget(sv, 0, 0, 20, 10)

	// hbar is at bottom row (y=9), click in the middle
	ev := tcell.NewEventMouse(10, 9, tcell.Button1, tcell.ModNone)
	result := sv.HandleEvent(ev)
	if result != EventConsumed {
		t.Error("click on horizontal scrollbar should be consumed")
	}
	if sv.scrollX == 0 {
		t.Error("clicking in middle of hbar should scroll horizontally")
	}
}

func TestScrollViewHorizontalBarRendersHalfBlock(t *testing.T) {
	child := newScrollTestChild(60, 5)
	sv := NewScrollViewWidget(child)
	s := renderWidget(sv, 0, 0, 20, 10)

	// No hbar needed since content height (5) fits in viewport (10)
	// but content width (60) > viewport width (20), so hbar at y=9
	foundHBar := false
	for x := range 20 {
		if s.cells[9][x].Ch == '▄' {
			foundHBar = true
			break
		}
	}
	if !foundHBar {
		t.Error("expected horizontal scrollbar with '▄' character")
	}
}
