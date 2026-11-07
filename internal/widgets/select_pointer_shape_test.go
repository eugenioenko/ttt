package widgets

import (
	"fmt"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestSelectPointerShapeCollapsibleClosed(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items:       makeSelectItems("Option A", "Option B", "Option C"),
		Collapsible: true,
	})
	sw.SetRect(Rect{X: 10, Y: 5, W: 20, H: 1})
	sw.Render(newVirtualSurface(20, 1))

	// Control row should show hand pointer across input and chevron
	for x := 10; x < 30; x++ {
		if got := sw.PointerShapeAt(x, 5); got != term.PointerHand {
			t.Errorf("closed collapsible at x=%d, y=5 got %q, want %q", x, got, term.PointerHand)
		}
	}

	// Outside control bounds should return empty
	outsidePoints := [][2]int{
		{9, 5}, {30, 5}, {10, 4}, {10, 6},
	}
	for _, pt := range outsidePoints {
		if got := sw.PointerShapeAt(pt[0], pt[1]); got != "" {
			t.Errorf("closed collapsible outside at %v got %q, want empty", pt, got)
		}
	}
}

func TestSelectPointerShapeCollapsibleOpen(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items:       makeSelectItems("Alpha", "Beta", "Gamma"),
		Collapsible: true,
	})
	sw.SetRect(Rect{X: 10, Y: 5, W: 20, H: 1})
	sw.setOpen(true)
	sw.Render(newVirtualSurface(20, 1))

	pr := sw.PopupRect()
	sw.RenderPopup(newVirtualSurface(pr.W, pr.H))

	// 1. Control row: editable filter input should be PointerText
	inputR := sw.input.GetRect()
	for x := inputR.X; x < inputR.X+inputR.W; x++ {
		if got := sw.PointerShapeAt(x, inputR.Y); got != term.PointerText {
			t.Errorf("open filter input at x=%d, y=%d got %q, want %q", x, inputR.Y, got, term.PointerText)
		}
	}

	// Chevron/control area outside editable filter should be PointerHand
	for x := inputR.X + inputR.W; x < 30; x++ {
		if got := sw.PointerShapeAt(x, inputR.Y); got != term.PointerHand {
			t.Errorf("open chevron area at x=%d, y=%d got %q, want %q", x, inputR.Y, got, term.PointerHand)
		}
	}

	// 2. Inside popup:
	// Borders should return PointerDefault
	borderPoints := [][2]int{
		{pr.X, pr.Y},                       // top-left
		{pr.X + 5, pr.Y},                   // top edge
		{pr.X + pr.W - 1, pr.Y},            // top-right
		{pr.X, pr.Y + pr.H - 1},            // bottom-left
		{pr.X + 5, pr.Y + pr.H - 1},        // bottom edge
		{pr.X + pr.W - 1, pr.Y + pr.H - 1}, // bottom-right
		{pr.X, pr.Y + 1},                   // left border
		{pr.X + pr.W - 1, pr.Y + 1},        // right border
	}
	for _, pt := range borderPoints {
		if got := sw.PointerShapeAt(pt[0], pt[1]); got != term.PointerDefault {
			t.Errorf("popup border at %v got %q, want %q", pt, got, term.PointerDefault)
		}
	}

	// Filtered options (3 items, rows pr.Y+1, pr.Y+2, pr.Y+3) should return PointerHand
	for row := 0; row < 3; row++ {
		y := pr.Y + 1 + row
		if got := sw.PointerShapeAt(pr.X+5, y); got != term.PointerHand {
			t.Errorf("popup option row %d at y=%d got %q, want %q", row, y, got, term.PointerHand)
		}
	}

	// Outside popup and control should return empty
	if got := sw.PointerShapeAt(pr.X-1, pr.Y+1); got != "" {
		t.Errorf("outside popup left got %q, want empty", got)
	}
	if got := sw.PointerShapeAt(pr.X+pr.W, pr.Y+1); got != "" {
		t.Errorf("outside popup right got %q, want empty", got)
	}
}

func TestSelectPointerShapeEmptyResultRowsInPopup(t *testing.T) {
	// 1 item with popup height capacity of at least 1 item + borders
	sw := NewSelectWidget(SelectConfig{
		Items:       makeSelectItems("OnlyOne"),
		Collapsible: true,
	})
	sw.SetRect(Rect{X: 10, Y: 5, W: 20, H: 1})
	sw.SetFocused(true)
	sw.setOpen(true)
	// Filter to something that produces 0 results
	sendRune(sw, 'z')
	if len(sw.filtered) != 0 {
		t.Fatalf("expected 0 filtered items, got %d", len(sw.filtered))
	}
	sw.Render(newVirtualSurface(20, 1))

	pr := sw.PopupRect()
	sw.RenderPopup(newVirtualSurface(pr.W, pr.H))

	// Because filtered list is empty, any row inside popup should return PointerDefault (empty-result space)
	for y := pr.Y + 1; y < pr.Y+pr.H-1; y++ {
		for x := pr.X + 1; x < pr.X+pr.W-1; x++ {
			if got := sw.PointerShapeAt(x, y); got != term.PointerDefault {
				t.Errorf("empty result row at x=%d, y=%d got %q, want %q", x, y, got, term.PointerDefault)
			}
		}
	}
}

func TestSelectPointerShapeScrolledPopupAndScrollbar(t *testing.T) {
	labels := make([]string, 15)
	for i := range labels {
		labels[i] = string(rune('A' + i))
	}
	sw := NewSelectWidget(SelectConfig{
		Items:       makeSelectItems(labels...),
		Collapsible: true,
	})
	sw.SetRect(Rect{X: 5, Y: 2, W: 20, H: 1})
	sw.setOpen(true)
	sw.Render(newVirtualSurface(20, 1))

	pr := sw.PopupRect()
	sw.RenderPopup(newVirtualSurface(pr.W, pr.H))

	// Scrollbar track should be at column pr.X + pr.W - 2
	barX := pr.X + pr.W - 2
	for y := pr.Y + 1; y < pr.Y+pr.H-1; y++ {
		if got := sw.PointerShapeAt(barX, y); got != term.PointerDefault {
			t.Errorf("scrollbar track hover at x=%d, y=%d got %q, want %q", barX, y, got, term.PointerDefault)
		}
	}

	// Options next to scrollbar should be PointerHand
	for y := pr.Y + 1; y < pr.Y+pr.H-1; y++ {
		if got := sw.PointerShapeAt(pr.X+2, y); got != term.PointerHand {
			t.Errorf("option next to scrollbar at y=%d got %q, want %q", y, got, term.PointerHand)
		}
	}

	// Now start dragging scrollbar
	sw.HandleEvent(tcell.NewEventMouse(barX, pr.Y+1, tcell.Button1, 0))
	if !sw.scrollbar.isDragging() {
		t.Fatal("expected scrollbar to be dragging")
	}

	// While dragging, everywhere returns PointerDefault (even over options or outside widget)
	dragPoints := [][2]int{
		{pr.X + 2, pr.Y + 1}, // over option
		{10, 2},              // over input
		{100, 100},           // far outside
		{-5, -5},             // negative
	}
	for _, pt := range dragPoints {
		if got := sw.PointerShapeAt(pt[0], pt[1]); got != term.PointerDefault {
			t.Errorf("during scrollbar drag at %v got %q, want %q", pt, got, term.PointerDefault)
		}
	}

	// Release drag
	sw.HandleEvent(tcell.NewEventMouse(barX, pr.Y+1, tcell.ButtonNone, 0))
	if sw.scrollbar.isDragging() {
		t.Fatal("expected scrollbar drag to be ended")
	}

	// Releasing restores the shape appropriate to the point
	if got := sw.PointerShapeAt(pr.X+2, pr.Y+1); got != term.PointerHand {
		t.Errorf("after release over option got %q, want %q", got, term.PointerHand)
	}
}

func TestSelectPointerShapePopupPositionedAbove(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items:       makeSelectItems("Item 1", "Item 2", "Item 3"),
		Collapsible: true,
	})
	// Place select near bottom of bounds (Y=20, bounds H=25) so popup opens above
	sw.SetRect(Rect{X: 10, Y: 20, W: 20, H: 1})
	sw.SetPopupBounds(Rect{X: 0, Y: 0, W: 50, H: 22})
	sw.setOpen(true)
	sw.Render(newVirtualSurface(20, 1))

	pr := sw.PopupRect()
	if pr.Y >= 20 {
		t.Fatalf("expected popup to be positioned above Y=20, got pr.Y=%d", pr.Y)
	}
	sw.RenderPopup(newVirtualSurface(pr.W, pr.H))

	// Option in above popup should be PointerHand
	if got := sw.PointerShapeAt(pr.X+5, pr.Y+1); got != term.PointerHand {
		t.Errorf("above popup option got %q, want %q", got, term.PointerHand)
	}

	// Border of above popup should be PointerDefault
	if got := sw.PointerShapeAt(pr.X, pr.Y); got != term.PointerDefault {
		t.Errorf("above popup top-left border got %q, want %q", got, term.PointerDefault)
	}
}

func TestSelectPointerShapeInlineListWithDividers(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items:       makeSelectItems("Alpha", "Beta", "Gamma"),
		ShowDivider: true,
		Collapsible: false,
	})
	sw.SetRect(Rect{X: 5, Y: 10, W: 25, H: 8})
	sw.Render(newVirtualSurface(25, 8))

	// Top divider row (Y=10)
	if got := sw.PointerShapeAt(10, 10); got != term.PointerDefault {
		t.Errorf("top divider row got %q, want %q", got, term.PointerDefault)
	}

	// Input row (Y=11): input area is PointerText
	inputR := sw.input.GetRect()
	for x := inputR.X; x < inputR.X+inputR.W; x++ {
		if got := sw.PointerShapeAt(x, 11); got != term.PointerText {
			t.Errorf("inline input at x=%d, y=11 got %q, want %q", x, got, term.PointerText)
		}
	}

	// Chevron on input row (decorative, no click action) is PointerDefault
	for x := inputR.X + inputR.W; x < 30; x++ {
		if got := sw.PointerShapeAt(x, 11); got != term.PointerDefault {
			t.Errorf("decorative chevron at x=%d, y=11 got %q, want %q", x, got, term.PointerDefault)
		}
	}

	// Bottom divider row (Y=12)
	if got := sw.PointerShapeAt(10, 12); got != term.PointerDefault {
		t.Errorf("bottom divider row got %q, want %q", got, term.PointerDefault)
	}

	// Option rows: Alpha at Y=13, Beta at Y=14, Gamma at Y=15
	for i, y := range []int{13, 14, 15} {
		if got := sw.PointerShapeAt(10, y); got != term.PointerHand {
			t.Errorf("inline option %d at y=%d got %q, want %q", i, y, got, term.PointerHand)
		}
	}

	// Empty rows below options (Y=16, 17) should be PointerDefault
	for y := 16; y < 18; y++ {
		if got := sw.PointerShapeAt(10, y); got != term.PointerDefault {
			t.Errorf("empty inline row at y=%d got %q, want %q", y, got, term.PointerDefault)
		}
	}

	// Outside inline list bounds
	if got := sw.PointerShapeAt(4, 13); got != "" {
		t.Errorf("outside inline left got %q, want empty", got)
	}
	if got := sw.PointerShapeAt(30, 13); got != "" {
		t.Errorf("outside inline right got %q, want empty", got)
	}
}

func TestSelectPointerShapeClosingPopupRemovesTargetsAndNoSideEffects(t *testing.T) {
	var (
		onSelectFired  bool
		onChangeFired  bool
		onDismissFired bool
		onOpenFired    bool
	)
	sw := NewSelectWidget(SelectConfig{
		Items:       makeSelectItems("Item 1", "Item 2"),
		Collapsible: true,
		OnSelect:    func(string) { onSelectFired = true },
		OnChange:    func(string) { onChangeFired = true },
		OnDismiss:   func() { onDismissFired = true },
		OnOpen:      func() { onOpenFired = true },
	})
	sw.SetRect(Rect{X: 10, Y: 5, W: 20, H: 1})
	sw.setOpen(true)
	onOpenFired = false // reset after manual setOpen
	sw.Render(newVirtualSurface(20, 1))

	pr := sw.PopupRect()
	sw.RenderPopup(newVirtualSurface(pr.W, pr.H))

	optX := pr.X + 2
	optY := pr.Y + 1

	// While open, option target returns PointerHand
	if got := sw.PointerShapeAt(optX, optY); got != term.PointerHand {
		t.Fatalf("open popup option got %q, want %q", got, term.PointerHand)
	}

	// Verify pointer query had no side effects
	if onSelectFired || onChangeFired || onDismissFired || onOpenFired {
		t.Fatalf("PointerShapeAt triggered callback: select=%v change=%v dismiss=%v open=%v",
			onSelectFired, onChangeFired, onDismissFired, onOpenFired)
	}
	if sw.selected != 0 {
		t.Fatalf("PointerShapeAt altered selection: got %d, want 0", sw.selected)
	}

	// Close popup
	sw.ClosePopup()

	// Once closed, querying former popup coordinates must return empty (no stale targets)
	if got := sw.PointerShapeAt(optX, optY); got != "" {
		t.Fatalf("closed popup old target got %q, want empty", got)
	}
}

func TestSelectPointerShapeFilteredPopupRows(t *testing.T) {
	sw := NewSelectWidget(SelectConfig{
		Items:       makeSelectItems("Apple", "Apricot", "Banana", "Cherry"),
		Collapsible: true,
	})
	sw.SetRect(Rect{X: 10, Y: 5, W: 20, H: 1})
	sw.SetFocused(true)
	sw.setOpen(true)
	sendRune(sw, 'a')
	sendRune(sw, 'p')
	if len(sw.filtered) != 2 {
		t.Fatalf("expected 2 filtered items, got %d", len(sw.filtered))
	}
	sw.Render(newVirtualSurface(20, 1))

	pr := sw.PopupRect()
	sw.RenderPopup(newVirtualSurface(pr.W, pr.H))

	// Both filtered rows are hand pointers
	for row := 0; row < 2; row++ {
		y := pr.Y + 1 + row
		for x := pr.X + 1; x < pr.X+pr.W-1; x++ {
			if got := sw.PointerShapeAt(x, y); got != term.PointerHand {
				t.Errorf("filtered row %d at x=%d, y=%d got %q, want %q", row, x, y, got, term.PointerHand)
			}
		}
	}

	// Popup borders stay default even around a filtered subset
	borderPoints := [][2]int{
		{pr.X, pr.Y},
		{pr.X + 3, pr.Y},
		{pr.X + pr.W - 1, pr.Y},
		{pr.X, pr.Y + 1},
		{pr.X + pr.W - 1, pr.Y + 1},
		{pr.X, pr.Y + pr.H - 1},
		{pr.X + 3, pr.Y + pr.H - 1},
		{pr.X + pr.W - 1, pr.Y + pr.H - 1},
	}
	for _, pt := range borderPoints {
		if got := sw.PointerShapeAt(pt[0], pt[1]); got != term.PointerDefault {
			t.Errorf("filtered popup border at %v got %q, want %q", pt, got, term.PointerDefault)
		}
	}

	// Filter input stays an editable text pointer while a subset is shown
	inputR := sw.input.GetRect()
	if got := sw.PointerShapeAt(inputR.X, inputR.Y); got != term.PointerText {
		t.Errorf("filter input while filtered got %q, want %q", got, term.PointerText)
	}

	// Outside control and popup stays empty
	if got := sw.PointerShapeAt(pr.X-1, pr.Y+1); got != "" {
		t.Errorf("outside filtered popup got %q, want empty", got)
	}
}

func TestSelectPointerShapeScrolledFilteredRows(t *testing.T) {
	labels := []string{"Zebra"}
	for i := 1; i <= 15; i++ {
		labels = append(labels, fmt.Sprintf("App %02d", i))
	}
	sw := NewSelectWidget(SelectConfig{
		Items:       makeSelectItems(labels...),
		Collapsible: true,
	})
	sw.SetRect(Rect{X: 5, Y: 2, W: 20, H: 1})
	sw.SetFocused(true)
	sw.setOpen(true)
	sendRune(sw, 'p')
	if len(sw.filtered) != 15 {
		t.Fatalf("expected 15 filtered items, got %d", len(sw.filtered))
	}

	// Advance selection past the first popup page so the offset becomes nonzero
	for i := 0; i < 9; i++ {
		sendKey(sw, tcell.KeyDown)
	}
	if sw.scrollTop == 0 {
		t.Fatal("expected nonzero scroll offset after navigating down")
	}

	sw.Render(newVirtualSurface(20, 1))
	pr := sw.PopupRect()
	sw.RenderPopup(newVirtualSurface(pr.W, pr.H))

	// Every visible option row (between the borders) is a hand pointer at a nonzero offset
	for y := pr.Y + 1; y < pr.Y+pr.H-1; y++ {
		if got := sw.PointerShapeAt(pr.X+2, y); got != term.PointerHand {
			t.Errorf("scrolled filtered option at y=%d (offset=%d) got %q, want %q", y, sw.scrollTop, got, term.PointerHand)
		}
	}

	// Scrollbar track and borders stay default at the same offset
	barX := pr.X + pr.W - 2
	for _, pt := range [][2]int{
		{barX, pr.Y + 1},
		{barX, pr.Y + pr.H - 2},
		{pr.X, pr.Y},
		{pr.X + pr.W - 1, pr.Y},
		{pr.X, pr.Y + pr.H - 1},
	} {
		if got := sw.PointerShapeAt(pt[0], pt[1]); got != term.PointerDefault {
			t.Errorf("scrolled popup chrome at %v got %q, want %q", pt, got, term.PointerDefault)
		}
	}
}

func TestSelectPointerShapeInlineFilteredEmptyRows(t *testing.T) {
	// Inline with dividers: rows start at rect.Y+3
	sw := NewSelectWidget(SelectConfig{
		Items:       makeSelectItems("Alpha", "Beta", "Gamma", "Delta"),
		ShowDivider: true,
		Collapsible: false,
	})
	sw.SetRect(Rect{X: 5, Y: 10, W: 25, H: 8})
	sw.SetFocused(true)
	sendRune(sw, 'g')
	if len(sw.filtered) != 1 {
		t.Fatalf("expected 1 filtered item, got %d", len(sw.filtered))
	}
	sw.Render(newVirtualSurface(25, 8))

	// The remaining filtered row is a hand pointer
	if got := sw.PointerShapeAt(10, 13); got != term.PointerHand {
		t.Errorf("inline filtered option row got %q, want %q", got, term.PointerHand)
	}

	// Rows below the filtered subset keep default pointers (list retains more space than items)
	for y := 14; y < 18; y++ {
		if got := sw.PointerShapeAt(10, y); got != term.PointerDefault {
			t.Errorf("inline empty row after filtering at y=%d got %q, want %q", y, got, term.PointerDefault)
		}
	}

	// Input row stays an editable text pointer with the filter query in it
	inputR := sw.input.GetRect()
	if got := sw.PointerShapeAt(inputR.X, 11); got != term.PointerText {
		t.Errorf("inline filter input got %q, want %q", got, term.PointerText)
	}

	// Inline without dividers: rows start at rect.Y+1
	sw2 := NewSelectWidget(SelectConfig{
		Items:       makeSelectItems("Alpha", "Beta", "Gamma", "Delta"),
		Collapsible: false,
	})
	sw2.SetRect(Rect{X: 5, Y: 10, W: 25, H: 6})
	sw2.SetFocused(true)
	sendRune(sw2, 'b')
	if len(sw2.filtered) != 1 {
		t.Fatalf("expected 1 filtered item, got %d", len(sw2.filtered))
	}
	sw2.Render(newVirtualSurface(25, 6))

	if got := sw2.PointerShapeAt(10, 11); got != term.PointerHand {
		t.Errorf("dividerless inline filtered option row got %q, want %q", got, term.PointerHand)
	}
	for y := 12; y < 16; y++ {
		if got := sw2.PointerShapeAt(10, y); got != term.PointerDefault {
			t.Errorf("dividerless inline empty row at y=%d got %q, want %q", y, got, term.PointerDefault)
		}
	}
}
