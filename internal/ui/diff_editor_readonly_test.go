package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v3"
)

func gapFile() (old, cur []string) {
	for i := 1; i <= 30; i++ {
		line := fmt.Sprintf("line %02d", i)
		old = append(old, line)
		if i == 15 {
			line = "changed 15"
		}
		cur = append(cur, line)
	}
	return old, cur
}

func commentDiffSides() (old, cur []string) {
	old = []string{"package main", "", "/*"}
	for i := 0; i < 8; i++ {
		old = append(old, "inside the comment")
	}
	old = append(old, "removed inside comment", "*/", "", "func main() {}")
	for _, line := range old {
		if line != "removed inside comment" {
			cur = append(cur, line)
		}
	}
	return old, cur
}

func styleOfText(t *testing.T, grid [][]term.Cell, text string) term.Style {
	t.Helper()
	for y := range grid {
		row := cellRow(grid, y)
		if i := strings.Index(row, text); i >= 0 {
			return grid[y][len([]rune(row[:i]))].Style
		}
	}
	t.Fatalf("%q not on screen", text)
	return 0
}

func diffWidgetRows(d *DiffEditorWidget, w, h int) []string {
	grid := renderDiffEditor(d, w, h)
	rows := make([]string, h)
	for y := range grid {
		rows[y] = cellRow(grid, y)
	}
	return rows
}

type gutterCells struct{ num, bg, sign term.Style }

func readGutter(grid [][]term.Cell, y, width int) gutterCells {
	var g gutterCells
	for x := 0; x < width; x++ {
		c := grid[y][x]
		switch {
		case c.Ch >= '0' && c.Ch <= '9' && g.num == 0:
			g.num, g.bg = c.Style, c.BgStyle
		case c.Ch == '+' || c.Ch == '−':
			g.sign = c.Style
		}
	}
	return g
}

func TestDiffEditorRemovedLineKeepsMultiLineCommentState(t *testing.T) {
	old, cur := commentDiffSides()
	for _, mode := range []DiffMode{DiffModeUnified, DiffModeSplit} {
		d := NewDiffEditorWidget("main.go", diff.Parse(diff.Generate(old, cur, "main.go")), old, cur, false)
		d.SetMode(mode)
		grid := renderDiffEditor(d, 100, 20)
		if s := styleOfText(t, grid, "removed inside"); s != term.StyleSyntaxComment {
			t.Fatalf("mode %v: removed line style %v, want comment", mode, s)
		}
	}
}

func TestDiffEditorShowsGapsAroundTheChanges(t *testing.T) {
	old, cur := gapFile()
	d := NewDiffEditorWidget("f.txt", diff.Parse(diff.Generate(old, cur, "f.txt")), old, cur, false)
	d.SetUnified(true)
	text := strings.Join(d.unified.Buf.Lines, "\n")
	if !strings.Contains(text, "⋯ 11 lines ⋯") || !strings.Contains(text, "⋯ 12 lines ⋯") {
		t.Fatalf("rows = %q, want gaps above and below the change", d.unified.Buf.Lines)
	}
	d.expandContextGap(d.unified.DiffOverlay.Gaps[0])
	if !strings.Contains(strings.Join(d.unified.Buf.Lines, "\n"), "line 01") {
		t.Fatalf("expanding the leading gap = %q, want the unchanged lines above the change", d.unified.Buf.Lines)
	}
}

func TestDiffEditorLeadingGapFetchesFileContent(t *testing.T) {
	old, cur := gapFile()
	d := NewDiffEditorWidget("f.txt", diff.Parse(diff.Generate(old, cur, "f.txt")), nil, nil, false)
	d.SetUnified(true)
	if d.unified.Buf.Lines[0] != collapsedDistanceLabel(11) {
		t.Fatalf("first row = %q, want the leading gap before the file is fetched", d.unified.Buf.Lines[0])
	}
	fetched := 0
	d.SetExtendedFetcher(func(dv *DiffEditorWidget) { fetched++ })
	d.unified.Cursor.Line = 0
	d.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if fetched != 1 || !d.Loading {
		t.Fatalf("Enter on the leading gap: fetched %d, loading %v", fetched, d.Loading)
	}
	d.SetOldLines(old)
	d.SetNewLines(cur)
	d.FinishLoading()
	if d.unified.Buf.Lines[0] != "line 01" || d.IsExtended() {
		t.Fatalf("after the fetch: first row %q, extended %v; want the gap expanded in changes-only mode", d.unified.Buf.Lines[0], d.IsExtended())
	}
}

func TestDiffEditorGapRowsFollowConfiguredChevron(t *testing.T) {
	old, cur := gapFile()
	for _, mode := range []DiffMode{DiffModeUnified, DiffModeSplit} {
		d := NewDiffEditorWidget("f.txt", diff.Parse(diff.Generate(old, cur, "f.txt")), old, cur, false)
		d.SetMode(mode)
		d.SetChevrons('»', 'v')
		rows := diffWidgetRows(d, 100, 20)
		found := false
		for _, row := range rows {
			if strings.Contains(row, "⋯ 11 lines ⋯") {
				found = true
				if !strings.ContainsRune(row, '»') || strings.ContainsRune(row, '▶') {
					t.Fatalf("mode %v: gap row %q, want the configured chevron", mode, row)
				}
			}
		}
		if !found {
			t.Fatalf("mode %v: no gap row in\n%s", mode, strings.Join(rows, "\n"))
		}
	}
}

func TestDiffEditorSignsFollowSettingAndGutterStyle(t *testing.T) {
	d := NewDiffEditorWidget("f.txt", diff.FileDiff{}, []string{"a", "b"}, []string{"a", "B"}, true)
	d.SetMode(DiffModeUnified)
	gutter := func() string {
		grid := renderDiffEditor(d, 30, 4)
		var sb strings.Builder
		for y := 0; y < 4; y++ {
			for x := 0; x < d.unified.GutterWidth(); x++ {
				sb.WriteRune(grid[y][x].Ch)
			}
		}
		return sb.String()
	}
	if g := gutter(); strings.ContainsAny(g, "+−") {
		t.Fatalf("signs drawn by default: %q", g)
	}
	d.SetDiffSigns(true)
	if g := gutter(); !strings.ContainsRune(g, '+') || !strings.ContainsRune(g, '−') {
		t.Fatalf("signs missing while on: %q", g)
	}
	d.setGutterStyle("minimal")
	if g := gutter(); strings.ContainsAny(g, "+−") {
		t.Fatalf("signs drawn in the minimal gutter: %q", g)
	}
}

func TestDiffEditorGutterHighlightAndSignColors(t *testing.T) {
	render := func(highlight, signsColor bool) [2]gutterCells {
		d := NewDiffEditorWidget("f.txt", diff.FileDiff{}, []string{"a", "b", "c"}, []string{"a", "B", "c"}, true)
		d.SetMode(DiffModeUnified)
		d.SetDiffSigns(true)
		d.SetDiffGutterHighlight(highlight)
		d.SetDiffSignsColor(signsColor)
		grid := renderDiffEditor(d, 30, 6)
		w := d.unified.GutterWidth()
		return [2]gutterCells{readGutter(grid, 1, w), readGutter(grid, 2, w)}
	}
	for _, tt := range []struct {
		highlight, signsColor bool
		want                  [2]gutterCells
	}{
		{true, true, [2]gutterCells{
			{num: term.StyleGutterDeleted, bg: term.StyleDiffDeleted, sign: term.StyleGutterDeleted},
			{num: term.StyleGutterAdded, bg: term.StyleDiffAdded, sign: term.StyleGutterAdded},
		}},
		{false, true, [2]gutterCells{
			{num: term.StyleLineNumber, sign: term.StyleGutterDeleted},
			{num: term.StyleLineNumber, sign: term.StyleGutterAdded},
		}},
		{true, false, [2]gutterCells{
			{num: term.StyleGutterDeleted, bg: term.StyleDiffDeleted, sign: term.StyleLineNumber},
			{num: term.StyleGutterAdded, bg: term.StyleDiffAdded, sign: term.StyleLineNumber},
		}},
	} {
		if got := render(tt.highlight, tt.signsColor); got != tt.want {
			t.Errorf("highlight %v, signs color %v: %+v, want %+v", tt.highlight, tt.signsColor, got, tt.want)
		}
	}
}

func TestDiffEditorPanesNeverFold(t *testing.T) {
	old, cur := gapFile()
	d := NewDiffEditorWidget("f.go", diff.Parse(diff.Generate(old, cur, "f.go")), old, cur, true)
	for _, mode := range []DiffMode{DiffModeUnified, DiffModeSplit} {
		d.SetMode(mode)
		renderDiffEditor(d, 100, 20)
		for _, p := range d.panes() {
			if p.Folds != nil {
				t.Fatalf("mode %v: a diff pane has fold state", mode)
			}
		}
	}
}

func TestDiffEditorCloseTurnsOffBackgroundHighlighting(t *testing.T) {
	var old, cur []string
	for i := 0; i < 30000; i++ {
		line := fmt.Sprintf("var v%d = %d // comment", i, i)
		old = append(old, line)
		if i == 5 {
			line = "var changed = 1"
		}
		cur = append(cur, line)
	}
	d := NewDiffEditorWidget("f.go", diff.Parse(diff.Generate(old, cur, "f.go")), old, cur, false)
	d.SetRedrawRequest(func() {})
	d.SetMode(DiffModeSplit)
	renderDiffEditor(d, 120, 20)
	d.Close()
	d.SetContextMode(DiffContextFullFile)
	d.ScrollToLine(29000)
	renderDiffEditor(d, 120, 20)
	if n := highlight.Warming(); n != 0 {
		t.Fatalf("a closed diff started %d background highlighters", n)
	}
}

func TestDiffEditorSplitWrapClearsColumnsPastTheSharedWidth(t *testing.T) {
	old := []string{"short", strings.Repeat("old words ", 12)}
	var cur []string
	for i := 0; i < 11; i++ {
		cur = append(cur, fmt.Sprintf("row %d", i))
	}
	cur = append(cur, strings.Repeat("new text ", 14))
	d := NewDiffEditorWidget("f.txt", diff.FileDiff{}, old, cur, true)
	d.SetMode(DiffModeSplit)
	d.SetWrapped(true)
	const w, h = 61, 24
	grid := renderDiffEditor(d, w, h)
	p := d.left
	if p.wrapCols == 0 || p.GutterWidth() == d.right.GutterWidth() {
		t.Fatal("fixture: the panes' gutters should differ so the split shares a capped wrap width")
	}
	pr := p.GetRect()
	for y := 0; y < h; y++ {
		for x := pr.X + p.GutterWidth() + p.Viewport.Width; x < pr.X+pr.W-1; x++ {
			if ch := grid[y][x].Ch; ch != ' ' {
				t.Fatalf("row %d column %d = %q past the shared wrap width: %q", y, x, ch, cellRow(grid, y))
			}
		}
	}
}

func TestDiffEditorUnifiedIndexMatchesAScanOfTheRows(t *testing.T) {
	old, cur := gapFile()
	cur = append(cur[:5], append([]string{"inserted a", "inserted b"}, cur[5:]...)...)
	cur = append(cur[:20], cur[22:]...)
	d := NewDiffEditorWidget("f.txt", diff.FileDiff{}, old, cur, true)
	d.SetMode(DiffModeUnified)
	scan := func(row int, right bool) int {
		for i, u := range d.unifiedRows {
			if u.sourceLine == row && u.right == right {
				return i
			}
		}
		return -1
	}
	for row := -1; row <= len(d.Lines); row++ {
		for _, right := range []bool{false, true} {
			if got, want := d.unifiedIndex(row, right), scan(row, right); got != want {
				t.Fatalf("unifiedIndex(%d, %v) = %d, want %d", row, right, got, want)
			}
		}
	}
}

func TestDiffEditorRemembersAMissingGrammar(t *testing.T) {
	d := NewDiffEditorWidget("notes.unknown-extension-xyz", diff.FileDiff{}, []string{"a"}, []string{"b"}, true)
	d.SetSyntaxHighlight(true)
	renderDiffEditor(d, 40, 5)
	if !d.noGrammar {
		t.Fatal("a file with no grammar should be remembered after the first lookup")
	}
	for _, p := range d.panes() {
		if p.Highlighter != nil {
			t.Fatal("a pane got a highlighter for a file with no grammar")
		}
	}
	d.FilePath = "main.go"
	d.attachHighlighters()
	if d.noGrammar {
		t.Fatal("attaching highlighters again should forget the miss")
	}
}

func TestDiffPanesHideGitGutterMarkers(t *testing.T) {
	marker := func(withDiff bool) bool {
		e := newRowsEditor([]string{"a", "B", "c"}, 30, 6)
		e.LineNumbers = true
		e.GutterStyle = "compact"
		e.LineChanges = []diff.LineChangeKind{diff.LineUnchanged, diff.LineModified, diff.LineUnchanged}
		if withDiff {
			e.SetDiffOverlay(NewDiffOverlay(diff.FullDiffLines([]string{"a", "b", "c"}, []string{"a", "B", "c"})))
		}
		grid := makeGrid(30, 6)
		e.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 30, H: 6}))
		for y := range grid {
			if grid[y][0].Ch == '▎' {
				return true
			}
		}
		return false
	}
	if !marker(false) {
		t.Fatal("fixture: a plain pane should draw the git gutter marker")
	}
	if marker(true) {
		t.Fatal("a diff pane drew a git gutter marker; the diff already marks its changes")
	}
}
