package ui

import (
	"fmt"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

func newPhantomBenchEditor(wrap bool) *EditorPaneWidget {
	const n = 50000
	lines := make([]string, n)
	kinds := make([]diff.LineKind, n)
	deleted := make(map[int][]diff.SideLine)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d with some text that is long enough to wrap when the pane is narrow enough", i)
		if i%50 == 0 {
			kinds[i] = diff.Added
			deleted[i] = []diff.SideLine{{Num: i, Text: fmt.Sprintf("removed %d", i), Kind: diff.Deleted}}
		}
	}
	e := newRowsEditor(lines, 60, 40)
	e.WordWrap = wrap
	e.SetDiffOverlay(&DiffOverlay{Kinds: kinds, Deleted: deleted})
	return e
}

func benchmarkPhantomLayout(b *testing.B, wrap bool) {
	e := newPhantomBenchEditor(wrap)
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

func BenchmarkPhantomLayoutScroll(b *testing.B)     { benchmarkPhantomLayout(b, false) }
func BenchmarkPhantomLayoutScrollWrap(b *testing.B) { benchmarkPhantomLayout(b, true) }

func TestRowLayoutIsCachedUntilItsInputsChange(t *testing.T) {
	e := newPhantomBenchEditor(true)
	e.Viewport.Width = 30
	l := e.layout()
	total := l.total()
	if e.layout() != l {
		t.Fatal("layout rebuilt with nothing changed")
	}

	e.ExecCommand(&undo.InsertLineCommand{Idx: 0, Text: "x"})
	l2 := e.layout()
	if l2 == l {
		t.Fatal("layout kept after an edit")
	}
	if l2.total() <= total {
		t.Fatalf("total %d after inserting a line, was %d", l2.total(), total)
	}

	e.SetDiffOverlay(&DiffOverlay{})
	if e.layout() == l2 {
		t.Fatal("layout kept after the overlay changed")
	}
	l3 := e.layout()
	e.Viewport.Width = 40
	if e.layout() == l3 {
		t.Fatal("wrapped layout kept after the width changed")
	}
}

func BenchmarkPhantomLayoutTypeWrap(b *testing.B) {
	e := newPhantomBenchEditor(true)
	grid := makeGrid(60, 40)
	surface := NewRenderSurface(grid, Rect{W: 60, H: 40})
	e.Cursor.Line = 25000
	e.Render(surface)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.exec(&undo.InsertRuneCommand{Line: 25000, Col: 0, Rune: 'x'})
		e.scrollViewport()
		e.Render(surface)
	}
}

func benchmarkSplitDiffScroll(b *testing.B, wrap bool) {
	const n = 50000
	old := make([]string, n)
	cur := make([]string, 0, n)
	for i := range old {
		old[i] = fmt.Sprintf("line %d with some text that is long enough to wrap when the pane is narrow", i)
		if i%50 == 0 {
			cur = append(cur, fmt.Sprintf("changed %d", i))
			continue
		}
		cur = append(cur, old[i])
	}
	d := NewDiffEditorWidget("bench.txt", diff.FileDiff{}, old, cur, true)
	d.SetWrapped(wrap)
	grid := makeGrid(120, 40)
	surface := NewRenderSurface(grid, Rect{W: 120, H: 40})
	d.SetRect(Rect{W: 120, H: 40})
	d.Render(surface)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.lead().scrollDown(3)
		d.Render(surface)
	}
}

func BenchmarkSplitDiffScroll(b *testing.B)     { benchmarkSplitDiffScroll(b, false) }
func BenchmarkSplitDiffScrollWrap(b *testing.B) { benchmarkSplitDiffScroll(b, true) }
