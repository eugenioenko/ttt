package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/term"
)

// selectionRows renders e and returns one string per row with '#' for each
// text cell drawn with the selection background, trailing blanks trimmed.
func selectionRows(e *EditorPaneWidget) []string {
	grid := renderGrid(e)
	gutterW := e.GutterWidth()
	rows := make([]string, len(grid))
	for y, row := range grid {
		var sb strings.Builder
		for x := gutterW; x < gutterW+e.Viewport.Width && x < len(row); x++ {
			if row[x].BgStyle == term.StyleSelection {
				sb.WriteByte('#')
			} else {
				sb.WriteByte('.')
			}
		}
		rows[y] = strings.TrimRight(sb.String(), ".")
	}
	return rows
}

func selectRange(e *EditorPaneWidget, anchorLine, anchorCol, curLine, curCol int) {
	e.Selection.Start(anchorLine, anchorCol)
	e.Cursor.Line, e.Cursor.Col = curLine, curCol
}

func assertSelectionRows(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got[:len(want)], want) {
		t.Fatalf("selection rows = %q, want %q", got[:len(want)], want)
	}
	for y, row := range got[len(want):] {
		if row != "" {
			t.Fatalf("row %d = %q, want no selection", len(want)+y, row)
		}
	}
}

func TestSelectionHighlightsTextAndOneNewlineCell(t *testing.T) {
	e := newRowsEditor([]string{"ab", "", "\tc", "end", "after"}, 30, 8)
	e.TabSize = 4
	selectRange(e, 0, 1, 3, 1)
	assertSelectionRows(t, selectionRows(e),
		".##",
		"#",
		"######",
		"#",
	)
}

func TestSelectionReversedMatchesForward(t *testing.T) {
	e := newRowsEditor([]string{"ab", "", "\tc", "end", "after"}, 30, 8)
	e.TabSize = 4
	selectRange(e, 3, 1, 0, 1)
	assertSelectionRows(t, selectionRows(e),
		".##",
		"#",
		"######",
		"#",
	)
}

func TestSelectionDoesNotInventNewlineAtEndOfFile(t *testing.T) {
	e := newRowsEditor([]string{"ab", "cd"}, 30, 4)
	selectRange(e, 0, 0, 1, 2)
	assertSelectionRows(t, selectionRows(e), "###", "##")
}

func TestSelectionEndingAtLineStartKeepsPreviousNewline(t *testing.T) {
	e := newRowsEditor([]string{"ab", "cd", "ef"}, 30, 4)
	selectRange(e, 0, 0, 2, 0)
	assertSelectionRows(t, selectionRows(e), "###", "###")
}

func TestSelectionSingleLineHasNoNewlineCell(t *testing.T) {
	e := newRowsEditor([]string{"hello", "x"}, 30, 4)
	selectRange(e, 0, 1, 0, 5)
	assertSelectionRows(t, selectionRows(e), ".####")
}

func TestSelectionCoversWideRunes(t *testing.T) {
	e := newRowsEditor([]string{"界x", "y"}, 30, 4)
	selectRange(e, 0, 0, 1, 0)
	assertSelectionRows(t, selectionRows(e), "####")
}

func TestSelectionFollowsHorizontalScroll(t *testing.T) {
	e := newRowsEditor([]string{"abcdefgh", "ab", "界ab", "z"}, 30, 6)
	selectRange(e, 0, 0, 3, 1)
	e.Viewport.LeftCol = 4
	// Line 1 ends left of the viewport, so its newline cell is off screen.
	assertSelectionRows(t, selectionRows(e), "#####", "", "#")
	e.Viewport.LeftCol = 1
	// Line 2's first visible cell is the second half of a wide rune.
	assertSelectionRows(t, selectionRows(e), "########", "##", "####")
}

func TestSelectionSoftWrapIsNotANewline(t *testing.T) {
	e := newRowsEditor([]string{"abcdefghijkl", "z"}, 40, 6)
	e.WordWrap, e.WordWrapColumn = true, 8
	selectRange(e, 0, 0, 1, 1)
	rows := selectionRows(e)
	text := renderRows(e)
	if text[0] == "abcdefghijkl" || text[2] != "z" {
		t.Fatalf("line did not wrap into two rows: %q", text[:3])
	}
	if rows[0] != strings.Repeat("#", len(text[0])) {
		t.Fatalf("wrapped row = %q for text %q, want only the text selected", rows[0], text[0])
	}
	if rows[1] != strings.Repeat("#", len(text[1])+1) {
		t.Fatalf("last wrapped row = %q for text %q, want the text plus one newline cell", rows[1], text[1])
	}
	if rows[2] != "#" {
		t.Fatalf("final line row = %q, want %q", rows[2], "#")
	}
}

func TestSelectionMultipleCursorsFollowSameExtent(t *testing.T) {
	e := newRowsEditor([]string{"ab", "", "cd", "ef", "gh"}, 30, 8)
	selectRange(e, 0, 1, 1, 0)
	e.ensureMulti()
	e.Multi.Add(4, 1)
	e.syncFromMulti()
	for i := range e.Multi.Cursors {
		if c := &e.Multi.Cursors[i]; c.Line == 4 {
			c.Sel = selection.Selection{Active: true, Anchor: selection.Position{Line: 2, Col: 1}}
		}
	}
	assertSelectionRows(t, selectionRows(e),
		".##",
		"",
		".##",
		"###",
		"#",
	)
}

func TestFullLineSelectionRestoresEdgeToEdgeHighlight(t *testing.T) {
	e := newRowsEditor([]string{"ab", "", "\tc", "end", "after"}, 30, 8)
	e.TabSize = 4
	e.FullLineSelection = true
	selectRange(e, 0, 1, 3, 1)
	rows := selectionRows(e)
	full := strings.Repeat("#", e.Viewport.Width)
	assertSelectionRows(t, rows, "."+full[1:], full, full, "#")
}
