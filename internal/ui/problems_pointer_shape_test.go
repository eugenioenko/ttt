package ui

import (
	"fmt"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestProblemsPointerShapePopulated(t *testing.T) {
	p := NewProblemsWidget()
	var navCalls int
	p.OnNavigate = func(file string, line, col int) {
		navCalls++
	}

	items := []ProblemItem{
		{File: "main.go", Line: 10, Col: 5, Severity: DiagError, Message: "syntax error"},
		{File: "main.go", Line: 15, Col: 8, Severity: DiagWarning, Message: "unused variable"},
		{File: "util.go", Line: 2, Col: 1, Severity: DiagInformation, Message: "hint: rename"},
	}
	p.SetItems(items)

	// Nonzero coordinates
	r := Rect{X: 10, Y: 5, W: 40, H: 8}
	p.SetRect(r)
	surface := NewRenderSurface(makeGrid(60, 20), r)
	p.Render(surface)
	initialSelected := p.tree.SelectedIndex()

	// 1. Diagnostic rows (y=5, 6, 7) should show term.PointerHand
	for i := range items {
		y := r.Y + i
		for x := r.X; x < r.X+r.W-1; x++ {
			if got := p.PointerShapeAt(x, y); got != term.PointerHand {
				t.Errorf("diagnostic row %d at (%d, %d) = %q, want %q", i, x, y, got, term.PointerHand)
			}
		}
	}

	// 2. Blank space below diagnostics (y=8..12) should return empty string (tree default)
	for y := r.Y + len(items); y < r.Y+r.H; y++ {
		if got := p.PointerShapeAt(r.X+5, y); got != "" {
			t.Errorf("blank space at y=%d = %q, want empty", y, got)
		}
	}

	// 3. Points outside the panel rectangle should return empty string
	outsidePoints := [][2]int{
		{r.X - 1, r.Y + 1},
		{r.X + r.W, r.Y + 1},
		{r.X + 5, r.Y - 1},
		{r.X + 5, r.Y + r.H},
	}
	for _, pt := range outsidePoints {
		if got := p.PointerShapeAt(pt[0], pt[1]); got != "" {
			t.Errorf("outside point %v = %q, want empty", pt, got)
		}
	}

	// 4. Verify shape queries did not change selection or invoke navigation callback
	if navCalls != 0 {
		t.Fatalf("PointerShapeAt triggered %d navigation callbacks", navCalls)
	}
	if p.tree.SelectedIndex() != initialSelected {
		t.Fatalf("PointerShapeAt changed selection: got %d, want %d", p.tree.SelectedIndex(), initialSelected)
	}
}

func TestProblemsPointerShapeEmpty(t *testing.T) {
	p := NewProblemsWidget()
	p.SetItems(nil)

	r := Rect{X: 5, Y: 3, W: 30, H: 6}
	p.SetRect(r)
	surface := NewRenderSurface(makeGrid(50, 15), r)
	p.Render(surface)

	// In empty state ("No problems detected"), no row is actionable
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			if got := p.PointerShapeAt(x, y); got != "" {
				t.Errorf("empty problems at (%d, %d) = %q, want empty", x, y, got)
			}
		}
	}
}

func TestProblemsPointerShapeScrollbar(t *testing.T) {
	p := NewProblemsWidget()
	items := make([]ProblemItem, 20)
	for i := range items {
		items[i] = ProblemItem{
			File:    "test.go",
			Line:    i + 1,
			Col:     1,
			Message: fmt.Sprintf("problem %d", i+1),
		}
	}
	p.SetItems(items)

	r := Rect{X: 10, Y: 5, W: 30, H: 6}
	p.SetRect(r)
	surface := NewRenderSurface(makeGrid(50, 15), r)
	p.Render(surface)

	// Scrollbar track is on the rightmost column of the tree
	barX := r.X + r.W - 1
	for y := r.Y; y < r.Y+r.H; y++ {
		if got := p.PointerShapeAt(barX, y); got != "" {
			t.Errorf("scrollbar track at (%d, %d) = %q, want empty", barX, y, got)
		}
	}
}

func TestProblemsPointerShapeComposedBottomPanelAndOverlay(t *testing.T) {
	p := NewProblemsWidget()
	items := []ProblemItem{
		{File: "alpha.go", Line: 1, Col: 1, Message: "error alpha"},
		{File: "beta.go", Line: 2, Col: 1, Message: "error beta"},
	}
	p.SetItems(items)

	bp := NewBottomPanelWidget(nil)
	bp.AddPanel("problems", "Problems", p)
	bp.SetActivePanel("problems")

	r := Rect{X: 0, Y: 0, W: 80, H: 10}
	bp.SetRect(r)
	root := NewRoot(bp)

	bp.Render(NewRenderSurface(makeGrid(80, 10), r))

	// In bottom panel, tab header is row 0, divider is row 1, content starts at row 2.
	diagY := r.Y + 2
	if got := root.PointerShapeAt(10, diagY); got != term.PointerHand {
		t.Fatalf("root.PointerShapeAt over diagnostic row = %q, want %q", got, term.PointerHand)
	}

	// Row 6 is blank content below items:
	if got := root.PointerShapeAt(10, r.Y+6); got != term.PointerDefault {
		t.Fatalf("root.PointerShapeAt over blank bottom panel = %q, want %q", got, term.PointerDefault)
	}

	// Push a covering top overlay
	root.PushOverlay(Overlay{Widget: &shapeWidget{shape: term.PointerText}, Modal: true})
	if got := root.PointerShapeAt(10, diagY); got != term.PointerText {
		t.Fatalf("root.PointerShapeAt with covering overlay = %q, want %q", got, term.PointerText)
	}

	// Pop overlay
	root.PopOverlay()
	if got := root.PointerShapeAt(10, diagY); got != term.PointerHand {
		t.Fatalf("root.PointerShapeAt after pop = %q, want %q", got, term.PointerHand)
	}
}

func TestOutputWidgetPointerShapeRemainsDefault(t *testing.T) {
	out := NewOutputWidget()
	out.AddLine(OutputLine{Level: "INFO", Message: "log message 1"})
	out.AddLine(OutputLine{Level: "WARN", Message: "log message 2"})

	bp := NewBottomPanelWidget(nil)
	bp.AddPanel("output", "Output", out)
	bp.SetActivePanel("output")

	r := Rect{X: 0, Y: 0, W: 80, H: 10}
	bp.SetRect(r)
	root := NewRoot(bp)

	bp.Render(NewRenderSurface(makeGrid(80, 10), r))

	// Output logs are non-interactive; hovering them must remain term.PointerDefault
	logY := r.Y + 2
	if got := root.PointerShapeAt(10, logY); got != term.PointerDefault {
		t.Fatalf("root.PointerShapeAt over Output row = %q, want %q", got, term.PointerDefault)
	}
}
