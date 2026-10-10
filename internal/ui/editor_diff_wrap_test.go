package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
)

func TestDeletedPhantomRowsWrap(t *testing.T) {
	removed := "removed " + strings.Repeat("abcd ", 8)
	e := newRowsEditor([]string{"keep", "new", "tail"}, 20, 10)
	e.WordWrap = true
	e.SetDiffOverlay(&DiffOverlay{
		Kinds:   []diff.LineKind{diff.Context, diff.Added, diff.Context},
		Deleted: map[int][]diff.SideLine{1: {{Num: 2, Text: removed, Kind: diff.Deleted}}},
	})
	rows := renderRows(e)
	segs := len(wrapLineSegments([]rune(removed), e.Viewport.Width, e.resolveTabSize()))
	if segs < 2 {
		t.Fatalf("test text should wrap at width %d", e.Viewport.Width)
	}
	got := strings.Join(rows[1:1+segs], "")
	if strings.ReplaceAll(got, " ", "") != strings.ReplaceAll(removed, " ", "") {
		t.Fatalf("deleted line not wrapped across rows:\n%s", strings.Join(rows, "\n"))
	}
	if rows[1+segs] != "new" {
		t.Fatalf("row after the wrapped deleted line = %q, want new:\n%s", rows[1+segs], strings.Join(rows, "\n"))
	}
	if total := e.layout().total(); total != 3+segs {
		t.Fatalf("layout total %d, want %d", total, 3+segs)
	}

	e.Cursor.Line = 1
	renderRows(e)
	if e.CursorY != 1+segs {
		t.Fatalf("cursor on the added line drawn at row %d, want %d", e.CursorY, 1+segs)
	}

	row := e.rowAt(2)
	if !row.isPhantom() || row.bufLine != 1 || row.startCol == 0 {
		t.Fatalf("second deleted row = %+v, want a continuation of the phantom", row)
	}
	if line, _ := e.mouseToPos(e.GetRect(), e.GutterWidth()+3, 2); line != 1 {
		t.Fatalf("click on a wrapped deleted row maps to line %d, want anchor 1", line)
	}
}
