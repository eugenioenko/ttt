package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestDiffWidgetDefaultsRemainInheritedUntilViewOverride(t *testing.T) {
	dv := NewDiffViewWidget("test.go", diff.FileDiff{}, nil, nil, false)

	dv.ApplyDefaultMode(DiffModeUnified)
	dv.ApplyDefaultContextMode(DiffContextFullFile)
	dv.ApplyDefaultWrapMode(DiffWrapOn)
	if dv.Mode() != DiffModeUnified || dv.ContextMode() != DiffContextFullFile || dv.WrapMode() != DiffWrapOn {
		t.Fatalf("inherited defaults = mode %v context %v wrap %v, want unified/full/on", dv.Mode(), dv.ContextMode(), dv.WrapMode())
	}

	dv.SetMode(DiffModeSplit)
	dv.SetContextMode(DiffContextChangesOnly)
	dv.SetWrapMode(DiffWrapOff)
	dv.ApplyDefaultMode(DiffModeUnified)
	dv.ApplyDefaultContextMode(DiffContextFullFile)
	dv.ApplyDefaultWrapMode(DiffWrapOn)
	if dv.Mode() != DiffModeSplit || dv.ContextMode() != DiffContextChangesOnly || dv.WrapMode() != DiffWrapOff {
		t.Fatalf("defaults replaced View overrides: mode %v context %v wrap %v", dv.Mode(), dv.ContextMode(), dv.WrapMode())
	}
}

func TestDiffWidgetDelayedExtendedFetcherHonorsInheritedFullContext(t *testing.T) {
	dv := NewDiffViewWidget("test.go", diff.FileDiff{}, nil, nil, false)
	dv.ApplyDefaultContextMode(DiffContextFullFile)
	if dv.Loading {
		t.Fatal("full context should wait until a lazy loader is attached")
	}

	fetches := 0
	dv.SetExtendedFetcher(func(*DiffViewWidget) { fetches++ })
	if fetches != 1 || !dv.Loading || dv.ContextMode() != DiffContextFullFile {
		t.Fatalf("attached loader state = fetches %d loading %v context %v", fetches, dv.Loading, dv.ContextMode())
	}
	dv.SetExtended(true)
	if fetches != 1 {
		t.Fatalf("loading full context started %d fetches, want one", fetches)
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
	if separator.Left.Text != " ⋯ 331 lines ⋯" || separator.Right.Text != " ⋯ 331 lines ⋯" {
		t.Fatalf("separator = %q / %q, want collapsed distance", separator.Left.Text, separator.Right.Text)
	}
}

func TestDiffWidgetCollapsedSeparatorUsesSingularLine(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -24,1 +24,1 @@\n line 24\n@@ -26,1 +26,1 @@\n line 26\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)

	if got := dv.Lines[1].Left.Text; got != " ⋯ 1 line ⋯" {
		t.Fatalf("separator = %q, want singular distance", got)
	}
}

func TestDiffWidgetCollapsedSeparatorClickExpandsOnlyThatGap(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -8,1 +8,1 @@\n eighth\n")
	oldLines := []string{"first", "second", "third", "fourth", "fifth", "sixth", "seventh", "eighth"}
	newLines := append([]string(nil), oldLines...)
	dv := NewDiffViewWidget("test.go", fd, oldLines, newLines, false)

	const width, height = 60, 8
	grid := makeGrid(width, height)
	dv.SetRect(Rect{W: width, H: height})
	dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))
	if got := dv.Lines[1].Left.Text; got != " ⋯ 6 lines ⋯" {
		t.Fatalf("precondition separator = %q", got)
	}

	if result := dv.HandleEvent(tcell.NewEventMouse(width-2, 1, tcell.Button1, 0)); result != EventConsumed {
		t.Fatalf("collapsed row click result = %v", result)
	}
	if len(dv.Lines) < 8 || dv.Lines[1].Left.Text != "second" || dv.Lines[6].Right.Text != "seventh" {
		t.Fatalf("expanded gap lines = %+v", dv.Lines)
	}
	if dv.ContextMode() != DiffContextChangesOnly {
		t.Fatalf("one expanded gap changed global context mode to %v", dv.ContextMode())
	}
}

func TestDiffWidgetWrappedCollapsedSeparatorUsesFullRowHitTarget(t *testing.T) {
	for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
		t.Run(fmt.Sprintf("mode-%d", mode), func(t *testing.T) {
			fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -20,1 +20,1 @@\n twentieth\n")
			lines := make([]string, 20)
			for index := range lines {
				lines[index] = fmt.Sprintf("line %d", index+1)
			}
			dv := NewDiffViewWidget("test.go", fd, lines, append([]string(nil), lines...), false)
			dv.SetMode(mode)
			dv.SetWrapMode(DiffWrapOn)
			const width, height = 18, 12
			dv.SetRect(Rect{W: width, H: height})
			dv.Render(NewRenderSurface(makeGrid(width, height), Rect{W: width, H: height}))
			separatorLine := -1
			for line := range dv.Lines {
				if _, ok := dv.gapByLine[line]; ok {
					separatorLine = line
					break
				}
			}
			clickY := -1
			for y, entry := range dv.wrapMap {
				if entry.line == separatorLine {
					clickY = y
				}
			}
			if clickY < 0 {
				t.Fatalf("wrapped separator mapping missing: line=%d map=%+v", separatorLine, dv.wrapMap)
			}
			if result := dv.HandleEvent(tcell.NewEventMouse(width-2, clickY, tcell.Button1, tcell.ModNone)); result != EventConsumed {
				t.Fatalf("full-row wrapped click result = %v", result)
			}
			if len(dv.gapByLine) != 0 || dv.ContextMode() != DiffContextChangesOnly {
				t.Fatalf("wrapped gap was not locally expanded: gaps=%v mode=%v", dv.gapByLine, dv.ContextMode())
			}
		})
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

func TestDiffWidgetCollapsedSeparatorIsQuietUntilHovered(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -20,1 +20,1 @@\n twentieth\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)

	const width, height = 50, 5
	grid := makeGrid(width, height)
	dv.SetRect(Rect{W: width, H: height})
	dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))

	separatorRow := 1
	contentCell := grid[separatorRow][dv.layoutLeftStart]
	if contentCell.Style != term.StyleMuted || contentCell.BgStyle != 0 {
		t.Fatalf("collapsed separator cell = %+v, want muted text on inherited background", contentCell)
	}
	gutterCell := grid[separatorRow][dv.layoutGutterW-1]
	if gutterCell.Ch != '▶' || gutterCell.Style != term.StyleLineNumber {
		t.Fatalf("collapsed separator gutter cell = %+v, want line-number disclosure triangle", gutterCell)
	}

	if result := dv.HandleEvent(tcell.NewEventMouse(dv.layoutLeftStart, separatorRow, tcell.ButtonNone, tcell.ModNone)); result != EventConsumed {
		t.Fatalf("collapsed separator hover result = %v, want EventConsumed for redraw", result)
	}
	dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))
	if got := grid[separatorRow][dv.layoutLeftStart].Style; got != term.StyleDiffCollapsedHover {
		t.Fatalf("hovered separator style = %v, want StyleDiffCollapsedHover", got)
	}
	if got := grid[separatorRow][dv.layoutGutterW-1]; got.Ch != '▶' || got.Style != term.StyleLineNumber {
		t.Fatalf("hovered separator gutter cell = %+v, want stable line-number disclosure triangle", got)
	}
}

func TestDiffWidgetCollapsedEmphasisCoversContentAndDisclosureUntilHovered(t *testing.T) {
	for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
		t.Run(fmt.Sprintf("mode-%d", mode), func(t *testing.T) {
			fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -20,1 +20,1 @@\n twentieth\n")
			dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
			dv.SetMode(mode)
			dv.SetDiffCollapsedEmphasis(true)

			const width, height = 50, 5
			grid := makeGrid(width, height)
			dv.SetRect(Rect{W: width, H: height})
			dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))

			const separatorRow = 1
			if got := grid[separatorRow][dv.layoutLeftStart].Style; got != term.StyleDiffCollapsedEmphasis {
				t.Fatalf("emphasized content style = %v", got)
			}
			if got := grid[separatorRow][dv.layoutGutterW-1]; got.Ch != '▶' || got.Style != term.StyleDiffCollapsedEmphasis {
				t.Fatalf("emphasized disclosure gutter = %+v", got)
			}

			if result := dv.HandleEvent(tcell.NewEventMouse(dv.layoutLeftStart, separatorRow, tcell.ButtonNone, tcell.ModNone)); result != EventConsumed {
				t.Fatalf("hover result = %v", result)
			}
			dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))
			if got := grid[separatorRow][dv.layoutLeftStart].Style; got != term.StyleDiffCollapsedHover {
				t.Fatalf("hovered content style = %v", got)
			}
			if got := grid[separatorRow][dv.layoutGutterW-1].Style; got != term.StyleDiffCollapsedHover {
				t.Fatalf("hovered disclosure style = %v", got)
			}
		})
	}
}

func TestDiffWidgetContextRebuildClearsCollapsedHoverUntilFreshMovement(t *testing.T) {
	for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
		t.Run(fmt.Sprintf("mode-%d", mode), func(t *testing.T) {
			fd, lines := diffWidgetContextFixture()
			dv := NewDiffViewWidget("test.go", fd, lines, lines, false)
			dv.SetMode(mode)
			dv.SetDiffCollapsedEmphasis(true)

			const width, height = 60, 8
			grid := makeGrid(width, height)
			dv.SetRect(Rect{W: width, H: height})
			dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))

			gapRow := -1
			if mode == DiffModeUnified {
				for rowIndex, line := range dv.unifiedLines {
					if _, ok := dv.gapByLine[line.sourceLine]; ok {
						gapRow = rowIndex
						break
					}
				}
			} else {
				for rowIndex := range dv.Lines {
					if _, ok := dv.gapByLine[rowIndex]; ok {
						gapRow = rowIndex
						break
					}
				}
			}
			if gapRow < 0 {
				t.Fatal("missing collapsed row")
			}

			pointer := tcell.NewEventMouse(dv.layoutLeftStart, gapRow, tcell.ButtonNone, tcell.ModNone)
			if result := dv.HandleEvent(pointer); result != EventConsumed {
				t.Fatalf("initial hover result = %v", result)
			}
			dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))
			if got := grid[gapRow][dv.layoutLeftStart].Style; got != term.StyleDiffCollapsedHover {
				t.Fatalf("initial hovered style = %v", got)
			}

			dv.SetContextMode(DiffContextFullFile)
			dv.SetContextMode(DiffContextChangesOnly)
			dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))
			if got := grid[gapRow][dv.layoutLeftStart].Style; got != term.StyleDiffCollapsedEmphasis {
				t.Fatalf("rebuilt idle style = %v, want emphasis", got)
			}
			if got := grid[gapRow][dv.layoutGutterW-1].Style; got != term.StyleDiffCollapsedEmphasis {
				t.Fatalf("rebuilt disclosure style = %v, want emphasis", got)
			}

			if result := dv.HandleEvent(pointer); result != EventConsumed {
				t.Fatalf("fresh hover result after rebuild = %v", result)
			}
			dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))
			if got := grid[gapRow][dv.layoutLeftStart].Style; got != term.StyleDiffCollapsedHover {
				t.Fatalf("fresh hovered style after rebuild = %v", got)
			}
		})
	}
}

func TestDiffGutterMarksAndColorsChangedLines(t *testing.T) {
	grid := makeGrid(10, 2)
	surface := NewRenderSurface(grid, Rect{W: 10, H: 2})
	renderDiffGutter(surface, 0, 0, 5, diff.SideLine{Num: 12, Kind: diff.Deleted})
	renderDiffGutter(surface, 0, 1, 5, diff.SideLine{Num: 13, Kind: diff.Added})

	if deleted, added := cellRow(grid, 0), cellRow(grid, 1); !strings.Contains(deleted, "12 −") || !strings.Contains(added, "13 +") {
		t.Fatalf("changed gutters do not show line markers:\n%s\n%s", deleted, added)
	}
	for column := 0; column < 5; column++ {
		if grid[0][column].Style != term.StyleGutterDeleted {
			t.Fatalf("deleted gutter column %d style = %v, want StyleGutterDeleted", column, grid[0][column].Style)
		}
		if grid[0][column].BgStyle != term.StyleDiffDeleted {
			t.Fatalf("deleted gutter column %d background = %v, want StyleDiffDeleted", column, grid[0][column].BgStyle)
		}
		if grid[1][column].Style != term.StyleGutterAdded {
			t.Fatalf("added gutter column %d style = %v, want StyleGutterAdded", column, grid[1][column].Style)
		}
		if grid[1][column].BgStyle != term.StyleDiffAdded {
			t.Fatalf("added gutter column %d background = %v, want StyleDiffAdded", column, grid[1][column].BgStyle)
		}
	}
}

func TestDiffGutterRightAlignsLineNumbers(t *testing.T) {
	grid := makeGrid(12, 1)
	surface := NewRenderSurface(grid, Rect{W: 12, H: 1})
	renderDiffGutter(surface, 0, 0, 6, diff.SideLine{Num: 1, Kind: diff.Added})
	renderDiffGutter(surface, 6, 0, 6, diff.SideLine{Num: 123, Kind: diff.Deleted})
	if got := cellRow(grid, 0); got != "   1 + 123 −" {
		t.Fatalf("aligned diff gutters = %q, want right-aligned line numbers", got)
	}
}

func TestDiffTextDoesNotDrawFullwidthRuneInLastColumn(t *testing.T) {
	grid := makeGrid(2, 1)
	surface := NewRenderSurface(grid, Rect{W: 2, H: 1})
	renderDiffText(surface, 0, 0, 2, "a界", term.StyleDiffAdded, term.StyleDefault, nil, 0, 0, nil)
	if grid[0][0].Ch != 'a' || grid[0][1].Ch != ' ' {
		t.Fatalf("last-column fullwidth rendering = %q / %q, want 'a' then a safe space", grid[0][0].Ch, grid[0][1].Ch)
	}
	if grid[0][1].BgStyle != term.StyleDiffAdded {
		t.Fatalf("substituted last-column cell lost diff background: %+v", grid[0][1])
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
	if got := standard[0][dv.layoutRightStart].Style; got != term.StyleSyntaxKeyword {
		t.Fatalf("standard added text style = %v, want syntax keyword", got)
	}
	if got := standard[0][dv.layoutLeftStart].BgStyle; got != term.StyleDiffDeleted {
		t.Fatalf("standard deleted syntax background = %v, want StyleDiffDeleted", got)
	}
	if got := standard[0][dv.layoutRightStart].BgStyle; got != term.StyleDiffAdded {
		t.Fatalf("standard added syntax background = %v, want StyleDiffAdded", got)
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

func TestDiffWidgetSyntaxSearchAndSelectionLayering(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-var oldValue = 1\n+var newValue = 2\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
	dv.ApplySearchHighlight("var", SearchOptions{})

	const width, height = 60, 2
	dv.SetRect(Rect{W: width, H: height})
	grid := makeGrid(width, height)
	dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))
	cell := grid[0][dv.layoutLeftStart]
	if cell.Style != term.StyleSearchMatch || cell.BgStyle != 0 {
		t.Fatalf("searched diff syntax cell = %+v, want search style replacing diff background", cell)
	}

	dv.hasSelection = true
	dv.selRight = false
	dv.selection.Anchor = diffSelPos{Line: 0, Col: 0}
	dv.selection.Current = diffSelPos{Line: 0, Col: 3}
	grid = makeGrid(width, height)
	dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))
	cell = grid[0][dv.layoutLeftStart]
	if cell.Style != term.StyleSearchMatch || cell.BgStyle != term.StyleSelection {
		t.Fatalf("selected searched diff syntax cell = %+v, want search style with selection background", cell)
	}
}

func TestDiffWidgetWrappedScrollbarUsesVisualRows(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n+abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
	dv.SetWrapped(true)

	const width, height = 24, 4
	grid := makeGrid(width, height)
	dv.SetRect(Rect{W: width, H: height})
	dv.Render(NewRenderSurface(grid, Rect{W: width, H: height}))
	if !dv.scrollbar.Visible() {
		t.Fatal("wrapped precondition: expected vertical scrollbar")
	}

	result := dv.HandleEvent(tcell.NewEventMouse(width-1, height-1, tcell.Button1, tcell.ModNone))
	if result != EventCaptured && result != EventConsumed {
		t.Fatalf("scrollbar click result = %v", result)
	}
	if dv.TopLine != 0 || dv.wrapTopOffset == 0 {
		t.Fatalf("wrapped scrollbar top = source line %d offset %d, want source line 0 with a visual-row offset", dv.TopLine, dv.wrapTopOffset)
	}
}

func TestDiffWidgetNarrowResizeClearsMouseLayout(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-old\n+new\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
	dv.SetRect(Rect{X: 4, Y: 2, W: 40, H: 4})
	dv.Render(NewRenderSurface(makeGrid(40, 4), Rect{W: 40, H: 4}))
	if dv.layoutLeftW == 0 {
		t.Fatal("wide precondition: expected cached content layout")
	}

	dv.SetRect(Rect{X: 4, Y: 2, W: 5, H: 4})
	dv.Render(NewRenderSurface(makeGrid(5, 4), Rect{W: 5, H: 4}))
	if dv.viewH != 0 || dv.layoutLeftW != 0 || dv.scrollbar.Visible() {
		t.Fatalf("narrow layout retained stale state: viewH=%d leftW=%d scrollbar=%+v", dv.viewH, dv.layoutLeftW, dv.scrollbar)
	}
	if got := dv.HandleEvent(tcell.NewEventMouse(8, 2, tcell.Button1, tcell.ModNone)); got != EventIgnored {
		t.Fatalf("mouse event against stale wide layout = %v, want ignored", got)
	}
}

func TestDiffWidgetFullwidthMouseCoordinatesUseTerminalColumns(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-界ab\n+界cd\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
	const width, height = 30, 3
	dv.SetRect(Rect{X: 7, Y: 5, W: width, H: height})
	dv.Render(NewRenderSurface(makeGrid(width, height), Rect{W: width, H: height}))

	startX := dv.GetRect().X + dv.layoutLeftStart
	for visualColumn, wantRune := range map[int]int{0: 0, 2: 1, 3: 2} {
		got, right, ok := dv.screenToSel(startX+visualColumn, dv.GetRect().Y)
		if !ok || right || got.Col != wantRune {
			t.Fatalf("left visual column %d = pos %+v right=%v ok=%v, want rune %d on left", visualColumn, got, right, ok, wantRune)
		}
	}

	rightX := dv.GetRect().X + dv.layoutRightStart
	got, right, ok := dv.screenToSel(rightX+2, dv.GetRect().Y)
	if !ok || !right || got.Col != 1 {
		t.Fatalf("right visual column 2 = pos %+v right=%v ok=%v, want rune 1 on right", got, right, ok)
	}
}

func TestDiffWidgetResizeKeepsWrappedLogicalTop(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,8 +1,8 @@\n-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789\n+ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz9876543210\n+ context-2\n+ context-3\n+ context-4\n+ context-5\n+ context-6\n+ context-7\n+ context-8\n")
	for _, unified := range []bool{false, true} {
		name := "split"
		if unified {
			name = "unified"
		}
		t.Run(name, func(t *testing.T) {
			dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
			dv.SetUnified(unified)
			dv.SetWrapped(true)
			dv.SetRect(Rect{W: 20, H: 3})
			dv.Render(NewRenderSurface(makeGrid(20, 3), Rect{W: 20, H: 3}))
			dv.setTopVisualRow(3)
			if dv.TopLine != 0 || dv.wrapTopOffset == 0 {
				t.Fatalf("narrow precondition top=%d offset=%d", dv.TopLine, dv.wrapTopOffset)
			}

			dv.SetRect(Rect{W: 70, H: 3})
			dv.Render(NewRenderSurface(makeGrid(70, 3), Rect{W: 70, H: 3}))
			if dv.TopLine != 0 {
				t.Fatalf("resize moved logical top from line 0 to line %d (offset %d)", dv.TopLine, dv.wrapTopOffset)
			}
		})
	}
}

func TestDiffWidgetGeometryChangesKeepScrolledLogicalTop(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,10 +1,10 @@\n context-1-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-2-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-3-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-4-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-5-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-6-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-7-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-8-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-9-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-10-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n")
	for _, unified := range []bool{false, true} {
		for _, widths := range [][2]int{{24, 80}, {80, 24}} {
			name := fmt.Sprintf("unified=%v/%d-to-%d", unified, widths[0], widths[1])
			t.Run(name, func(t *testing.T) {
				dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
				dv.SetUnified(unified)
				dv.SetWrapped(true)
				dv.SetRect(Rect{W: widths[0], H: 3})
				dv.Render(NewRenderSurface(makeGrid(widths[0], 3), Rect{W: widths[0], H: 3}))
				dv.TopLine = dv.displayLineForSourceLine(3)
				dv.wrapTopOffset = 0

				dv.SetRect(Rect{W: widths[1], H: 3})
				dv.Render(NewRenderSurface(makeGrid(widths[1], 3), Rect{W: widths[1], H: 3}))
				if got := dv.topSourceLine(); got != 3 {
					t.Fatalf("logical top after resize = %d, want source row 3", got)
				}
			})
		}
	}
}

func TestDiffWidgetModeAndWrapChangesKeepLogicalTop(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,8 +1,8 @@\n context-1-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n-old-2-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n+new-2-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-3-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-4-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-5-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-6-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-7-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-8-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
	dv.SetWrapped(true)
	dv.SetRect(Rect{W: 40, H: 3})
	dv.Render(NewRenderSurface(makeGrid(40, 3), Rect{W: 40, H: 3}))
	dv.TopLine = 3
	dv.wrapTopOffset = 1

	dv.SetUnified(true)
	dv.Render(NewRenderSurface(makeGrid(40, 3), Rect{W: 40, H: 3}))
	if got := dv.topSourceLine(); got != 3 {
		t.Fatalf("unified mode top source row = %d, want 3", got)
	}
	dv.SetWrapped(false)
	dv.Render(NewRenderSurface(makeGrid(40, 3), Rect{W: 40, H: 3}))
	if got := dv.topSourceLine(); got != 3 {
		t.Fatalf("unwrapped top source row = %d, want 3", got)
	}
	dv.SetUnified(false)
	dv.SetWrapped(true)
	dv.Render(NewRenderSurface(makeGrid(40, 3), Rect{W: 40, H: 3}))
	if got := dv.topSourceLine(); got != 3 {
		t.Fatalf("restored split/wrapped top source row = %d, want 3", got)
	}
}

func TestDiffWidgetResizeClampsLogicalTopAtEnd(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,4 +1,4 @@\n context-1-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-2-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-3-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n context-4-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
	dv.SetUnified(true)
	dv.SetWrapped(true)
	dv.SetRect(Rect{W: 20, H: 3})
	dv.Render(NewRenderSurface(makeGrid(20, 3), Rect{W: 20, H: 3}))
	dv.setTopVisualRow(dv.totalVisualRows)

	dv.SetRect(Rect{W: 100, H: 3})
	dv.Render(NewRenderSurface(makeGrid(100, 3), Rect{W: 100, H: 3}))
	wantTop := max(dv.totalVisualRows-dv.viewH, 0)
	if got := dv.topVisualRow(); got != wantTop {
		t.Fatalf("clamped visual top = %d, want %d", got, wantTop)
	}
}

func TestDiffWidgetLoadingCannotRefetchCollapsedGap(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -20,1 +20,1 @@\n twentieth\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
	dv.SetRect(Rect{W: 50, H: 5})
	dv.Render(NewRenderSurface(makeGrid(50, 5), Rect{W: 50, H: 5}))

	fetches := 0
	dv.SetExtendedFetcher(func(*DiffViewWidget) { fetches++ })
	formerGapX, formerGapY := dv.layoutLeftStart, 1
	dv.SetContextMode(DiffContextFullFile)
	if fetches != 1 || !dv.Loading {
		t.Fatalf("load precondition fetches=%d loading=%v", fetches, dv.Loading)
	}
	if dv.viewH != 0 || dv.layoutLeftW != 0 || dv.scrollbar.Visible() {
		t.Fatalf("loading retained interactive layout: viewH=%d leftW=%d scrollbar=%+v", dv.viewH, dv.layoutLeftW, dv.scrollbar)
	}
	dv.SetContextMode(DiffContextFullFile)
	dv.SetExtended(true)
	dv.HandleEvent(tcell.NewEventMouse(formerGapX, formerGapY, tcell.Button1, tcell.ModNone))
	if fetches != 1 {
		t.Fatalf("loading surface launched %d fetches after stale collapsed-row click, want one", fetches)
	}
	if got := dv.HandleEvent(tcell.NewEventMouse(formerGapX, formerGapY, tcell.Button1, tcell.ModNone)); got != EventIgnored {
		t.Fatalf("loading mouse event = %v, want ignored", got)
	}
}

func TestDiffWidgetFullFileRequestSupersedesInflightGapExpansion(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -4,1 +4,1 @@\n fourth\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
	fetches := 0
	dv.SetExtendedFetcher(func(*DiffViewWidget) { fetches++ })

	dv.expandContextGap(0)
	if fetches != 1 || dv.pendingGap != 0 || !dv.Loading {
		t.Fatalf("gap fetch state = fetches %d pending %d loading %v", fetches, dv.pendingGap, dv.Loading)
	}
	dv.SetContextMode(DiffContextFullFile)
	dv.SetOldLines([]string{"first", "old-two", "old-three", "fourth"})
	dv.SetNewLines([]string{"first", "new-two", "new-three", "fourth"})
	dv.FinishLoading()

	if fetches != 1 || dv.ContextMode() != DiffContextFullFile || !dv.IsExtended() || len(dv.Lines) != 4 {
		t.Fatalf("full-file completion = fetches %d context %v extended %v lines %d", fetches, dv.ContextMode(), dv.IsExtended(), len(dv.Lines))
	}
}

func TestDiffWidgetGapFetchCompletionRemainsChangesOnly(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -4,1 +4,1 @@\n fourth\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
	dv.SetExtendedFetcher(func(*DiffViewWidget) {})

	dv.expandContextGap(0)
	dv.SetOldLines([]string{"first", "old-two", "old-three", "fourth"})
	dv.SetNewLines([]string{"first", "new-two", "new-three", "fourth"})
	dv.FinishLoading()

	if dv.ContextMode() != DiffContextChangesOnly || dv.IsExtended() || !dv.expandedGaps[0] {
		t.Fatalf("gap completion = context %v extended %v expanded %v", dv.ContextMode(), dv.IsExtended(), dv.expandedGaps)
	}
}

func TestDiffWidgetExtendedFetchErrorRetryAndCompletionLifecycle(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -4,1 +4,1 @@\n fourth\n")
	dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
	fetches := 0
	dv.SetExtendedFetcher(func(*DiffViewWidget) { fetches++ })

	dv.SetContextMode(DiffContextFullFile)
	if fetches != 1 || !dv.Loading || !dv.extendedFetching {
		t.Fatalf("first fetch state = fetches %d loading %v inFlight %v", fetches, dv.Loading, dv.extendedFetching)
	}
	dv.FailLoading()
	if dv.Loading || dv.extendedFetching || dv.ContextMode() != DiffContextChangesOnly {
		t.Fatalf("failed fetch state = loading %v inFlight %v context %v", dv.Loading, dv.extendedFetching, dv.ContextMode())
	}

	dv.SetContextMode(DiffContextFullFile)
	if fetches != 2 || !dv.Loading || !dv.extendedFetching {
		t.Fatalf("retry state = fetches %d loading %v inFlight %v", fetches, dv.Loading, dv.extendedFetching)
	}
	dv.SetOldLines([]string{"first", "hidden-old-2", "hidden-old-3", "fourth"})
	dv.SetNewLines([]string{"first", "hidden-new-2", "hidden-new-3", "fourth"})
	dv.FinishLoading()
	if dv.Loading || dv.extendedFetching || dv.ContextMode() != DiffContextFullFile || len(dv.Lines) != 4 {
		t.Fatalf("completed fetch state = loading %v inFlight %v context %v lines %d", dv.Loading, dv.extendedFetching, dv.ContextMode(), len(dv.Lines))
	}
	dv.SetContextMode(DiffContextFullFile)
	if fetches != 2 {
		t.Fatalf("loaded full context refetched: %d fetches", fetches)
	}
}

func diffWidgetTopText(dv *DiffViewWidget) string {
	if dv.IsUnified() {
		if dv.TopLine < 0 || dv.TopLine >= len(dv.unifiedLines) {
			return ""
		}
		return dv.unifiedLines[dv.TopLine].side.Text
	}
	if dv.TopLine < 0 || dv.TopLine >= len(dv.Lines) {
		return ""
	}
	return dv.Lines[dv.TopLine].Left.Text
}

func diffWidgetFindTop(dv *DiffViewWidget, text string) int {
	if dv.IsUnified() {
		for i, line := range dv.unifiedLines {
			if line.side.Text == text {
				return i
			}
		}
		return -1
	}
	for i, line := range dv.Lines {
		if line.Left.Text == text {
			return i
		}
	}
	return -1
}

func diffWidgetContextFixture() (diff.FileDiff, []string) {
	patch := "--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n line-01\n@@ -10,1 +10,1 @@\n line-10\n@@ -30,1 +30,1 @@\n line-30\n"
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%02d", i+1)
	}
	return diff.Parse(patch), lines
}

func TestDiffWidgetContextTransitionsPreserveLogicalTop(t *testing.T) {
	fd, lines := diffWidgetContextFixture()
	for _, unified := range []bool{false, true} {
		for _, transition := range []struct {
			name string
			from DiffContextMode
			to   DiffContextMode
		}{
			{"changes-to-full", DiffContextChangesOnly, DiffContextFullFile},
			{"full-to-changes", DiffContextFullFile, DiffContextChangesOnly},
		} {
			t.Run(fmt.Sprintf("unified=%v/%s", unified, transition.name), func(t *testing.T) {
				dv := NewDiffViewWidget("test.go", fd, lines, lines, false)
				dv.SetUnified(unified)
				dv.SetContextMode(transition.from)
				dv.TopLine = diffWidgetFindTop(dv, "line-10")
				if dv.TopLine < 0 || diffWidgetTopText(dv) != "line-10" {
					t.Fatalf("precondition top=%d text=%q", dv.TopLine, diffWidgetTopText(dv))
				}
				dv.SetContextMode(transition.to)
				if got := diffWidgetTopText(dv); got != "line-10" {
					t.Fatalf("logical top after %s = %q at display row %d, want line-10", transition.name, got, dv.TopLine)
				}
			})
		}
	}
}

func TestDiffWidgetContextTransitionChoosesNearestVisibleLogicalTop(t *testing.T) {
	fd, lines := diffWidgetContextFixture()
	for _, unified := range []bool{false, true} {
		for _, test := range []struct {
			anchor string
			want   string
		}{
			{"line-20", "line-10"},
			{"line-29", "line-30"},
		} {
			t.Run(fmt.Sprintf("unified=%v/%s", unified, test.anchor), func(t *testing.T) {
				dv := NewDiffViewWidget("test.go", fd, lines, lines, true)
				dv.SetUnified(unified)
				dv.SetWrapped(true)
				dv.TopLine = diffWidgetFindTop(dv, test.anchor)
				dv.wrapTopOffset = 2
				if dv.TopLine < 0 {
					t.Fatalf("missing full-file anchor %q", test.anchor)
				}

				dv.SetContextMode(DiffContextChangesOnly)
				if got := diffWidgetTopText(dv); got != test.want {
					t.Fatalf("nearest logical top for %s = %q at display row %d, want %s", test.anchor, got, dv.TopLine, test.want)
				}
				if dv.wrapTopOffset != 0 {
					t.Fatalf("context transition retained obsolete wrap offset %d", dv.wrapTopOffset)
				}
			})
		}
	}
}

func TestDiffWidgetContextTransitionClampsAtEnd(t *testing.T) {
	fd, lines := diffWidgetContextFixture()
	for _, unified := range []bool{false, true} {
		t.Run(fmt.Sprintf("unified=%v", unified), func(t *testing.T) {
			dv := NewDiffViewWidget("test.go", fd, lines, lines, true)
			dv.SetUnified(unified)
			dv.SetRect(Rect{W: 60, H: 3})
			dv.Render(NewRenderSurface(makeGrid(60, 3), Rect{W: 60, H: 3}))
			dv.TopLine = diffWidgetFindTop(dv, "line-30")

			dv.SetContextMode(DiffContextChangesOnly)
			dv.Render(NewRenderSurface(makeGrid(60, 3), Rect{W: 60, H: 3}))
			wantTop := max(dv.totalVisualRows-dv.viewH, 0)
			if got := dv.topVisualRow(); got != wantTop {
				t.Fatalf("clamped context top = %d, want %d", got, wantTop)
			}
			if last := diffWidgetFindTop(dv, "line-30"); last < dv.TopLine || last >= dv.TopLine+dv.viewH {
				t.Fatalf("end anchor line-30 at %d is outside visible rows [%d,%d)", last, dv.TopLine, dv.TopLine+dv.viewH)
			}
		})
	}
}

func TestDiffWidgetAsyncFullContextCompletionPreservesLogicalTop(t *testing.T) {
	fd, lines := diffWidgetContextFixture()
	for _, unified := range []bool{false, true} {
		t.Run(fmt.Sprintf("unified=%v", unified), func(t *testing.T) {
			dv := NewDiffViewWidget("test.go", fd, nil, nil, false)
			dv.SetUnified(unified)
			dv.TopLine = diffWidgetFindTop(dv, "line-10")
			dv.wrapTopOffset = 2
			fetches := 0
			dv.SetExtendedFetcher(func(*DiffViewWidget) { fetches++ })

			dv.SetContextMode(DiffContextFullFile)
			if fetches != 1 || !dv.Loading || !dv.extendedFetching {
				t.Fatalf("fetch state = fetches %d loading %v inFlight %v", fetches, dv.Loading, dv.extendedFetching)
			}
			dv.SetOldLines(lines)
			dv.SetNewLines(lines)
			dv.FinishLoading()
			if got := diffWidgetTopText(dv); got != "line-10" {
				t.Fatalf("logical top after async completion = %q at display row %d, want line-10", got, dv.TopLine)
			}
			if dv.wrapTopOffset != 0 {
				t.Fatalf("async completion retained obsolete wrap offset %d", dv.wrapTopOffset)
			}
		})
	}
}

func TestDiffWidgetCollapsedPseudoRowIsNotCopied(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -20,1 +20,1 @@\n twentieth\n")
	oldLines := make([]string, 20)
	newLines := make([]string, 20)
	for line := range oldLines {
		oldLines[line] = fmt.Sprintf("hidden-old-%d", line+1)
		newLines[line] = fmt.Sprintf("hidden-new-%d", line+1)
	}
	oldLines[0], oldLines[19] = "first", "twentieth"
	newLines[0], newLines[19] = "first", "twentieth"
	for _, unified := range []bool{false, true} {
		for _, right := range []bool{false, true} {
			if unified && right {
				continue
			}
			name := fmt.Sprintf("unified=%v/right=%v", unified, right)
			t.Run(name, func(t *testing.T) {
				dv := NewDiffViewWidget("test.go", fd, oldLines, newLines, false)
				dv.SetUnified(unified)
				dv.selRight = right
				collapsedLine := -1
				for line := 0; line < dv.selectionLineCount(); line++ {
					if _, selectable := dv.selectionTextAt(line); !selectable {
						collapsedLine = line
						break
					}
				}
				if collapsedLine < 1 || collapsedLine+1 >= dv.selectionLineCount() {
					t.Fatalf("collapsed row index = %d", collapsedLine)
				}
				dv.hasSelection = true
				dv.selection.Anchor = diffSelPos{Line: collapsedLine - 1, Col: 0}
				lastLine := collapsedLine + 1
				lastText, _ := dv.selectionTextAt(lastLine)
				dv.selection.Current = diffSelPos{Line: lastLine, Col: len([]rune(lastText))}
				if !dv.selection.Contains(collapsedLine, 0) {
					t.Fatal("selection does not cross collapsed pseudo row")
				}
				if got := dv.CopySelection(); got != "first\ntwentieth" {
					t.Fatalf("copy across collapsed row = %q, want only visible source lines", got)
				}
			})
		}
	}
}
