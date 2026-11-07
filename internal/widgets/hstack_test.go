package widgets

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

// --- HStackWidget rendering verification ---

func TestHStackRenderFirstChildGrowsRemainingFillsWidth(t *testing.T) {
	// First child grows (Width()==0), second child has fixed width
	grow := NewLabelWidget(LabelConfig{Text: "GROW"})
	fixed := NewLabelWidget(LabelConfig{Text: "FIX"})
	fixed.FixedWidth = 5

	hs := NewHStackWidget(grow, fixed)
	s := renderWidget(hs, 0, 0, 20, 5)

	// Grow child text at X=0
	if s.cells[0][0].Ch != 'G' {
		t.Errorf("expected 'G' at (0,0), got %q", s.cells[0][0].Ch)
	}

	rg := grow.GetRect()
	rf := fixed.GetRect()

	// Grow child should fill remaining: 20 - 5 = 15
	if rg.W != 15 {
		t.Errorf("grow child should have W=15, got W=%d", rg.W)
	}
	// Fixed child at X=15
	if rf.X != 15 || rf.W != 5 {
		t.Errorf("fixed child should have X=15 W=5, got X=%d W=%d", rf.X, rf.W)
	}

	// Fixed child text at X=15
	if s.cells[0][15].Ch != 'F' {
		t.Errorf("expected 'F' at (15,0), got %q", s.cells[0][15].Ch)
	}
}

func TestHStackRenderGapSpacing(t *testing.T) {
	a := NewLabelWidget(LabelConfig{Text: "AA"})
	a.FixedWidth = 3
	b := NewLabelWidget(LabelConfig{Text: "BB"})
	b.FixedWidth = 3
	hs := NewHStackWidget(a, b)
	hs.Gap = 2

	s := renderWidget(hs, 0, 0, 20, 5)

	// A at X=0
	if s.cells[0][0].Ch != 'A' {
		t.Errorf("expected 'A' at (0,0), got %q", s.cells[0][0].Ch)
	}
	// Gap at X=3 and X=4 should be empty
	if s.cells[0][3].Ch != 0 {
		t.Errorf("expected empty at (3,0) in gap, got %q", s.cells[0][3].Ch)
	}
	if s.cells[0][4].Ch != 0 {
		t.Errorf("expected empty at (4,0) in gap, got %q", s.cells[0][4].Ch)
	}
	// B at X=5 (3 width + 2 gap)
	if s.cells[0][5].Ch != 'B' {
		t.Errorf("expected 'B' at (5,0), got %q", s.cells[0][5].Ch)
	}
}

func TestHStackFixedHeightRespected(t *testing.T) {
	hs := NewHStackWidget()
	hs.FixedHeight = 7

	if hs.Height() != 7 {
		t.Errorf("expected FixedHeight=7, got %d", hs.Height())
	}
}

func TestHStackHeightZeroWithoutFixedHeight(t *testing.T) {
	hs := NewHStackWidget()
	if hs.Height() != 0 {
		t.Errorf("expected Height()=0 without FixedHeight, got %d", hs.Height())
	}
}

func TestHStackMouseClickRoutesToCorrectChild(t *testing.T) {
	a := &clickableWidget{fixedWidget: fixedWidget{h: 0, w: 5}}
	b := &clickableWidget{fixedWidget: fixedWidget{h: 0, w: 5}}
	c := &clickableWidget{fixedWidget: fixedWidget{h: 0, w: 5}}
	hs := NewHStackWidget(a, b, c)

	renderWidget(hs, 10, 5, 15, 10)

	// Click in child A's area (X: 10..14)
	click := mouseClick(12, 7)
	result := hs.HandleEvent(click)
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

	// Click in child B's area (X: 15..19)
	click = mouseClick(17, 7)
	result = hs.HandleEvent(click)
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

	// Click in child C's area (X: 20..24)
	click = mouseClick(22, 7)
	result = hs.HandleEvent(click)
	if result != EventConsumed {
		t.Error("click inside child C should be consumed")
	}
	if !c.clicked {
		t.Error("child C should have received the click")
	}
}

func TestHStackMouseClickOutsideAllChildren(t *testing.T) {
	a := &clickableWidget{fixedWidget: fixedWidget{h: 0, w: 5}}
	hs := NewHStackWidget(a)

	renderWidget(hs, 10, 10, 5, 5)

	// Click outside the container
	click := mouseClick(0, 0)
	result := hs.HandleEvent(click)
	if result == EventConsumed {
		t.Error("click outside all children should not be consumed")
	}
	if a.clicked {
		t.Error("child should not have been clicked")
	}
}

func TestHStackEmptyRendersNothing(t *testing.T) {
	hs := NewHStackWidget()
	s := renderWidget(hs, 0, 0, 20, 10)

	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			if s.cells[y][x].Ch != 0 {
				t.Errorf("empty hstack should render nothing, but found %q at (%d,%d)", s.cells[y][x].Ch, x, y)
			}
		}
	}
}

func TestHStackSingleChildFillsEntireWidth(t *testing.T) {
	child := NewLabelWidget(LabelConfig{Text: "Only child"})
	hs := NewHStackWidget(child)

	renderWidget(hs, 0, 0, 30, 5)

	r := child.GetRect()
	if r.W != 30 {
		t.Errorf("single grow child should fill entire width 30, got W=%d", r.W)
	}
	if r.X != 0 {
		t.Errorf("single child should start at X=0, got X=%d", r.X)
	}
}

func TestHStackSingleFixedChildWidth(t *testing.T) {
	child := NewLabelWidget(LabelConfig{Text: "Fixed"})
	child.FixedWidth = 8
	hs := NewHStackWidget(child)

	renderWidget(hs, 0, 0, 30, 5)

	r := child.GetRect()
	if r.W != 8 {
		t.Errorf("single fixed child should keep its width 8, got W=%d", r.W)
	}
}

func TestHStackMultipleFixedWidthChildrenPositioned(t *testing.T) {
	a := NewLabelWidget(LabelConfig{Text: "AA"})
	a.FixedWidth = 4
	b := NewLabelWidget(LabelConfig{Text: "BB"})
	b.FixedWidth = 6
	c := NewLabelWidget(LabelConfig{Text: "CC"})
	c.FixedWidth = 3
	hs := NewHStackWidget(a, b, c)

	s := renderWidget(hs, 0, 0, 30, 5)

	ra := a.GetRect()
	rb := b.GetRect()
	rc := c.GetRect()

	if ra.X != 0 || ra.W != 4 {
		t.Errorf("child A: expected X=0 W=4, got X=%d W=%d", ra.X, ra.W)
	}
	if rb.X != 4 || rb.W != 6 {
		t.Errorf("child B: expected X=4 W=6, got X=%d W=%d", rb.X, rb.W)
	}
	if rc.X != 10 || rc.W != 3 {
		t.Errorf("child C: expected X=10 W=3, got X=%d W=%d", rc.X, rc.W)
	}

	// Verify rendered text
	if s.cells[0][0].Ch != 'A' {
		t.Errorf("expected 'A' at (0,0), got %q", s.cells[0][0].Ch)
	}
	if s.cells[0][4].Ch != 'B' {
		t.Errorf("expected 'B' at (4,0), got %q", s.cells[0][4].Ch)
	}
	if s.cells[0][10].Ch != 'C' {
		t.Errorf("expected 'C' at (10,0), got %q", s.cells[0][10].Ch)
	}
}

func TestHStackChildrenGetFullHeight(t *testing.T) {
	a := NewLabelWidget(LabelConfig{Text: "A"})
	a.FixedWidth = 5
	b := NewLabelWidget(LabelConfig{Text: "B"})
	b.FixedWidth = 5
	hs := NewHStackWidget(a, b)

	renderWidget(hs, 0, 0, 20, 15)

	ra := a.GetRect()
	rb := b.GetRect()

	if ra.H != 15 {
		t.Errorf("child A should get full height 15, got H=%d", ra.H)
	}
	if rb.H != 15 {
		t.Errorf("child B should get full height 15, got H=%d", rb.H)
	}
}

func TestHStackWidgetChildren(t *testing.T) {
	a := NewLabelWidget(LabelConfig{Text: "A"})
	b := NewLabelWidget(LabelConfig{Text: "B"})
	hs := NewHStackWidget(a, b)

	children := hs.WidgetChildren()
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
}

func TestHStackWidthReturnsZero(t *testing.T) {
	hs := NewHStackWidget()
	if hs.Width() != 0 {
		t.Errorf("HStack Width() should always return 0, got %d", hs.Width())
	}
}

func TestHStackEmptyHandleEventReturnsIgnored(t *testing.T) {
	hs := NewHStackWidget()
	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	result := hs.HandleEvent(ev)
	if result != EventIgnored {
		t.Error("empty HStack should return EventIgnored")
	}
}

func TestHStackRenderChildrenWithStyle(t *testing.T) {
	a := NewLabelWidget(LabelConfig{Text: "Warn", Style: term.StyleWarning})
	a.FixedWidth = 6
	b := NewLabelWidget(LabelConfig{Text: "Danger", Style: term.StyleDanger})
	b.FixedWidth = 8
	hs := NewHStackWidget(a, b)

	s := renderWidget(hs, 0, 0, 20, 5)

	if s.cells[0][0].Style != term.StyleWarning {
		t.Errorf("expected StyleWarning at (0,0), got %v", s.cells[0][0].Style)
	}
	if s.cells[0][6].Style != term.StyleDanger {
		t.Errorf("expected StyleDanger at (6,0), got %v", s.cells[0][6].Style)
	}
}

func TestHStackGrowChildWithMultipleFixed(t *testing.T) {
	// Pattern: fixed(5) | grow | fixed(5) — grow should fill the middle
	left := NewLabelWidget(LabelConfig{Text: "LEFT"})
	left.FixedWidth = 5
	middle := NewLabelWidget(LabelConfig{Text: "MIDDLE"})
	right := NewLabelWidget(LabelConfig{Text: "RIGHT"})
	right.FixedWidth = 5
	hs := NewHStackWidget(left, middle, right)

	s := renderWidget(hs, 0, 0, 30, 5)

	rl := left.GetRect()
	rm := middle.GetRect()
	rr := right.GetRect()

	if rl.X != 0 || rl.W != 5 {
		t.Errorf("left: expected X=0 W=5, got X=%d W=%d", rl.X, rl.W)
	}
	// Middle grows: 30 - 5 - 5 = 20
	if rm.X != 5 || rm.W != 20 {
		t.Errorf("middle (grow): expected X=5 W=20, got X=%d W=%d", rm.X, rm.W)
	}
	if rr.X != 25 || rr.W != 5 {
		t.Errorf("right: expected X=25 W=5, got X=%d W=%d", rr.X, rr.W)
	}

	// Verify rendered text positions
	if s.cells[0][0].Ch != 'L' {
		t.Errorf("expected 'L' at (0,0), got %q", s.cells[0][0].Ch)
	}
	if s.cells[0][5].Ch != 'M' {
		t.Errorf("expected 'M' at (5,0), got %q", s.cells[0][5].Ch)
	}
	if s.cells[0][25].Ch != 'R' {
		t.Errorf("expected 'R' at (25,0), got %q", s.cells[0][25].Ch)
	}
}

func TestHStackGapWithGrowChild(t *testing.T) {
	a := NewLabelWidget(LabelConfig{Text: "A"})
	a.FixedWidth = 3
	b := NewLabelWidget(LabelConfig{Text: "B"})
	// b is grow (Width()==0)
	hs := NewHStackWidget(a, b)
	hs.Gap = 2

	renderWidget(hs, 0, 0, 20, 5)

	ra := a.GetRect()
	rb := b.GetRect()

	if ra.X != 0 || ra.W != 3 {
		t.Errorf("child A: expected X=0 W=3, got X=%d W=%d", ra.X, ra.W)
	}
	// B starts at 3 + 2 gap = 5, fills remaining: 20 - 3 - 2 = 15
	if rb.X != 5 || rb.W != 15 {
		t.Errorf("child B: expected X=5 W=15, got X=%d W=%d", rb.X, rb.W)
	}
}

func TestHStackBoxModelApplied(t *testing.T) {
	label := NewLabelWidget(LabelConfig{Text: "Hello"})
	label.FixedWidth = 10
	hs := NewHStackWidget(label)
	hs.Box = BoxModel{
		MarginTop:   1,
		MarginLeft:  2,
		PaddingTop:  1,
		PaddingLeft: 1,
	}

	s := renderWidget(hs, 0, 0, 20, 10)

	// With margin_top=1, padding_top=1, the inner content starts at Y=2
	// With margin_left=2, padding_left=1, the inner content starts at X=3
	if s.cells[2][3].Ch != 'H' {
		t.Errorf("expected 'H' at (3,2) with box model offsets, got %q", s.cells[2][3].Ch)
	}
	if s.cells[0][0].Ch != 0 {
		t.Errorf("expected empty at (0,0) due to margins, got %q", s.cells[0][0].Ch)
	}
}

func TestHStackFixedHeightIncludesMargins(t *testing.T) {
	h := NewHStackWidget(NewLabelWidget(LabelConfig{Text: "x"}))
	h.FixedHeight = 1
	h.Box.MarginTop = 1
	if got := h.Height(); got != 2 {
		t.Fatalf("height = %d, want the row plus its top margin", got)
	}
}
