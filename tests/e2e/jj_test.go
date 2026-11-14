package e2e

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/jjtest"
	"github.com/eugenioenko/ttt/internal/scm"
)

func initializeHarnessJJRepository(t *testing.T, dir string) {
	t.Helper()
	jjtest.Require(t)
	jjtest.Isolate(t)
	jjtest.Write(t, dir, ".gitignore", "config/\n*.log\n")
	jjtest.Run(t, dir, "git", "init")
	jjtest.Write(t, dir, "file.txt", "one\ntwo\nthree\n")
	jjtest.Run(t, dir, "commit", "-m", "seed")
	jjtest.Run(t, dir, "bookmark", "create", "-r", "@", "feature")
}

func (h *testHarness) gutterFor(path string) []diff.LineChangeKind {
	h.t.Helper()
	h.app.EditorGroup.OpenFile(path)
	h.app.SyncRepositoryBranch()
	h.app.RequestGitGutterForActiveFile()
	h.redraw()
	return h.app.EditorGroup.Editor.LineChanges
}

func TestJJGutterComparesAgainstWorkingCopyParent(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	initializeHarnessJJRepository(t, h.dir)
	jjtest.Write(t, h.dir, "file.txt", "one\nTWO\nthree\n")
	jjtest.Write(t, h.dir, "new.txt", "a\nb\n")
	jjtest.Write(t, h.dir, "app.log", "ignored\n")

	changes := h.gutterFor(filepath.Join(h.dir, "file.txt"))
	if len(changes) < 3 || changes[0] != diff.LineUnchanged || changes[1] != diff.LineModified || changes[2] != diff.LineUnchanged {
		t.Fatalf("modified file gutter = %v", changes)
	}
	h.assertContains("feature")

	changes = h.gutterFor(filepath.Join(h.dir, "new.txt"))
	if len(changes) == 0 || slices.ContainsFunc(changes[:2], func(c diff.LineChangeKind) bool { return c != diff.LineAdded }) {
		t.Fatalf("new file gutter = %v, want added lines", changes)
	}

	if changes := h.gutterFor(filepath.Join(h.dir, "app.log")); len(changes) != 0 {
		t.Fatalf("ignored file gutter = %v, want no markers", changes)
	}
}

func TestJJChangesPanelRendersWithoutStaging(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	initializeHarnessJJRepository(t, h.dir)
	jjtest.Write(t, h.dir, "file.txt", "changed\n")

	h.app.Changes.SetDirs([]string{h.dir})
	h.exec("sidebar.changes")
	h.redraw()
	h.assertContains("Changes (1)")
	h.assertContains("file.txt")
	h.assertContains("seed")
	h.assertNotContains("Staged")

	h.app.Changes.Tree.SetSelectedIndex(1)
	h.pressRune('s')
	h.redraw()
	h.assertNotContains("Staged")
	h.assertContains("Changes (1)")
	h.assertContains("Not available in jj repositories")
}

func TestJJRenamedFileDiffsAgainstItsSource(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	jjtest.Write(t, h.dir, "old.txt", "one\ntwo\nthree\nfour\nfive\n")
	initializeHarnessJJRepository(t, h.dir)
	if err := os.Remove(filepath.Join(h.dir, "old.txt")); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(h.dir, "new.txt")
	jjtest.Write(t, h.dir, "new.txt", "ONE\ntwo\nthree\nfour\nfive\n")

	changes := h.gutterFor(newPath)
	want := []diff.LineChangeKind{diff.LineModified, diff.LineUnchanged, diff.LineUnchanged, diff.LineUnchanged, diff.LineUnchanged}
	if len(changes) < len(want) || !slices.Equal(changes[:len(want)], want) {
		t.Fatalf("renamed file gutter = %v, want %v", changes, want)
	}

	h.app.OpenChangeDiff(h.dir, scm.Change{Path: "new.txt", OldPath: "old.txt", Kind: scm.Renamed, Group: scm.WorkingTree}, false)
	h.redraw()
	if !h.app.EditorGroup.IsInlineDiffActive() {
		t.Fatal("inline diff not active")
	}
	h.assertContains("ONE")
	if screen := h.screenText(); !strings.Contains(screen, "one") || strings.Count(screen, "two") != 2 {
		t.Fatalf("renamed file diff should show only the first line as changed:\n%s", screen)
	}
}
