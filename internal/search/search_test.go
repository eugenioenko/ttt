package search

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireRipgrep(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep not installed")
	}
}

// The result list stops at 100 matching lines per file, but a replacement must
// cover every match, so the re-scan must not inherit that cap (#965).
func TestFindAllMatchesIgnoresResultListCap(t *testing.T) {
	requireRipgrep(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "many.txt")
	var content string
	for i := 0; i < 150; i++ {
		content += fmt.Sprintf("line %d foo\n", i)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	found, err := FindAllMatches(context.Background(), Options{Query: "foo"}, []string{path})
	if err != nil {
		t.Fatalf("FindAllMatches: %v", err)
	}
	matches := found[filepath.Clean(path)]
	if len(matches) != 150 {
		t.Fatalf("matches = %d, want 150 (the list cap must not apply)", len(matches))
	}
}

// A match past the 1000-column result limit is not listed, so it must still be
// replaced.
func TestFindAllMatchesIgnoresColumnCap(t *testing.T) {
	requireRipgrep(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "wide.txt")
	long := make([]byte, 1200)
	for i := range long {
		long[i] = 'x'
	}
	if err := os.WriteFile(path, []byte(string(long)+" foo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	found, err := FindAllMatches(context.Background(), Options{Query: "foo"}, []string{path})
	if err != nil {
		t.Fatalf("FindAllMatches: %v", err)
	}
	if got := len(found[filepath.Clean(path)]); got != 1 {
		t.Fatalf("matches = %d, want 1 past the column cap", got)
	}
}

func TestFindAllMatchesTreatsNoMatchesAsSuccess(t *testing.T) {
	requireRipgrep(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte("nothing here\n"), 0644); err != nil {
		t.Fatal(err)
	}

	found, err := FindAllMatches(context.Background(), Options{Query: "foo"}, []string{path})
	if err != nil {
		t.Fatalf("FindAllMatches: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("matches = %v, want none", found)
	}
}

func TestFindAllMatchesContinuesAfterEmptyBatch(t *testing.T) {
	requireRipgrep(t)
	dir := t.TempDir()
	matching := filepath.Join(dir, "matching.txt")
	content := strings.Repeat("foo\n", 150)
	if err := os.WriteFile(matching, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var paths []string
	for i := 0; len(batchPaths(append(paths, matching))) == 1; i++ {
		path := filepath.Join(dir, fmt.Sprintf("no-match-%04d-%064d.txt", i, i))
		if err := os.WriteFile(path, []byte("nothing here\n"), 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}

	paths = append(paths, matching)
	if got := len(batchPaths(paths)); got != 2 {
		t.Fatalf("batches = %d, want 2", got)
	}

	found, err := FindAllMatches(context.Background(), Options{Query: "foo"}, paths)
	if err != nil {
		t.Fatalf("FindAllMatches: %v", err)
	}
	if got := len(found[filepath.Clean(matching)]); got != 150 {
		t.Fatalf("matches = %d, want 150 after the empty batch", got)
	}
}

func TestScanBatchPreservesRipgrepErrorsWithPartialMatches(t *testing.T) {
	requireRipgrep(t)
	dir := t.TempDir()
	matching := filepath.Join(dir, "matching.txt")
	if err := os.WriteFile(matching, []byte("foo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing.txt")

	found, err := scanBatch(
		context.Background(),
		Options{Query: "foo"},
		[]string{matching, missing},
	)
	if err == nil {
		t.Fatal("scanBatch error = nil, want ripgrep status 2")
	}
	if got := len(found[filepath.Clean(matching)]); got != 1 {
		t.Fatalf("partial matches = %d, want 1", got)
	}
}

func TestFindAllMatchesReportsColumnsAndLine(t *testing.T) {
	requireRipgrep(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "one.txt")
	if err := os.WriteFile(path, []byte("first\nsay foo now\n"), 0644); err != nil {
		t.Fatal(err)
	}

	found, err := FindAllMatches(context.Background(), Options{Query: "foo"}, []string{path})
	if err != nil {
		t.Fatalf("FindAllMatches: %v", err)
	}
	matches := found[filepath.Clean(path)]
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	m := matches[0]
	if m.LineNum != 2 || m.ColStart != 4 || m.ColEnd != 7 {
		t.Errorf("got line %d cols %d-%d, want line 2 cols 4-7", m.LineNum, m.ColStart, m.ColEnd)
	}
	if m.LineText != "say foo now" {
		t.Errorf("LineText = %q, want %q", m.LineText, "say foo now")
	}
	if m.FilePath != filepath.Clean(path) {
		t.Errorf("FilePath = %q, want %q", m.FilePath, filepath.Clean(path))
	}
}

func TestFindAllMatchesEmptyQueryOrPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "one.txt")
	if err := os.WriteFile(path, []byte("foo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if found, err := FindAllMatches(context.Background(), Options{}, []string{path}); err != nil || found != nil {
		t.Errorf("empty query: found %v, err %v", found, err)
	}
	if found, err := FindAllMatches(context.Background(), Options{Query: "foo"}, nil); err != nil || found != nil {
		t.Errorf("no paths: found %v, err %v", found, err)
	}
}

func TestFindAllMatchesHonorsOptionsAndGlobs(t *testing.T) {
	requireRipgrep(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.go"), []byte("Foo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("Foo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	found, err := FindAllMatches(context.Background(), Options{CaseSensitive: true, Include: "*.go", Query: "Foo"}, []string{dir})
	if err != nil {
		t.Fatalf("FindAllMatches: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("files = %d, want 1 (the *.go include)", len(found))
	}
	if _, ok := found[filepath.Clean(filepath.Join(dir, "keep.go"))]; !ok {
		t.Errorf("include glob was not applied: %v", found)
	}
}

// A long result list has to be scanned in batches, because the platform
// command-line limit is far below the number of paths a large workspace
// produces.
func TestFindAllMatchesBatchesLongPathLists(t *testing.T) {
	requireRipgrep(t)
	dir := t.TempDir()
	var paths []string
	for i := 0; i < 200; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file%03d.txt", i))
		if err := os.WriteFile(path, []byte("foo\n"), 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}

	found, err := FindAllMatches(context.Background(), Options{Query: "foo"}, paths)
	if err != nil {
		t.Fatalf("FindAllMatches: %v", err)
	}
	if len(found) != len(paths) {
		t.Fatalf("files = %d, want %d", len(found), len(paths))
	}
}
