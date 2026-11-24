package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
)

// openHunkFile writes and opens a 20-line file, then stamps a synthetic
// LineChanges slice on the active editor. Git state is not wired here on
// purpose: the value under test is the hunk-grouping/navigation logic, which is
// isolated cleanly by setting LineChanges directly (per the phase-7 plan).
func openHunkFile(t *testing.T, h *testHarness) {
	t.Helper()
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, "line "+string(rune('0'+i/10))+string(rune('0'+i%10)))
	}
	f := filepath.Join(h.dir, "hunks.txt")
	requireNoError(t, os.WriteFile(f, []byte(strings.Join(lines, "\n")+"\n"), 0644))
	h.openFile(f)
	h.redraw()

	// Two hunks: a run at lines 5-6 (0-based 4-5) and a run at line 15
	// (0-based 14). Everything else unchanged. SetLineChanges stores the slice on
	// the tab so a redraw does not overwrite it from the (empty) tab state.
	changes := make([]diff.LineChangeKind, 20)
	changes[4] = diff.LineModified
	changes[5] = diff.LineAdded
	changes[14] = diff.LineModified
	h.app.EditorGroup.SetLineChanges(f, changes)
	h.redraw()
}

func cursorLine1(h *testHarness) int {
	line, _ := h.app.EditorGroup.ActiveCursor()
	return line + 1
}

func TestDiffNextHunk(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openHunkFile(t, h)

	// Start at line 1.
	h.app.EditorGroup.GoToLine(1)
	h.redraw()

	h.exec("diff.nextHunk")
	if got := cursorLine1(h); got != 5 {
		t.Fatalf("nextHunk from line 1: expected line 5, got %d", got)
	}

	// From inside the first hunk (line 5), next jumps past the run to line 15.
	h.exec("diff.nextHunk")
	if got := cursorLine1(h); got != 15 {
		t.Fatalf("nextHunk from line 5: expected line 15, got %d", got)
	}

	// No hunk below line 15: cursor stays put.
	h.exec("diff.nextHunk")
	if got := cursorLine1(h); got != 15 {
		t.Fatalf("nextHunk from last hunk: expected line 15 (no-op), got %d", got)
	}
}

func TestDiffPrevHunk(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openHunkFile(t, h)

	h.app.EditorGroup.GoToLine(20)
	h.redraw()

	h.exec("diff.prevHunk")
	if got := cursorLine1(h); got != 15 {
		t.Fatalf("prevHunk from line 20: expected line 15, got %d", got)
	}

	h.exec("diff.prevHunk")
	if got := cursorLine1(h); got != 5 {
		t.Fatalf("prevHunk from line 15: expected line 5, got %d", got)
	}

	// No hunk above line 5: cursor stays put.
	h.exec("diff.prevHunk")
	if got := cursorLine1(h); got != 5 {
		t.Fatalf("prevHunk from first hunk: expected line 5 (no-op), got %d", got)
	}
}

func TestDiffHunkNoChanges(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	openHunkFile(t, h)

	// Clear the synthetic changes: both directions must be safe no-ops.
	h.app.EditorGroup.SetLineChanges(filepath.Join(h.dir, "hunks.txt"), nil)
	h.app.EditorGroup.GoToLine(10)
	h.redraw()

	h.exec("diff.nextHunk")
	if got := cursorLine1(h); got != 10 {
		t.Fatalf("nextHunk with no changes: expected line 10 (no-op), got %d", got)
	}
	h.exec("diff.prevHunk")
	if got := cursorLine1(h); got != 10 {
		t.Fatalf("prevHunk with no changes: expected line 10 (no-op), got %d", got)
	}
}
