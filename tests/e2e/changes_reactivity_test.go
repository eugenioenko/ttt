package e2e

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/gdamore/tcell/v3"
)

func initializeHarnessRepo(t *testing.T, dir string) {
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

func TestSavedFileAppearsInVisibleChangesWithoutManualRefresh(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	initializeHarnessRepo(t, h.dir)
	h.app.Repository.RefreshNow(app.RepositoryStatus | app.RepositoryHistory)

	path := filepath.Join(h.dir, "alpha.txt")
	h.app.EditorGroup.OpenFile(path)
	h.exec("sidebar.changes")
	h.assertContains("No changes")

	h.exec("editor.focus")
	h.pressKey(tcell.KeyEnd, tcell.ModNone)
	h.pressRune('X')
	h.exec("file.save")

	h.assertContains("Changes (1)")
	h.assertContains("alpha.txt")
}
