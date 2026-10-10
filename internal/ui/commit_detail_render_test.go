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
