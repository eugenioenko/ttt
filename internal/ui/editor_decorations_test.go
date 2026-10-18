package ui

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
)

func TestAnnotationLineBgTintsLines(t *testing.T) {
	e := newRowsEditor([]string{"a", "b", "c"}, 20, 4)
	e.Cursor.Line = 2
	o := NewAnnotationOverlay()
	o.LineBg[1] = term.StyleDiffAdded
	e.SetDiffOverlay(o)
	grid := renderGrid(e)
	gutterW := e.GutterWidth()
	if got := grid[1][gutterW].BgStyle; got != term.StyleDiffAdded {
		t.Fatalf("tinted line bg = %v, want StyleDiffAdded", got)
	}
	if got := grid[0][gutterW].BgStyle; got == term.StyleDiffAdded {
		t.Fatal("untinted line got the tint")
	}
}

func TestAnnotationSpansHighlightPartOfALine(t *testing.T) {
	e := newRowsEditor([]string{"x", "a 世界 b", "c"}, 20, 4)
	o := NewAnnotationOverlay()
	o.LineBg[1] = term.StyleDiffAdded
	o.Spans[1] = []ColSpan{{Start: 2, End: 4, Style: term.StyleSearchMatch}}
	e.SetDiffOverlay(o)
	grid := renderGrid(e)
	gutterW := e.GutterWidth()
	// Columns 3 and 5 are covered by the wide runes drawn at 2 and 4.
	for x, want := range map[int]term.Style{0: term.StyleDiffAdded, 1: term.StyleDiffAdded, 2: term.StyleSearchMatch, 4: term.StyleSearchMatch, 6: term.StyleDiffAdded} {
		if got := grid[1][gutterW+x].BgStyle; got != want {
			t.Errorf("column %d bg = %v, want %v", x, got, want)
		}
	}
	e.Buf.InsertLine(0, "new")
	renderGrid(e)
	if got := e.DiffOverlay.Spans; len(got[2]) != 1 || len(got[1]) != 0 {
		t.Fatalf("span did not move with its line: %v", got)
	}
	if got := e.DiffOverlay.LineBg; !reflect.DeepEqual(got, map[int]term.Style{2: term.StyleDiffAdded}) {
		t.Fatalf("tint did not move with its line: %v", got)
	}
}

func TestAnnotationSpansDropWhenTheirLineIsEdited(t *testing.T) {
	e := newRowsEditor([]string{"abc", "def"}, 20, 4)
	o := NewAnnotationOverlay()
	o.Spans[0] = []ColSpan{{Start: 1, End: 2, Style: term.StyleSearchMatch}}
	o.Spans[1] = []ColSpan{{Start: 0, End: 1, Style: term.StyleSearchMatch}}
	e.SetDiffOverlay(o)
	e.Buf.InsertRune(0, 0, 'x')
	renderGrid(e)
	if got := e.DiffOverlay.Spans; len(got[0]) != 0 || len(got[1]) != 1 {
		t.Fatalf("spans = %v, want the edited line's span dropped and the other kept", got)
	}
}

func TestAnnotationsKeepGitGutterMarkers(t *testing.T) {
	e := newRowsEditor([]string{"a", "B", "c"}, 30, 6)
	e.LineNumbers = true
	e.GutterStyle = "compact"
	e.LineChanges = []diff.LineChangeKind{diff.LineUnchanged, diff.LineModified, diff.LineUnchanged}
	o := NewAnnotationOverlay()
	o.LineBg[0] = term.StyleDiffAdded
	e.SetDiffOverlay(o)
	grid := renderGrid(e)
	for y := range grid {
		if grid[y][0].Ch == '▎' {
			return
		}
	}
	t.Fatal("annotations hid the git gutter marker")
}

func TestDecorationOverlaySplitsTextRangesAndClampsToTheFile(t *testing.T) {
	o := decorationOverlay([]Decoration{
		{StartLine: 0, EndLine: 1, Style: term.StyleDiffAdded},
		{StartLine: 1, StartCol: 2, EndLine: 3, EndCol: 1, Text: true, Style: term.StyleSearchMatch},
		{StartLine: 4, StartCol: 3, EndLine: 4, EndCol: 3, Text: true, Style: term.StyleSearchMatch},
		{StartLine: 5, StartCol: 3, EndLine: 6, EndCol: 0, Text: true, Style: term.StyleSearchMatch},
		{StartLine: 8, EndLine: math.MaxInt32, Style: term.StyleDiffDeleted},
		{StartLine: 10, EndLine: 12, Style: term.StyleDiffDeleted},
	}, 10)
	if want := map[int]term.Style{0: term.StyleDiffAdded, 1: term.StyleDiffAdded, 8: term.StyleDiffDeleted, 9: term.StyleDiffDeleted}; !reflect.DeepEqual(o.LineBg, want) {
		t.Errorf("LineBg = %v, want %v", o.LineBg, want)
	}
	want := map[int][]ColSpan{
		1: {{Start: 2, End: math.MaxInt, Style: term.StyleSearchMatch}},
		2: {{Start: 0, End: math.MaxInt, Style: term.StyleSearchMatch}},
		3: {{Start: 0, End: 1, Style: term.StyleSearchMatch}},
		5: {{Start: 3, End: math.MaxInt, Style: term.StyleSearchMatch}},
	}
	if !reflect.DeepEqual(o.Spans, want) {
		t.Errorf("Spans = %v, want %v", o.Spans, want)
	}
}

func newDecoratedGroup(t *testing.T, files map[string]string) (*EditorGroupWidget, string) {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return NewEditorGroupWidget(nil, 4, false, "extended"), dir
}

func openCommitted(g *EditorGroupWidget, path string) {
	g.OpenFile(path)
	g.CommitActiveTab()
}

func tint(line int, style term.Style) Decoration {
	return Decoration{StartLine: line, EndLine: line, Style: style}
}

func TestDecorationsFollowTheirFileAcrossTabs(t *testing.T) {
	g, dir := newDecoratedGroup(t, map[string]string{"a.txt": "one\ntwo\n", "b.txt": "x\n"})
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	g.SetDecorations("p", b, []Decoration{tint(0, term.StyleDiffAdded)})
	openCommitted(g, a)
	if g.Editor.DiffOverlay != nil {
		t.Fatal("decorations of b.txt shown on a.txt")
	}
	openCommitted(g, b)
	if got := g.Editor.DiffOverlay; got == nil || got.LineBg[0] != term.StyleDiffAdded {
		t.Fatal("decorations set before b.txt opened were not applied")
	}
	g.SwitchToTabByPath(a)
	g.SetDecorations("p", a, []Decoration{tint(1, term.StyleDiffDeleted)})
	g.SwitchToTabByPath(b)
	g.SwitchToTabByPath(a)
	if got := g.Editor.DiffOverlay; got == nil || got.LineBg[1] != term.StyleDiffDeleted {
		t.Fatal("decorations lost after switching tabs")
	}
	g.SetDecorations("p", a, nil)
	if g.Editor.DiffOverlay != nil {
		t.Fatal("setting no decorations left an overlay")
	}
}

func TestDecorationSourcesMerge(t *testing.T) {
	g, dir := newDecoratedGroup(t, map[string]string{"a.txt": "one\ntwo\nthree\n"})
	a := filepath.Join(dir, "a.txt")
	openCommitted(g, a)
	g.SetDecorations("plugin:b", a, []Decoration{tint(0, term.StyleDiffDeleted), tint(1, term.StyleDiffDeleted)})
	g.SetDecorations("plugin:a", a, []Decoration{tint(1, term.StyleDiffAdded), tint(2, term.StyleDiffAdded)})
	want := map[int]term.Style{0: term.StyleDiffDeleted, 1: term.StyleDiffDeleted, 2: term.StyleDiffAdded}
	if got := g.Editor.DiffOverlay.LineBg; !reflect.DeepEqual(got, want) {
		t.Fatalf("merged tints = %v, want %v (later source wins)", got, want)
	}
	g.ClearDecorationsSource("plugin:b")
	want = map[int]term.Style{1: term.StyleDiffAdded, 2: term.StyleDiffAdded}
	if got := g.Editor.DiffOverlay.LineBg; !reflect.DeepEqual(got, want) {
		t.Fatalf("after clearing a source = %v, want %v", got, want)
	}
}

func TestDecorationSourcesKeepMovingWithEdits(t *testing.T) {
	g, dir := newDecoratedGroup(t, map[string]string{"a.txt": "one\ntwo\n"})
	a := filepath.Join(dir, "a.txt")
	openCommitted(g, a)
	g.SetDecorations("first", a, []Decoration{tint(1, term.StyleDiffAdded)})
	// More edits than the buffer's change log holds, each seen by the pane.
	for range 100 {
		g.Editor.Buf.InsertLine(0, "new")
		g.Editor.syncDiffOverlay()
	}
	g.Editor.Buf.InsertLine(0, "unrendered")
	g.SetDecorations("second", a, []Decoration{tint(0, term.StyleDiffDeleted)})
	want := map[int]term.Style{0: term.StyleDiffDeleted, 102: term.StyleDiffAdded}
	if got := g.Editor.DiffOverlay.LineBg; !reflect.DeepEqual(got, want) {
		t.Fatalf("tints = %v, want %v", got, want)
	}
}

func TestDecorationsOnInactiveTabFollowItsEdits(t *testing.T) {
	g, dir := newDecoratedGroup(t, map[string]string{"a.txt": "one\ntwo\n", "b.txt": "x\n"})
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	openCommitted(g, a)
	openCommitted(g, b)
	g.SetDecorations("p", a, []Decoration{tint(1, term.StyleDiffAdded)})
	g.tabs[g.tabIndexByPath(a)].Buf.InsertLine(0, "zero")
	g.SwitchToTabByPath(a)
	renderGrid(g.Editor)
	if got := g.Editor.DiffOverlay.LineBg; !reflect.DeepEqual(got, map[int]term.Style{2: term.StyleDiffAdded}) {
		t.Fatalf("tint = %v, want it on line 2 after the insert above", got)
	}
}

func TestDecorationTintGoesWithADeletedLine(t *testing.T) {
	g, dir := newDecoratedGroup(t, map[string]string{"a.txt": "l0\nl1\nl2\nl3\n"})
	a := filepath.Join(dir, "a.txt")
	openCommitted(g, a)
	g.SetDecorations("p", a, []Decoration{tint(1, term.StyleDiffAdded), tint(2, term.StyleDiffDeleted)})
	g.Editor.Buf.DeleteLine(1)
	renderGrid(g.Editor)
	if got := g.Editor.DiffOverlay.LineBg; !reflect.DeepEqual(got, map[int]term.Style{1: term.StyleDiffDeleted}) {
		t.Fatalf("tints = %v, want only l2's tint, now on line 1", got)
	}
}

func TestDecorationsOnLinesAReloadRemovesAreDropped(t *testing.T) {
	g, dir := newDecoratedGroup(t, map[string]string{"a.txt": "0\n1\n2\n3\n4\n5\n6\n"})
	a := filepath.Join(dir, "a.txt")
	openCommitted(g, a)
	g.SetDecorations("p", a, []Decoration{tint(1, term.StyleDiffAdded), tint(4, term.StyleDiffAdded), tint(5, term.StyleDiffDeleted)})
	if err := os.WriteFile(a, []byte("0\n1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g.ReloadFile(a)
	renderGrid(g.Editor)
	if got := g.Editor.DiffOverlay.LineBg; !reflect.DeepEqual(got, map[int]term.Style{1: term.StyleDiffAdded}) {
		t.Fatalf("tints = %v, want only the tint on a line that still exists", got)
	}
}

func TestDecorationsDropWhenTooManyEditsGoUnseen(t *testing.T) {
	g, dir := newDecoratedGroup(t, map[string]string{"a.txt": "one\ntwo\n"})
	a := filepath.Join(dir, "a.txt")
	openCommitted(g, a)
	g.SetDecorations("p", a, []Decoration{tint(1, term.StyleDiffAdded)})
	for range 65 {
		g.Editor.Buf.InsertLine(0, "new")
	}
	g.Editor.SetRect(Rect{W: 30, H: 5})
	grid := renderGrid(g.Editor)
	if got := grid[1][g.Editor.GutterWidth()].BgStyle; got == term.StyleDiffAdded {
		t.Fatal("tint drawn on a line it never belonged to")
	}
	g.SetDecorations("q", a, []Decoration{tint(0, term.StyleDiffDeleted)})
	if got := g.Editor.DiffOverlay.LineBg; !reflect.DeepEqual(got, map[int]term.Style{0: term.StyleDiffDeleted}) {
		t.Fatalf("tints = %v, want the stale source dropped and the new one shown", got)
	}
}

func TestDecorationsFollowSaveAs(t *testing.T) {
	g, dir := newDecoratedGroup(t, map[string]string{"a.txt": "one\ntwo\n"})
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	openCommitted(g, a)
	g.SetDecorations("plugin:x", a, []Decoration{tint(1, term.StyleDiffAdded)})
	if !g.SaveAs(b) {
		t.Fatal("SaveAs failed")
	}
	g.ClearDecorationsSource("plugin:x")
	if g.Editor.DiffOverlay != nil {
		t.Fatalf("decorations left after their source was cleared: %v", g.Editor.DiffOverlay.LineBg)
	}
}

func TestDecorationsAreDroppedWhenATabClosesWithUnsavedEdits(t *testing.T) {
	g, dir := newDecoratedGroup(t, map[string]string{"a.txt": "one\ntwo\n", "b.txt": "x\n"})
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	openCommitted(g, b)
	openCommitted(g, a)
	g.SetDecorations("p", a, []Decoration{tint(0, term.StyleDiffAdded)})
	g.Editor.Buf.InsertLine(0, "new")
	g.Editor.Buf.Dirty = true
	renderGrid(g.Editor)
	g.CloseTab()
	openCommitted(g, a)
	if g.Editor.DiffOverlay != nil {
		t.Fatalf("tints = %v after reopening, want none", g.Editor.DiffOverlay.LineBg)
	}

	g.SetDecorations("p", a, []Decoration{tint(1, term.StyleDiffAdded)})
	g.CloseTab()
	openCommitted(g, a)
	if got := g.Editor.DiffOverlay; got == nil || !reflect.DeepEqual(got.LineBg, map[int]term.Style{1: term.StyleDiffAdded}) {
		t.Fatal("decorations of a tab closed without edits did not come back")
	}
}

func TestAnnotationReverseSwapsTextColorsOnly(t *testing.T) {
	e := newRowsEditor([]string{"x", "abc", "def"}, 20, 4)
	o := NewAnnotationOverlay()
	o.LineBg[1] = term.StyleReverse
	o.Spans[2] = []ColSpan{{Start: 1, End: 2, Style: term.StyleReverse}}
	e.SetDiffOverlay(o)
	grid := renderGrid(e)
	gutterW := e.GutterWidth()
	for _, tc := range []struct {
		y, x     int
		reversed bool
	}{{1, 0, true}, {1, 2, true}, {1, 3, false}, {2, 0, false}, {2, 1, true}, {2, 2, false}} {
		cell := grid[tc.y][gutterW+tc.x]
		if got := cell.Attrs&term.CellAttrReverse != 0; got != tc.reversed {
			t.Errorf("row %d column %d reversed = %v, want %v", tc.y, tc.x, got, tc.reversed)
		}
		if cell.BgStyle == term.StyleReverse {
			t.Errorf("row %d column %d kept StyleReverse as its background", tc.y, tc.x)
		}
	}
}

func TestAnnotationReverseKeepsTheActiveLinePastTheText(t *testing.T) {
	e := newRowsEditor([]string{"x", "abc", "def"}, 20, 4)
	e.Cursor.Line = 1
	o := NewAnnotationOverlay()
	o.LineBg[1] = term.StyleReverse
	e.SetDiffOverlay(o)
	grid := renderGrid(e)
	cell := grid[1][e.GutterWidth()+5]
	if cell.BgStyle != term.StyleActiveLine || cell.Attrs != 0 {
		t.Errorf("cell past the text = bg %v attrs %v, want the active line and no attrs", cell.BgStyle, cell.Attrs)
	}
}
