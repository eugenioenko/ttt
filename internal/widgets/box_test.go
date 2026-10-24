package widgets

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestBoxRenderWithBorder(t *testing.T) {
	borders := term.SingleBorderSet()
	box := NewBoxWithBorder(borders)
	box.Child = NewLabelWidget(LabelConfig{Text: "Hi"})

	// Label height=1, box overhead = border top(1) + border bottom(1) = 2, total = 3
	s := renderWidget(box, 0, 0, 10, 3)

	// Top-left corner
	if s.cells[0][0].Ch != borders.TopLeft {
		t.Errorf("expected top-left corner %c, got %c", borders.TopLeft, s.cells[0][0].Ch)
	}
	// Top-right corner
	if s.cells[0][9].Ch != borders.TopRight {
		t.Errorf("expected top-right corner %c, got %c", borders.TopRight, s.cells[0][9].Ch)
	}
	// Bottom-left corner
	if s.cells[2][0].Ch != borders.BottomLeft {
		t.Errorf("expected bottom-left corner %c, got %c", borders.BottomLeft, s.cells[2][0].Ch)
	}
	// Bottom-right corner
	if s.cells[2][9].Ch != borders.BottomRight {
		t.Errorf("expected bottom-right corner %c, got %c", borders.BottomRight, s.cells[2][9].Ch)
	}
	// Top horizontal border
	if s.cells[0][1].Ch != borders.Horizontal {
		t.Errorf("expected horizontal border %c at top, got %c", borders.Horizontal, s.cells[0][1].Ch)
	}
	// Bottom horizontal border
	if s.cells[2][1].Ch != borders.Horizontal {
		t.Errorf("expected horizontal border %c at bottom, got %c", borders.Horizontal, s.cells[2][1].Ch)
	}
	// Left vertical border
	if s.cells[1][0].Ch != borders.Vertical {
		t.Errorf("expected vertical border %c on left, got %c", borders.Vertical, s.cells[1][0].Ch)
	}
	// Right vertical border
	if s.cells[1][9].Ch != borders.Vertical {
		t.Errorf("expected vertical border %c on right, got %c", borders.Vertical, s.cells[1][9].Ch)
	}
}

func TestBoxRenderWithoutBorder(t *testing.T) {
	box := NewBoxWidget(BoxModel{})
	box.Child = NewLabelWidget(LabelConfig{Text: "No border"})

	s := renderWidget(box, 0, 0, 20, 1)

	// No border characters should appear
	borders := term.SingleBorderSet()
	for x := range 20 {
		ch := s.cells[0][x].Ch
		if ch == borders.TopLeft || ch == borders.TopRight ||
			ch == borders.BottomLeft || ch == borders.BottomRight ||
			ch == borders.Horizontal || ch == borders.Vertical {
			t.Errorf("unexpected border character %c at x=%d", ch, x)
		}
	}
	// The label text should render directly
	if s.cells[0][0].Ch != 'N' {
		t.Errorf("expected 'N' at x=0, got %c", s.cells[0][0].Ch)
	}
}

func TestBoxFixedHeight(t *testing.T) {
	box := NewBoxWidget(BoxModel{})
	box.FixedHeight = 5
	box.Child = NewLabelWidget(LabelConfig{Text: "Fixed"})

	h := box.Height()
	if h != 5 {
		t.Errorf("expected fixed height=5, got %d", h)
	}
}

func TestBoxFixedWidth(t *testing.T) {
	box := NewBoxWidget(BoxModel{})
	box.FixedWidth = 15
	box.Child = NewLabelWidget(LabelConfig{Text: "Fixed"})

	w := box.Width()
	if w != 15 {
		t.Errorf("expected fixed width=15, got %d", w)
	}
}

func TestBoxHeightFromChild(t *testing.T) {
	borders := term.SingleBorderSet()
	box := NewBoxWithBorder(borders)
	label := NewLabelWidget(LabelConfig{Text: "Hi"})
	box.Child = label

	// Label height = 1, box overhead = 2 (top + bottom border)
	h := box.Height()
	expected := 1 + 2
	if h != expected {
		t.Errorf("expected height=%d, got %d", expected, h)
	}
}

func TestBoxChildRendersInsideBorder(t *testing.T) {
	borders := term.SingleBorderSet()
	box := NewBoxWithBorder(borders)
	box.Child = NewLabelWidget(LabelConfig{Text: "AB"})

	s := renderWidget(box, 0, 0, 10, 3)

	// Child content should appear inside the border (row 1, starting at col 1)
	if s.cells[1][1].Ch != 'A' {
		t.Errorf("expected 'A' inside border at (1,1), got %c", s.cells[1][1].Ch)
	}
	if s.cells[1][2].Ch != 'B' {
		t.Errorf("expected 'B' inside border at (2,1), got %c", s.cells[1][2].Ch)
	}
}

func TestBoxChildRendersInsidePadding(t *testing.T) {
	box := NewBoxWithPadding(1)
	box.Child = NewLabelWidget(LabelConfig{Text: "XY"})

	// Padding 1 on all sides, label height=1, overhead=2 (top+bottom pad), total h=3
	s := renderWidget(box, 0, 0, 10, 3)

	// Child content should appear at (1,1) due to padding
	if s.cells[1][1].Ch != 'X' {
		t.Errorf("expected 'X' inside padding at (1,1), got %c", s.cells[1][1].Ch)
	}
	if s.cells[1][2].Ch != 'Y' {
		t.Errorf("expected 'Y' inside padding at (2,1), got %c", s.cells[1][2].Ch)
	}
}

func TestBoxChildRendersInsideBorderAndPadding(t *testing.T) {
	borders := term.SingleBorderSet()
	box := NewBoxWithBorderAndPadding(borders, 1)
	box.Child = NewLabelWidget(LabelConfig{Text: "Z"})

	// Border(1) + padding(1) on each side: overhead = 4 vertical, label height=1, total=5
	s := renderWidget(box, 0, 0, 10, 5)

	// Border at row 0 and 4, padding at row 1 and 3, content at row 2
	if s.cells[0][0].Ch != borders.TopLeft {
		t.Errorf("expected border top-left, got %c", s.cells[0][0].Ch)
	}
	// Content at (2,2): border(1) + padding(1) = offset 2
	if s.cells[2][2].Ch != 'Z' {
		t.Errorf("expected 'Z' at (2,2), got %c", s.cells[2][2].Ch)
	}
}

func TestBoxNestedBox(t *testing.T) {
	borders := term.SingleBorderSet()
	outer := NewBoxWithBorder(borders)
	inner := NewBoxWithBorder(borders)
	inner.Child = NewLabelWidget(LabelConfig{Text: "N"})
	outer.Child = inner

	// outer border: 2, inner border: 2, label: 1 = 5
	s := renderWidget(outer, 0, 0, 12, 5)

	// Outer border at (0,0)
	if s.cells[0][0].Ch != borders.TopLeft {
		t.Errorf("expected outer top-left corner, got %c", s.cells[0][0].Ch)
	}
	// Inner border at (1,1)
	if s.cells[1][1].Ch != borders.TopLeft {
		t.Errorf("expected inner top-left corner at (1,1), got %c", s.cells[1][1].Ch)
	}
	// Content "N" at (2,2)
	if s.cells[2][2].Ch != 'N' {
		t.Errorf("expected 'N' at (2,2), got %c", s.cells[2][2].Ch)
	}
}

func TestBoxMargins(t *testing.T) {
	box := NewBoxWidget(BoxModel{
		MarginTop:    1,
		MarginLeft:   2,
		BorderTop:    true,
		BorderBottom: true,
		BorderLeft:   true,
		BorderRight:  true,
		Borders:      term.SingleBorderSet(),
	})
	box.Child = NewLabelWidget(LabelConfig{Text: "M"})

	s := renderWidget(box, 0, 0, 20, 5)

	// Margin top=1, margin left=2, so border starts at (2,1)
	borders := term.SingleBorderSet()
	if s.cells[1][2].Ch != borders.TopLeft {
		t.Errorf("expected top-left corner at (2,1) after margins, got %c", s.cells[1][2].Ch)
	}
	// Content at (3,2) -- margin left(2) + border(1) = 3, margin top(1) + border(1) = 2
	if s.cells[2][3].Ch != 'M' {
		t.Errorf("expected 'M' at (3,2), got %c", s.cells[2][3].Ch)
	}
}

func TestBoxHeightWithOverhead(t *testing.T) {
	box := NewBoxWidget(BoxModel{
		MarginTop:     1,
		MarginBottom:  1,
		PaddingTop:    1,
		PaddingBottom: 1,
		BorderTop:     true,
		BorderBottom:  true,
		BorderLeft:    true,
		BorderRight:   true,
		Borders:       term.SingleBorderSet(),
	})
	box.Child = NewLabelWidget(LabelConfig{Text: "Test"})

	// Label height=1, overhead = margins(2) + padding(2) + borders(2) = 6, total = 7
	h := box.Height()
	if h != 7 {
		t.Errorf("expected height=7, got %d", h)
	}
}

func TestBoxWidthFromChild(t *testing.T) {
	borders := term.SingleBorderSet()
	box := NewBoxWithBorder(borders)
	label := NewLabelWidget(LabelConfig{Text: "Test"})
	label.FixedWidth = 10
	box.Child = label

	// Label width=10, box overhead = border left(1) + border right(1) = 2
	w := box.Width()
	expected := 10 + 2
	if w != expected {
		t.Errorf("expected width=%d, got %d", expected, w)
	}
}

func TestBoxNilChild(t *testing.T) {
	box := NewBoxWithBorder(term.SingleBorderSet())
	// No child set

	// Should not panic
	s := renderWidget(box, 0, 0, 10, 3)
	_ = s

	// Height with nil child should be 0
	if box.Height() != 0 {
		t.Errorf("expected height=0 with nil child, got %d", box.Height())
	}
}

func TestBoxHandleEventDelegatesToChild(t *testing.T) {
	clicked := false
	btn := NewButtonWidget(ButtonConfig{
		Label:   "Inside",
		OnClick: func() { clicked = true },
	})

	box := NewBoxWithBorder(term.SingleBorderSet())
	box.Child = btn

	// Render to set rects
	renderWidget(box, 5, 5, 20, 3)

	// Press and release inside the button (which is inside the box).
	r := btn.GetRect()
	result := box.HandleEvent(mousePress(r.X+1, r.Y))
	if result != EventCaptured {
		t.Errorf("box press result = %v, want captured child", result)
	}
	if clicked {
		t.Fatal("child button ran before mouse-up")
	}
	box.HandleEvent(mouseRelease(r.X+1, r.Y))
	if !clicked {
		t.Error("child button OnClick should run on mouse-up")
	}
}

func TestBoxHandleEventNilChild(t *testing.T) {
	box := NewBoxWidget(BoxModel{})
	// No child

	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	result := box.HandleEvent(ev)
	if result != EventIgnored {
		t.Error("box with nil child should return EventIgnored")
	}
}

func TestBoxWidgetChildren(t *testing.T) {
	box := NewBoxWidget(BoxModel{})

	// No child
	children := box.WidgetChildren()
	if children != nil {
		t.Error("WidgetChildren should return nil when no child")
	}

	// With child
	label := NewLabelWidget(LabelConfig{Text: "child"})
	box.Child = label
	children = box.WidgetChildren()
	if len(children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(children))
	}
	if children[0] != label {
		t.Error("WidgetChildren should return the child widget")
	}
}

func TestBoxDoubleBorder(t *testing.T) {
	borders := term.DoubleBorderSet()
	box := NewBoxWithBorder(borders)
	box.Child = NewLabelWidget(LabelConfig{Text: "D"})

	s := renderWidget(box, 0, 0, 10, 3)

	if s.cells[0][0].Ch != borders.TopLeft {
		t.Errorf("expected double top-left %c, got %c", borders.TopLeft, s.cells[0][0].Ch)
	}
	if s.cells[0][5].Ch != borders.Horizontal {
		t.Errorf("expected double horizontal %c, got %c", borders.Horizontal, s.cells[0][5].Ch)
	}
}
