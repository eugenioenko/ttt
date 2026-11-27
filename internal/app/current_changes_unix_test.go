//go:build linux || darwin || freebsd

package app

import (
	"context"
	"github.com/eugenioenko/ttt/internal/git"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadCurrentChangesStableTrackedAndUntrackedSymlinksRenderMetadata(t *testing.T) {
	dir := testAppRepository(t)
	out := t.TempDir()
	first := filepath.Join(out, "first-target")
	second := filepath.Join(out, "second-target")
	if err := os.WriteFile(first, []byte("FIRST SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("SECOND SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	tracked := filepath.Join(dir, "tracked-link")
	if err := os.Symlink(first, tracked); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	testAppGit(t, dir, "add", "tracked-link")
	testAppGit(t, dir, "commit", "-m", "tracked symlink")
	if err := os.Remove(tracked); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(second, tracked); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(first, filepath.Join(dir, "untracked-link")); err != nil {
		t.Fatal(err)
	}

	result := readGitCurrentChanges(context.Background(), dir, git.RevisionIdentity(dir), currentChangesTabID(dir), 1, 1, currentChangesStatuses(t, dir))
	if result.Err != nil || len(result.Files) != 2 {
		t.Fatalf("symlink result=%+v", result)
	}
	for _, file := range result.Files {
		text := ""
		for _, line := range file.Diff.AllLines() {
			text += line.Left.Text + line.Right.Text
		}
		if strings.Contains(text, "FIRST SECRET") || strings.Contains(text, "SECOND SECRET") {
			t.Fatalf("target bytes escaped through %q: %q", file.Path, text)
		}
		if !strings.Contains(text, "target") {
			t.Fatalf("link metadata missing for %q: %q", file.Path, text)
		}
	}
}

func TestReadCurrentChangesSubmoduleRendersObjectMetadata(t *testing.T) {
	child := testAppRepository(t)
	parent := testAppRepository(t)
	testAppGit(t, parent, "-c", "protocol.file.allow=always", "submodule", "add", child, "sub")
	testAppGit(t, parent, "commit", "-m", "add submodule")
	if err := os.WriteFile(filepath.Join(child, "initial.txt"), []byte("child second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testAppGit(t, child, "commit", "-am", "child second")
	childHead, err := git.RevisionIdentityContext(context.Background(), child)
	if err != nil {
		t.Fatal(err)
	}
	testAppGit(t, parent, "-C", "sub", "fetch", "origin")
	testAppGit(t, parent, "-C", "sub", "checkout", "--detach", childHead)
	result := readGitCurrentChanges(context.Background(), parent, git.RevisionIdentity(parent), currentChangesTabID(parent), 1, 1, currentChangesStatuses(t, parent))
	if result.Err != nil || len(result.Files) != 1 {
		t.Fatalf("submodule result=%+v", result)
	}
	found := false
	for _, line := range result.Files[0].Diff.AllLines() {
		found = found || strings.Contains(line.Left.Text, "Subproject commit") || strings.Contains(line.Right.Text, "Subproject commit")
	}
	if !found {
		t.Fatalf("submodule object metadata missing: %+v", result.Files[0].Diff.AllLines())
	}
}
