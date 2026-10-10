package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func renderDiffEditor(d *DiffEditorWidget, w, h int) [][]term.Cell {
	grid := makeGrid(w, h)
	d.SetRect(Rect{W: w, H: h})
	d.Render(NewRenderSurface(grid, Rect{W: w, H: h}))
	return grid
}

func diffEditorRowOf(grid [][]term.Cell, text string) int {
	for y := range grid {
		if strings.Contains(cellRow(grid, y), text) {
			return y
		}
	}
	return -1
}

func diffEditorTopText(d *DiffEditorWidget) string {
	p := d.lead()
	l := p.layout()
	line, _ := l.rowToTop(p.topRow(l))
	if line < 0 || line >= len(p.Buf.Lines) {
		return ""
	}
	return p.Buf.Lines[line]
}

func diffEditorSetTop(t *testing.T, d *DiffEditorWidget, text string) {
	t.Helper()
	p := d.lead()
	for i, line := range p.Buf.Lines {
		if line == text {
			l := p.layout()
			p.setTopRow(l, l.startRow(i))
			return
		}
	}
	t.Fatalf("missing line %q", text)
}

func TestDiffEditorDefaultsRemainInheritedUntilViewOverride(t *testing.T) {
	d := NewDiffEditorWidget("test.go", diff.FileDiff{}, nil, nil, false)
	d.ApplyDefaultMode(DiffModeUnified)
	d.ApplyDefaultContextMode(DiffContextFullFile)
	d.ApplyDefaultWrapMode(DiffWrapOn)
	if d.Mode() != DiffModeUnified || d.ContextMode() != DiffContextFullFile || d.WrapMode() != DiffWrapOn {
		t.Fatalf("inherited defaults = %v %v %v", d.Mode(), d.ContextMode(), d.WrapMode())
	}
	d.SetMode(DiffModeSplit)
	d.SetContextMode(DiffContextChangesOnly)
	d.SetWrapMode(DiffWrapOff)
	d.ApplyDefaultMode(DiffModeUnified)
	d.ApplyDefaultContextMode(DiffContextFullFile)
	d.ApplyDefaultWrapMode(DiffWrapOn)
	if d.Mode() != DiffModeSplit || d.ContextMode() != DiffContextChangesOnly || d.WrapMode() != DiffWrapOff {
		t.Fatalf("defaults replaced overrides: %v %v %v", d.Mode(), d.ContextMode(), d.WrapMode())
	}
}

func TestDiffEditorDelayedFetcherHonorsInheritedFullContext(t *testing.T) {
	d := NewDiffEditorWidget("test.go", diff.FileDiff{}, nil, nil, false)
	d.ApplyDefaultContextMode(DiffContextFullFile)
	if d.Loading {
		t.Fatal("full context should wait for a loader")
	}
	fetches := 0
	d.SetExtendedFetcher(func(*DiffEditorWidget) { fetches++ })
	if fetches != 1 || !d.Loading {
		t.Fatalf("fetches %d loading %v", fetches, d.Loading)
	}
	d.SetExtended(true)
	if fetches != 1 {
		t.Fatalf("refetched while loading: %d", fetches)
	}
}

func TestDiffEditorSearchMapsDiffRowsToPanes(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,3 +1,3 @@\n hello world\n-old line\n+new line\n context\n")
	d := NewDiffEditorWidget("test.go", fd, nil, nil, false)
	left, _ := FindInLines(d.LeftLines(), "line", SearchOptions{})
	right, _ := FindInLines(d.RightLines(), "line", SearchOptions{})
	merged := d.SetSearchMatches(left, right)
	if len(merged) != 2 {
		t.Fatalf("merged = %v", merged)
	}
	if len(d.left.SearchMatches) != 1 || len(d.right.SearchMatches) != 1 {
		t.Fatalf("pane matches left=%v right=%v", d.left.SearchMatches, d.right.SearchMatches)
	}
	d.SetActiveMatch(1)
	if d.right.SearchActive != 0 || d.left.SearchActive != -1 || d.focusLeft {
		t.Fatalf("active match left=%d right=%d", d.left.SearchActive, d.right.SearchActive)
	}
}

func TestDiffEditorExtendedModeTogglesContext(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -2,3 +2,3 @@\n hello world\n-old line\n+new line\n context\n")
	oldLines := []string{"first", "hello world", "old line", "context", "last line", "another"}
	newLines := []string{"first", "hello world", "new line", "context", "last line", "another"}
	d := NewDiffEditorWidget("test.go", fd, oldLines, newLines, false)
	compact := len(d.Lines)
	d.SetExtended(true)
	if !d.IsExtended() || len(d.Lines) <= compact {
		t.Fatalf("extended lines %d vs compact %d", len(d.Lines), compact)
	}
	if got := strings.Join(d.right.Buf.Lines, "|"); got != strings.Join(newLines, "|") {
		t.Fatalf("right pane = %q", got)
	}
	d.SetExtended(false)
	if d.IsExtended() || len(d.Lines) != compact {
		t.Fatalf("compact lines %d vs %d", len(d.Lines), compact)
	}
}

func TestDiffEditorUnifiedBufferStacksRemovedBeforeAdded(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,2 +1,2 @@\n-old one\n-old two\n+new one\n+new two\n")
	d := NewDiffEditorWidget("test.go", fd, nil, nil, false)
	d.SetUnified(true)
	want := "old one|old two|new one|new two"
	if got := strings.Join(d.unified.Buf.Lines[:4], "|"); got != want {
		t.Fatalf("unified buffer = %q, want %q", got, want)
	}
	grid := renderDiffEditor(d, 40, 5)
	if !strings.Contains(cellRow(grid, 0), "1 −old one") || !strings.Contains(cellRow(grid, 2), "1 +new one") {
		t.Fatalf("unified gutter:\n%s\n%s", cellRow(grid, 0), cellRow(grid, 2))
	}
}

func TestDiffEditorUnifiedSearchAndCopyUseRealLines(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-old value\n+new value\n")
	d := NewDiffEditorWidget("test.go", fd, nil, nil, false)
	d.SetUnified(true)
	d.ApplySearchHighlight("new", SearchOptions{})
	if len(d.SearchMatchesRight) != 1 || len(d.unified.SearchMatches) != 1 {
		t.Fatalf("unified search = %v / %v", d.SearchMatchesRight, d.unified.SearchMatches)
	}
	grid := renderDiffEditor(d, 40, 4)
	found := false
	for _, row := range grid {
		for _, cell := range row {
			found = found || cell.Style == term.StyleSearchMatch
		}
	}
	if !found {
		t.Fatal("search match not drawn")
	}
	d.unified.Selection.Start(0, 0)
	d.unified.Cursor.Line, d.unified.Cursor.Col = 1, len("new value")
	if got := d.CopySelection(); got != "old value\nnew value" {
		t.Fatalf("copy = %q", got)
	}
}

func TestDiffEditorCollapsedRowIsNotCopied(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -20,1 +20,1 @@\n twentieth\n")
	for _, unified := range []bool{false, true} {
		for _, onLeft := range []bool{false, true} {
			t.Run(fmt.Sprintf("unified=%v/left=%v", unified, onLeft), func(t *testing.T) {
				d := NewDiffEditorWidget("test.go", fd, nil, nil, false)
				d.SetUnified(unified)
				d.focusLeft = onLeft
				p := d.lead()
				if len(p.Buf.Lines) < 3 || p.DiffOverlay.kind(1) != diff.Collapsed {
					t.Fatalf("pane lines = %q", p.Buf.Lines)
				}
				p.Selection.Start(0, 0)
				p.Cursor.Line, p.Cursor.Col = 2, len("twentieth")
				if got := d.CopySelection(); got != "first\ntwentieth" {
					t.Fatalf("copy = %q", got)
				}
			})
		}
	}
}

func TestDiffEditorHunkOnlyDiffKeepsHunkNumbersAndGap(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -22,3 +22,3 @@\n line 22\n line 23\n line 24\n@@ -356,2 +356,2 @@\n line 356\n line 357\n")
	d := NewDiffEditorWidget("test.go", fd, nil, nil, false)
	grid := renderDiffEditor(d, 70, 8)
	if !strings.Contains(cellRow(grid, 0), "▶ ⋯ 21 lines ⋯") {
		t.Fatalf("leading gap row:\n%s", cellRow(grid, 0))
	}
	if !strings.Contains(cellRow(grid, 1), "22  line 22") {
		t.Fatalf("hunk numbers:\n%s", cellRow(grid, 1))
	}
	if !strings.Contains(cellRow(grid, 4), "▶ ⋯ 331 lines ⋯") {
		t.Fatalf("gap row:\n%s", cellRow(grid, 4))
	}
	if !strings.Contains(cellRow(grid, 5), "356  line 356") {
		t.Fatalf("second hunk:\n%s", cellRow(grid, 5))
	}
}

func TestDiffEditorGapClickExpandsOnlyThatGap(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -8,1 +8,1 @@\n eighth\n")
	lines := []string{"first", "second", "third", "fourth", "fifth", "sixth", "seventh", "eighth"}
	for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
		for _, wrap := range []bool{false, true} {
			t.Run(fmt.Sprintf("mode=%d/wrap=%v", mode, wrap), func(t *testing.T) {
				d := NewDiffEditorWidget("test.go", fd, lines, append([]string(nil), lines...), false)
				d.SetMode(mode)
				d.SetWrapped(wrap)
				const width, height = 60, 10
				grid := renderDiffEditor(d, width, height)
				y := diffEditorRowOf(grid, "⋯ 6 lines ⋯")
				if y < 0 {
					t.Fatalf("no gap row:\n%v", grid)
				}
				if got := d.HandleEvent(tcell.NewEventMouse(width-3, y, tcell.Button1, 0)); got != EventConsumed {
					t.Fatalf("click result = %v", got)
				}
				if len(d.gapByLine) != 0 || d.ContextMode() != DiffContextChangesOnly || d.Lines[1].Left.Text != "second" {
					t.Fatalf("gap not expanded locally: gaps=%v mode=%v lines=%d", d.gapByLine, d.ContextMode(), len(d.Lines))
				}
			})
		}
	}
}

func TestDiffEditorEnterExpandsGapUnderCursor(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -8,1 +8,1 @@\n eighth\n")
	lines := []string{"first", "second", "third", "fourth", "fifth", "sixth", "seventh", "eighth"}
	d := NewDiffEditorWidget("test.go", fd, lines, lines, false)
	renderDiffEditor(d, 60, 10)
	d.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", 0))
	if got := d.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0)); got != EventConsumed || d.Lines[1].Left.Text != "second" {
		t.Fatalf("enter result %v lines %d", got, len(d.Lines))
	}
}

func TestDiffEditorCollapsedRowQuietUntilHovered(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -20,1 +20,1 @@\n twentieth\n")
	for _, emphasized := range []bool{false, true} {
		t.Run(fmt.Sprintf("emphasis=%v", emphasized), func(t *testing.T) {
			d := NewDiffEditorWidget("test.go", fd, nil, nil, false)
			d.SetUnified(true)
			d.SetDiffCollapsedEmphasis(emphasized)
			const width, height = 50, 5
			grid := renderDiffEditor(d, width, height)
			gutterW := d.unified.GutterWidth()
			content, disclosure := grid[1][gutterW], grid[1][gutterW-1]
			wantContent, wantGutter := term.StyleMuted, term.StyleLineNumber
			if emphasized {
				wantContent, wantGutter = term.StyleDiffCollapsedEmphasis, term.StyleDiffCollapsedEmphasis
			}
			if content.Style != wantContent || disclosure.Ch != '▶' || disclosure.Style != wantGutter {
				t.Fatalf("idle content=%+v disclosure=%+v", content, disclosure)
			}
			if got := d.HandleEvent(tcell.NewEventMouse(gutterW, 1, tcell.ButtonNone, 0)); got != EventConsumed {
				t.Fatalf("hover result = %v", got)
			}
			grid = renderDiffEditor(d, width, height)
			if got := grid[1][gutterW].Style; got != term.StyleDiffCollapsedHover {
				t.Fatalf("hovered style = %v", got)
			}
			d.SetContextMode(DiffContextFullFile)
			d.SetContextMode(DiffContextChangesOnly)
			grid = renderDiffEditor(d, width, height)
			if got := grid[1][gutterW].Style; got != wantContent {
				t.Fatalf("rebuild kept hover: %v", got)
			}
		})
	}
}

func TestDiffEditorHighContrastUsesSemanticForegrounds(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-var oldValue = 1\n+var newValue = 2\n")
	d := NewDiffEditorWidget("test.go", fd, nil, nil, false)
	const width, height = 60, 3
	grid := renderDiffEditor(d, width, height)
	lx := d.left.GetRect().X + d.left.GutterWidth()
	rx := d.right.GetRect().X + d.right.GutterWidth()
	if grid[0][lx].Style != term.StyleSyntaxKeyword || grid[0][lx].BgStyle != term.StyleDiffDeleted {
		t.Fatalf("standard deleted cell = %+v", grid[0][lx])
	}
	if grid[0][rx].Style != term.StyleSyntaxKeyword || grid[0][rx].BgStyle != term.StyleDiffAdded {
		t.Fatalf("standard added cell = %+v", grid[0][rx])
	}
	d.SetDiffHighContrast(true)
	grid = renderDiffEditor(d, width, height)
	if grid[0][lx].Style != term.StyleGutterDeleted || grid[0][rx].Style != term.StyleGutterAdded {
		t.Fatalf("high contrast cells = %+v / %+v", grid[0][lx], grid[0][rx])
	}
}

func TestDiffEditorLoadingLifecycle(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -4,1 +4,1 @@\n fourth\n")
	d := NewDiffEditorWidget("test.go", fd, nil, nil, false)
	fetches := 0
	d.SetExtendedFetcher(func(*DiffEditorWidget) { fetches++ })
	d.SetContextMode(DiffContextFullFile)
	if fetches != 1 || !d.Loading {
		t.Fatalf("fetch state %d %v", fetches, d.Loading)
	}
	grid := renderDiffEditor(d, 40, 4)
	if !strings.HasPrefix(cellRow(grid, 0), "Loading...") {
		t.Fatalf("loading row = %q", cellRow(grid, 0))
	}
	if got := d.HandleEvent(tcell.NewEventMouse(5, 1, tcell.Button1, 0)); got != EventIgnored || fetches != 1 {
		t.Fatalf("loading click = %v fetches %d", got, fetches)
	}
	d.FailLoading()
	if d.Loading || d.ContextMode() != DiffContextChangesOnly {
		t.Fatalf("failed state loading=%v context=%v", d.Loading, d.ContextMode())
	}
	d.SetContextMode(DiffContextFullFile)
	d.SetOldLines([]string{"first", "o2", "o3", "fourth"})
	d.SetNewLines([]string{"first", "n2", "n3", "fourth"})
	d.FinishLoading()
	if fetches != 2 || d.Loading || !d.IsExtended() || len(d.Lines) != 4 {
		t.Fatalf("finished state fetches=%d loading=%v lines=%d", fetches, d.Loading, len(d.Lines))
	}
}

func TestDiffEditorGapFetchCompletionRemainsChangesOnly(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -4,1 +4,1 @@\n fourth\n")
	d := NewDiffEditorWidget("test.go", fd, nil, nil, false)
	d.SetExtendedFetcher(func(*DiffEditorWidget) {})
	d.expandContextGap(0)
	d.SetOldLines([]string{"first", "two", "three", "fourth"})
	d.SetNewLines([]string{"first", "two", "three", "fourth"})
	d.FinishLoading()
	if d.ContextMode() != DiffContextChangesOnly || d.IsExtended() || !d.expandedGaps[0] || d.Lines[1].Left.Text != "two" {
		t.Fatalf("gap completion: context %v expanded %v lines %d", d.ContextMode(), d.expandedGaps, len(d.Lines))
	}
}

func TestDiffEditorContextTransitionsPreserveLogicalTop(t *testing.T) {
	patch := "--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n line-01\n@@ -10,1 +10,1 @@\n line-10\n@@ -30,1 +30,1 @@\n line-30\n"
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%02d", i+1)
	}
	fd := diff.Parse(patch)
	for _, unified := range []bool{false, true} {
		for _, to := range []DiffContextMode{DiffContextFullFile, DiffContextChangesOnly} {
			t.Run(fmt.Sprintf("unified=%v/to=%d", unified, to), func(t *testing.T) {
				d := NewDiffEditorWidget("test.go", fd, lines, lines, false)
				d.SetUnified(unified)
				d.SetContextMode(1 - to)
				renderDiffEditor(d, 60, 3)
				diffEditorSetTop(t, d, "line-10")
				d.SetContextMode(to)
				if got := diffEditorTopText(d); got != "line-10" {
					t.Fatalf("top after transition = %q", got)
				}
			})
		}
	}
}

func TestDiffEditorSplitWrapKeepsRowsAligned(t *testing.T) {
	d := NewDiffEditorWidget("test.go", diff.FileDiff{}, []string{"a", "b"}, []string{"a", "x " + strings.Repeat("long ", 20), "b"}, true)
	d.SetWrapped(true)
	grid := renderDiffEditor(d, 60, 12)
	leftB, rightB := -1, -1
	for y := range grid {
		row := cellRow(grid, y)
		half := len([]rune(row)) / 2
		if strings.Contains(string([]rune(row)[:half]), " b") {
			leftB = y
		}
		if strings.Contains(string([]rune(row)[half:]), " b") {
			rightB = y
		}
	}
	if leftB < 0 || leftB != rightB {
		t.Fatalf("wrapped split rows misaligned: left %d right %d", leftB, rightB)
	}
}

func TestDiffEditorSplitFollowerScrollsWithLead(t *testing.T) {
	old := make([]string, 60)
	for i := range old {
		old[i] = fmt.Sprintf("line %d", i)
	}
	cur := append([]string{"new head"}, old...)
	d := NewDiffEditorWidget("test.go", diff.FileDiff{}, old, cur, true)
	renderDiffEditor(d, 60, 10)
	d.HandleEvent(tcell.NewEventMouse(5, 3, tcell.WheelDown, 0))
	renderDiffEditor(d, 60, 10)
	if d.right.topRow(d.right.layout()) != 3 || d.left.topRow(d.left.layout()) != 3 {
		t.Fatalf("top rows left %d right %d", d.left.topRow(d.left.layout()), d.right.topRow(d.right.layout()))
	}
}

func TestDiffEditorSplitWrapUsesFullSymmetricWidth(t *testing.T) {
	long := strings.Repeat("wrapme ", 30)
	old := []string{long, "a"}
	cur := []string{long, "b"}
	for _, w := range []int{60, 61} {
		d := NewDiffEditorWidget("test.txt", diff.FileDiff{}, old, cur, true)
		d.SetWrapped(true)
		renderDiffEditor(d, w, 4)
		lw, rw := d.left.Viewport.Width, d.right.Viewport.Width
		if lw != rw {
			t.Fatalf("width %d: left wraps at %d, right at %d", w, lw, rw)
		}
		widest := min(d.left.GetRect().W-d.left.GutterWidth(), d.right.GetRect().W-d.right.GutterWidth()) - 1
		if rw != widest {
			t.Fatalf("width %d: wraps at %d, want the full text width %d", w, rw, widest)
		}
	}
}
