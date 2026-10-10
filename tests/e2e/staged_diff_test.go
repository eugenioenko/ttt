package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func awaitCurrentGitGutter(t *testing.T, h *testHarness) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev := <-h.screen.EventQ():
			if intr, ok := ev.(*tcell.EventInterrupt); ok {
				if res, ok := intr.Data().(*app.GitGutterResult); ok {
					h.app.ApplyGitGutterResult(res)
					if res.Gen == h.app.GitGutterGen {
						h.redraw()
						return
					}
				}
			}
		case <-timeout:
			t.Fatal("current git gutter result never arrived")
		}
	}
}

func harnessGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func setupPartiallyStagedFile(t *testing.T, h *testHarness) string {
	t.Helper()
	path := filepath.Join(h.dir, "staged.txt")
	var lines []string
	for i := 1; i <= 30; i++ {
		lines = append(lines, fmt.Sprintf("row %02d", i))
	}
	write := func() {
		os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
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
	awaitCurrentGitGutter(t, h)

	h.assertContains("UNSTAGED 25")
	if strings.Contains(h.screenText(), "row 25") {
		t.Fatalf("staged line still diffed against the old index:\n%s", h.screenText())
	}
}

func TestConflictedFileOpensWithoutADiff(t *testing.T) {
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
	cmd := exec.Command("git", "merge", "-q", "other")
	cmd.Dir = h.dir
	if err := cmd.Run(); err == nil {
		t.Fatal("merge did not conflict")
	}
	h.app.RefreshChanges()

	h.app.OpenChangeDiff(h.dir, git.FileStatus{Path: "conflict.txt", Status: "U"}, false, false)
	h.redraw()
	if h.app.EditorGroup.ActiveFilePath() != path {
		t.Fatalf("active file %q, want the conflicted file", h.app.EditorGroup.ActiveFilePath())
	}
	if h.app.EditorGroup.ActiveInlineDiff() != nil {
		t.Fatal("a conflicted file opened as a diff against the index")
	}
	h.assertContains("merge conflicts")
}
