package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
)

func TestOpenCurrentChangesReusesOneLiveSharedDiffDocument(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()
	initializeHarnessRepository(t, h.dir)
	path := filepath.Join(h.dir, "alpha.txt")
	if err := os.WriteFile(path, []byte("first working version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.app.Repository.RefreshNow(app.RepositoryWorktree)
	h.exec("changes.viewAll")
	first := h.app.EditorGroup.ActiveCurrentChangesWidget()
	if first == nil {
		t.Fatal("Open Current Changes did not create the shared document")
	}
	for _, want := range []string{"Current changes", "M  alpha.txt · unstaged", "first working version"} {
		h.assertContains(want)
	}

	h.exec("changes.viewAll")
	if h.app.EditorGroup.ActiveCurrentChangesWidget() != first {
		t.Fatal("reopening Current Changes replaced its stable tab incarnation")
	}
	lines := strings.Split(h.screenText(), "\n")
	headingY := -1
	for y, line := range lines {
		if strings.Contains(line, "M  alpha.txt · unstaged") {
			headingY = y
			break
		}
	}
	if headingY < 0 {
		t.Fatal("current changes file heading was not rendered")
	}
	headingX := first.GetRect().X + 8
	h.click(headingX, headingY)
	if strings.Contains(h.screenText(), "first working version") {
		t.Fatal("clicking the current-change title row did not collapse its diff")
	}

	if err := os.WriteFile(path, []byte("second external version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.app.Repository.RefreshNow(app.RepositoryWorktree)
	h.redraw()
	if strings.Contains(h.screenText(), "second external version") {
		t.Fatal("live refresh lost the collapsed file state")
	}
	h.click(headingX, headingY)
	h.assertContains("second external version")
}

func TestOpenCurrentChangesShowsOneExplicitUDConflictSection(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()
	initializeHarnessRepository(t, h.dir)
	path := filepath.Join(h.dir, "alpha.txt")
	runHistoryGit(t, h.dir, "checkout", "-qb", "other")
	runHistoryGit(t, h.dir, "rm", "--", "alpha.txt")
	runHistoryGit(t, h.dir, "commit", "-m", "other deletes")
	runHistoryGit(t, h.dir, "checkout", "-q", "main")
	if err := os.WriteFile(path, []byte("main content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runHistoryGit(t, h.dir, "commit", "-am", "main modifies")
	if err := exec.Command("git", "-C", h.dir, "merge", "other").Run(); err == nil {
		t.Fatal("merge unexpectedly succeeded")
	}

	h.app.Repository.RefreshNow(app.RepositoryWorktree)
	h.exec("changes.viewAll")
	screen := h.screenText()
	if strings.Count(screen, "U  alpha.txt - conflict (UD)") != 1 {
		t.Fatalf("UD conflict did not render as one section:\n%s", screen)
	}
	for _, want := range []string{"Current changes", "No line changes"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("UD conflict omitted %q:\n%s", want, screen)
		}
	}
	if strings.Contains(screen, "alpha.txt · staged") || strings.Contains(screen, "alpha.txt · unstaged") {
		t.Fatalf("UD conflict was coerced into staged or unstaged claims:\n%s", screen)
	}
}
