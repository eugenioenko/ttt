package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/scm"
)

func TestProviderPreservesStatusAndLiteralStaging(t *testing.T) {
	dir := setupTestRepo(t)
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("tracked.txt", "initial\n")
	write("old.txt", "rename\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "initial")
	gitRun(t, dir, "mv", "old.txt", "renamed.txt")
	write("tracked.txt", "staged\n")
	gitRun(t, dir, "add", "tracked.txt")
	write("tracked.txt", "unstaged\n")
	write("[literal].txt", "new\n")
	write("l.txt", "must remain untracked\n")
	ctx := context.Background()
	repo, err := (Provider{}).Discover(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := repo.Changes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	files, err := StatusFilesContext(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]FileStatus, 0, len(changes))
	for _, c := range changes {
		got = append(got, FileStatus{Path: c.Path, OldPath: c.OldPath, Status: c.Status, Staged: c.Group == scm.Staged})
	}
	if !reflect.DeepEqual(got, files) {
		t.Fatalf("provider status changed: %v / %v", got, files)
	}
	identity, err := repo.Identity(ctx)
	if err != nil || identity.Root != dir || identity.Label != BranchName(dir) {
		t.Fatalf("identity: %+v, %v", identity, err)
	}
	revision, err := repo.Revision(ctx)
	if err != nil || revision != RevisionIdentity(dir) {
		t.Fatalf("revision: %q, %v", revision, err)
	}
	staging := repo.(scm.Staging)
	if err := staging.Stage(ctx, []string{"[literal].txt"}); err != nil {
		t.Fatal(err)
	}
	files, err = StatusFilesContext(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "[literal].txt" && !f.Staged || f.Path == "l.txt" && f.Staged {
			t.Fatalf("literal paths not preserved: %v", files)
		}
	}
	if err := staging.Unstage(ctx, []string{"[literal].txt"}); err != nil {
		t.Fatal(err)
	}
	after, err := repo.Changes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, changes) {
		t.Fatalf("staging round trip: %v / %v", after, changes)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := staging.Stage(cancelled, []string{"tracked.txt"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if err := staging.Unstage(cancelled, []string{"tracked.txt"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	for _, op := range []func(context.Context, []string) error{staging.Stage, staging.Unstage} {
		if err := op(cancelled, nil); err != nil {
			t.Fatalf("empty paths: %v", err)
		}
	}
}

func TestProviderUnbornRepository(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := (Provider{}).Discover(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Identity(context.Background()); err != nil {
		t.Fatal(err)
	}
	changes, err := repo.Changes(context.Background())
	if err != nil || len(changes) != 0 {
		t.Fatalf("empty status: %v, %v", changes, err)
	}
	revision, err := repo.Revision(context.Background())
	if err != nil || revision != "" {
		t.Fatalf("unborn revision: %q, %v", revision, err)
	}
}
