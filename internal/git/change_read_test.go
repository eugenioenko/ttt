package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/scm"
)

func TestReadChangeEndpointsAndMetadata(t *testing.T) {
	dir := setupTestRepo(t)
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("[日本語].txt", "HEAD\n")
	write("old.txt", "renamed\n")
	write("mode.txt", "same\n")
	write("binary.dat", "old\x00bytes")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-qm", "initial")
	write("[日本語].txt", "index\n")
	write("binary.dat", "new\x00bytes")
	write("empty.txt", "")
	gitRun(t, dir, "add", ".")
	write("[日本語].txt", "working\n")
	gitRun(t, dir, "mv", "old.txt", "new.txt")
	gitRun(t, dir, "config", "diff.renames", "false")
	gitRun(t, dir, "update-index", "--chmod=+x", "mode.txt")
	for _, tt := range []struct {
		change        scm.Change
		before, after scm.FileContent
		patch         string
	}{
		{scm.Change{Path: "[日本語].txt", Group: scm.Staged}, scm.FileContent{Content: "HEAD\n", Exists: true}, scm.FileContent{Content: "index\n", Exists: true}, "+index"},
		{scm.Change{Path: "[日本語].txt"}, scm.FileContent{Content: "index\n", Exists: true}, scm.FileContent{Content: "working\n", Exists: true}, "+working"},
		{scm.Change{Path: "new.txt", OldPath: "old.txt", Kind: scm.Renamed, Group: scm.Staged}, scm.FileContent{Content: "renamed\n", Exists: true}, scm.FileContent{Content: "renamed\n", Exists: true}, "rename from old.txt"},
		{scm.Change{Path: "empty.txt", Kind: scm.Added, Group: scm.Staged}, scm.FileContent{}, scm.FileContent{Exists: true}, "new file mode"},
		{scm.Change{Path: "mode.txt", Group: scm.Staged}, scm.FileContent{Content: "same\n", Exists: true}, scm.FileContent{Content: "same\n", Exists: true}, "new mode 100755"},
		{scm.Change{Path: "binary.dat", Group: scm.Staged}, scm.FileContent{Content: "old\x00bytes", Exists: true}, scm.FileContent{Content: "new\x00bytes", Exists: true}, "Binary files"},
	} {
		t.Run(tt.change.Path+string(tt.change.Group), func(t *testing.T) {
			got, err := (repository{root: dir}).ReadChange(context.Background(), tt.change)
			if err != nil || got.Before != tt.before || got.After != tt.after || !strings.Contains(got.Patch, tt.patch) {
				t.Fatalf("change = %+v, %v", got, err)
			}
		})
	}
	if err := os.Remove(filepath.Join(dir, "[日本語].txt")); err != nil {
		t.Fatal(err)
	}
	for _, staged := range []bool{false, true} {
		group := scm.WorkingTree
		if staged {
			gitRun(t, dir, "add", "-u")
			group = scm.Staged
		}
		got, err := (repository{root: dir}).ReadChange(context.Background(), scm.Change{Path: "[日本語].txt", Kind: scm.Deleted, Group: group})
		if err != nil || !got.Before.Exists || got.After.Exists || !strings.Contains(got.Patch, "deleted file mode") {
			t.Fatalf("deleted = %+v, %v", got, err)
		}
	}
}

func TestReadChangeUnbornConflictAndFailures(t *testing.T) {
	dir := setupTestRepo(t)
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", ".")
	r := repository{root: dir}
	change := scm.Change{Path: "file.txt", Kind: scm.Added, Group: scm.Staged}
	got, err := r.ReadChange(context.Background(), change)
	if err != nil || got.Before.Exists || got.After.Content != "first\n" {
		t.Fatalf("unborn = %+v, %v", got, err)
	}
	entries, err := IndexEntriesContext(context.Background(), dir, change.Path)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries: %v, %v", entries, err)
	}
	gitRun(t, dir, "update-index", "--force-remove", change.Path)
	cmd := gitCommandContext(context.Background(), "-C", dir, "update-index", "--index-info")
	cmd.Stdin = strings.NewReader("100644 " + entries[0].Object + " 2\tfile.txt\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("conflict: %s, %v", out, err)
	}
	got, err = r.ReadChange(context.Background(), change)
	if err != nil || !got.Conflicted {
		t.Fatalf("conflict = %+v, %v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.ReadChange(ctx, change); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled: %v", err)
	}
	gitRun(t, dir, "update-index", "--add", "--info-only", "--cacheinfo", "100644,1111111111111111111111111111111111111111,broken.txt")
	if _, err := r.ReadChange(context.Background(), scm.Change{Path: "broken.txt", Group: scm.Staged}); err == nil {
		t.Fatal("unreadable blob became missing content")
	}
}

func TestReadChangeGitlinkEndpoints(t *testing.T) {
	dir := setupTestRepo(t)
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, sub, "init", "-q")
	gitRun(t, sub, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "first")
	before := RevisionIdentity(sub)
	gitRun(t, dir, "add", "sub")
	gitRun(t, dir, "commit", "-qm", "submodule")
	gitRun(t, sub, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "second")
	after := RevisionIdentity(sub)
	for _, group := range []scm.ChangeGroup{scm.WorkingTree, scm.Staged} {
		if group == scm.Staged {
			gitRun(t, dir, "add", "sub")
		}
		got, err := (repository{root: dir}).ReadChange(context.Background(), scm.Change{Path: "sub", Kind: scm.Modified, Group: group})
		if err != nil || got.Before.Content != "Subproject commit "+before+"\n" || got.After.Content != "Subproject commit "+after+"\n" {
			t.Fatalf("gitlink %s = %+v, %v", group, got, err)
		}
	}
}

func TestReadChangeSymlinkReadsLinkText(t *testing.T) {
	dir := setupTestRepo(t)
	path := filepath.Join(dir, "link")
	if err := os.Symlink("old-target", path); err != nil {
		t.Skip(err)
	}
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-qm", "link")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing-target", path); err != nil {
		t.Fatal(err)
	}
	got, err := (repository{root: dir}).ReadChange(context.Background(), scm.Change{Path: "link", Kind: scm.Modified})
	if err != nil || got.Before.Content != "old-target" || got.After.Content != "missing-target" || !got.After.Exists {
		t.Fatalf("symlink = %+v, %v", got, err)
	}
}
