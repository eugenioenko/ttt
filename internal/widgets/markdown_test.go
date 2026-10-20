package widgets

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/markdown"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func mkLine(text string) markdown.Line {
	return markdown.Line{Spans: []markdown.Span{{Text: text, Style: term.StyleDefault}}}
}

func TestMarkdownInSelection(t *testing.T) {
	m := NewMarkdownWidget()
	m.sel.Start(1, 2)
	m.selEnd = selection.Position{Line: 3, Col: 4}

	tests := []struct {
		line, col int
		want      bool
	}{
		{0, 5, false},
		{1, 1, false},
		{1, 2, true},
		{1, 5, true},
		{2, 0, true},
		{2, 10, true},
		{3, 0, true},
		{3, 3, true},
		{3, 4, false},
		{4, 0, false},
	}
	for _, tt := range tests {
		got := m.sel.Contains(tt.line, tt.col, m.selEnd.Line, m.selEnd.Col)
		if got != tt.want {
			t.Errorf("Contains(%d,%d) = %v, want %v", tt.line, tt.col, got, tt.want)
		}
	}
}

func TestMarkdownSelectedTextSingleLine(t *testing.T) {
	m := NewMarkdownWidget()
	m.wrapped = []markdown.Line{
		mkLine("Hello World"),
		mkLine("Second line"),
	}
	m.sel.Start(0, 0)
	m.selEnd = selection.Position{Line: 0, Col: 5}

	got := m.sel.Text(m.wrappedTextLines(), m.selEnd.Line, m.selEnd.Col)
	if got != "Hello" {
		t.Errorf("got %q, want %q", got, "Hello")
	}
}

func TestMarkdownSelectedTextMultiLine(t *testing.T) {
	m := NewMarkdownWidget()
	m.wrapped = []markdown.Line{
		mkLine("First line"),
		mkLine("Second line"),
		mkLine("Third line"),
	}
	m.sel.Start(0, 6)
	m.selEnd = selection.Position{Line: 2, Col: 5}

	got := m.sel.Text(m.wrappedTextLines(), m.selEnd.Line, m.selEnd.Col)
	want := "line\nSecond line\nThird"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMarkdownRenderSelectionBgStyle(t *testing.T) {
	m := NewMarkdownWidget()
	m.lines = []markdown.Line{
		mkLine("Hello World"),
		mkLine("Second line"),
	}
	m.sel.Start(0, 2)
	m.selEnd = selection.Position{Line: 0, Col: 7}

	virt := newVirtualSurface(20, 5)
	m.SetRect(Rect{X: 0, Y: 0, W: 20, H: 5})
	m.Render(virt)

	// Cols 2-6 should have BgStyle=StyleSelection
	for col := 2; col < 7; col++ {
		if virt.cells[0][col].BgStyle != term.StyleSelection {
			t.Errorf("col %d: expected BgStyle=StyleSelection(%d), got %d",
				col, term.StyleSelection, virt.cells[0][col].BgStyle)
		}
	}
	// Col 0,1 should NOT have BgStyle
	if virt.cells[0][0].BgStyle != 0 {
		t.Errorf("col 0: expected no BgStyle, got %d", virt.cells[0][0].BgStyle)
	}
	if virt.cells[0][1].BgStyle != 0 {
		t.Errorf("col 1: expected no BgStyle, got %d", virt.cells[0][1].BgStyle)
	}
	// Col 7 should NOT have BgStyle
	if virt.cells[0][7].BgStyle != 0 {
		t.Errorf("col 7: expected no BgStyle, got %d", virt.cells[0][7].BgStyle)
	}
}

func TestMarkdownSelectedTextEmpty(t *testing.T) {
	m := NewMarkdownWidget()
	m.wrapped = []markdown.Line{mkLine("Hello")}

	got := m.sel.Text(m.wrappedTextLines(), 0, 0)
	if got != "" {
		t.Errorf("no selection: got %q, want empty", got)
	}

	m.sel.Start(0, 3)
	m.selEnd = selection.Position{Line: 0, Col: 3}
	got = m.sel.Text(m.wrappedTextLines(), m.selEnd.Line, m.selEnd.Col)
	if got != "" {
		t.Errorf("zero-width: got %q, want empty", got)
	}
}

func TestMarkdownProseWrapsInsideVerticalScrollbar(t *testing.T) {
	m := NewMarkdownWidget()
	m.Box.PaddingLeft, m.Box.PaddingRight = 1, 1
	m.SetContent(strings.Repeat("alpha beta gamma delta epsilon ", 20))
	sv := NewScrollViewWidget(m)
	m.SetScrollParent(sv)
	renderWidget(sv, 0, 0, 24, 8)
	if sv.lastContentH <= sv.lastViewH || sv.lastContentW > sv.lastViewW {
		t.Fatalf("prose should scroll vertically only: content %dx%d, viewport %dx%d", sv.lastContentW, sv.lastContentH, sv.lastViewW, sv.lastViewH)
	}
	if m.wrapWidth != sv.lastViewW-2 {
		t.Fatalf("wrap width = %d, want viewport minus padding = %d", m.wrapWidth, sv.lastViewW-2)
	}
}

func TestMarkdownCodeOverflowDoesNotReflowProse(t *testing.T) {
	m := NewMarkdownWidget()
	m.Box.PaddingLeft, m.Box.PaddingRight = 1, 1
	m.SetContent(strings.Repeat("alpha beta gamma delta ", 10) + "\n\n```\n" + strings.Repeat("x", 120) + "\n```")
	sv := NewScrollViewWidget(m)
	m.SetScrollParent(sv)
	for _, width := range []int{24, 34} {
		renderWidget(sv, 0, 0, width, 8)
		wrapWidth, rows := m.wrapWidth, len(m.wrapped)
		if wrapWidth != sv.lastViewW-2 || rows != sv.lastContentH {
			t.Fatalf("measurement and render disagree: wrap=%d, viewport=%d, rows=%d, measured=%d", wrapWidth, sv.lastViewW, rows, sv.lastContentH)
		}
		if sv.lastContentW <= sv.lastViewW {
			t.Fatal("long code should retain horizontal scrolling")
		}
		sv.HandleEvent(tcell.NewEventMouse(5, 5, tcell.WheelRight, 0))
		renderWidget(sv, 0, 0, width, 8)
		if m.wrapWidth != wrapWidth || len(m.wrapped) != rows {
			t.Fatal("horizontal scroll changed paragraph wrapping")
		}
	}
}
