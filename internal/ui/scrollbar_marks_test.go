package ui

import (
	"fmt"
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"
)

func TestScrollbarMarkSingleItemInHugeRangeStaysVisible(t *testing.T) {
	scrollbar := Scrollbar{Height: 10, TotalItems: 100000, Marks: []ScrollMark{{Item: 50000, Count: 1, Style: term.StyleScrollMarkAdded, Rank: 1}}}
	var marked []int
	for slot, style := range scrollbar.markSlots() {
		if style != term.StyleDefault {
			marked = append(marked, slot)
		}
	}
	if fmt.Sprint(marked) != "[40]" {
		t.Fatalf("marked slots = %v, want [40]", marked)
	}
}

func TestScrollbarMarkHigherRankWinsSharedSlot(t *testing.T) {
	scrollbar := Scrollbar{Height: 2, TotalItems: 400, Marks: []ScrollMark{
		{Item: 0, Count: 1, Style: term.StyleScrollMarkDeleted, Rank: 3},
		{Item: 1, Count: 1, Style: term.StyleScrollMarkAdded, Rank: 1},
		{Item: 2, Count: 1, Style: term.StyleScrollMarkModified, Rank: 2},
	}}
	if got := scrollbar.markSlots()[0]; got != term.StyleScrollMarkDeleted {
		t.Fatalf("shared slot = %v, want deleted", got)
	}
}

func drawnSlots(t *testing.T, cell term.Cell) [cellSlots]term.Style {
	t.Helper()
	bg := cell.BgStyle
	switch bg {
	case term.StyleScrollbarFill:
		bg = term.StyleScrollbar
	case term.StyleScrollbarThumbFill:
		bg = term.StyleScrollbarThumb
	}
	for _, glyph := range legacyGlyphs {
		if glyph.ch == cell.Ch {
			return glyph.draw(cell.Style, bg)
		}
	}
	t.Fatalf("unknown track glyph %q", cell.Ch)
	return [cellSlots]term.Style{}
}

func slotPattern(marks map[int]term.Style) []term.Style {
	slots := make([]term.Style, cellSlots)
	for slot, style := range marks {
		slots[slot] = style
	}
	return slots
}

func TestScrollbarMarkCellReproducesPattern(t *testing.T) {
	track := term.StyleScrollbar
	added, modified, deleted := term.StyleScrollMarkAdded, term.StyleScrollMarkModified, term.StyleScrollMarkDeleted
	fullCell := func(style term.Style) map[int]term.Style {
		marks := map[int]term.Style{}
		for slot := range cellSlots {
			marks[slot] = style
		}
		return marks
	}
	cases := []struct {
		name   string
		glyphs []scrollGlyph
		marks  map[int]term.Style
	}{
		{"no marks", blockGlyphs, nil},
		{"full cell", blockGlyphs, fullCell(added)},
		{"top half", blockGlyphs, map[int]term.Style{0: modified, 1: modified, 2: modified, 3: modified}},
		{"bottom half", blockGlyphs, map[int]term.Style{4: deleted, 5: deleted, 6: deleted, 7: deleted}},
		{"top eighth", blockGlyphs, map[int]term.Style{0: added}},
		{"bottom eighth", blockGlyphs, map[int]term.Style{7: added}},
		{"bottom quarter", blockGlyphs, map[int]term.Style{6: modified, 7: modified}},
		{"two colors split", blockGlyphs, map[int]term.Style{0: added, 1: added, 2: added, 3: added, 4: modified, 5: modified, 6: modified, 7: modified}},
		{"thin bar row 3", legacyGlyphs, map[int]term.Style{3: added}},
		{"thin bar row 6", legacyGlyphs, map[int]term.Style{6: deleted}},
		{"upper quarter", legacyGlyphs, map[int]term.Style{0: modified, 1: modified}},
	}
	for _, testCase := range cases {
		want := slotPattern(testCase.marks)
		for slot := range want {
			if want[slot] == term.StyleDefault {
				want[slot] = track
			}
		}
		got := drawnSlots(t, markCell(track, slotPattern(testCase.marks), testCase.glyphs))
		if fmt.Sprint(got[:]) != fmt.Sprint(want) {
			t.Errorf("%s: drew %v, want %v", testCase.name, got, want)
		}
	}
}

func TestScrollbarMarkCellNeverHidesAMark(t *testing.T) {
	track := term.StyleScrollbarThumb
	added, deleted := term.StyleScrollMarkAdded, term.StyleScrollMarkDeleted
	for _, glyphs := range [][]scrollGlyph{blockGlyphs, legacyGlyphs} {
		for addedSlot := range cellSlots {
			for deletedSlot := range cellSlots {
				if addedSlot == deletedSlot {
					continue
				}
				marks := slotPattern(map[int]term.Style{addedSlot: added, deletedSlot: deleted})
				got := drawnSlots(t, markCell(track, marks, glyphs))
				if !slices.Contains(got[:], added) || !slices.Contains(got[:], deleted) {
					t.Fatalf("added at %d, deleted at %d: drew %v, a mark is hidden", addedSlot, deletedSlot, got)
				}
			}
		}
	}
}

func TestScrollbarMarkCellBlocksFallBackToNearestHalf(t *testing.T) {
	track := term.StyleScrollbar
	added := term.StyleScrollMarkAdded
	got := drawnSlots(t, markCell(track, slotPattern(map[int]term.Style{2: added}), blockGlyphs))
	if got[2] != added || got[6] != track {
		t.Fatalf("mid-top mark without legacy glyphs drew %v, want it in the top half", got)
	}
}

func newMarksTestEditor(lineCount int) *EditorPaneWidget {
	lines := make([]string, lineCount)
	for lineIdx := range lines {
		lines[lineIdx] = fmt.Sprintf("line %d", lineIdx)
	}
	buf := &buffer.Buffer{Lines: lines}
	return NewEditorPaneWidget(buf, &cursor.Cursor{}, &view.Viewport{Width: 40, Height: 10})
}

func TestEditorRendersGitChangeOnScrollbar(t *testing.T) {
	editor := newMarksTestEditor(200)
	changes := make([]diff.LineChangeKind, 200)
	changes[150] = diff.LineModified
	editor.LineChanges = changes
	editor.SetRect(Rect{W: 40, H: 10})

	grid := makeGrid(40, 10)
	editor.Render(NewRenderSurface(grid, Rect{W: 40, H: 10}))

	// TotalItems = 200+10-1 = 209, 80 slots: line 150 -> slot 57, row 7.
	for row := range 10 {
		slots := drawnSlots(t, grid[row][39])
		if marked := slices.Contains(slots[:], term.StyleScrollMarkModified); marked != (row == 7) {
			t.Fatalf("scrollbar row %d = %+v, want the modified mark only on row 7", row, grid[row][39])
		}
	}
}

func TestGitScrollMarksMergeRuns(t *testing.T) {
	editor := newMarksTestEditor(10)
	editor.LineChanges = []diff.LineChangeKind{0, 1, 1, 1, 2, 0, 1, 0, 0, 3}
	got := editor.gitScrollMarks(editor.rowLayout(40))
	want := []ScrollMark{
		{Item: 1, Count: 3, Style: term.StyleScrollMarkAdded, Rank: 1},
		{Item: 4, Count: 1, Style: term.StyleScrollMarkModified, Rank: 2},
		{Item: 6, Count: 1, Style: term.StyleScrollMarkAdded, Rank: 1},
		{Item: 9, Count: 1, Style: term.StyleScrollMarkDeleted, Rank: 3},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("marks = %v, want %v", got, want)
	}
}

func TestGitScrollMarksAttributeFoldedChangesToHeader(t *testing.T) {
	editor := newMarksTestEditor(10)
	editor.Folds = fold.NewState()
	editor.Folds.SetRanges([]fold.Range{{StartLine: 2, EndLine: 6}})
	editor.Folds.Toggle(2)
	changes := make([]diff.LineChangeKind, 10)
	changes[4] = diff.LineAdded
	changes[5] = diff.LineDeleted
	changes[8] = diff.LineModified
	editor.LineChanges = changes

	got := editor.gitScrollMarks(editor.rowLayout(40))
	want := []ScrollMark{
		{Item: 2, Count: 1, Style: term.StyleScrollMarkDeleted, Rank: 3},
		{Item: 4, Count: 1, Style: term.StyleScrollMarkModified, Rank: 2},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("marks = %v, want %v", got, want)
	}
}

func TestGitScrollMarksUseVisualRowsUnderWordWrap(t *testing.T) {
	editor := newMarksTestEditor(3)
	editor.WordWrap = true
	editor.Buf.Lines = []string{"aaaa bbbb cccc dddd", "short", "x"}
	editor.LineChanges = []diff.LineChangeKind{diff.LineModified, 0, diff.LineAdded}

	got := editor.gitScrollMarks(editor.rowLayout(5))
	firstLineRows := wrapLineVisualRows(editor.Buf.Lines[0], 5, editor.resolveTabSize())
	want := []ScrollMark{
		{Item: 0, Count: firstLineRows, Style: term.StyleScrollMarkModified, Rank: 2},
		{Item: firstLineRows + 1, Count: 1, Style: term.StyleScrollMarkAdded, Rank: 1},
	}
	if firstLineRows < 2 || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("marks = %v, want %v", got, want)
	}
}

func TestGitScrollMarksToleratesChangesShorterThanBuffer(t *testing.T) {
	editor := newMarksTestEditor(10)
	editor.Folds = fold.NewState()
	editor.Folds.SetRanges([]fold.Range{{StartLine: 1, EndLine: 3}})
	editor.Folds.Toggle(1)
	editor.LineChanges = []diff.LineChangeKind{0, 0, diff.LineAdded}

	got := editor.gitScrollMarks(editor.rowLayout(40))
	want := []ScrollMark{{Item: 1, Count: 1, Style: term.StyleScrollMarkAdded, Rank: 1}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("marks = %v, want %v", got, want)
	}
}
