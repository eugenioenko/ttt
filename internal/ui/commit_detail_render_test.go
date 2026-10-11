package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func renderDetail(detail *CommitDetailWidget, w, h int) [][]term.Cell {
	rect := Rect{W: w, H: h}
	detail.SetRect(rect)
	grid := makeGrid(w, h)
	detail.Render(NewRenderSurface(grid, rect))
	return grid
}

func detailFind(grid [][]term.Cell, text string) (x, y int, ok bool) {
	for y, line := range strings.Split(detailTestText(grid), "\n") {
		if i := strings.Index(line, text); i >= 0 {
			return len([]rune(line[:i])), y, true
		}
	}
	return 0, 0, false
}

func twoHunkDetail() *CommitDetailWidget {
	detail := NewCommitDetailWidget("/repo", "full", "abc1234", false)
	detail.SetDetail("Subject", []CommitDetailFile{{
		Path: "test.go",
		Diff: diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n-old one\n+new one\n@@ -10,1 +10,1 @@\n-old ten\n+new ten\n"),
	}}, "")
	return detail
}

func TestCommitDetailSplitShowsSidesOnOneRow(t *testing.T) {
	grid := renderDetail(twoHunkDetail(), 60, 15)
	_, oldY, okOld := detailFind(grid, "old one")
	_, newY, okNew := detailFind(grid, "new one")
	if !okOld || !okNew || oldY != newY {
		t.Fatalf("split sides not aligned (old %v@%d, new %v@%d):\n%s", okOld, oldY, okNew, newY, detailTestText(grid))
	}
}

func TestCommitDetailUnifiedShowsRemovalBeforeAddition(t *testing.T) {
	detail := twoHunkDetail()
	detail.SetMode(DiffModeUnified)
	grid := renderDetail(detail, 60, 15)
	_, oldY, okOld := detailFind(grid, "old one")
	_, newY, okNew := detailFind(grid, "new one")
	if !okOld || !okNew || newY != oldY+1 {
		t.Fatalf("unified rows (old %v@%d, new %v@%d):\n%s", okOld, oldY, okNew, newY, detailTestText(grid))
	}
}

func TestCommitDetailDragCopiesDiffText(t *testing.T) {
	detail := twoHunkDetail()
	grid := renderDetail(detail, 60, 15)
	x, y, ok := detailFind(grid, "new one")
	if !ok {
		t.Fatal("missing diff text")
	}
	detail.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
	detail.HandleEvent(tcell.NewEventMouse(x+len("new one"), y, tcell.Button1, tcell.ModNone))
	detail.HandleEvent(tcell.NewEventMouse(x+len("new one"), y, tcell.ButtonNone, tcell.ModNone))
	if got := detail.CopySelection(); got != "new one" {
		t.Fatalf("copied %q, want the dragged diff text", got)
	}
}

func TestCommitDetailStickyHeadingWhileScrolledIntoFile(t *testing.T) {
	detail := twoHunkDetail()
	renderDetail(detail, 60, 4)
	detail.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, "", tcell.ModNone))
	grid := renderDetail(detail, 60, 4)
	if first := strings.Split(detailTestText(grid), "\n")[0]; !strings.Contains(first, "▼ test.go") {
		t.Fatalf("top row = %q, want the sticky file heading", first)
	}
}

func TestCommitDetailKeysScrollDocument(t *testing.T) {
	detail := twoHunkDetail()
	renderDetail(detail, 60, 4)
	detail.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	if detail.TopLine != 1 {
		t.Fatalf("down key TopLine = %d, want 1", detail.TopLine)
	}
	detail.HandleEvent(tcell.NewEventKey(tcell.KeyEnd, "", tcell.ModNone))
	if detail.TopLine != detail.totalVisualRows-detail.viewH {
		t.Fatalf("end key TopLine = %d, want %d", detail.TopLine, detail.totalVisualRows-detail.viewH)
	}
}

func TestCommitDetailWrapsLongDiffLines(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full", "abc1234", false)
	detail.SetDetail("Subject", []CommitDetailFile{{
		Path: "test.go",
		Diff: diff.Parse("--- a/test.go\n+++ b/test.go\n@@ -1,1 +1,1 @@\n-old\n+alpha beta gamma delta epsilon zeta\n"),
	}}, "")
	detail.SetWrapMode(DiffWrapOn)
	grid := renderDetail(detail, 40, 15)
	text := detailTestText(grid)
	if !strings.Contains(text, "alpha") || !strings.Contains(text, "delta") {
		t.Fatalf("wrapped line lost text:\n%s", text)
	}
	_, alphaY, _ := detailFind(grid, "alpha")
	_, deltaY, _ := detailFind(grid, "delta")
	if deltaY <= alphaY {
		t.Fatalf("long line did not wrap (alpha@%d delta@%d):\n%s", alphaY, deltaY, text)
	}
}

func TestCommitDetailShowsGapBetweenHunks(t *testing.T) {
	grid := renderDetail(twoHunkDetail(), 60, 15)
	if _, _, ok := detailFind(grid, "8 lines"); !ok {
		t.Fatalf("missing the collapsed-lines row between hunks:\n%s", detailTestText(grid))
	}
}

func TestCommitDetailCollapseHidesTheFileDiff(t *testing.T) {
	detail := twoHunkDetail()
	renderDetail(detail, 60, 15)
	detail.CollapseAllFiles()
	grid := renderDetail(detail, 60, 15)
	text := detailTestText(grid)
	if !strings.Contains(text, "test.go") || strings.Contains(text, "new one") {
		t.Fatalf("collapsed file should keep its heading and hide its diff:\n%s", text)
	}
}

func TestCommitDetailGutterShowsSourceLineNumbers(t *testing.T) {
	grid := renderDetail(twoHunkDetail(), 60, 15)
	_, y, ok := detailFind(grid, "new ten")
	if !ok {
		t.Fatal("missing the second hunk")
	}
	if row := strings.Split(detailTestText(grid), "\n")[y]; !strings.Contains(row, "10") {
		t.Fatalf("second hunk row %q lacks its line number 10", row)
	}
}

func TestCommitDetailSteadyFrameKeepsItsVisualRows(t *testing.T) {
	detail := manyFileDetail(20)
	renderDetail(detail, 80, 10)
	if detail.totalVisualRows <= 10 {
		t.Fatal("fixture: the commit should need a vertical scrollbar")
	}
	before := &detail.visualRows[0]
	renderDetail(detail, 80, 10)
	if &detail.visualRows[0] != before {
		t.Fatal("a steady frame rebuilt the visual rows; the scrollbar width toggles the cached layout")
	}
}

func dragSelect(t *testing.T, detail *CommitDetailWidget, grid [][]term.Cell, text string) {
	t.Helper()
	x, y, ok := detailFind(grid, text)
	if !ok {
		t.Fatalf("%q not on screen:\n%s", text, detailTestText(grid))
	}
	detail.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
	detail.HandleEvent(tcell.NewEventMouse(x+len(text), y, tcell.Button1, tcell.ModNone))
	detail.HandleEvent(tcell.NewEventMouse(x+len(text), y, tcell.ButtonNone, tcell.ModNone))
}

func TestCommitDetailKeepsSelectionWhenAnotherFileLoadsContext(t *testing.T) {
	detail := manyFileDetail(2)
	detail.OnFetchContext = func(int, CommitDetailFile) {}
	dragSelect(t, detail, renderDetail(detail, 80, 40), "new line 0")
	if !detail.Files[0].view.hasSelection() {
		t.Fatal("fixture: dragging should select text in the first file")
	}

	detail.requestFileContext(1, -1)
	renderDetail(detail, 80, 40)
	if !detail.Files[0].view.hasSelection() {
		t.Fatal("requesting context for another file cleared the first file's selection")
	}
}

func TestCommitDetailDragSurvivesFewerFiles(t *testing.T) {
	detail := manyFileDetail(2)
	detail.CurrentChanges = true
	grid := renderDetail(detail, 80, 40)
	x, y, ok := detailFind(grid, "new line 1")
	if !ok {
		t.Fatalf("second file not on screen:\n%s", detailTestText(grid))
	}
	detail.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
	if detail.capturedFile != 1 {
		t.Fatalf("fixture: drag should capture file 1, got %d", detail.capturedFile)
	}

	detail.SetCurrentChanges("Subject", detail.Files[:1], "")
	if len(detail.Files) != 1 {
		t.Fatalf("fixture: the refresh should leave 1 file, got %d", len(detail.Files))
	}
	detail.HandleEvent(tcell.NewEventMouse(x+3, y, tcell.Button1, tcell.ModNone))
	detail.HandleEvent(tcell.NewEventMouse(x+3, y, tcell.ButtonNone, tcell.ModNone))
	if detail.capturedFile != -1 {
		t.Fatalf("captured file = %d after the file list shrank, want -1", detail.capturedFile)
	}
}

func TestCommitDetailRefreshCancelsTheFileViewPress(t *testing.T) {
	detail := manyFileDetail(2)
	detail.CurrentChanges = true
	grid := renderDetail(detail, 80, 40)
	x, y, ok := detailFind(grid, "new line 0")
	if !ok {
		t.Fatalf("first file not on screen:\n%s", detailTestText(grid))
	}
	detail.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
	view := detail.Files[0].view
	if !view.pressed {
		t.Fatal("fixture: the press should reach the file's diff view")
	}

	detail.SetCurrentChanges("Subject", detail.Files, "")
	if detail.Files[0].view != view {
		t.Fatal("fixture: the refresh should keep the file's diff view")
	}
	if view.pressed || view.captured != nil {
		t.Fatal("the refresh cancelled the drag but left the file view pressed")
	}
}
