package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLinkedWorktreeBranchFollowsActiveFileAndClearsOutsideRepository(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	container := t.TempDir()
	repo := filepath.Join(container, "primary")
	linked := filepath.Join(container, "linked")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "initial.txt"), []byte("initial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	initializeHarnessRepository(t, repo)
	cmd := exec.Command("git", "-C", repo, "worktree", "add", "-q", "-b", "linkedbranch", linked)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v\n%s", err, output)
	}
	info, err := os.Stat(filepath.Join(linked, ".git"))
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("linked worktree .git = (%v, %v), want regular file", info, err)
	}

	nested := filepath.Join(linked, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	linkedFile := filepath.Join(nested, "linked.txt")
	if err := os.WriteFile(linkedFile, []byte("linked\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h.app.EditorGroup.OpenFile(linkedFile)
	h.app.SyncRepositoryBranch()
	h.redraw()
	h.assertContains("linkedbranch")

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.app.SyncRepositoryBranch()
	h.redraw()
	h.assertNotContains("linkedbranch")
}
