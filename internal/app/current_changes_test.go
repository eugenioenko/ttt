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

func TestReadCurrentChangesCombinesMixedAndUntrackedFiles(t *testing.T) {
	dir := commitLogRepo(t)
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "a.txt")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("stage: %v\n%s", err, output)
	}
	if err := os.WriteFile(path, []byte("final working tree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := readCurrentChanges(context.Background(), dir, currentChangesTabID(dir), 4)
	if result.Err != "" {
		t.Fatal(result.Err)
	}
	if len(result.Files) != 2 {
		t.Fatalf("current change files = %d, want 2: %+v", len(result.Files), result.Files)
	}
	if !strings.Contains(result.Summary, "2 files") || !strings.Contains(result.Summary, "1 mixed") || !strings.Contains(result.Summary, "1 unstaged") {
		t.Fatalf("summary = %q", result.Summary)
	}
	if !strings.Contains(result.Files[0].Heading, "mixed") {
		t.Fatalf("mixed heading = %q", result.Files[0].Heading)
	}
	lines := result.Files[0].Diff.AllLines()
	foundFinal := false
	for _, line := range lines {
		if line.Right.Text == "final working tree" {
			foundFinal = true
		}
		if line.Right.Text == "staged" {
			t.Fatal("combined diff stopped at the index instead of the working tree")
		}
	}
	if !foundFinal {
		t.Fatalf("combined diff omitted final working tree: %+v", lines)
	}
	if !strings.HasPrefix(result.Files[1].Heading, "U  ") || len(result.Files[1].Diff.Hunks) == 0 {
		t.Fatalf("untracked file was not synthesized as an addition: %+v", result.Files[1])
	}
}

func TestApplyCurrentChangesDropsSupersededResult(t *testing.T) {
	dir := "/repo"
	tabID := currentChangesTabID(dir)
	detail := ui.NewCurrentChangesWidget(dir, false)
	detail.LoadGen = 2
	a := &App{EditorGroup: ui.NewEditorGroupWidget(nil, 4, true, "default")}
	a.EditorGroup.OpenPluginTab(tabID, "Current Changes", detail)

	a.ApplyCurrentChanges(&CurrentChangesResult{Dir: dir, TabID: tabID, Gen: 1, Summary: "stale"})
	if detail.Message == "stale" {
		t.Fatal("superseded current changes result was applied")
	}
	a.ApplyCurrentChanges(&CurrentChangesResult{Dir: dir, TabID: tabID, Gen: 2, Summary: "fresh"})
	if detail.Message != "fresh" {
		t.Fatalf("current result message = %q", detail.Message)
	}
}

func TestActiveCurrentChangesSkipsUnchangedPollAndReloadsSameStatusContent(t *testing.T) {
	dir := commitLogRepo(t)
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("first external edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statuses, err := git.StatusFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	group := changesGroup{Dir: dir}
	for _, status := range statuses {
		if status.Staged {
			group.Staged = append(group.Staged, status)
		} else {
			group.Unstaged = append(group.Unstaged, status)
		}
	}
	detail := ui.NewCurrentChangesWidget(dir, false)
	tabID := currentChangesTabID(dir)
	a := &App{
		Changes:     NewChangesPanel(dir),
		EditorGroup: ui.NewEditorGroupWidget(nil, 4, true, "default"),
	}
	a.Changes.groups = []changesGroup{group}
	a.EditorGroup.OpenPluginTab(tabID, "Current Changes", detail)
	fingerprint := a.currentChangesSourceFingerprint(dir)
	a.currentChangesLoadState(tabID).appliedFingerprint = fingerprint

	a.refreshActiveCurrentChanges()
	if detail.LoadGen != 0 {
		t.Fatalf("unchanged poll started diff load generation %d", detail.LoadGen)
	}

	if err := os.WriteFile(path, []byte("second external edit with a different size\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.refreshActiveCurrentChanges()
	if detail.LoadGen != 1 {
		t.Fatalf("same-status content edit did not start one diff load: generation %d", detail.LoadGen)
	}
	refreshed := false
	if len(detail.Files) > 0 {
		for _, line := range detail.Files[0].Diff.AllLines() {
			if strings.Contains(line.Right.Text, "second external edit") {
				refreshed = true
			}
		}
	}
	if !refreshed {
		t.Fatalf("refreshed diff omitted new content: %+v", detail.Files)
	}
}
