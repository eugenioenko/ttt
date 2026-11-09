package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
)

func statsDetail() *CommitDetailWidget {
	detail := NewCommitDetailWidget("/repo", "full", "abc1234", false)
	detail.SetDetail("Subject", []CommitDetailFile{
		{Path: "a.go", Diff: diff.Parse("--- a/a.go\n+++ b/a.go\n@@ -1,2 +1,3 @@\n-old\n+new\n+more\n keep\n")},
		{Path: "b.go", Diff: diff.Parse("--- a/b.go\n+++ b/b.go\n@@ -1,2 +1,1 @@\n-gone\n-gone too\n+kept\n")},
	}, "")
	return detail
}

func TestCommitDetailSummaryShowsColoredTotals(t *testing.T) {
	grid := renderDetail(statsDetail(), 60, 30)
	x, y, ok := detailFind(grid, "2 files · +3 −3")
	if !ok {
		t.Fatalf("summary not rendered:\n%s", detailTestText(grid))
	}
	plusX := x + len([]rune("2 files · "))
	if grid[y][plusX].Style != term.StyleSuccess || grid[y][plusX+3].Style != term.StyleDanger {
		t.Fatalf("summary styles = %v %v, want success and danger", grid[y][plusX].Style, grid[y][plusX+3].Style)
	}
}

func TestCommitDetailHeadingShowsFileStatsAtRightEdge(t *testing.T) {
	grid := renderDetail(statsDetail(), 60, 30)
	text := detailTestText(grid)
	for _, want := range []string{"+2 −1", "+1 −2"} {
		x, y, ok := detailFind(grid, want)
		if !ok {
			t.Fatalf("file stats %q not rendered:\n%s", want, text)
		}
		if end := x + len([]rune(want)); end != 59 {
			t.Fatalf("file stats %q end at column %d, want 59", want, end)
		}
		if grid[y][x].Style != term.StyleSuccess {
			t.Fatalf("file stats %q style = %v, want success", want, grid[y][x].Style)
		}
	}
}

func TestCommitDetailHeadingHidesStatsWhenTheyDoNotFit(t *testing.T) {
	detail := NewCommitDetailWidget("/repo", "full", "abc1234", false)
	path := strings.Repeat("long/", 10) + "file.go"
	detail.SetDetail("Subject", []CommitDetailFile{{
		Path: path, Diff: diff.Parse("--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n"),
	}}, "")
	grid := renderDetail(detail, 60, 20)
	_, y, ok := detailFind(grid, "long/long")
	if !ok {
		t.Fatalf("heading not rendered:\n%s", detailTestText(grid))
	}
	if strings.Contains(strings.Split(detailTestText(grid), "\n")[y], "+1 −1") {
		t.Fatalf("file stats overlapped a heading that fills the row:\n%s", detailTestText(grid))
	}
}

func TestCurrentChangesSkipsDuplicateSummary(t *testing.T) {
	detail := NewCurrentChangesWidget("/repo", false)
	detail.SetCurrentChanges("2 files · +3 −3", statsDetail().Files, "")
	grid := renderDetail(detail, 60, 30)
	if got := strings.Count(detailTestText(grid), "2 files · +3 −3"); got != 1 {
		t.Fatalf("summary shown %d times, want 1:\n%s", got, detailTestText(grid))
	}
}
