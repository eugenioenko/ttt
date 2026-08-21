package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/gdamore/tcell/v3"
)

func (h *testHarness) awaitCurrentChanges() *app.CurrentChangesResult {
	h.t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case event := <-h.screen.EventQ():
			interrupt, ok := event.(*tcell.EventInterrupt)
			if !ok {
				continue
			}
			result, ok := interrupt.Data().(*app.CurrentChangesResult)
			if !ok {
				continue
			}
			h.app.ApplyCurrentChanges(result)
			h.redraw()
			return result
		case <-deadline:
			h.t.Fatal("timed out waiting for current changes")
			return nil
		}
	}
}

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

func TestViewAllCurrentChangesOpensCombinedScrollableDocument(t *testing.T) {
	h := newTestHarness(t, 110, 32)
	initializeHarnessRepo(t, h.dir)
	path := filepath.Join(h.dir, "alpha.txt")
	if err := os.WriteFile(path, []byte("staged version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "alpha.txt")
	cmd.Dir = h.dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("stage alpha: %v\n%s", err, output)
	}
	if err := os.WriteFile(path, []byte("final working version\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h.app.Repository.RefreshNow(app.RepositoryStatus)
	h.exec("changes.viewAll")
	h.awaitCurrentChanges()
	h.assertContains("Current changes")
	h.assertContains("M  alpha.txt · mixed")
	h.assertContains("final working version")
}
