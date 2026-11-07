package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/view"

	"github.com/gdamore/tcell/v3"
)

// hunkDiffPatch has two change blocks separated by more unchanged context than
// the compact projection keeps, so the widget shows a collapsed gap between
// them and the second block is only reachable by navigation.
const hunkDiffPatch = `diff --git a/f.txt b/f.txt
--- a/f.txt
+++ b/f.txt
@@ -3,5 +3,5 @@
 line 3
 line 4
-line 5
+changed 5
 line 6
 line 7
@@ -20,4 +20,3 @@
 line 20
-line 21
 line 22
 line 23
`

func newHunkDiffWidget(t *testing.T) *DiffEditorWidget {
	t.Helper()
	d := NewDiffEditorWidget("f.txt", diff.Parse(hunkDiffPatch), nil, nil, false)
	d.rebuild()
	return d
}

func TestDiffHunksCountAndKinds(t *testing.T) {
	d := newHunkDiffWidget(t)
	hunks := d.Hunks()
	if len(hunks) != 2 {
		t.Fatalf("expected 2 change blocks, got %d: %+v", len(hunks), hunks)
	}
	if hunks[0].Kind != DiffHunkModified {
		t.Errorf("first block kind = %q, want modified", hunks[0].Kind)
	}
	if hunks[1].Kind != DiffHunkDeleted {
		t.Errorf("second block kind = %q, want deleted", hunks[1].Kind)
	}
	if hunks[0].FilePath != "f.txt" {
		t.Errorf("file path = %q, want f.txt", hunks[0].FilePath)
	}
	if hunks[1].Row <= hunks[0].Row {
		t.Errorf("blocks must be in document order: %d then %d", hunks[0].Row, hunks[1].Row)
	}
}

// A collapsed gap hides unchanged context. It must not merge two changes into
// one block, or a jump would skip the second.
func TestDiffHunksSurviveCollapsedGaps(t *testing.T) {
	d := newHunkDiffWidget(t)
	collapsed := false
	for _, line := range d.Lines {
		if line.Left.Kind == diff.Collapsed {
			collapsed = true
			break
		}
	}
	if !collapsed {
		t.Fatal("expected a collapsed gap between the two blocks")
	}
	if got := len(d.Hunks()); got != 2 {
		t.Errorf("collapsed gap changed the block count: got %d, want 2", got)
	}
}

func TestDiffHunkAtReportsPositionBetweenBlocks(t *testing.T) {
	d := newHunkDiffWidget(t)
	hunks := d.Hunks()
	if len(hunks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(hunks))
	}
	if got := d.HunkAt(0); got != 0 {
		t.Errorf("before first block: got %d, want 0", got)
	}
	if got := d.HunkAt(hunks[0].Row); got != 1 {
		t.Errorf("at first block: got %d, want 1", got)
	}
	if got := d.HunkAt(hunks[0].Row + 1); got != 1 {
		t.Errorf("between blocks: got %d, want 1", got)
	}
	if got := d.HunkAt(hunks[1].Row); got != 2 {
		t.Errorf("at second block: got %d, want 2", got)
	}
	if got := d.HunkAt(hunks[1].Row + 5); got != 2 {
		t.Errorf("after last block: got %d, want 2", got)
	}
}

// After a jump the reported position must be the block that was jumped to.
// The cursor row itself can differ from the block's first row: in split mode a
// deletion has no row on the right pane, so the cursor settles on the next
// real line. HunkAt is what callers navigate from, so it must round-trip.
func TestGoToHunkLandsOnRequestedBlock(t *testing.T) {
	for _, unified := range []bool{false, true} {
		d := newHunkDiffWidget(t)
		d.SetRect(Rect{X: 0, Y: 0, W: 120, H: 20})
		d.SetUnified(unified)
		hunks := d.Hunks()
		if len(hunks) != 2 {
			t.Fatalf("unified=%v: expected 2 blocks, got %d", unified, len(hunks))
		}
		for i := range hunks {
			if err := d.GoToHunk(i + 1); err != nil {
				t.Fatalf("unified=%v GoToHunk(%d): %v", unified, i+1, err)
			}
			if got := d.HunkAt(d.CursorRow()); got != i+1 {
				t.Errorf("unified=%v: after GoToHunk(%d) position = %d", unified, i+1, got)
			}
		}
	}
}

func TestGoToHunkRejectsOutOfRange(t *testing.T) {
	d := newHunkDiffWidget(t)
	if err := d.GoToHunk(0); err == nil {
		t.Error("GoToHunk(0) must fail")
	}
	if err := d.GoToHunk(len(d.Hunks()) + 1); err == nil {
		t.Error("GoToHunk past the end must fail")
	}
}

// An added-only block and a deleted-only block must be told apart, because a
// plugin labels them differently.
func TestDiffHunkKindsForPureAddAndDelete(t *testing.T) {
	lines := []diff.DiffLine{
		{Right: diff.SideLine{Kind: diff.Added, Text: "new"}},
		{Left: diff.SideLine{Kind: diff.Context, Text: "same"}, Right: diff.SideLine{Kind: diff.Context, Text: "same"}},
		{Left: diff.SideLine{Kind: diff.Deleted, Text: "gone"}},
	}
	hunks := diffHunksInRows(lines, "a.txt", 3)
	if len(hunks) != 2 {
		t.Fatalf("expected 2 blocks, got %d: %+v", len(hunks), hunks)
	}
	if hunks[0].Kind != DiffHunkAdded {
		t.Errorf("kind = %q, want added", hunks[0].Kind)
	}
	if hunks[1].Kind != DiffHunkDeleted {
		t.Errorf("kind = %q, want deleted", hunks[1].Kind)
	}
	if hunks[0].FileIndex != 3 {
		t.Errorf("file index = %d, want 3", hunks[0].FileIndex)
	}
}

// commitDetailWithFiles builds a detail whose diff is short enough that the
// viewport cannot scroll, which is the case where a viewport-derived position
// would stay stuck at the first block.
func commitDetailWithFiles(t *testing.T, paths ...string) *CommitDetailWidget {
	t.Helper()
	files := make([]CommitDetailFile, 0, len(paths))
	for _, path := range paths {
		files = append(files, CommitDetailFile{Path: path, Status: "M", Diff: diff.Parse(hunkDiffPatch)})
	}
	d := NewCommitDetailWidget("", "", "", false)
	d.SetDetail("", files, "")
	d.SetRect(Rect{X: 0, Y: 0, W: 167, H: 24})
	return d
}

func TestCommitDetailHunksSpanAllFiles(t *testing.T) {
	d := commitDetailWithFiles(t, "a.txt", "b.txt")
	hunks := d.Hunks()
	if len(hunks) != 4 {
		t.Fatalf("expected 4 blocks across 2 files, got %d: %+v", len(hunks), hunks)
	}
	if hunks[0].FilePath != "a.txt" || hunks[2].FilePath != "b.txt" {
		t.Errorf("file order wrong: %q then %q", hunks[0].FilePath, hunks[2].FilePath)
	}
	if hunks[2].FileIndex != 1 {
		t.Errorf("second file index = %d, want 1", hunks[2].FileIndex)
	}
}

// Navigation must advance even when the content fits the viewport, because a
// viewport position would stay at the first block forever.
func TestCommitDetailGoToHunkAdvancesPosition(t *testing.T) {
	d := commitDetailWithFiles(t, "a.txt", "b.txt")
	total := len(d.Hunks())
	for i := 1; i <= total; i++ {
		if err := d.GoToHunk(i); err != nil {
			t.Fatalf("GoToHunk(%d): %v", i, err)
		}
		fileIndex, row := d.ViewportFileAndRow()
		if got := d.HunkAt(fileIndex, row); got != i {
			t.Errorf("after GoToHunk(%d): position = %d", i, got)
		}
	}
}

func TestCommitDetailGoToHunkRejectsOutOfRange(t *testing.T) {
	d := commitDetailWithFiles(t, "a.txt")
	if err := d.GoToHunk(0); err == nil {
		t.Error("GoToHunk(0) must fail")
	}
	if err := d.GoToHunk(len(d.Hunks()) + 1); err == nil {
		t.Error("GoToHunk past the end must fail")
	}
}

// A collapsed file keeps its blocks listed so a jump reveals it instead of
// skipping its changes.
func TestCommitDetailHunksIncludeCollapsedFiles(t *testing.T) {
	d := commitDetailWithFiles(t, "a.txt", "b.txt")
	before := len(d.Hunks())
	d.CollapseAllFiles()
	if got := len(d.Hunks()); got != before {
		t.Errorf("collapsing changed the block count: got %d, want %d", got, before)
	}
	if err := d.GoToHunk(3); err != nil {
		t.Fatalf("GoToHunk into a collapsed file: %v", err)
	}
	if d.collapsedFiles[1] {
		t.Error("jumping into a collapsed file must expand it")
	}
}

// An editable diff enumerates blocks in its uncompacted rows, so a jump lands
// on the same block that editor.diff() reported.
func TestEditableDiffHunksAndNavigation(t *testing.T) {
	old := []string{"line 1", "line 2", "line 3", "line 4", "line 5", "line 6", "line 7", "line 8", "line 9", "line 10"}
	cur := append([]string{}, old...)
	cur[2] = "changed 3"
	cur = append(cur[:9], "line 11")
	d := NewEditableDiffWidget("f.txt", old, diff.FullDiffLines(old, cur))
	d.SetRect(Rect{X: 0, Y: 0, W: 160, H: 30})
	// An editable diff borrows the editor's live pane; bind one so the cursor
	// this test reads is the cursor navigation moved.
	pane := NewEditorPaneWidget(&buffer.Buffer{Lines: cur}, &cursor.Cursor{}, &view.Viewport{})
	d.bind(pane)
	d.rebuild()

	hunks := d.Hunks()
	if len(hunks) != 2 {
		t.Fatalf("expected 2 blocks, got %d: %+v", len(hunks), hunks)
	}
	for i := range hunks {
		if err := d.GoToHunk(i + 1); err != nil {
			t.Fatalf("GoToHunk(%d) on an editable diff: %v", i+1, err)
		}
		if got := d.HunkAt(d.CursorRow()); got != i+1 {
			t.Errorf("after GoToHunk(%d): position = %d", i+1, got)
		}
	}
}

// Scrolling moves the visible position, so a stale jump anchor must not keep
// answering for it.
func TestCommitDetailScrollingClearsTheJumpAnchor(t *testing.T) {
	d := commitDetailWithFiles(t, "a.txt")
	if err := d.GoToHunk(2); err != nil {
		t.Fatalf("GoToHunk(2): %v", err)
	}
	fileIndex, row := d.ViewportFileAndRow()
	if got := d.HunkAt(fileIndex, row); got != 2 {
		t.Fatalf("after jump: position = %d, want 2", got)
	}
	d.HandleEvent(tcell.NewEventKey(tcell.KeyHome, "", 0))
	fileIndex, row = d.ViewportFileAndRow()
	if got := d.HunkAt(fileIndex, row); got == 2 {
		t.Error("Home must clear the jump anchor and report the visible position")
	}
}

// An editable inline diff is attached to the file tab, not opened as a content
// tab, so it needs its own accessor. Testing the widget alone missed this: the
// widget worked while the tab-level lookup returned nil.
func TestActiveInlineDiffFindsAnInlineDiff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\nd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := NewEditorGroupWidget(nil, 4, false, "compact")
	g.OpenFile(path)
	if !g.EnableInlineDiff(path, []string{"a", "B", "c", "d"}) {
		t.Fatal("inline diff did not turn on")
	}
	if g.ActiveInlineDiff() == nil {
		t.Fatal("ActiveInlineDiff returned nil for an inline diff")
	}
	if got := len(g.ActiveInlineDiff().Hunks()); got != 1 {
		t.Errorf("hunks = %d, want 1", got)
	}
	// ActiveDiffWidget stays reserved for read-only diff tabs.
	if g.ActiveDiffWidget() != nil {
		t.Error("ActiveDiffWidget must stay nil for an editable inline diff")
	}
	// A plain file tab must still report no diff.
	g.DisableInlineDiff(path)
	if g.ActiveInlineDiff() != nil {
		t.Error("ActiveInlineDiff must be nil once the inline diff is off")
	}
}
