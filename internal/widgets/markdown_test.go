package widgets

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/markdown"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
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

func TestMarkdownWideProseScrollSize(t *testing.T) {
	m := NewMarkdownWidget()
	m.Box.PaddingLeft, m.Box.PaddingRight = 1, 1
	m.SetContent(strings.Repeat("界界 語語 abc ", 20))
	sv := NewScrollViewWidget(m)
	m.SetScrollParent(sv)
	renderWidget(sv, 0, 0, 24, 8)
	if sv.lastContentW > sv.lastViewW || sv.lastContentH <= sv.lastViewH {
		t.Fatalf("wide prose should scroll vertically only: content %dx%d, viewport %dx%d", sv.lastContentW, sv.lastContentH, sv.lastViewW, sv.lastViewH)
	}
	for _, row := range m.wrapped {
		if textwidth.String(row.Text()) > m.wrapWidth {
			t.Fatalf("row %q exceeds wrap width %d", row.Text(), m.wrapWidth)
		}
	}
}

func TestMarkdownWideRenderingAndMouseSelection(t *testing.T) {
	m := NewMarkdownWidget()
	m.SetContent("a界b語c")
	sv := NewScrollViewWidget(m)
	m.SetScrollParent(sv)
	renderWidget(sv, 3, 2, 12, 4)
	for _, tt := range []struct{ x, col int }{{3, 0}, {4, 1}, {5, 1}, {6, 2}, {7, 3}, {8, 3}, {9, 4}, {10, 5}} {
		if pos := m.mouseToContent(tt.x, 2); pos.Col != tt.col {
			t.Errorf("mouse column %d = rune %d, want %d", tt.x, pos.Col, tt.col)
		}
	}
	m.sel.Start(0, 1)
	m.selEnd = selection.Position{Line: 0, Col: 4}
	v := newVirtualSurface(12, 4)
	m.Render(v)
	for _, tt := range []struct {
		x        int
		ch       rune
		selected bool
	}{{0, 'a', false}, {1, '界', true}, {3, 'b', true}, {4, '語', true}, {6, 'c', false}} {
		cell := v.cells[0][tt.x]
		if cell.Ch != tt.ch || (cell.BgStyle == term.StyleSelection) != tt.selected {
			t.Errorf("column %d: got %+v", tt.x, cell)
		}
	}
	if got := m.sel.Text(m.wrappedTextLines(), m.selEnd.Line, m.selEnd.Col); got != "界b語" {
		t.Fatalf("selected text = %q", got)
	}
	m.SetScrollParent(nil)
	m.lines = []markdown.Line{{Kind: markdown.KindCode, Spans: []markdown.Span{{Text: "a界"}}}}
	m.wrapWidth = 0
	v = newVirtualSurface(2, 1)
	m.Render(v)
	if v.cells[0][1].Ch != ' ' {
		t.Fatal("wide rune must not straddle the clipping boundary")
	}
}

func TestMarkdownFullWidthDividerPreservesContentPadding(t *testing.T) {
	m := NewMarkdownWidget()
	m.FullWidthDividers = true
	m.Box.PaddingLeft, m.Box.PaddingRight = 1, 1
	m.SetContent("text\n\n---\n\ntext")
	sv := NewScrollViewWidget(m)
	m.SetScrollParent(sv)
	s := renderWidget(sv, 0, 0, 20, 8)
	for y, line := range m.wrapped {
		if line.Kind == markdown.KindDivider {
			for x := range sv.lastViewW {
				if s.cells[y][x].Ch != '─' {
					t.Fatalf("divider did not extend through padding at x=%d", x)
				}
			}
		}
	}
	if s.cells[0][0].Ch != ' ' || s.cells[0][1].Ch != 't' {
		t.Fatal("paragraph lost its left padding")
	}
	pos := m.mouseToContent(1, 0)
	if pos.Line != 0 || pos.Col != 0 {
		t.Fatalf("padded text hit = %+v, want first character", pos)
	}
}

func TestMarkdownFullWidthDividerPreservesBordersAndMargins(t *testing.T) {
	for _, tc := range []struct {
		name        string
		left, right bool
	}{{"left", true, false}, {"right", false, true}, {"both", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewMarkdownWidget()
			m.FullWidthDividers = true
			m.Box = BoxModel{
				BorderTop: true, BorderLeft: tc.left, BorderRight: tc.right,
				MarginLeft: 1, MarginRight: 2, MarginTop: 1,
				PaddingLeft: 2, PaddingRight: 1, PaddingTop: 1,
				Borders: term.RoundedBorderSet(),
			}
			m.SetContent("text\n\n---\n\ntext")
			s := renderWidget(m, 0, 0, 24, 10)
			for row, line := range m.wrapped {
				if line.Kind != markdown.KindDivider {
					continue
				}
				y := row + 3
				for x := range 24 {
					want := '─'
					if x < 1 || x >= 22 {
						want = 0
					} else if (x == 1 && tc.left) || (x == 21 && tc.right) {
						want = '│'
					}
					if got := s.cells[y][x].Ch; got != want {
						t.Fatalf("divider cell (%d,%d) = %q, want %q", x, y, got, want)
					}
				}
				return
			}
			t.Fatal("divider not rendered")
		})
	}
}
