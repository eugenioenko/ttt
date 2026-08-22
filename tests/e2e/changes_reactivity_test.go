package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/gdamore/tcell/v3"
)

func initializeHarnessRepository(t *testing.T, dir string) {
	t.Helper()
	commands := [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test User"},
		{"config", "commit.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-qm", "initial commit"},
	}
	for _, args := range commands {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
}

func showCleanChanges(t *testing.T, h *testHarness) {
	t.Helper()
	h.app.Repository.RefreshNow(app.RepositoryWorktree | app.RepositoryHistory)
	h.exec("sidebar.changes")
	h.assertContains("No changes")
}

func TestSavedFileAppearsInVisibleChangesWithoutManualRefresh(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	initializeHarnessRepository(t, h.dir)
	showCleanChanges(t, h)

	path := filepath.Join(h.dir, "alpha.txt")
	h.app.EditorGroup.OpenFile(path)
	h.exec("editor.focus")
	h.pressKey(tcell.KeyEnd, tcell.ModNone)
	h.pressRune('X')
	h.exec("file.save")

	h.assertContains("Changes (1)")
	h.assertContains("alpha.txt")
}

func TestSaveAsAppearsInVisibleChangesWithoutManualRefresh(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	initializeHarnessRepository(t, h.dir)
	showCleanChanges(t, h)

	h.exec("file.new")
	h.pressRune('n')
	h.exec("file.save")
	path := filepath.Join(h.dir, "saved-as.txt")
	h.app.PasteText(path)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Save As did not write %s: %v", path, err)
	}
	h.assertContains("Changes (1)")
	h.assertContains("saved-as.txt")
}

func TestWatcherReconciliationInvalidatesVisibleChanges(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	initializeHarnessRepository(t, h.dir)
	showCleanChanges(t, h)

	path := filepath.Join(h.dir, "beta.txt")
	h.app.EditorGroup.OpenFile(path)
	if err := os.WriteFile(path, []byte("external\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.app.HandleFileChanged(path)
	h.redraw()

	h.assertContains("Changes (1)")
	h.assertContains("beta.txt")
}
