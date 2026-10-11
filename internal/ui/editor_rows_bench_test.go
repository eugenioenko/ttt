package ui

import (
	"fmt"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/undo"
)

func newLayoutBenchEditor(n int, wrap bool) *EditorPaneWidget {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d with some text that is long enough to wrap when the pane is narrow enough", i)
	}
	e := newRowsEditor(lines, 60, 40)
	e.WordWrap = wrap
	return e
}

func benchmarkLayoutScroll(b *testing.B, n int, wrap bool) {
	e := newLayoutBenchEditor(n, wrap)
	grid := makeGrid(60, 40)
	surface := NewRenderSurface(grid, Rect{W: 60, H: 40})
	e.Render(surface)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Cursor.Line = (i * 997) % len(e.Buf.Lines)
		e.scrollViewport()
		e.scrollDown(3)
		e.Render(surface)
	}
}

func BenchmarkLayoutScroll50k(b *testing.B)      { benchmarkLayoutScroll(b, 50000, false) }
func BenchmarkLayoutScrollWrap50k(b *testing.B)  { benchmarkLayoutScroll(b, 50000, true) }
func BenchmarkLayoutScrollWrap500k(b *testing.B) { benchmarkLayoutScroll(b, 500000, true) }

func benchmarkTypeWrap(b *testing.B, n int, newline bool) {
	e := newLayoutBenchEditor(n, true)
	grid := makeGrid(60, 40)
	surface := NewRenderSurface(grid, Rect{W: 60, H: 40})
	at := n / 2
	e.Cursor.Line = at
	e.Render(surface)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		switch {
		case newline && i%2 == 1:
			e.exec(&undo.JoinLineCommand{Line: at + 1})
		case newline:
			e.exec(&undo.SplitLineCommand{Line: at, Col: 4})
		case i%2 == 1:
			e.exec(&undo.DeleteRuneCommand{Line: at, Col: 0})
		default:
			e.exec(&undo.InsertRuneCommand{Line: at, Col: 0, Rune: 'x'})
		}
		e.scrollViewport()
		e.Render(surface)
	}
}

func BenchmarkLayoutTypeWrap50k(b *testing.B)         { benchmarkTypeWrap(b, 50000, false) }
func BenchmarkLayoutTypeWrap500k(b *testing.B)        { benchmarkTypeWrap(b, 500000, false) }
func BenchmarkLayoutTypeNewlineWrap50k(b *testing.B)  { benchmarkTypeWrap(b, 50000, true) }
func BenchmarkLayoutTypeNewlineWrap500k(b *testing.B) { benchmarkTypeWrap(b, 500000, true) }

func TestRowLayoutIsCachedUntilItsInputsChange(t *testing.T) {
	e := newLayoutBenchEditor(200, true)
	e.Viewport.Width = 30
	l := e.layout()
	total := l.total()
	if e.layout() != l {
		t.Fatal("layout rebuilt with nothing changed")
	}

	e.ExecCommand(&undo.InsertLineCommand{Idx: 0, Text: "x"})
	l2 := e.layout()
	if l2.total() <= total {
		t.Fatalf("total %d after inserting a line, was %d", l2.total(), total)
	}

	e.setPhantoms(map[int]int{3: 1})
	if e.layout() == l2 {
		t.Fatal("layout kept after the non-buffer rows changed")
	}
	l3 := e.layout()
	e.Viewport.Width = 40
	if e.layout() == l3 {
		t.Fatal("wrapped layout kept after the width changed")
	}
}

func TestEditorGroupRenderKeepsTheLayoutCache(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, true, "compact")
	g.NewFile()
	g.WordWrap = true
	grid := makeGrid(40, 12)
	g.SetRect(Rect{W: 40, H: 12})
	g.Render(NewRenderSurface(grid, Rect{W: 40, H: 12}))
	l := g.Editor.layout()
	g.Render(NewRenderSurface(grid, Rect{W: 40, H: 12}))
	if g.Editor.layout() != l {
		t.Fatal("a render with nothing changed rebuilt the row layout")
	}
}
