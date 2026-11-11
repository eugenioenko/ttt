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

func TestBaselinePreservesIndexAndMissingStates(t *testing.T) {
	dir := setupTestRepo(t)
	write := func(path, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, path), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("[日本語].txt", "committed\n")
	write("empty.txt", "")
	write(".gitignore", "ignored.txt\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-qm", "initial")
	write("[日本語].txt", "staged\n")
	gitRun(t, dir, "add", "--", "[日本語].txt")
	write("[日本語].txt", "working\n")
	write("ignored.txt", "ignored\n")
	write("untracked.txt", "new\n")
	reader := repository{root: dir}
	for _, tt := range []struct {
		path string
		want scm.Baseline
	}{
		{"[日本語].txt", scm.Baseline{Content: "staged\n", Exists: true}},
		{"empty.txt", scm.Baseline{Exists: true}},
		{"untracked.txt", scm.Baseline{}},
		{"missing.txt", scm.Baseline{}},
		{"ignored.txt", scm.Baseline{Ignored: true}},
	} {
		t.Run(tt.path, func(t *testing.T) {
			got, err := reader.ReadBaseline(context.Background(), tt.path)
			if err != nil || got != tt.want {
				t.Fatalf("baseline = %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.ReadBaseline(ctx, "[日本語].txt"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}

func TestBaselineConflictUsesHeadFallback(t *testing.T) {
	dir := setupTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("HEAD content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-qm", "initial")
	entries, err := IndexEntriesContext(context.Background(), dir, "file.txt")
	if err != nil || len(entries) != 1 {
		t.Fatalf("index entries: %v, %v", entries, err)
	}
	gitRun(t, dir, "update-index", "--force-remove", "file.txt")
	// Higher index stages model unresolved content, including an add/add path
	// that has no HEAD version for the gutter to fall back to.
	for _, path := range []string{"file.txt", "new.txt"} {
		cmd := gitCommandContext(context.Background(), "-C", dir, "update-index", "--index-info")
		cmd.Stdin = strings.NewReader("100644 " + entries[0].Object + " 2\t" + path + "\n")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("unmerged index: %s, %v", out, err)
		}
		got, err := (repository{root: dir}).ReadBaseline(context.Background(), path)
		want := scm.Baseline{Conflicted: true}
		if path == "file.txt" {
			want.Content, want.Exists = "HEAD content\n", true
		}
		if err != nil || got != want {
			t.Fatalf("%s baseline = %+v, %v; want %+v", path, got, err, want)
		}
	}
}

func TestBaselineReadFailureIsNotMissing(t *testing.T) {
	dir := setupTestRepo(t)
	gitRun(t, dir, "update-index", "--add", "--info-only", "--cacheinfo", "100644,1111111111111111111111111111111111111111,broken.txt")
	if base, err := (repository{root: dir}).ReadBaseline(context.Background(), "broken.txt"); err == nil {
		t.Fatalf("unreadable indexed blob became missing content: %+v", base)
	}
}
