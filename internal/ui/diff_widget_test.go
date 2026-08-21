package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestDiffWidgetDefaultsRemainInheritedUntilViewOverride(t *testing.T) {
	dv := NewDiffViewWidget("test.go", diff.FileDiff{}, nil, nil, false)

	dv.ApplyDefaultMode(DiffModeUnified)
	dv.ApplyDefaultWrapMode(DiffWrapOn)
	if dv.Mode() != DiffModeUnified || dv.WrapMode() != DiffWrapOn {
		t.Fatalf("inherited defaults = mode %v wrap %v, want unified/on", dv.Mode(), dv.WrapMode())
	}

	dv.SetMode(DiffModeSplit)
	dv.SetWrapMode(DiffWrapOff)
	dv.ApplyDefaultMode(DiffModeUnified)
	dv.ApplyDefaultWrapMode(DiffWrapOn)
	if dv.Mode() != DiffModeSplit || dv.WrapMode() != DiffWrapOff {
		t.Fatalf("defaults replaced View overrides: mode %v wrap %v", dv.Mode(), dv.WrapMode())
	}
}

func TestDiffWidgetModeAndWrapDefaultsAreIndependent(t *testing.T) {
	dv := NewDiffViewWidget("test.go", diff.FileDiff{}, nil, nil, false)
	dv.SetMode(DiffModeUnified)

	dv.ApplyDefaultMode(DiffModeSplit)
	dv.ApplyDefaultWrapMode(DiffWrapOn)
	if dv.Mode() != DiffModeUnified {
		t.Fatalf("mode default replaced explicit unified mode: %v", dv.Mode())
	}
	if dv.WrapMode() != DiffWrapOn {
		t.Fatalf("mode override prevented inherited wrap update: %v", dv.WrapMode())
	}
}

func TestDiffWidgetLeftRightLines(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,3 +1,3 @@\n hello world\n-old line\n+new line\n context\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)

	left := dv.LeftLines()
	right := dv.RightLines()

	if len(left) == 0 {
		t.Fatal("LeftLines returned empty")
	}
	if len(right) == 0 {
		t.Fatal("RightLines returned empty")
	}
	if len(left) != len(right) {
		t.Fatalf("left/right length mismatch: %d vs %d", len(left), len(right))
	}

	foundOld := false
	foundNew := false
	for _, l := range left {
		if l == "old line" {
			foundOld = true
		}
	}
	for _, r := range right {
		if r == "new line" {
			foundNew = true
		}
	}
	if !foundOld {
		t.Errorf("expected 'old line' in left lines, got: %v", left)
	}
	if !foundNew {
		t.Errorf("expected 'new line' in right lines, got: %v", right)
	}
}

func TestDiffWidgetSearchFindsMatches(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,3 +1,3 @@\n hello world\n-old line\n+new line\n context\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)

	leftMatches, err := FindInLines(dv.LeftLines(), "old", SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rightMatches, err := FindInLines(dv.RightLines(), "new", SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(leftMatches) == 0 {
		t.Errorf("expected matches for 'old' in left, got none. Left lines: %v", dv.LeftLines())
	}
	if len(rightMatches) == 0 {
		t.Errorf("expected matches for 'new' in right, got none. Right lines: %v", dv.RightLines())
	}
}

func TestDiffWidgetSetSearchMatches(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,3 +1,3 @@\n hello world\n-old line\n+new line\n context\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)

	leftMatches, _ := FindInLines(dv.LeftLines(), "line", SearchOptions{})
	rightMatches, _ := FindInLines(dv.RightLines(), "line", SearchOptions{})

	merged := dv.SetSearchMatches(leftMatches, rightMatches)

	if len(merged) == 0 {
		t.Fatal("expected merged matches, got none")
	}
	if len(merged) != len(leftMatches)+len(rightMatches) {
		t.Errorf("merged count %d != left %d + right %d", len(merged), len(leftMatches), len(rightMatches))
	}

	dv.SetActiveMatch(0)
	if dv.searchActiveSideIdx < 0 {
		t.Error("expected active side index >= 0 after SetActiveMatch(0)")
	}
}

func TestDiffWidgetExtendedMode(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -2,3 +2,3 @@\n hello world\n-old line\n+new line\n context\n")
	oldLines := []string{"first", "hello world", "old line", "context", "last line", "another"}
	newLines := []string{"first", "hello world", "new line", "context", "last line", "another"}
	dv := NewDiffViewWidget("test.go", fd, oldLines, newLines, false)

	compactCount := len(dv.Lines)

	dv.SetExtended(true)
	if !dv.IsExtended() {
		t.Error("expected extended mode to be true")
	}
	extendedCount := len(dv.Lines)
	if extendedCount <= compactCount {
		t.Errorf("extended should have more lines than compact: %d vs %d", extendedCount, compactCount)
	}

	dv.SetExtended(false)
	if dv.IsExtended() {
		t.Error("expected extended mode to be false")
	}
	if len(dv.Lines) != compactCount {
		t.Errorf("compact line count changed: %d vs %d", len(dv.Lines), compactCount)
	}
}

func TestDiffWidgetSearchContext(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,3 +1,3 @@\n hello world\n-old line\n+new line\n context\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)

	leftMatches, _ := FindInLines(dv.LeftLines(), "hello", SearchOptions{})
	rightMatches, _ := FindInLines(dv.RightLines(), "hello", SearchOptions{})

	total := len(leftMatches) + len(rightMatches)
	if total == 0 {
		t.Errorf("expected matches for 'hello' in context lines, got none. Left: %v, Right: %v", dv.LeftLines(), dv.RightLines())
	}
}

func TestDiffWidgetWrapRendersContinuationWithoutLineNumber(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-abcdefghijklmno\n+ABCDEFGHIJKLMNO\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
	dv.SetWrapped(true)

	const width, height = 30, 4
	grid := makeGrid(width, height)
	dv.SetRect(Rect{W: width, H: height})
	dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))

	row := func(y int) string {
		var b strings.Builder
		for _, cell := range grid[y] {
			if cell.Ch == 0 {
				b.WriteRune(' ')
			} else {
				b.WriteRune(cell.Ch)
			}
		}
		return b.String()
	}
	if !strings.Contains(row(0), "abcdefghij") || !strings.Contains(row(1), "klmno") {
		t.Fatalf("expected the long line to wrap across rows, got:\n%s\n%s", row(0), row(1))
	}
	if strings.Contains(row(1)[:4], "1") {
		t.Fatalf("continuation gutter must not repeat the line number, got %q", row(1)[:4])
	}
}

func TestDiffWidgetWrappedSelectionCopiesOriginalText(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-abcdefghij\n+ABCDEFGHIJ\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
	dv.SetWrapped(true)

	const width, height = 24, 4
	grid := make([][]term.Cell, height)
	for y := range grid {
		grid[y] = make([]term.Cell, width)
	}
	dv.SetRect(Rect{W: width, H: height})
	dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))

	// The left pane is seven columns wide at this size. Select from rune 2 on
	// the first visual row through rune 10 on its continuation row.
	dv.HandleEvent(tcell.NewEventMouse(6, 0, tcell.Button1, tcell.ModNone))
	dv.HandleEvent(tcell.NewEventMouse(7, 1, tcell.Button1, tcell.ModNone))
	dv.HandleEvent(tcell.NewEventMouse(7, 1, tcell.ButtonNone, tcell.ModNone))

	if got := dv.CopySelection(); got != "cdefghij" {
		t.Fatalf("wrapped copy = %q, want original unwrapped text %q", got, "cdefghij")
	}
}

func TestDiffWidgetFullwidthWrapAndHorizontalExtent(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-界界a\n+界界b\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)

	// Each side is five terminal columns wide (2 + 2 + 1), despite containing
	// only three runes. The unwrapped scrollbar must expose that full extent.
	if dv.maxLineW != 5 {
		t.Fatalf("fullwidth line extent = %d, want 5 terminal columns", dv.maxLineW)
	}
	const width, height = 15, 4 // three content columns per side
	grid := makeGrid(width, height)
	dv.SetRect(Rect{W: width, H: height})
	dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))
	if dv.hscrollbar.TotalCols != 5 || dv.rhscrollbar.TotalCols != 5 {
		t.Fatalf("horizontal extents = %d / %d, want 5 / 5", dv.hscrollbar.TotalCols, dv.rhscrollbar.TotalCols)
	}

	// At three columns, only one double-width rune fits on each row. A
	// rune-count implementation would keep all three runes on one row.
	dv.SetWrapped(true)
	grid = makeGrid(width, height)
	dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))
	if len(dv.wrapMap) < 2 {
		t.Fatalf("expected a continuation row, got wrap map %v", dv.wrapMap)
	}
	first, continuation := dv.wrapMap[0], dv.wrapMap[1]
	if first.line != 0 || first.leftStart != 0 || first.rightStart != 0 {
		t.Fatalf("first fullwidth segment = %+v, want rune offset 0 on both sides", first)
	}
	if continuation.line != 0 || continuation.leftStart != 1 || continuation.rightStart != 1 {
		t.Fatalf("fullwidth continuation = %+v, want rune offset 1 on both sides", continuation)
	}
}

func TestDiffWidgetUnifiedOrdersRemovedBeforeAdded(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,2 +1,2 @@\n-old one\n-old two\n+new one\n+new two\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
	dv.SetUnified(true)

	if !dv.IsUnified() {
		t.Fatal("unified mode should be enabled")
	}
	got := make([]string, len(dv.unifiedLines))
	for i, line := range dv.unifiedLines {
		got[i] = line.side.Text
	}
	want := []string{"old one", "old two", "new one", "new two"}
	if len(got) < len(want) || strings.Join(got[:len(want)], "|") != strings.Join(want, "|") {
		t.Fatalf("unified order = %v, want %v", got, want)
	}
}

func TestDiffWidgetUnifiedSelectionAndSearchUseProjectedLines(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-old value\n+new value\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
	dv.SetUnified(true)
	dv.ApplySearchHighlight("new", SearchOptions{})

	if len(dv.SearchMatchesRight) != 1 {
		t.Fatalf("unified search should retain the added-side match, got %v", dv.SearchMatchesRight)
	}
	const width, height = 40, 4
	grid := makeGrid(width, height)
	dv.SetRect(Rect{W: width, H: height})
	dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))
	foundSearchStyle := false
	for _, row := range grid {
		for _, cell := range row {
			if cell.Style == term.StyleSearchMatch {
				foundSearchStyle = true
			}
		}
	}
	if !foundSearchStyle {
		t.Fatal("unified rendering should draw the projected search match")
	}

	dv.hasSelection = true
	dv.selection.Anchor = diffSelPos{Line: 0, Col: 0}
	dv.selection.Current = diffSelPos{Line: 1, Col: len([]rune("new value"))}
	if got := dv.CopySelection(); got != "old value\nnew value" {
		t.Fatalf("unified copy = %q, want stacked original lines", got)
	}
}

func TestDiffWidgetCollapsedSeparatorShowsLineDistance(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -22,3 +22,3 @@\n line 22\n line 23\n line 24\n@@ -356,2 +356,2 @@\n line 356\n line 357\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)

	if len(dv.Lines) < 4 {
		t.Fatalf("expected two hunks and a separator, got %v", dv.Lines)
	}
	separator := dv.Lines[3]
	if separator.Left.Text != "⋯ 331 lines ⋯" || separator.Right.Text != "⋯ 331 lines ⋯" {
		t.Fatalf("separator = %q / %q, want collapsed distance", separator.Left.Text, separator.Right.Text)
	}
}

func TestDiffWidgetCollapsedSeparatorUsesSingularLine(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -24,1 +24,1 @@\n line 24\n@@ -26,1 +26,1 @@\n line 26\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)

	if got := dv.Lines[1].Left.Text; got != "⋯ 1 line ⋯" {
		t.Fatalf("separator = %q, want singular distance", got)
	}
}

func TestDiffWidgetCollapsedSeparatorOmitsAdjacentLines(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -24,1 +24,1 @@\n line 24\n@@ -25,1 +25,1 @@\n line 25\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)

	if len(dv.Lines) != 3 {
		t.Fatalf("lines = %d, want adjacent diff rows with no separator: %v", len(dv.Lines), dv.Lines)
	}
	if dv.Lines[0].Left.Text != "line 24" || dv.Lines[1].Left.Text != "line 25" {
		t.Fatalf("adjacent rows shifted: %q then %q", dv.Lines[0].Left.Text, dv.Lines[1].Left.Text)
	}
}

func TestDiffWidgetCollapsedSeparatorUsesDedicatedStyle(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -20,1 +20,1 @@\n twentieth\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)

	const width, height = 50, 5
	grid := makeGrid(width, height)
	dv.SetRect(Rect{W: width, H: height})
	dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))

	separatorRow := 1
	contentCell := grid[separatorRow][dv.layoutLeftStart]
	if contentCell.Style != term.StyleDiffCollapsed {
		t.Fatalf("collapsed separator style = %v, want StyleDiffCollapsed", contentCell.Style)
	}
	if contentCell.Style == term.StyleDefault {
		t.Fatal("collapsed separator must not resolve to StyleDefault")
	}
	if contentCell.BgStyle != 0 {
		t.Fatalf("collapsed separator should use its full style, got BgStyle %v", contentCell.BgStyle)
	}
	if gutterCell := grid[separatorRow][0]; gutterCell.Style != term.StyleDiffCollapsed {
		t.Fatalf("collapsed separator gutter style = %v, want StyleDiffCollapsed", gutterCell.Style)
	}
}

func TestDiffGutterMarksAndColorsChangedLines(t *testing.T) {
	grid := makeGrid(10, 2)
	surface := NewRenderSurface(grid, Rect{W: 10, H: 2})
	renderDiffGutter(surface, 0, 0, 5, diff.SideLine{Num: 12, Kind: diff.Deleted}, term.StyleDiffDeleted)
	renderDiffGutter(surface, 0, 1, 5, diff.SideLine{Num: 13, Kind: diff.Added}, term.StyleDiffAdded)

	if got := commitDetailGridText(grid); !strings.Contains(got, "12 -") || !strings.Contains(got, "13 +") {
		t.Fatalf("changed gutters do not show line markers:\n%s", got)
	}
	for column := 0; column < 5; column++ {
		if grid[0][column].Style != term.StyleGutterDeleted {
			t.Fatalf("deleted gutter column %d style = %v, want StyleGutterDeleted", column, grid[0][column].Style)
		}
		if grid[1][column].Style != term.StyleGutterAdded {
			t.Fatalf("added gutter column %d style = %v, want StyleGutterAdded", column, grid[1][column].Style)
		}
	}
}

func TestDiffWidgetHighContrastUsesSemanticChangeForegrounds(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-var oldValue = 1\n+var newValue = 2\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)

	const width, height = 60, 2
	standard := makeGrid(width, height)
	dv.SetRect(Rect{W: width, H: height})
	dv.Render(NewRenderSurface(standard, Rect{W: width, H: height}))
	if got := standard[0][dv.layoutLeftStart].Style; got != term.StyleSyntaxKeyword {
		t.Fatalf("standard diff text style = %v, want syntax keyword", got)
	}

	dv.SetDiffHighContrast(true)
	grid := makeGrid(width, height)
	dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))

	if got := grid[0][dv.layoutLeftStart].Style; got != term.StyleGutterDeleted {
		t.Fatalf("deleted text style = %v, want semantic deleted foreground", got)
	}
	if got := grid[0][dv.layoutRightStart].Style; got != term.StyleGutterAdded {
		t.Fatalf("added text style = %v, want semantic added foreground", got)
	}
	if got := grid[0][dv.layoutLeftStart].BgStyle; got != term.StyleDiffDeleted {
		t.Fatalf("deleted text background = %v, want StyleDiffDeleted", got)
	}
	if got := grid[0][dv.layoutRightStart].BgStyle; got != term.StyleDiffAdded {
		t.Fatalf("added text background = %v, want StyleDiffAdded", got)
	}
}
