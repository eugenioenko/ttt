package ui

import (
	"fmt"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type shapeWidget struct {
	mockWidget
	shape string
}

func (s *shapeWidget) PointerShapeAt(mx, my int) string { return s.shape }

func TestRootPointerShapeFallsBackToDefault(t *testing.T) {
	root := NewRoot(&mockWidget{})
	if got := root.PointerShapeAt(1, 1); got != term.PointerDefault {
		t.Fatalf("shape = %q, want %q", got, term.PointerDefault)
	}
}

func TestRootPointerShapeOnlyAsksTopOverlay(t *testing.T) {
	root := NewRoot(&shapeWidget{shape: term.PointerText})
	if got := root.PointerShapeAt(1, 1); got != term.PointerText {
		t.Fatalf("no overlay = %q, want %q", got, term.PointerText)
	}
	root.PushOverlay(Overlay{Widget: &shapeWidget{shape: term.PointerHand}, Modal: true})
	if got := root.PointerShapeAt(1, 1); got != term.PointerHand {
		t.Fatalf("shaped overlay = %q, want %q", got, term.PointerHand)
	}
	root.PushOverlay(Overlay{Widget: &mockWidget{}, Modal: false})
	if got := root.PointerShapeAt(1, 1); got != term.PointerDefault {
		t.Fatalf("shapeless top overlay = %q, want %q", got, term.PointerDefault)
	}
}

func TestSplitPanelPointerShapeRoutesAroundDivider(t *testing.T) {
	split, _, _ := newResizeTestSplit()
	split.Left = &shapeWidget{shape: term.PointerHand}
	split.Right = &shapeWidget{shape: term.PointerText}
	divX := split.DividerScreenX()

	for _, tt := range []struct {
		name string
		x    int
		want string
	}{
		{"left", divX - 1, term.PointerHand},
		{"divider", divX, term.PointerEWResize},
		{"right", divX + 1, term.PointerText},
	} {
		if got := split.PointerShapeAt(tt.x, 3); got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, got, tt.want)
		}
	}

	split.HandleEvent(tcell.NewEventMouse(divX, 3, tcell.Button1, 0))
	if got := split.PointerShapeAt(divX+10, 3); got != term.PointerEWResize {
		t.Errorf("dragging away from the divider = %q, want %q", got, term.PointerEWResize)
	}
}

func TestContentSplitPointerShapeRoutesAroundDivider(t *testing.T) {
	cs := NewContentSplitWidget()
	cs.Top = &shapeWidget{shape: term.PointerText}
	bottom := &shapeWidget{shape: term.PointerHand}
	bottom.SetRect(Rect{X: 0, Y: 21, W: 80, H: 9})
	cs.Bottom = bottom
	cs.ShowBottom = true
	cs.BottomH = 9
	cs.SetRect(Rect{W: 80, H: 30})
	divY := cs.DividerScreenY()

	for _, tt := range []struct {
		name string
		y    int
		want string
	}{
		{"top", divY - 1, term.PointerText},
		{"divider", divY, term.PointerNSResize},
		{"bottom", 25, term.PointerHand},
	} {
		if got := cs.PointerShapeAt(10, tt.y); got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, got, tt.want)
		}
	}

	cs.ShowBottom = false
	if got := cs.PointerShapeAt(10, 25); got != term.PointerText {
		t.Errorf("hidden panel's stale rect = %q, want %q", got, term.PointerText)
	}
}

func TestTabBarPointerShape(t *testing.T) {
	tb := NewTabBarWidget()
	tb.SetTabs([]Tab{{Name: "one.go", Active: true}, {Name: "two.go"}})
	tb.OnTabReorder = func(from, to int) {}
	tb.SetRect(Rect{X: 4, Y: 2, W: 50, H: 3})
	tb.Render(NewRenderSurface(makeGrid(50, 3), Rect{W: 50, H: 3}))

	tabX := tb.GetRect().X + tb.tabSpans[1].start + 1
	if got := tb.PointerShapeAt(tabX, 3); got != term.PointerHand {
		t.Fatalf("over a tab = %q, want %q", got, term.PointerHand)
	}
	if got := tb.PointerShapeAt(50, 3); got != "" {
		t.Fatalf("empty bar = %q, want none", got)
	}

	tb.HandleEvent(tcell.NewEventMouse(tabX, 3, tcell.Button1, 0))
	tb.HandleEvent(tcell.NewEventMouse(tabX+5, 3, tcell.Button1, 0))
	if got := tb.PointerShapeAt(tabX+5, 20); got != term.PointerGrabbing {
		t.Fatalf("dragging outside the bar = %q, want %q", got, term.PointerGrabbing)
	}
	tb.HandleEvent(tcell.NewEventMouse(tabX+5, 3, tcell.ButtonNone, 0))
	if got := tb.PointerShapeAt(50, 3); got != "" {
		t.Fatalf("after release = %q, want none", got)
	}
}

func TestTabBarArrowsOnlyShowHandWithHiddenTabs(t *testing.T) {
	tb := NewTabBarWidget()
	tb.renderArrowW = 2
	tb.renderInnerRight = 18
	tb.SetRect(Rect{W: 20, H: 3})

	if got := tb.PointerShapeAt(0, 1); got != term.PointerDefault {
		t.Errorf("left arrow with nothing hidden = %q, want %q", got, term.PointerDefault)
	}
	tb.hasOverflowLeft = true
	if got := tb.PointerShapeAt(0, 1); got != term.PointerHand {
		t.Errorf("left arrow with hidden tabs = %q, want %q", got, term.PointerHand)
	}
	if got := tb.PointerShapeAt(19, 1); got != term.PointerDefault {
		t.Errorf("right arrow with nothing hidden = %q, want %q", got, term.PointerDefault)
	}
	tb.hasOverflowRight = true
	if got := tb.PointerShapeAt(19, 1); got != term.PointerHand {
		t.Errorf("right arrow with hidden tabs = %q, want %q", got, term.PointerHand)
	}
}

func TestEditorPointerShape(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	e := newRowsEditor(lines, 40, 10)
	e.LineNumbers = true
	e.SetRect(Rect{X: 3, Y: 2, W: 40, H: 10})
	e.Render(NewRenderSurface(makeGrid(50, 20), Rect{X: 3, Y: 2, W: 40, H: 10}))
	if !e.scrollbar.Visible() {
		t.Fatal("test setup: scrollbar should be visible")
	}
	textX := 3 + e.GutterWidth() + 1

	for _, tt := range []struct {
		name string
		x, y int
		want string
	}{
		{"text", textX, 4, term.PointerText},
		{"gutter", 3, 4, term.PointerDefault},
		{"scrollbar", e.scrollbar.X, e.scrollbar.Y, term.PointerDefault},
		{"outside", 45, 4, ""},
	} {
		if got := e.PointerShapeAt(tt.x, tt.y); got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, got, tt.want)
		}
	}

	e.HandleEvent(tcell.NewEventMouse(textX, 4, tcell.Button1, 0))
	if got := e.PointerShapeAt(45, 4); got != term.PointerText {
		t.Errorf("selecting past the edge = %q, want %q", got, term.PointerText)
	}
	e.HandleEvent(tcell.NewEventMouse(textX, 4, tcell.ButtonNone, 0))

	e.HandleEvent(tcell.NewEventMouse(e.scrollbar.X, e.scrollbar.Y, tcell.Button1, 0))
	if got := e.PointerShapeAt(textX, 4); got != term.PointerDefault {
		t.Errorf("scrollbar drag over text = %q, want %q", got, term.PointerDefault)
	}
}

func TestEditorPointerShapeOnPhantomRowIsDefault(t *testing.T) {
	e := newRowsEditor(numberedLines(10), 20, 5)
	e.setPhantoms(map[int]int{2: 2})
	renderRows(e)
	x := e.GutterWidth() + 1

	if got := e.PointerShapeAt(x, 2); got != term.PointerDefault {
		t.Errorf("phantom row = %q, want %q", got, term.PointerDefault)
	}
	if got := e.PointerShapeAt(x, 4); got != term.PointerText {
		t.Errorf("buffer row = %q, want %q", got, term.PointerText)
	}
}

func TestDiffPointerShapeShowsHandOnGap(t *testing.T) {
	old, cur := gapFile()
	f := newLiveDiffFixture(t, old, cur, DiffModeUnified, 100, 20)
	rows := f.render()
	gapY := rowWith(rows, "⋯ 12 lines ⋯")

	if got := f.d.PointerShapeAt(20, gapY); got != term.PointerHand {
		t.Fatalf("gap row = %q, want %q", got, term.PointerHand)
	}
}
