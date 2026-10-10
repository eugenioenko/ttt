package ui

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

func randomLine(r *rand.Rand) string {
	return strings.Repeat("ab\tc 字", r.Intn(12))
}

func randomEdit(r *rand.Rand, e *EditorPaneWidget, sameLineCount bool) undo.EditCommand {
	n := len(e.Buf.Lines)
	line := r.Intn(n)
	op := r.Intn(9)
	if sameLineCount {
		op = r.Intn(3)
	}
	switch op {
	case 0:
		return &undo.InsertStringCommand{Line: line, Col: 0, Text: randomLine(r)}
	case 1:
		return &undo.SwapLineCommand{Line1: line, Line2: r.Intn(n)}
	case 2:
		return &undo.ReplaceLinesCommand{Start: line, OldLines: []string{e.Buf.Lines[line]}, NewLines: []string{randomLine(r)}}
	case 3:
		return &undo.SplitLineCommand{Line: line, Col: r.Intn(5)}
	case 4:
		if line == 0 {
			return &undo.InsertLineCommand{Idx: 0, Text: randomLine(r)}
		}
		return &undo.JoinLineCommand{Line: line}
	case 5:
		return &undo.PasteCommand{Line: line, Col: 0, Text: randomLine(r) + "\n" + randomLine(r) + "\n" + randomLine(r)}
	case 6:
		end := min(n-1, line+r.Intn(4))
		return &undo.DeleteSelectionCommand{StartLine: line, StartCol: 0, EndLine: end, EndCol: 1}
	case 7:
		if n > 1 {
			return &undo.DeleteLineCommand{Idx: line}
		}
		return &undo.InsertLineCommand{Idx: line, Text: randomLine(r)}
	default:
		k := min(n-line, 1+r.Intn(3))
		old := slices.Clone(e.Buf.Lines[line : line+k])
		return &undo.ReplaceLinesCommand{Start: line, OldLines: old, NewLines: []string{randomLine(r), randomLine(r)}}
	}
}

func assertLayoutMatchesRebuild(t *testing.T, step int, e *EditorPaneWidget) {
	t.Helper()
	got := e.layout()
	gotTotal := got.total()
	gotStarts := slices.Clone(got.starts)
	e.layoutCache = [len(e.layoutCache)]*rowLayout{}
	want := e.layout()
	if wantTotal := want.total(); gotTotal != wantTotal || !slices.Equal(gotStarts, want.starts) {
		t.Fatalf("step %d: incremental layout total %d starts %v, rebuilt %d %v", step, gotTotal, gotStarts, wantTotal, want.starts)
	}
}

func TestRowLayoutFollowsEditsLikeARebuild(t *testing.T) {
	for _, tc := range []struct {
		name     string
		wrap     bool
		phantoms bool
	}{
		{"wrap", true, false},
		{"wrap with deleted rows", true, true},
		{"deleted rows", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := rand.New(rand.NewSource(7))
			lines := make([]string, 40)
			for i := range lines {
				lines[i] = randomLine(r)
			}
			e := newRowsEditor(lines, 20, 10)
			e.WordWrap = tc.wrap
			if tc.phantoms {
				deleted := map[int][]diff.SideLine{}
				for i := 0; i <= len(lines); i += 7 {
					deleted[i] = []diff.SideLine{{Text: fmt.Sprintf("old %d %s", i, randomLine(r)), Kind: diff.Deleted}}
				}
				e.SetDiffOverlay(&DiffOverlay{Deleted: deleted})
			}
			e.layout().total()
			for step := 0; step < 300; step++ {
				for k := r.Intn(3); k >= 0; k-- {
					e.exec(randomEdit(r, e, tc.phantoms))
				}
				assertLayoutMatchesRebuild(t, step, e)
			}
		})
	}
}

func TestRowLayoutAdvancesInPlaceOnEdits(t *testing.T) {
	e := newPhantomBenchEditorN(2000, true, false)
	e.Viewport.Width = 30
	l := e.layout()
	l.total()
	e.exec(&undo.InsertRuneCommand{Line: 1000, Col: 0, Rune: 'x'})
	e.exec(&undo.SplitLineCommand{Line: 500, Col: 3})
	if e.layout() != l {
		t.Fatal("an edit rebuilt the layout instead of advancing it")
	}
	assertLayoutMatchesRebuild(t, 0, e)
}
