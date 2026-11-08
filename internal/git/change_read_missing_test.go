package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/scm"
)

func TestReadChangeMissingParentIsMissingEndpoint(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		name := "deleted"
		if conflict {
			name = "conflicted"
		}
		t.Run(name, func(t *testing.T) {
			dir := setupTestRepo(t)
			path := "parent/nested/file.txt"
			if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, path), []byte("before\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitRun(t, dir, "add", ".")
			gitRun(t, dir, "commit", "-qm", "nested file")
			change := scm.Change{Path: path, Kind: scm.Deleted, Group: scm.WorkingTree}
			if conflict {
				entries, err := IndexEntriesContext(context.Background(), dir, path)
				if err != nil || len(entries) != 1 {
					t.Fatalf("entries=%+v err=%v", entries, err)
				}
				gitRun(t, dir, "update-index", "--force-remove", path)
				cmd := gitCommandContext(context.Background(), "-C", dir, "update-index", "--index-info")
				cmd.Stdin = strings.NewReader("100644 " + entries[0].Object + " 2\t" + path + "\n")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("conflict: %s %v", out, err)
				}
				change.Kind, change.Detail = scm.Conflicted, "AU"
			}
			if err := os.RemoveAll(filepath.Join(dir, "parent")); err != nil {
				t.Fatal(err)
			}
			got, err := (repository{root: dir}).ReadChange(context.Background(), change)
			if err != nil || got.Before.Content != "before\n" || !got.Before.Exists || got.After.Exists || got.Conflicted != conflict {
				t.Fatalf("missing parent: %+v err=%v", got, err)
			}
			if !conflict && !strings.Contains(got.Patch, "deleted file mode") {
				t.Fatalf("deleted patch lost: %q", got.Patch)
			}
			if conflict && (got.ConflictDetail != "AU" || len(got.ConflictSources) != 1 || got.ConflictSources[0] != "2") {
				t.Fatalf("conflict metadata lost: %+v", got)
			}
		})
	}
}

func TestReadChangeDeletedChildOfReplacedDirectory(t *testing.T) {
	dir := setupTestRepo(t)
	path := "parent/file.txt"
	if err := os.Mkdir(filepath.Join(dir, "parent"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, path), []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-qm", "nested file")
	if err := os.RemoveAll(filepath.Join(dir, "parent")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "parent"), []byte("replacement\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := (repository{root: dir}).ReadChange(context.Background(), scm.Change{Path: path, Kind: scm.Deleted, Group: scm.WorkingTree})
	if err != nil || !got.Before.Exists || got.Before.Content != "before\n" || got.After.Exists || !strings.Contains(got.Patch, "deleted file mode") {
		t.Fatalf("deleted child: %+v err=%v", got, err)
	}
}

func TestReadChangeStaleDeletionReadsRestoredContent(t *testing.T) {
	dir := setupTestRepo(t)
	path := "file.txt"
	if err := os.WriteFile(filepath.Join(dir, path), []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-qm", "initial")
	if err := os.WriteFile(filepath.Join(dir, path), []byte("restored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := (repository{root: dir}).ReadChange(context.Background(), scm.Change{Path: path, Kind: scm.Deleted, Group: scm.WorkingTree})
	if err != nil || got.After.Content != "restored\n" || !got.After.Exists {
		t.Fatalf("restored endpoint omitted: %+v err=%v", got, err)
	}
}
