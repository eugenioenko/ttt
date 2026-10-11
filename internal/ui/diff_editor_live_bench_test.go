package ui

import (
	"fmt"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

func liveBenchDiff(n int) (old, cur []string, lines []diff.DiffLine) {
	for i := 0; i < n; i++ {
		text := fmt.Sprintf("line %d with some text that is long enough to wrap when the pane is narrow enough", i)
		cur = append(cur, text)
		if i%50 == 0 {
			removed := fmt.Sprintf("removed %d", i)
			old = append(old, removed)
			lines = append(lines,
				diff.DiffLine{Left: diff.SideLine{Num: len(old), Text: removed, Kind: diff.Deleted}, Right: diff.SideLine{Kind: diff.Blank}},
				diff.DiffLine{Left: diff.SideLine{Kind: diff.Blank}, Right: diff.SideLine{Num: len(cur), Text: text, Kind: diff.Added}})
			continue
		}
		old = append(old, text)
		lines = append(lines, diff.DiffLine{
			Left:  diff.SideLine{Num: len(old), Text: text, Kind: diff.Context},
			Right: diff.SideLine{Num: len(cur), Text: text, Kind: diff.Context},
		})
	}
	return old, cur, lines
}

func benchmarkLiveDiffType(b *testing.B, n int, mode DiffMode, newline bool) {
	old, cur, lines := liveBenchDiff(n)
	const w, h = 120, 40
	e := newRowsEditor(cur, w, h)
	e.LineNumbers = true
	e.Undo = &undo.UndoStack{}
	d := NewEditableDiffWidget("f.txt", old, lines)
	d.SetMode(mode)
	d.SetContextMode(DiffContextFullFile)
	d.SetWrapped(true)
	d.SetRect(Rect{W: w, H: h})
	grid := makeGrid(w, h)
	surface := NewRenderSurface(grid, Rect{W: w, H: h})
	at := n/2 + 1
	e.Cursor.Line = at
	render := func() {
		d.bind(e)
		d.Render(surface)
	}
	render()
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
		render()
	}
}

func BenchmarkLiveDiffTypeUnified50k(b *testing.B) {
	benchmarkLiveDiffType(b, 50000, DiffModeUnified, false)
}
func BenchmarkLiveDiffTypeUnified500k(b *testing.B) {
	benchmarkLiveDiffType(b, 500000, DiffModeUnified, false)
}
func BenchmarkLiveDiffTypeSplit50k(b *testing.B) {
	benchmarkLiveDiffType(b, 50000, DiffModeSplit, false)
}
func BenchmarkLiveDiffTypeSplit500k(b *testing.B) {
	benchmarkLiveDiffType(b, 500000, DiffModeSplit, false)
}
func BenchmarkLiveDiffNewlineUnified50k(b *testing.B) {
	benchmarkLiveDiffType(b, 50000, DiffModeUnified, true)
}
func BenchmarkLiveDiffNewlineUnified500k(b *testing.B) {
	benchmarkLiveDiffType(b, 500000, DiffModeUnified, true)
}
func BenchmarkLiveDiffNewlineSplit50k(b *testing.B) {
	benchmarkLiveDiffType(b, 50000, DiffModeSplit, true)
}
func BenchmarkLiveDiffNewlineSplit500k(b *testing.B) {
	benchmarkLiveDiffType(b, 500000, DiffModeSplit, true)
}
