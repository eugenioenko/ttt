package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/scm"
)

func TestHistoryReaderKeepsImmutableSnapshot(t *testing.T) {
	dir := setupTestRepo(t)
	r := repository{root: dir}
	ctx := context.Background()
	empty, err := r.ReadHistory(ctx, scm.HistoryRequest{Limit: 1})
	if err != nil || empty.Anchor != "" || len(empty.Entries) != 0 {
		t.Fatalf("unborn: %+v %v", empty, err)
	}
	for _, name := range []string{"first", "second"} {
		if err := os.WriteFile(filepath.Join(dir, "file"), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(t, dir, "add", ".")
		gitRun(t, dir, "commit", "-qm", name)
	}
	first, err := r.ReadHistory(ctx, scm.HistoryRequest{Limit: 1})
	if err != nil || !first.HasMore || first.Entries[0].Message != "second" {
		t.Fatalf("first: %+v %v", first, err)
	}
	gitRun(t, dir, "commit", "--allow-empty", "-qm", "new head")
	next, err := r.ReadHistory(ctx, scm.HistoryRequest{Anchor: first.Anchor, Offset: 1, Limit: 1})
	if err != nil || next.Anchor != first.Anchor || next.HasMore || next.Entries[0].Message != "first" {
		t.Fatalf("snapshot moved: %+v %v", next, err)
	}
	if _, err := r.ReadHistory(ctx, scm.HistoryRequest{Anchor: "HEAD", Limit: 1}); err == nil {
		t.Fatal("Git adapter accepted mutable ref as snapshot")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := r.ReadHistory(canceled, scm.HistoryRequest{Limit: 1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestProviderClassifiesNonRepository(t *testing.T) {
	if _, err := (Provider{}).Discover(context.Background(), t.TempDir()); !errors.Is(err, scm.ErrNotRepository) {
		t.Fatalf("non-repository: %v", err)
	}
}
