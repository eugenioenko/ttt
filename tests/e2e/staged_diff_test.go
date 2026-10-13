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
func setupPartiallyStagedFile(t *testing.T, h *testHarness) string {
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
	h.app.RefreshChanges()
	return path
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
	path := setupPartiallyStagedFile(t, h)

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "staged.txt", Status: "M"}, false, false)
	d := h.app.EditorGroup.ActiveInlineDiff()
	if d == nil || !d.Editable() {
		t.Fatal("unstaged entry did not open an editable diff")
	}
	d.SetContextMode(ui.DiffContextFullFile)
	h.redraw()

	h.assertContains("staged.txt (diff)")
	h.assertContains("STAGED 03")
	h.assertContains("UNSTAGED 25")
	h.assertContains("row 25")
	if strings.Contains(h.screenText(), "row 03") {
		t.Fatalf("staged change shown in the unstaged diff:\n%s", h.screenText())
	}

	h.app.EditorGroup.GoToLine(1)
	h.exec("editor.focus")
	h.pressRune('X')
	if got := h.app.EditorGroup.ActiveBuffer().Lines[0]; got != "Xrow 01" {
		t.Fatalf("edit did not reach the buffer: %q", got)
	}
	data, _ := os.ReadFile(path)
	if strings.HasPrefix(string(data), "X") {
		t.Fatal("edit saved without a save command")
	}
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

func TestStagedChangesWithoutLineEditsOpenADiff(t *testing.T) {
	h := newTestHarness(t, 100, 45)
	defer h.stop()
	if err := os.WriteFile(filepath.Join(h.dir, "old.txt"), []byte("kept line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	initializeHarnessRepository(t, h.dir)
	harnessGit(t, h.dir, "mv", "old.txt", "new.txt")
	if err := os.WriteFile(filepath.Join(h.dir, "empty.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	harnessGit(t, h.dir, "add", "empty.txt")

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "new.txt", OldPath: "old.txt", Status: "R", Staged: true}, true, false)
	if h.app.EditorGroup.ActiveDiffWidget() == nil {
		t.Fatal("staged rename without line edits did not open a diff")
	}
	h.redraw()
	h.assertContains("new.txt (staged)")
	h.assertContains("kept line")

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "empty.txt", Status: "A", Staged: true}, true, false)
	if h.app.EditorGroup.ActiveDiffWidget() == nil {
		t.Fatal("staged empty file did not open a diff")
	}
	h.redraw()
	h.assertContains("empty.txt (staged)")
}

func TestStagedEntryDiffsHeadAgainstIndexReadOnly(t *testing.T) {
	h := newTestHarness(t, 100, 45)
	defer h.stop()
	setupPartiallyStagedFile(t, h)

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "staged.txt", Status: "M"}, false, false)
	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "staged.txt", Status: "M", Staged: true}, true, false)
	d := h.app.EditorGroup.ActiveDiffWidget()
	if d == nil || d.Editable() {
		t.Fatal("staged entry did not open a read-only diff")
	}
	d.SetContextMode(ui.DiffContextFullFile)
	h.redraw()

	h.assertContains("staged.txt (staged)")
	h.assertContains("staged.txt (diff)")
	h.assertContains("row 03")
	h.assertContains("STAGED 03")
	h.assertContains("row 25")
	if strings.Contains(h.screenText(), "UNSTAGED") {
		t.Fatalf("unstaged change shown in the staged diff:\n%s", h.screenText())
	}
}

func TestStagingUpdatesOpenUnstagedDiff(t *testing.T) {
	h := newTestHarness(t, 100, 45)
	defer h.stop()
	setupPartiallyStagedFile(t, h)

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "staged.txt", Status: "M"}, false, false)
	h.app.EditorGroup.ActiveInlineDiff().SetContextMode(ui.DiffContextFullFile)
	h.redraw()
	h.assertContains("row 25")

	harnessGit(t, h.dir, "add", "staged.txt")
	h.app.RefreshChanges()
	h.redraw()

	h.assertContains("UNSTAGED 25")
	if strings.Contains(h.screenText(), "row 25") {
		t.Fatalf("staged line still diffed against the old index:\n%s", h.screenText())
	}
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

func TestConflictDuringAnOpenDiffClosesIt(t *testing.T) {
	h := newTestHarness(t, 100, 45)
	defer h.stop()
	path := filepath.Join(h.dir, "conflict.txt")
	os.WriteFile(path, []byte("base\n"), 0644)
	initializeHarnessRepository(t, h.dir)
	harnessGit(t, h.dir, "checkout", "-q", "-b", "other")
	os.WriteFile(path, []byte("other side\n"), 0644)
	harnessGit(t, h.dir, "commit", "-qam", "other")
	harnessGit(t, h.dir, "checkout", "-q", "main")
	os.WriteFile(path, []byte("main side\n"), 0644)
	harnessGit(t, h.dir, "commit", "-qam", "main")
	os.WriteFile(path, []byte("main side\nedited\n"), 0644)
	h.app.RefreshChanges()

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "conflict.txt", Status: "M"}, false, false)
	h.redraw()
	if h.app.EditorGroup.ActiveInlineDiff() == nil {
		t.Fatal("modified file did not open as a diff")
	}

	harnessGit(t, h.dir, "checkout", "-q", "--", "conflict.txt")
	cmd := exec.Command("git", "merge", "-q", "other")
	cmd.Dir = h.dir
	if err := cmd.Run(); err == nil {
		t.Fatal("merge did not conflict")
	}
	h.app.RequestGitGutterForActiveFile()
	h.redraw()

	if h.app.EditorGroup.ActiveInlineDiff() != nil {
		t.Fatal("diff against the index stayed open after the file got merge conflicts")
	}
	if h.app.EditorGroup.ActiveFilePath() != path {
		t.Fatalf("active file %q, want the conflicted file", h.app.EditorGroup.ActiveFilePath())
	}
	h.assertContains("merge conflicts")
}

func TestOpenChangesExtendedOpensTheFullFile(t *testing.T) {
	h := newTestHarness(t, 100, 45)
	defer h.stop()
	setupPartiallyStagedFile(t, h)

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "staged.txt", Status: "M"}, false, true)
	d := h.app.EditorGroup.ActiveInlineDiff()
	if d == nil {
		t.Fatal("open changes did not open a diff")
	}
	if d.ContextMode() != ui.DiffContextFullFile {
		t.Fatalf("extended open changes context = %v, want the full file", d.ContextMode())
	}
}
