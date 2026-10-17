package ui

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/term"
)

func noteOverlay(notes map[int]string) *DiffOverlay {
	o := NewAnnotationOverlay()
	for line, text := range notes {
		o.NoteTexts[line] = append(o.NoteTexts[line], NoteText{Text: text, Style: term.StyleMuted})
	}
	return o
}

func TestAnnotationNotesRenderBelowTheirLine(t *testing.T) {
	e := newRowsEditor([]string{"a", "b", "c"}, 20, 6)
	e.SetDiffOverlay(noteOverlay(map[int]string{0: "first\n\nsecond"}))
	got := renderRows(e)
	want := []string{"a", " first", "", " second", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %q, want %q", got, want)
	}
	grid := renderGrid(e)
	gutterW := e.GutterWidth()
	if c := grid[1][gutterW+1]; c.Ch != 'f' || c.Style != term.StyleMuted {
		t.Fatalf("note cell = %q style %v, want 'f' StyleMuted", c.Ch, c.Style)
	}
	if c := grid[1][gutterW+15]; c.Style != term.StyleMuted {
		t.Fatalf("note fill style = %v, want StyleMuted to the right edge", c.Style)
	}
}

func TestAnnotationNoteTextWrapsToPaneWidth(t *testing.T) {
	e := newRowsEditor([]string{"    a", "b"}, 20, 8)
	o := NewAnnotationOverlay()
	o.NoteTexts[0] = []NoteText{{Text: "one two three four five", Indent: 4, Style: term.StyleMuted}}
	e.SetDiffOverlay(o)
	got := renderRows(e)
	want := []string{"    a", "     one two three", "     four five", "b", "", "", "", ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %q, want %q", got, want)
	}

	e.SetRect(Rect{X: 0, Y: 0, W: 40, H: 8})
	e.Viewport.Width = 40
	got = renderRows(e)
	want = []string{"    a", "     one two three four five", "b", "", "", "", "", ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("after widening rows = %q, want %q", got, want)
	}
}

func TestWrapNoteTextDropsDeepIndentAndSplitsLongWords(t *testing.T) {
	rows := wrapNoteText(NoteText{Text: strings.Repeat("x", 25), Indent: 30}, 20)
	var texts []string
	for _, r := range rows {
		if r.Indent != 0 {
			t.Errorf("indent = %d, want 0 when it would take over half the width", r.Indent)
		}
		texts = append(texts, r.Text)
	}
	if want := []string{" " + strings.Repeat("x", 19), " xxxxxx"}; !reflect.DeepEqual(texts, want) {
		t.Fatalf("rows = %q, want %q", texts, want)
	}
}

func TestAnnotationNotesMoveWithTheirLine(t *testing.T) {
	cases := []struct {
		name string
		edit func(e *EditorPaneWidget)
		want []string
	}{
		{"insert above", func(e *EditorPaneWidget) { e.Buf.InsertLine(0, "new") }, []string{"new", "a", "b", " note", "c", ""}},
		{"split the line", func(e *EditorPaneWidget) { e.Buf.Splice(1, 1, "b", "") }, []string{"a", "b", " note", "", "c", ""}},
		{"join the next line", func(e *EditorPaneWidget) { e.Buf.Splice(1, 2, "bc") }, []string{"a", "bc", " note", "", "", ""}},
		{"delete the line", func(e *EditorPaneWidget) { e.Buf.DeleteLine(1) }, []string{"a", "c", "", "", "", ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newRowsEditor([]string{"a", "b", "c"}, 20, 6)
			e.SetDiffOverlay(noteOverlay(map[int]string{1: "note"}))
			renderRows(e)
			tc.edit(e)
			if got := renderRows(e); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("rows = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAnnotationNotesKeepGitGutterMarkers(t *testing.T) {
	e := newRowsEditor([]string{"a", "B", "c"}, 30, 6)
	e.LineNumbers = true
	e.GutterStyle = "compact"
	e.LineChanges = []diff.LineChangeKind{diff.LineUnchanged, diff.LineModified, diff.LineUnchanged}
	e.SetDiffOverlay(noteOverlay(map[int]string{0: "note"}))
	grid := renderGrid(e)
	for y := range grid {
		if grid[y][0].Ch == '▎' {
			return
		}
	}
	t.Fatal("notes hid the git gutter marker")
}

func TestDecorationNotesMergeAndIndentLikeTheirLine(t *testing.T) {
	g, dir := newDecoratedGroup(t, map[string]string{"a.txt": "\tfunc a() {\n}\n"})
	a := filepath.Join(dir, "a.txt")
	openCommitted(g, a)
	g.Editor.SetRect(Rect{W: 40, H: 6})
	g.Editor.Viewport.Width, g.Editor.Viewport.Height = 40, 6
	g.SetDecorations("b", a, []Decoration{{StartLine: 0, EndLine: 0, Note: "from b", Style: term.StyleMuted}})
	g.SetDecorations("a", a, []Decoration{{StartLine: 0, EndLine: 0, Note: "from a", Style: term.StyleMuted}})
	got := renderRows(g.Editor)
	want := []string{"    func a() {", "     from a", "     from b", "}", "", ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %q, want %q", got, want)
	}
}

func TestStaleNotesLeaveNoRowsBehind(t *testing.T) {
	e := newRowsEditor([]string{"a", "b"}, 20, 4)
	o := noteOverlay(map[int]string{0: "note"})
	o.annotate = true
	e.SetDiffOverlay(o)
	renderRows(e)
	for range 65 {
		e.Buf.InsertLine(2, "x")
	}
	if got, want := renderRows(e), []string{"a", "b", "x", "x"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %q, want %q", got, want)
	}
}

func TestFoldedLineKeepsItsNoteAndHidesNotesInside(t *testing.T) {
	lines := []string{"head", "  x", "  y", "tail"}
	e := newRowsEditor(lines, 20, 6)
	e.Folds = fold.NewState()
	e.Folds.SetRanges(fold.ComputeIndentRanges(lines))
	e.SetDiffOverlay(noteOverlay(map[int]string{0: "note on head", 2: "note on y"}))
	if got, want := renderRows(e), []string{"head", " note on head", "  x", "  y", " note on y", "tail"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unfolded rows = %q, want %q", got, want)
	}
	e.Folds.Toggle(0)
	if got, want := renderRows(e), []string{"head ⋯", " note on head", "tail", "", "", ""}; !reflect.DeepEqual(got, want) {
		t.Fatalf("folded rows = %q, want %q", got, want)
	}
}

func TestNotesWrapToTheWidthDrawnWithWordWrap(t *testing.T) {
	for _, text := range []string{strings.Repeat("x", 30) + "END", strings.Repeat("一", 13)} {
		e := newRowsEditor([]string{"a", "b"}, 30, 8)
		e.WordWrap = true
		e.GutterStyle = "extended"
		o := NewAnnotationOverlay()
		o.NoteTexts[0] = []NoteText{{Text: text, Style: term.StyleMuted}}
		e.SetDiffOverlay(o)
		got := strings.Join(renderRows(e), "")
		for _, r := range []rune{'x', 'E', 'N', 'D', '一'} {
			if strings.Count(got, string(r)) != strings.Count(text, string(r)) {
				t.Errorf("rows %q lost part of %q", renderRows(e), text)
				break
			}
		}
	}
}

func TestHidingALineLeavesItsNoteReadable(t *testing.T) {
	e := newRowsEditor([]string{"secret", "b"}, 20, 4)
	o := noteOverlay(map[int]string{0: "shown"})
	o.LineBg[0] = term.StyleHide
	e.SetDiffOverlay(o)
	grid := renderGrid(e)
	gutterW := e.GutterWidth()
	if c := grid[0][gutterW]; c.Attrs&term.CellAttrHide == 0 && c.Style != term.StyleHide {
		t.Fatalf("hidden line cell = %+v, want it hidden", c)
	}
	for x := gutterW + 1; x < gutterW+6; x++ {
		if c := grid[1][x]; c.Attrs&term.CellAttrHide != 0 || c.Style != term.StyleMuted {
			t.Fatalf("note cell %d = %+v, want it drawn in StyleMuted", x, c)
		}
	}
}

// Note rows count toward the scrollbar on the frame that first wraps them.
func TestNotesThatOverflowThePaneShowTheScrollbarOnTheFirstFrame(t *testing.T) {
	e := newRowsEditor([]string{"a", "b", "c"}, 20, 4)
	e.SetDiffOverlay(noteOverlay(map[int]string{0: "one\n\ntwo\n\nthree"}))
	first := renderGrid(e)
	width := e.Viewport.Width
	second := renderGrid(e)
	if e.Viewport.Width != width {
		t.Errorf("viewport width %d on the first frame, %d on the second", width, e.Viewport.Width)
	}
	for y := range first {
		if first[y][19] != second[y][19] {
			t.Errorf("row %d scrollbar column differs: %+v then %+v", y, first[y][19], second[y][19])
		}
	}
}

func TestNotesInANarrowPaneWrapInsteadOfBeingCut(t *testing.T) {
	e := newRowsEditor([]string{"a", "b"}, 8, 8)
	e.SetDiffOverlay(noteOverlay(map[int]string{0: "abcdefghij"}))
	if got := strings.Join(renderRows(e), "|"); !strings.Contains(strings.ReplaceAll(got, "|", ""), "ghij") {
		t.Errorf("rows %q lose the end of the note", got)
	}
}
