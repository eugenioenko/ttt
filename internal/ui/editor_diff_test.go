package ui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v3"
)

func deletedTexts(o *DiffOverlay) map[int][]string {
	out := map[int][]string{}
	for anchor, block := range o.Deleted {
		for _, l := range block {
			out[anchor] = append(out[anchor], l.Text)
		}
	}
	return out
}

func renderGrid(e *EditorPaneWidget) [][]term.Cell {
	r := e.GetRect()
	grid := makeGrid(r.W, r.H)
	e.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: r.W, H: r.H}))
	return grid
}

func gutterText(grid [][]term.Cell, y, gutterW int) string {
	var sb strings.Builder
	for x := 0; x < gutterW; x++ {
		sb.WriteRune(grid[y][x].Ch)
	}
	return strings.TrimSpace(sb.String())
}

func TestNewDiffOverlay(t *testing.T) {
	C, A := diff.Context, diff.Added
	tests := []struct {
		name      string
		old, new  []string
		wantKinds []diff.LineKind
		wantDel   map[int][]string
	}{
		{"modify and append", []string{"a", "b", "c", "d"}, []string{"a", "B", "c", "e", "f"},
			[]diff.LineKind{C, A, C, A, A}, map[int][]string{1: {"b"}, 3: {"d"}}},
		{"delete at end of file", []string{"a", "b"}, []string{"a"},
			[]diff.LineKind{C}, map[int][]string{1: {"b"}}},
		{"delete in middle", []string{"a", "b", "c"}, []string{"a", "c"},
			[]diff.LineKind{C, C}, map[int][]string{1: {"b"}}},
		{"block replace", []string{"a", "x", "y", "z"}, []string{"a", "P", "Q"},
			[]diff.LineKind{C, A, A}, map[int][]string{1: {"x", "y", "z"}}},
		{"no changes", []string{"a"}, []string{"a"},
			[]diff.LineKind{C}, map[int][]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewDiffOverlay(diff.FullDiffLines(tt.old, tt.new))
			if !reflect.DeepEqual(o.Kinds, tt.wantKinds) {
				t.Errorf("kinds = %v, want %v", o.Kinds, tt.wantKinds)
			}
			if got := deletedTexts(o); !reflect.DeepEqual(got, tt.wantDel) {
				t.Errorf("deleted = %v, want %v", got, tt.wantDel)
			}
		})
	}
}

func TestNewSplitDiffOverlaysAlignRows(t *testing.T) {
	old := []string{"a", "b", "c", "d", "e"}
	cur := []string{"a", "B", "x", "y", "c", "e", "f"}
	left, right := NewSplitDiffOverlays(diff.FullDiffLines(old, cur))
	if !reflect.DeepEqual(left.Lines, old) || !reflect.DeepEqual(right.Lines, cur) {
		t.Fatalf("pane lines: left %v right %v", left.Lines, right.Lines)
	}
	if left.Overlay.Nums[3] != 4 || right.Overlay.Nums[6] != 7 {
		t.Fatalf("nums: left %v right %v", left.Overlay.Nums, right.Overlay.Nums)
	}

	leftPane := newRowsEditor(left.Lines, 30, 12)
	rightPane := newRowsEditor(right.Lines, 30, 12)
	leftPane.SetDiffOverlay(left.Overlay)
	rightPane.SetDiffOverlay(right.Overlay)
	ll, rl := leftPane.layout(), rightPane.layout()
	if ll.total() != rl.total() {
		t.Fatalf("row totals differ: left %d right %d", ll.total(), rl.total())
	}
	for _, pair := range [][2]int{{0, 0}, {2, 4}, {4, 5}} {
		l, _ := ll.rowOf(pair[0], 0)
		r, _ := rl.rowOf(pair[1], 0)
		if l != r {
			t.Fatalf("old line %d at row %d, new line %d at row %d", pair[0], l, pair[1], r)
		}
	}
}

func TestSplitDiffGapLinesCarryLabels(t *testing.T) {
	ctx := func(l, r int, text string) diff.DiffLine {
		return diff.DiffLine{Left: diff.SideLine{Num: l, Text: text, Kind: diff.Context}, Right: diff.SideLine{Num: r, Text: text, Kind: diff.Context}}
	}
	gap := func(n int) diff.DiffLine {
		label := collapsedDistanceLabel(n)
		return diff.DiffLine{Left: diff.SideLine{Text: label, Kind: diff.Collapsed}, Right: diff.SideLine{Text: label, Kind: diff.Collapsed}}
	}
	lines := []diff.DiffLine{
		ctx(1, 1, "a"),
		{Left: diff.SideLine{Kind: diff.Blank}, Right: diff.SideLine{Num: 2, Text: "new", Kind: diff.Added}},
		gap(1),
		ctx(3, 4, "c"),
		gap(4),
		ctx(8, 9, "h"),
	}
	left, right := NewSplitDiffOverlays(lines)

	byGap := func(side DiffSide) map[int]string {
		out := map[int]string{}
		for line, g := range side.Overlay.Gaps {
			if side.Overlay.Kinds[line] != diff.Collapsed || side.Lines[line] != side.Overlay.Labels[line] {
				t.Fatalf("gap line %d: kind %v, text %q, label %q", line, side.Overlay.Kinds[line], side.Lines[line], side.Overlay.Labels[line])
			}
			out[g] = side.Overlay.Labels[line]
		}
		return out
	}
	want := map[int]string{0: " ⋯ 1 line ⋯", 1: " ⋯ 4 lines ⋯"}
	if got := byGap(left); !reflect.DeepEqual(got, want) {
		t.Fatalf("left gaps = %q, want %q", got, want)
	}
	if got := byGap(right); !reflect.DeepEqual(got, want) {
		t.Fatalf("right gaps = %q, want %q", got, want)
	}
	if left.Overlay.Fillers[1] != 1 || left.Lines[1] != collapsedDistanceLabel(1) || right.Lines[2] != collapsedDistanceLabel(1) {
		t.Fatalf("filler before the first gap: left %v %q, right %q", left.Overlay.Fillers, left.Lines, right.Lines)
	}
	if left.Overlay.Nums[1] != 0 || right.Overlay.Nums[5] != 9 {
		t.Fatalf("nums: left %v right %v", left.Overlay.Nums, right.Overlay.Nums)
	}
}

func TestEditorRendersDiffOverlay(t *testing.T) {
	oldLines := []string{"a", "b", "c", "d"}
	newLines := []string{"a", "B", "c", "e", "f"}
	e := newRowsEditor(newLines, 20, 8)
	e.LineNumbers = true
	o := NewDiffOverlay(diff.FullDiffLines(oldLines, newLines))
	o.GutterHighlight = true
	e.SetDiffOverlay(o)

	grid := renderGrid(e)
	g := e.GutterWidth()
	rows := renderRows(e)
	want := []string{"a", "b", "B", "c", "d", "e", "f", ""}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %q, want %q", rows, want)
	}
	if grid[1][g].BgStyle != term.StyleDiffDeleted || grid[1][0].BgStyle != term.StyleDiffDeleted {
		t.Errorf("deleted row not tinted: %+v %+v", grid[1][0], grid[1][g])
	}
	if got := gutterText(grid, 1, g); got != "2" {
		t.Errorf("deleted row gutter = %q, want old line number 2", got)
	}
	if grid[2][g].BgStyle != term.StyleDiffAdded || grid[2][g+5].BgStyle != term.StyleDiffAdded {
		t.Errorf("added row not tinted: %+v", grid[2][g])
	}
	if got := gutterText(grid, 2, g); got != "2" {
		t.Errorf("added row gutter = %q, want its buffer line number 2", got)
	}
	if grid[3][g].BgStyle == term.StyleDiffAdded {
		t.Errorf("context row tinted")
	}
}

func TestEditorRendersFillerAndGapRows(t *testing.T) {
	old := []string{"a", "b", "c"}
	cur := []string{"a", "x", "y", "b", "c"}
	left, _ := NewSplitDiffOverlays(diff.FullDiffLines(old, cur))
	left.Overlay.Labels[2] = " ⋯ 4 lines ⋯"
	e := newRowsEditor(left.Lines, 30, 6)
	e.LineNumbers = true
	e.SetDiffOverlay(left.Overlay)

	grid := renderGrid(e)
	rows := renderRows(e)
	want := []string{"a", "", "", "b", " ⋯ 4 lines ⋯", ""}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %q, want %q", rows, want)
	}
	g := e.GutterWidth()
	if got := gutterText(grid, 1, g); got != "" {
		t.Errorf("filler row gutter = %q, want blank", got)
	}
	if grid[4][g+1].Style != term.StyleMuted {
		t.Errorf("gap label style = %v, want muted", grid[4][g+1].Style)
	}
	if got := gutterText(grid, 4, g); got != "▶" {
		t.Errorf("gap row gutter = %q, want the expand chevron", got)
	}
}

func TestDiffGutterFitsSourceLineNumbers(t *testing.T) {
	e := newRowsEditor([]string{"a", "b"}, 30, 4)
	e.LineNumbers = true
	plain := e.GutterWidth()
	e.SetDiffOverlay(&DiffOverlay{Kinds: []diff.LineKind{diff.Context, diff.Context}, Nums: []int{12345, 12346}})
	if e.GutterWidth() != plain+3 {
		t.Fatalf("gutter width = %d, want %d for five-digit numbers", e.GutterWidth(), plain+3)
	}
	grid := renderGrid(e)
	if got := gutterText(grid, 0, e.GutterWidth()); got != "12345" {
		t.Fatalf("gutter = %q, want the source line number", got)
	}
	e.SetDiffOverlay(nil)
	if e.GutterWidth() != plain {
		t.Fatalf("gutter width after clearing the overlay = %d, want %d", e.GutterWidth(), plain)
	}
}

func TestDeletedRowsWrap(t *testing.T) {
	long := strings.Repeat("abcdefghij", 3)
	e := newRowsEditor([]string{"keep", "new"}, 16, 8)
	e.LineNumbers = true
	e.WordWrap = true
	e.SetDiffOverlay(NewDiffOverlay(diff.FullDiffLines([]string{"keep", long}, []string{"keep", "new"})))
	rows := renderRows(e)
	grid := renderGrid(e)
	g := e.GutterWidth()

	if rows[0] != "keep" || !strings.HasPrefix(long, rows[1]) || rows[1] == long {
		t.Fatalf("deleted line did not wrap: %q", rows)
	}
	segs := 0
	for y := 1; y < len(rows) && rows[y] != "new"; y++ {
		segs++
		if grid[y][g].BgStyle != term.StyleDiffDeleted {
			t.Fatalf("wrapped deleted row %d not tinted", y)
		}
	}
	if segs < 2 || strings.Join(rows[1:1+segs], "") != long {
		t.Fatalf("deleted segments = %q, want %q split over rows", rows[1:1+segs], long)
	}
	if gutterText(grid, 1, g) != "2" || gutterText(grid, 2, g) != "" {
		t.Fatalf("old line number should show only on the first segment: %q %q", gutterText(grid, 1, g), gutterText(grid, 2, g))
	}
	if total := e.layout().total(); total != 2+segs {
		t.Fatalf("layout total = %d, want %d", total, 2+segs)
	}
}

func TestCursorSkipsDiffRows(t *testing.T) {
	e := newRowsEditor([]string{"a", "B", "c"}, 20, 8)
	e.LineNumbers = true
	e.SetDiffOverlay(NewDiffOverlay(diff.FullDiffLines([]string{"a", "b1", "b2", "c"}, []string{"a", "B", "c"})))
	renderRows(e)
	r := e.GetRect()

	e.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", 0))
	renderRows(e)
	if e.Cursor.Line != 1 || e.CursorY != r.Y+3 {
		t.Fatalf("down from line 0 = line %d at row %d, want line 1 at row 3 below two deleted rows", e.Cursor.Line, e.CursorY-r.Y)
	}
	e.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", 0))
	renderRows(e)
	if e.Cursor.Line != 0 || e.CursorY != r.Y {
		t.Fatalf("up = line %d at row %d, want line 0 at row 0", e.Cursor.Line, e.CursorY-r.Y)
	}

	clickAt(e, r.X+e.GutterWidth(), r.Y+2)
	if e.Cursor.Line != 1 {
		t.Fatalf("click on a deleted row = line %d, want the line it precedes (1)", e.Cursor.Line)
	}
	if total := e.layout().total(); total != 5 {
		t.Fatalf("row count = %d, want 3 buffer lines + 2 deleted rows", total)
	}
}

func TestScrollbarCountsDiffRows(t *testing.T) {
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, "line")
	}
	e := newRowsEditor(lines, 20, 5)
	o := &DiffOverlay{Kinds: make([]diff.LineKind, len(lines)), Deleted: map[int][]diff.SideLine{}, Fillers: map[int]int{3: 4}}
	for i := 0; i < 6; i++ {
		o.Deleted[5] = append(o.Deleted[5], diff.SideLine{Num: i + 1, Text: "gone", Kind: diff.Deleted})
	}
	e.SetDiffOverlay(o)
	renderRows(e)
	if e.scrollbar.TotalItems != 20+5-1 {
		t.Fatalf("scrollbar total = %d, want 20 rows (10 lines, 4 fillers, 6 deleted) + height - 1", e.scrollbar.TotalItems)
	}
	e.setTopRow(e.layout(), 9)
	rows := renderRows(e)
	if rows[0] != "gone" || e.Viewport.TopLine != 5 {
		t.Fatalf("top row 9 = %q at top line %d, want a deleted row anchored at line 5", rows[0], e.Viewport.TopLine)
	}
}

func TestEditableDiffOverlay(t *testing.T) {
	C, A := diff.Context, diff.Added
	tests := []struct {
		name      string
		old, new  []string
		wantKinds []diff.LineKind
		wantDel   map[int][]string
	}{
		{"modify and append", []string{"a", "b", "c", "d"}, []string{"a", "B", "c", "e", "f"},
			[]diff.LineKind{C, A, C, A, A}, map[int][]string{1: {"b"}, 3: {"d"}}},
		{"delete at end", []string{"a", "b"}, []string{"a"},
			[]diff.LineKind{C}, map[int][]string{1: {"b"}}},
		{"delete in middle", []string{"a", "b", "c"}, []string{"a", "c"},
			[]diff.LineKind{C, C}, map[int][]string{1: {"b"}}},
		{"block replace", []string{"a", "x", "y", "z"}, []string{"a", "P", "Q"},
			[]diff.LineKind{C, A, A}, map[int][]string{1: {"x", "y", "z"}}},
		{"no changes", []string{"a"}, []string{"a"},
			[]diff.LineKind{C}, map[int][]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := fullLiveDiff(tt.old, tt.new).liveUnified
			if !reflect.DeepEqual(o.Kinds, tt.wantKinds) {
				t.Errorf("kinds = %v, want %v", o.Kinds, tt.wantKinds)
			}
			if got := deletedTexts(o); !reflect.DeepEqual(got, tt.wantDel) {
				t.Errorf("deleted = %v, want %v", got, tt.wantDel)
			}
		})
	}
}

func TestEditableSplitAlignsRows(t *testing.T) {
	old := []string{"a", "b", "c", "d", "e"}
	cur := []string{"a", "B", "x", "y", "c", "e", "f"}
	d := fullLiveDiff(old, cur)
	d.SetMode(DiffModeSplit)
	e := newRowsEditor(cur, 30, 10)
	e.LineNumbers = true
	d.bind(e)
	d.SetRect(Rect{X: 0, Y: 0, W: 61, H: 10})
	grid := makeGrid(61, 10)
	d.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 61, H: 10}))

	row := func(y int) string {
		var sb strings.Builder
		for x := 0; x < 61; x++ {
			sb.WriteRune(grid[y][x].Ch)
		}
		return sb.String()
	}
	for _, text := range []string{"a", "c", "e"} {
		found := false
		for y := 0; y < 10; y++ {
			r := row(y)
			left, right := r[:strings.IndexRune(r, '│')], r[strings.IndexRune(r, '│'):]
			if strings.Contains(left, " "+text+" ") && strings.Contains(right, " "+text+" ") {
				found = true
			}
		}
		if !found {
			t.Fatalf("context line %q not on one row in both panes:\n%s", text, strings.Join(func() []string {
				var out []string
				for y := 0; y < 10; y++ {
					out = append(out, row(y))
				}
				return out
			}(), "\n"))
		}
	}
}

func TestEditableAndReadOnlyDiffGuttersMatch(t *testing.T) {
	oldLines := []string{"a", "b", "c"}
	newLines := []string{"a", "B", "c"}
	for _, style := range []string{"compact", "extended"} {
		e := newRowsEditor(newLines, 30, 6)
		e.LineNumbers = true
		e.GutterStyle = style
		e.Passive = true
		e.SetDiffOverlay(fullLiveDiff(oldLines, newLines).liveUnified)
		editable := makeGrid(30, 6)
		e.Render(NewRenderSurface(editable, Rect{W: 30, H: 6}))

		d := NewDiffEditorWidget("f.txt", diff.FileDiff{}, oldLines, newLines, true)
		d.SetMode(DiffModeUnified)
		d.setGutterStyle(style)
		readOnly := makeGrid(30, 6)
		d.SetRect(Rect{W: 30, H: 6})
		d.Render(NewRenderSurface(readOnly, Rect{W: 30, H: 6}))

		for row, kind := range []string{"context", "deleted", "added", "context"} {
			got := readGutter(editable, row, e.GutterWidth())
			want := readGutter(readOnly, row, d.unified.GutterWidth())
			if got != want {
				t.Errorf("%s gutter, %s row %d: editable %+v, read-only %+v", style, kind, row, got, want)
			}
		}
	}
}

func fullLiveDiff(old, cur []string) *DiffEditorWidget {
	d := NewEditableDiffWidget("f.txt", old, diff.FullDiffLines(old, cur))
	d.SetMode(DiffModeUnified)
	d.SetContextMode(DiffContextFullFile)
	return d
}
