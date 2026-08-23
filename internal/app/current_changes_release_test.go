package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestReleaseContractMixedChangeKeepsIndexBoundary(t *testing.T) {
	dir := testAppRepository(t)
	path := filepath.Join(dir, "initial.txt")
	if err := os.WriteFile(path, []byte("staged snapshot\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testAppGit(t, dir, "add", "--", "initial.txt")
	if err := os.WriteFile(path, []byte("unstaged snapshot\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := readCurrentChanges(context.Background(), dir, git.RevisionIdentity(dir), currentChangesTabID(dir), 1, 1, currentChangesStatuses(t, dir))
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if len(result.Files) != 2 {
		t.Fatalf("files = %d, want staged and unstaged boundaries", len(result.Files))
	}
	staged, unstaged := result.Files[0], result.Files[1]
	if staged.Stage != ui.CommitDetailStageStaged || staged.Boundary != ui.CommitDetailBoundaryHeadToIndex ||
		unstaged.Stage != ui.CommitDetailStageUnstaged || unstaged.Boundary != ui.CommitDetailBoundaryIndexToWorktree {
		t.Fatalf("boundary identities = %+v", result.Files)
	}
	if ui.CommitDetailContextKey(staged) == ui.CommitDetailContextKey(unstaged) {
		t.Fatal("identical paths collapsed to the same typed key")
	}
	seenStaged := false
	for _, line := range staged.Diff.AllLines() {
		seenStaged = seenStaged || line.Right.Text == "staged snapshot"
	}
	seenIndex, seenFinal := false, false
	for _, line := range unstaged.Diff.AllLines() {
		seenIndex = seenIndex || line.Left.Text == "staged snapshot"
		seenFinal = seenFinal || line.Right.Text == "unstaged snapshot"
	}
	if !seenStaged || !seenIndex || !seenFinal {
		t.Fatalf("mixed boundaries lost an endpoint: staged=%+v unstaged=%+v", staged.Diff.AllLines(), unstaged.Diff.AllLines())
	}
	if !strings.Contains(result.Summary, "1 file") || !strings.Contains(result.Summary, "1 staged") || !strings.Contains(result.Summary, "1 unstaged") {
		t.Fatalf("mixed summary = %q", result.Summary)
	}
}

func TestReleaseContractIntentToAddRenders(t *testing.T) {
	dir := testAppRepository(t)
	path := filepath.Join(dir, "intent.txt")
	if err := os.WriteFile(path, []byte("intent content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testAppGit(t, dir, "add", "-N", "--", "intent.txt")
	result := readCurrentChanges(context.Background(), dir, git.RevisionIdentity(dir), currentChangesTabID(dir), 1, 1, currentChangesStatuses(t, dir))
	if result.Err != nil || len(result.Files) != 1 {
		t.Fatalf("intent-to-add result = %+v", result)
	}
	if file := result.Files[0]; file.Status != "A" || file.Stage != ui.CommitDetailStageUnstaged || file.Boundary != ui.CommitDetailBoundaryIndexToWorktree {
		t.Fatalf("intent-to-add identity = %+v", file)
	}
}

func TestReadCurrentChangesCoversUntrackedStagedDeletedRenameCopyAndConflictShapes(t *testing.T) {
	dir := testAppRepository(t)
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("delete.txt", "delete\n")
	write("rename.txt", "rename\n")
	testAppGit(t, dir, "add", "-A")
	testAppGit(t, dir, "commit", "-m", "shapes")
	write("untracked.txt", "untracked\n")
	write("staged.txt", "staged\n")
	testAppGit(t, dir, "add", "staged.txt")
	if err := os.Remove(filepath.Join(dir, "delete.txt")); err != nil {
		t.Fatal(err)
	}
	testAppGit(t, dir, "mv", "rename.txt", "renamed.txt")

	result := readCurrentChanges(context.Background(), dir, git.RevisionIdentity(dir), currentChangesTabID(dir), 1, 1, currentChangesStatuses(t, dir))
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	wants := map[string]string{"untracked.txt": "A", "staged.txt": "A", "delete.txt": "D", "renamed.txt": "R"}
	for path, status := range wants {
		found := false
		for _, file := range result.Files {
			found = found || file.Path == path && file.Status == status
		}
		if !found {
			t.Fatalf("missing %s %s in %+v", status, path, result.Files)
		}
	}

	copyPath := filepath.Join(dir, "copy.txt")
	write("copy.txt", "initial\n")
	testAppGit(t, dir, "add", "copy.txt")
	copyResult := readCurrentChanges(context.Background(), dir, git.RevisionIdentity(dir), currentChangesTabID(dir), 2, 2,
		[]git.FileStatus{{Status: "C", OldPath: "initial.txt", Path: "copy.txt", Staged: true}})
	if copyResult.Err != nil || len(copyResult.Files) != 1 || copyResult.Files[0].OldPath != "initial.txt" {
		t.Fatalf("copy boundary=%+v path=%s", copyResult, copyPath)
	}
}

func TestReadCurrentChangesPreservesUnmergedIndexStages(t *testing.T) {
	dir := testAppRepository(t)
	testAppGit(t, dir, "checkout", "-qb", "other")
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testAppGit(t, dir, "commit", "-am", "other")
	testAppGit(t, dir, "checkout", "-q", "main")
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testAppGit(t, dir, "commit", "-am", "main")
	if err := exec.Command("git", "-C", dir, "merge", "other").Run(); err == nil {
		t.Fatal("merge unexpectedly succeeded")
	}

	result := readCurrentChanges(context.Background(), dir, git.RevisionIdentity(dir), currentChangesTabID(dir), 1, 1, currentChangesStatuses(t, dir))
	if result.Err != nil || len(result.Files) != 1 {
		t.Fatalf("conflict result=%+v", result)
	}
	file := result.Files[0]
	if file.Status != "U" || file.Stage != ui.CommitDetailStageConflict || file.Boundary != ui.CommitDetailBoundaryConflictToWorktree || file.ConflictCode != "UU" {
		t.Fatalf("conflict identity=%+v", file)
	}
	if string(file.IndexStages) != string([]byte{1, 2, 3}) {
		t.Fatalf("conflict stages = %v", file.IndexStages)
	}
}
