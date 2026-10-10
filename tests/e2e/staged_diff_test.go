package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"
)

func harnessGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// setupPartiallyStagedFile commits a 30-line file, stages a change to line 3
// and leaves an unstaged change to line 25.
func setupPartiallyStagedFile(t *testing.T, h *testHarness) {
	t.Helper()
	path := filepath.Join(h.dir, "staged.txt")
	var lines []string
	for i := 1; i <= 30; i++ {
		lines = append(lines, fmt.Sprintf("row %02d", i))
	}
	write := func() {
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write()
	initializeHarnessRepository(t, h.dir)
	lines[2] = "STAGED 03"
	write()
	harnessGit(t, h.dir, "add", "staged.txt")
	lines[24] = "UNSTAGED 25"
	write()
}

func openFullFileDiff(h *testHarness) {
	if d := h.app.EditorGroup.ActiveDiffWidget(); d != nil {
		d.SetContextMode(ui.DiffContextFullFile)
	}
	h.redraw()
}

func TestUnstagedEntryDiffsIndexAgainstWorkingTree(t *testing.T) {
	h := newTestHarness(t, 100, 45)
	defer h.stop()
	setupPartiallyStagedFile(t, h)

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "staged.txt", Status: "M"}, false, false)
	if h.app.EditorGroup.ActiveDiffWidget() == nil {
		t.Fatal("unstaged entry did not open a diff")
	}
	h.redraw()
	h.assertContains("staged.txt (diff)")
	h.assertContains("UNSTAGED 25")
	h.assertNotContains("row 03")

	openFullFileDiff(h)
	h.assertContains("STAGED 03")
	h.assertContains("row 25")
	h.assertNotContains("row 03")
}

func TestStagedEntryDiffsHeadAgainstIndex(t *testing.T) {
	h := newTestHarness(t, 100, 45)
	defer h.stop()
	setupPartiallyStagedFile(t, h)

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "staged.txt", Status: "M", Staged: true}, true, false)
	if h.app.EditorGroup.ActiveDiffWidget() == nil {
		t.Fatal("staged entry did not open a diff")
	}
	h.redraw()
	h.assertContains("staged.txt (staged)")
	h.assertContains("row 03")
	h.assertContains("STAGED 03")
	h.assertNotContains("UNSTAGED")

	openFullFileDiff(h)
	h.assertContains("row 25")
	h.assertNotContains("UNSTAGED")
}

func TestStagedAndUnstagedDiffsOpenSideBySide(t *testing.T) {
	h := newTestHarness(t, 140, 45)
	defer h.stop()
	setupPartiallyStagedFile(t, h)

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "staged.txt", Status: "M"}, false, false)
	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "staged.txt", Status: "M", Staged: true}, true, false)
	h.redraw()
	h.assertContains("staged.txt (diff)")
	h.assertContains("staged.txt (staged)")

	if !h.app.EditorGroup.SwitchToTabByPath("staged.txt (diff)") {
		t.Fatal("unstaged diff tab was replaced by the staged one")
	}
	h.redraw()
	h.assertContains("UNSTAGED 25")
}

func TestConflictedFileOpensWithoutADiff(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	path := filepath.Join(h.dir, "conflict.txt")
	if err := os.WriteFile(path, []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	initializeHarnessRepository(t, h.dir)
	harnessGit(t, h.dir, "checkout", "-q", "-b", "other")
	os.WriteFile(path, []byte("other\n"), 0o644)
	harnessGit(t, h.dir, "commit", "-qam", "other")
	harnessGit(t, h.dir, "checkout", "-q", "main")
	os.WriteFile(path, []byte("mine\n"), 0o644)
	harnessGit(t, h.dir, "commit", "-qam", "mine")
	if exec.Command("git", "-C", h.dir, "merge", "-q", "other").Run() == nil {
		t.Fatal("merge did not conflict")
	}

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "conflict.txt", Status: "U"}, false, false)
	h.redraw()
	if h.app.EditorGroup.ActiveDiffWidget() != nil {
		t.Fatal("conflicted file opened as a diff")
	}
	if got := h.app.EditorGroup.ActiveFilePath(); got != path {
		t.Fatalf("active file = %q, want %q", got, path)
	}
	h.assertContains("has merge conflicts")
}
