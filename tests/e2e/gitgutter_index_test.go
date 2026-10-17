package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/gdamore/tcell/v3"
)

func writeGutterFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gutterFor opens path, computes its git gutter and returns the markers.
func gutterFor(t *testing.T, h *testHarness, path string) []diff.LineChangeKind {
	t.Helper()
	h.app.EditorGroup.OpenFile(path)
	h.app.SyncRepositoryBranch()
	h.app.RequestGitGutterForActiveFile()
	h.redraw()
	return activeGutter(t, h)
}

func activeGutter(t *testing.T, h *testHarness) []diff.LineChangeKind {
	t.Helper()
	changes := h.app.EditorGroup.Editor.LineChanges
	if want := len(h.app.EditorGroup.ActiveBuffer().Lines); len(changes) != want {
		t.Fatalf("gutter has %d entries, want one per buffer line (%d): %v", len(changes), want, changes)
	}
	return changes
}

func hasMarkers(changes []diff.LineChangeKind) bool {
	return slices.ContainsFunc(changes, func(c diff.LineChangeKind) bool { return c != diff.LineUnchanged })
}

func TestGitGutterHidesStagedEdits(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	path := filepath.Join(h.dir, "file.txt")
	writeGutterFile(t, path, "one\ntwo\nthree\n")
	initializeHarnessRepository(t, h.dir)
	writeGutterFile(t, path, "one\nTWO\nthree\n")
	harnessGit(t, h.dir, "add", "file.txt")

	if changes := gutterFor(t, h, path); hasMarkers(changes) {
		t.Fatalf("staged edit marked in the gutter: %v, want no markers", changes)
	}

	h.app.EditorGroup.GoToLine(3)
	h.exec("editor.focus")
	h.pressKey(tcell.KeyEnd, tcell.ModNone)
	h.pressRune('X')
	h.app.RequestGitGutterForActiveFile()
	h.redraw()
	changes := activeGutter(t, h)
	want := []diff.LineChangeKind{diff.LineUnchanged, diff.LineUnchanged, diff.LineModified}
	if !slices.Equal(changes[:3], want) || hasMarkers(changes[3:]) {
		t.Fatalf("gutter after an unstaged edit = %v, want only line 3 modified", changes)
	}
}

func TestGitGutterClearsWhenTheOpenFileIsStaged(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	path := filepath.Join(h.dir, "file.txt")
	writeGutterFile(t, path, "one\ntwo\nthree\n")
	initializeHarnessRepository(t, h.dir)
	writeGutterFile(t, path, "one\nTWO\nthree\n")
	h.app.RefreshChanges()

	if changes := gutterFor(t, h, path); changes[1] != diff.LineModified {
		t.Fatalf("unstaged edit gutter = %v, want line 2 modified", changes)
	}

	if err := git.Stage(h.dir, "file.txt"); err != nil {
		t.Fatal(err)
	}
	h.app.RefreshChanges()
	h.redraw()
	if changes := activeGutter(t, h); hasMarkers(changes) {
		t.Fatalf("gutter after staging = %v, want the markers cleared without an edit", changes)
	}
}

func TestGitGutterStagedNewFileHasNoMarkers(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	initializeHarnessRepository(t, h.dir)
	path := filepath.Join(h.dir, "new.txt")
	writeGutterFile(t, path, "one\ntwo\n")
	harnessGit(t, h.dir, "add", "new.txt")

	if changes := gutterFor(t, h, path); hasMarkers(changes) {
		t.Fatalf("staged new file gutter = %v, want no markers", changes)
	}
}

func TestGitGutterConflictedFileComparesAgainstHead(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	path := filepath.Join(h.dir, "conflict.txt")
	writeGutterFile(t, path, "base\n")
	initializeHarnessRepository(t, h.dir)
	harnessGit(t, h.dir, "checkout", "-q", "-b", "other")
	writeGutterFile(t, path, "other\n")
	harnessGit(t, h.dir, "commit", "-qam", "other")
	harnessGit(t, h.dir, "checkout", "-q", "main")
	writeGutterFile(t, path, "mine\n")
	harnessGit(t, h.dir, "commit", "-qam", "mine")
	if exec.Command("git", "-C", h.dir, "merge", "-q", "other").Run() == nil {
		t.Fatal("merge did not conflict")
	}

	changes := gutterFor(t, h, path)
	lines := h.app.EditorGroup.ActiveBuffer().Lines
	mine := slices.Index(lines, "mine")
	if mine < 0 {
		t.Fatalf("conflicted file has no %q line: %q", "mine", lines)
	}
	if changes[mine] != diff.LineUnchanged {
		t.Fatalf("gutter = %v for %q, want HEAD's line %q unchanged", changes, lines, "mine")
	}
	if changes[0] != diff.LineAdded {
		t.Fatalf("gutter = %v for %q, want the conflict marker line added", changes, lines)
	}
}
