package widgets

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

// clickableWidget is a test helper that consumes mouse clicks inside its rect.
// This allows testing position-based event routing through containers.
type clickableWidget struct {
	fixedWidget
	clicked bool
}

func (c *clickableWidget) HandleEvent(ev tcell.Event) EventResult {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		if e.Buttons()&tcell.Button1 != 0 {
			mx, my := e.Position()
			r := c.GetRect()
			if mx >= r.X && mx < r.X+r.W && my >= r.Y && my < r.Y+r.H {
				c.clicked = true
				return EventConsumed
			}
		}
	}
	return EventIgnored
}

// --- VStackWidget rendering verification ---

func TestVStackRenderChildrenAtCorrectYOffsets(t *testing.T) {
	a := NewLabelWidget(LabelConfig{Text: "AAA"})
	b := NewLabelWidget(LabelConfig{Text: "BBB"})
	c := NewLabelWidget(LabelConfig{Text: "CCC"})
	vs := NewVStackWidget(a, b, c)

	s := renderWidget(vs, 0, 0, 20, 10)

	// Label A at Y=0
	if s.cells[0][0].Ch != 'A' {
		t.Errorf("expected 'A' at (0,0), got %q", s.cells[0][0].Ch)
	}
	// Label B at Y=1
	if s.cells[1][0].Ch != 'B' {
		t.Errorf("expected 'B' at (0,1), got %q", s.cells[1][0].Ch)
	}
	// Label C at Y=2
	if s.cells[2][0].Ch != 'C' {
		t.Errorf("expected 'C' at (0,2), got %q", s.cells[2][0].Ch)
	}
}

func TestVStackRenderWithGapSpacing(t *testing.T) {
	a := NewLabelWidget(LabelConfig{Text: "AAA"})
	b := NewLabelWidget(LabelConfig{Text: "BBB"})
	vs := NewVStackWidget(a, b)
	vs.Gap = 2

	s := renderWidget(vs, 0, 0, 20, 10)

	// Label A at Y=0
	if s.cells[0][0].Ch != 'A' {
		t.Errorf("expected 'A' at (0,0), got %q", s.cells[0][0].Ch)
	}
	// Gap rows Y=1 and Y=2 should be empty
	if s.cells[1][0].Ch != 0 {
		t.Errorf("expected empty cell at (0,1) in gap, got %q", s.cells[1][0].Ch)
	}
	if s.cells[2][0].Ch != 0 {
		t.Errorf("expected empty cell at (0,2) in gap, got %q", s.cells[2][0].Ch)
	}
	// Label B at Y=3 (1 height + 2 gap)
	if s.cells[3][0].Ch != 'B' {
		t.Errorf("expected 'B' at (0,3), got %q", s.cells[3][0].Ch)
	}
}

func TestVStackChildGetsFullWidth(t *testing.T) {
	label := NewLabelWidget(LabelConfig{Text: "Hello World This Is A Test"})
	vs := NewVStackWidget(label)

	renderWidget(vs, 0, 0, 30, 10)

	r := label.GetRect()
	if r.W != 30 {
		t.Errorf("child should get full container width 30, got W=%d", r.W)
	}
}

func TestVStackChildHeightsRespected(t *testing.T) {
	// Labels have Height() == 1
	a := NewLabelWidget(LabelConfig{Text: "A"})
	b := NewLabelWidget(LabelConfig{Text: "B"})
	vs := NewVStackWidget(a, b)

	if vs.Height() != 2 {
		t.Errorf("expected VStack Height()=2 (two labels), got %d", vs.Height())
	}

	renderWidget(vs, 0, 0, 20, 10)

	ra := a.GetRect()
	rb := b.GetRect()
	if ra.H != 1 {
		t.Errorf("label A should get H=1, got H=%d", ra.H)
	}
	if rb.H != 1 {
		t.Errorf("label B should get H=1, got H=%d", rb.H)
	}
	if rb.Y != 1 {
		t.Errorf("label B should start at Y=1, got Y=%d", rb.Y)
	}
}

func TestVStackMouseClickRoutesToCorrectChild(t *testing.T) {
	a := &clickableWidget{fixedWidget: fixedWidget{h: 3, w: 0}}
	b := &clickableWidget{fixedWidget: fixedWidget{h: 3, w: 0}}
	c := &clickableWidget{fixedWidget: fixedWidget{h: 3, w: 0}}
	vs := NewVStackWidget(a, b, c)

	renderWidget(vs, 5, 10, 20, 9)

	// Click in child A's area (Y: 10..12)
	click := mouseClick(10, 11)
	result := vs.HandleEvent(click)
	if result != EventConsumed {
		t.Error("click inside child A should be consumed")
	}
	if !a.clicked {
		t.Error("child A should have received the click")
	}
	if b.clicked || c.clicked {
		t.Error("children B and C should not have received the click")
	}

	// Reset
	a.clicked, b.clicked, c.clicked = false, false, false

	// Click in child B's area (Y: 13..15)
	click = mouseClick(10, 14)
	result = vs.HandleEvent(click)
	if result != EventConsumed {
		t.Error("click inside child B should be consumed")
	}
	if !b.clicked {
		t.Error("child B should have received the click")
	}
	if a.clicked || c.clicked {
		t.Error("children A and C should not have received the click")
	}

	// Reset
	a.clicked, b.clicked, c.clicked = false, false, false

	// Click in child C's area (Y: 16..18)
	click = mouseClick(10, 17)
	result = vs.HandleEvent(click)
	if result != EventConsumed {
		t.Error("click inside child C should be consumed")
	}
	if !c.clicked {
		t.Error("child C should have received the click")
	}
}

func TestVStackMouseClickOutsideAllChildren(t *testing.T) {
	a := &clickableWidget{fixedWidget: fixedWidget{h: 3, w: 0}}
	vs := NewVStackWidget(a)

	renderWidget(vs, 5, 10, 20, 10)

	// Click outside the container entirely
	click := mouseClick(0, 0)
	result := vs.HandleEvent(click)
	if result == EventConsumed {
		t.Error("click outside all children should not be consumed")
	}
	if a.clicked {
		t.Error("child should not have been clicked")
	}
}

func TestVStackEmptyRendersNothing(t *testing.T) {
	vs := NewVStackWidget()
	s := renderWidget(vs, 0, 0, 20, 10)

	// All cells should be empty (zero rune)
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			if s.cells[y][x].Ch != 0 {
				t.Errorf("empty vstack should render nothing, but found %q at (%d,%d)", s.cells[y][x].Ch, x, y)
			}
		}
	}
}

func TestVStackHeightForWidthAllFixed(t *testing.T) {
	a := &fixedWidget{h: 3, w: 10}
	b := &fixedWidget{h: 5, w: 10}
	vs := NewVStackWidget(a, b)

	hfw := vs.HeightForWidth(20)
	if hfw != 8 {
		t.Errorf("HeightForWidth should be 8 (3+5) for fixed children, got %d", hfw)
	}
}

func TestVStackHeightForWidthWithGap(t *testing.T) {
	a := &fixedWidget{h: 3, w: 10}
	b := &fixedWidget{h: 5, w: 10}
	c := &fixedWidget{h: 2, w: 10}
	vs := NewVStackWidget(a, b, c)
	vs.Gap = 2

	hfw := vs.HeightForWidth(20)
	// 3 + 5 + 2 = 10 fixed, plus 2 gaps * 2 = 4, total = 14
	if hfw != 14 {
		t.Errorf("HeightForWidth should be 14, got %d", hfw)
	}
}

func TestVStackHeightForWidthWithHeightForWidthChild(t *testing.T) {
	// ParagraphWidget implements HeightForWidth
	para := NewParagraphWidget("Hello world, this is a longer text that should wrap")
	label := NewLabelWidget(LabelConfig{Text: "Title"})
	vs := NewVStackWidget(label, para)

	hfw := vs.HeightForWidth(10)
	// label: 1 line
	// paragraph wraps at width 10 — HeightForWidth returns wrapped line count
	paraHFW := para.HeightForWidth(10)
	expected := 1 + paraHFW
	if hfw != expected {
		t.Errorf("HeightForWidth(10) should be %d (1 label + %d para), got %d", expected, paraHFW, hfw)
	}
}

func TestVStackHeightForWidthZeroWhenGrowChildNoHFW(t *testing.T) {
	a := &fixedWidget{h: 3, w: 10}
	b := &fixedWidget{h: 0, w: 10} // grow child, no HeightForWidth
	vs := NewVStackWidget(a, b)

	hfw := vs.HeightForWidth(20)
	if hfw != 0 {
		t.Errorf("HeightForWidth should be 0 when grow child has no HeightForWidth, got %d", hfw)
	}
}

func TestVStackScrollSize(t *testing.T) {
	a := &fixedWidget{h: 3, w: 10}
	b := &fixedWidget{h: 5, w: 10}
	vs := NewVStackWidget(a, b)
	vs.SetRect(Rect{X: 0, Y: 0, W: 30, H: 20})

	sw, sh := vs.ScrollSize()
	if sw != 30 {
		t.Errorf("ScrollSize width should be rect width 30, got %d", sw)
	}
	if sh != 8 {
		t.Errorf("ScrollSize height should be HeightForWidth(30)=8, got %d", sh)
	}
}

func TestVStackBoxModelApplied(t *testing.T) {
	label := NewLabelWidget(LabelConfig{Text: "Hello"})
	vs := NewVStackWidget(label)
	vs.Box = BoxModel{
		MarginTop:   1,
		MarginLeft:  2,
		PaddingTop:  1,
		PaddingLeft: 1,
	}

	s := renderWidget(vs, 0, 0, 20, 10)

	// With margin_top=1, padding_top=1, the inner content starts at Y=2
	// With margin_left=2, padding_left=1, the inner content starts at X=3
	if s.cells[2][3].Ch != 'H' {
		t.Errorf("expected 'H' at (3,2) with box model offsets, got %q", s.cells[2][3].Ch)
	}
	// Original position should be empty
	if s.cells[0][0].Ch != 0 {
		t.Errorf("expected empty at (0,0) due to margins, got %q", s.cells[0][0].Ch)
	}
}

func TestVStackBoxModelInHeight(t *testing.T) {
	a := &fixedWidget{h: 3, w: 10}
	b := &fixedWidget{h: 5, w: 10}
	vs := NewVStackWidget(a, b)
	vs.Box = BoxModel{
		MarginTop:     1,
		MarginBottom:  1,
		PaddingTop:    2,
		PaddingBottom: 2,
	}

	// Height = children (3+5) + BoxOverheadH (1+1+2+2 = 6) = 14
	if got := vs.Height(); got != 14 {
		t.Errorf("expected Height()=14 (8 children + 6 box overhead), got %d", got)
	}
}

func TestVStackRenderWithThreeGapChildren(t *testing.T) {
	a := NewLabelWidget(LabelConfig{Text: "First"})
	b := NewLabelWidget(LabelConfig{Text: "Second"})
	c := NewLabelWidget(LabelConfig{Text: "Third"})
	vs := NewVStackWidget(a, b, c)
	vs.Gap = 1

	s := renderWidget(vs, 0, 0, 20, 10)

	// A at Y=0, gap at Y=1, B at Y=2, gap at Y=3, C at Y=4
	if s.cells[0][0].Ch != 'F' {
		t.Errorf("expected 'F' at (0,0), got %q", s.cells[0][0].Ch)
	}
	if s.cells[1][0].Ch != 0 {
		t.Errorf("expected empty at (0,1) in gap, got %q", s.cells[1][0].Ch)
	}
	if s.cells[2][0].Ch != 'S' {
		t.Errorf("expected 'S' at (0,2), got %q", s.cells[2][0].Ch)
	}
	if s.cells[3][0].Ch != 0 {
		t.Errorf("expected empty at (0,3) in gap, got %q", s.cells[3][0].Ch)
	}
	if s.cells[4][0].Ch != 'T' {
		t.Errorf("expected 'T' at (0,4), got %q", s.cells[4][0].Ch)
	}
}

func TestVStackWidgetChildren(t *testing.T) {
	a := NewLabelWidget(LabelConfig{Text: "A"})
	b := NewLabelWidget(LabelConfig{Text: "B"})
	vs := NewVStackWidget(a, b)

	children := vs.WidgetChildren()
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
}

func TestVStackWidthReturnsZero(t *testing.T) {
	vs := NewVStackWidget()
	if vs.Width() != 0 {
		t.Errorf("VStack Width() should always return 0, got %d", vs.Width())
	}
}

func TestVStackRenderChildrenWithStyle(t *testing.T) {
	a := NewLabelWidget(LabelConfig{Text: "Styled", Style: term.StyleWarning})
	vs := NewVStackWidget(a)

	s := renderWidget(vs, 0, 0, 20, 5)

	if s.cells[0][0].Style != term.StyleWarning {
		t.Errorf("expected StyleWarning, got %v", s.cells[0][0].Style)
	}
}

func TestVStackEmptyHandleEventReturnsIgnored(t *testing.T) {
	vs := NewVStackWidget()
	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	result := vs.HandleEvent(ev)
	if result != EventIgnored {
		t.Error("empty VStack should return EventIgnored")
	}
}

type testCaptureOwnerWidget struct {
	fixedWidget
	owns      bool
	lastEvent tcell.Event
}

func (c *testCaptureOwnerWidget) OwnsPointerCapture() bool { return c.owns }
func (c *testCaptureOwnerWidget) HandleEvent(ev tcell.Event) EventResult {
	c.lastEvent = ev
	return EventConsumed
}

func TestVStackHandleEventRoutesToCaptureOwnerFirst(t *testing.T) {
	first := &testCaptureOwnerWidget{owns: false}
	second := &testCaptureOwnerWidget{owns: true}
	vs := NewVStackWidget(first, second)

	ev := tcell.NewEventMouse(5, 5, tcell.Button1, 0)
	res := vs.HandleEvent(ev)
	if res != EventConsumed {
		t.Fatalf("res = %v, want EventConsumed", res)
	}
	if second.lastEvent != ev {
		t.Fatal("expected second (capture owner) to receive event")
	}
	if first.lastEvent != nil {
		t.Fatal("expected first to not receive event while second owns capture")
	}

	first.lastEvent = nil
	second.lastEvent = nil

	keyEv := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	res = vs.HandleEvent(keyEv)
	if res != EventConsumed {
		t.Fatalf("key event res = %v, want EventConsumed", res)
	}
	if first.lastEvent != keyEv {
		t.Fatal("expected first child to receive key event during normal routing")
	}
	if second.lastEvent != nil {
		t.Fatal("expected second child to not receive key event after first consumed it")
	}
}
