package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"

	"github.com/gdamore/tcell/v3"
)

const portedOneLinePatch = "--- a/test.go\n+++ b/test.go\n@@ -1,3 +1,3 @@\n hello world\n-old line\n+new line\n context\n"

func TestDiffEditorModeAndWrapDefaultsAreIndependent(t *testing.T) {
	d := NewDiffEditorWidget("test.go", diff.FileDiff{}, nil, nil, false)
	d.SetMode(DiffModeUnified)
	d.ApplyDefaultMode(DiffModeSplit)
	d.ApplyDefaultWrapMode(DiffWrapOn)
	if d.Mode() != DiffModeUnified {
		t.Fatalf("mode default replaced explicit unified mode: %v", d.Mode())
	}
	if d.WrapMode() != DiffWrapOn {
		t.Fatalf("mode override prevented inherited wrap update: %v", d.WrapMode())
	}
}

func TestDiffEditorSideLinesAndMergedSearchMatches(t *testing.T) {
	d := NewDiffEditorWidget("test.go", diff.Parse(portedOneLinePatch), nil, nil, false)
	left, right := d.LeftLines(), d.RightLines()
	if len(left) == 0 || len(left) != len(right) {
		t.Fatalf("side lines left=%v right=%v", left, right)
	}
	if !strings.Contains(strings.Join(left, "|"), "old line") || !strings.Contains(strings.Join(right, "|"), "new line") {
		t.Fatalf("sides missing their change: left=%v right=%v", left, right)
	}

	leftMatches, _ := FindInLines(left, "line", SearchOptions{})
	rightMatches, _ := FindInLines(right, "line", SearchOptions{})
	merged := d.SetSearchMatches(leftMatches, rightMatches)
	if len(merged) == 0 || len(merged) != len(leftMatches)+len(rightMatches) {
		t.Fatalf("merged %d matches, want %d + %d", len(merged), len(leftMatches), len(rightMatches))
	}
	d.SetActiveMatch(0)
	active := false
	for _, p := range d.panes() {
		active = active || p.SearchActive >= 0
	}
	if !active {
		t.Fatal("SetActiveMatch(0) left no active match")
	}

	contextLeft, _ := FindInLines(left, "hello", SearchOptions{})
	contextRight, _ := FindInLines(right, "hello", SearchOptions{})
	if len(contextLeft)+len(contextRight) == 0 {
		t.Fatal("search found nothing in the context line")
	}
}

func TestDiffEditorWrapContinuationHasNoLineNumber(t *testing.T) {
	d := NewDiffEditorWidget("test.go", diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-abcdefghijklmno\n+ABCDEFGHIJKLMNO\n"), nil, nil, false)
	d.SetWrapped(true)
	grid := renderDiffEditor(d, 30, 4)
	first, cont := cellRow(grid, 0), cellRow(grid, 1)
	if !strings.Contains(first, "abc") || !strings.Contains(cont, "lmno") {
		t.Fatalf("expected the long line to wrap:\n%s\n%s", first, cont)
	}
	if gutter := []rune(cont)[:d.left.GutterWidth()]; strings.ContainsRune(string(gutter), '1') {
		t.Fatalf("continuation gutter repeats the line number: %q", string(gutter))
	}
}

func TestDiffEditorWrappedSelectionCopiesOriginalText(t *testing.T) {
	d := NewDiffEditorWidget("test.go", diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-abcdefghij\n+ABCDEFGHIJ\n"), nil, nil, false)
	d.SetUnified(true)
	d.SetWrapped(true)
	grid := renderDiffEditor(d, 12, 6)
	sx, sy := cellAtText(t, grid, "c")
	if strings.Contains(cellRow(grid, sy), "j") {
		t.Fatalf("precondition: the line should wrap at this width:\n%s", cellRow(grid, sy))
	}
	ex, ey := cellAtText(t, grid, "j")
	d.HandleEvent(tcell.NewEventMouse(sx, sy, tcell.Button1, tcell.ModNone))
	d.HandleEvent(tcell.NewEventMouse(ex+1, ey, tcell.Button1, tcell.ModNone))
	d.HandleEvent(tcell.NewEventMouse(ex+1, ey, tcell.ButtonNone, tcell.ModNone))
	if got := d.CopySelection(); got != "cdefghij" {
		t.Fatalf("wrapped copy = %q, want the unwrapped text %q", got, "cdefghij")
	}
}

func TestDiffEditorAdjacentHunksHaveNoGapBetweenThem(t *testing.T) {
	d := NewDiffEditorWidget("test.go", diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -24,1 +24,1 @@\n line 24\n@@ -25,1 +25,1 @@\n line 25\n"), nil, nil, false)
	var texts []string
	for _, dl := range d.Lines {
		if dl.Left.Kind == diff.Collapsed && dl.Left.Num == 0 && strings.Contains(dl.Left.Text, "⋯") && len(texts) > 0 {
			t.Fatalf("gap row between adjacent hunks: %v", d.Lines)
		}
		if dl.Left.Kind != diff.Collapsed {
			texts = append(texts, dl.Left.Text)
		}
	}
	if strings.Join(texts, "|") != "line 24|line 25" {
		t.Fatalf("adjacent rows = %v, want line 24 then line 25", texts)
	}
}

func TestDiffEditorWrappedGapRowExpandsFromAnyColumn(t *testing.T) {
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -20,1 +20,1 @@\n twentieth\n")
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	for _, mode := range []DiffMode{DiffModeSplit, DiffModeUnified} {
		t.Run(fmt.Sprintf("mode-%d", mode), func(t *testing.T) {
			d := NewDiffEditorWidget("test.go", fd, lines, append([]string(nil), lines...), false)
			d.SetMode(mode)
			d.SetWrapMode(DiffWrapOn)
			const width, height = 40, 12
			grid := renderDiffEditor(d, width, height)
			gapY := -1
			for y := range grid {
				if strings.Contains(cellRow(grid, y), "⋯") {
					gapY = y
					break
				}
			}
			if gapY < 0 {
				t.Fatal("no gap row on screen")
			}
			d.HandleEvent(tcell.NewEventMouse(width-2, gapY, tcell.Button1, tcell.ModNone))
			d.HandleEvent(tcell.NewEventMouse(width-2, gapY, tcell.ButtonNone, tcell.ModNone))
			for _, dl := range d.Lines {
				if dl.Left.Kind == diff.Collapsed {
					t.Fatalf("gap not expanded by a click at the row's far end: %v", d.Lines)
				}
			}
			if d.ContextMode() != DiffContextChangesOnly {
				t.Fatalf("expanding one gap switched the context mode to %v", d.ContextMode())
			}
		})
	}
}

func TestDiffEditorFullFileRequestSupersedesInflightGapExpansion(t *testing.T) {
	d := NewDiffEditorWidget("test.go", diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n first\n@@ -4,1 +4,1 @@\n fourth\n"), nil, nil, false)
	fetches := 0
	d.SetExtendedFetcher(func(*DiffEditorWidget) { fetches++ })
	d.expandContextGap(0)
	if fetches != 1 || !d.Loading {
		t.Fatalf("gap fetch state = fetches %d loading %v", fetches, d.Loading)
	}
	d.SetContextMode(DiffContextFullFile)
	d.SetOldLines([]string{"first", "old-two", "old-three", "fourth"})
	d.SetNewLines([]string{"first", "new-two", "new-three", "fourth"})
	d.FinishLoading()
	if fetches != 1 || d.ContextMode() != DiffContextFullFile || !d.IsExtended() || len(d.Lines) != 4 {
		t.Fatalf("full-file completion = fetches %d context %v extended %v lines %d", fetches, d.ContextMode(), d.IsExtended(), len(d.Lines))
	}
}

func portedContextFixture() (diff.FileDiff, []string) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i+1)
	}
	fd := diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -10,1 +10,1 @@\n line-10\n@@ -30,1 +30,1 @@\n line-30\n")
	return fd, lines
}

func TestDiffEditorContextTransitionChoosesNearestVisibleTop(t *testing.T) {
	fd, lines := portedContextFixture()
	for _, unified := range []bool{false, true} {
		for _, tc := range []struct{ anchor, want string }{{"line-20", "line-10"}, {"line-29", "line-30"}} {
			t.Run(fmt.Sprintf("unified=%v/%s", unified, tc.anchor), func(t *testing.T) {
				d := NewDiffEditorWidget("test.go", fd, lines, lines, true)
				d.SetUnified(unified)
				renderDiffEditor(d, 60, 2)
				diffEditorSetTop(t, d, tc.anchor)
				d.SetContextMode(DiffContextChangesOnly)
				grid := renderDiffEditor(d, 60, 2)
				if diffEditorRowOf(grid, tc.want) < 0 {
					t.Fatalf("collapsing from %s left %s off screen:\n%s\n%s", tc.anchor, tc.want, cellRow(grid, 0), cellRow(grid, 1))
				}
			})
		}
	}
}

func TestDiffEditorContextTransitionKeepsTheEndInView(t *testing.T) {
	fd, lines := portedContextFixture()
	for _, unified := range []bool{false, true} {
		t.Run(fmt.Sprintf("unified=%v", unified), func(t *testing.T) {
			d := NewDiffEditorWidget("test.go", fd, lines, lines, true)
			d.SetUnified(unified)
			renderDiffEditor(d, 60, 3)
			diffEditorSetTop(t, d, "line-30")
			d.SetContextMode(DiffContextChangesOnly)
			grid := renderDiffEditor(d, 60, 3)
			if diffEditorRowOf(grid, "line-30") < 0 {
				t.Fatalf("line-30 scrolled out of view after collapsing:\n%s\n%s\n%s", cellRow(grid, 0), cellRow(grid, 1), cellRow(grid, 2))
			}
		})
	}
}

func TestDiffEditorAsyncFullContextKeepsTheTop(t *testing.T) {
	fd, lines := portedContextFixture()
	for _, unified := range []bool{false, true} {
		t.Run(fmt.Sprintf("unified=%v", unified), func(t *testing.T) {
			d := NewDiffEditorWidget("test.go", fd, nil, nil, false)
			d.SetUnified(unified)
			renderDiffEditor(d, 60, 3)
			diffEditorSetTop(t, d, "line-10")
			fetches := 0
			d.SetExtendedFetcher(func(*DiffEditorWidget) { fetches++ })
			d.SetContextMode(DiffContextFullFile)
			if fetches != 1 || !d.Loading {
				t.Fatalf("fetch state = fetches %d loading %v", fetches, d.Loading)
			}
			d.SetOldLines(lines)
			d.SetNewLines(lines)
			d.FinishLoading()
			renderDiffEditor(d, 60, 3)
			if got := diffEditorTopText(d); got != "line-10" {
				t.Fatalf("top after the async full-file load = %q, want line-10", got)
			}
		})
	}
}

func TestDiffEditorModeAndWrapChangesKeepTheTop(t *testing.T) {
	var old, cur []string
	for i := 1; i <= 8; i++ {
		text := fmt.Sprintf("row-%d-abcdefghijklmnopqrstuvwxyz", i)
		old = append(old, text)
		if i == 2 {
			text = "changed-2-abcdefghijklmnopqrstuvwxyz"
		}
		cur = append(cur, text)
	}
	d := NewDiffEditorWidget("test.go", diff.FileDiff{}, old, cur, true)
	d.SetWrapped(true)
	renderDiffEditor(d, 40, 3)
	diffEditorSetTop(t, d, "row-4-abcdefghijklmnopqrstuvwxyz")
	steps := []func(){
		func() { d.SetUnified(true) },
		func() { d.SetWrapped(false) },
		func() { d.SetUnified(false); d.SetWrapped(true) },
	}
	for i, step := range steps {
		step()
		renderDiffEditor(d, 40, 3)
		if got := diffEditorTopText(d); got != "row-4-abcdefghijklmnopqrstuvwxyz" {
			t.Fatalf("step %d moved the top to %q", i, got)
		}
	}
}
