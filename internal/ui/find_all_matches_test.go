package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	found, err := FindAllMatches(context.Background(), SearchOptions{}, "", "", "foo", []string{path})
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

	found, err := FindAllMatches(context.Background(), SearchOptions{}, "", "", "foo", []string{path})
	if err != nil {
		t.Fatalf("FindAllMatches: %v", err)
	}
	if got := len(found[filepath.Clean(path)]); got != 1 {
		t.Fatalf("matches = %d, want 1 past the column cap", got)
	}
}

func TestFindAllMatchesReportsColumnsAndLine(t *testing.T) {
	requireRipgrep(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "one.txt")
	if err := os.WriteFile(path, []byte("first\nsay foo now\n"), 0644); err != nil {
		t.Fatal(err)
	}

	found, err := FindAllMatches(context.Background(), SearchOptions{}, "", "", "foo", []string{path})
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

// Replace All hands the matches to ApplyReplacements, so offsets must line up
// with the file as it is read back.
func TestFindAllMatchesOffsetsDriveReplacement(t *testing.T) {
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

	found, err := FindAllMatches(context.Background(), SearchOptions{}, "", "", "foo", []string{path})
	if err != nil {
		t.Fatalf("FindAllMatches: %v", err)
	}
	lines := make([]string, 0, 150)
	for i := 0; i < 150; i++ {
		lines = append(lines, fmt.Sprintf("line %d foo", i))
	}
	replaced := ApplyReplacements(lines, found[filepath.Clean(path)], "bar", SearchOptions{})
	for i, line := range replaced {
		if want := fmt.Sprintf("line %d bar", i); line != want {
			t.Fatalf("line %d = %q, want %q", i, line, want)
		}
	}
}

func TestFindAllMatchesEmptyQueryOrPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "one.txt")
	if err := os.WriteFile(path, []byte("foo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if found, err := FindAllMatches(context.Background(), SearchOptions{}, "", "", "", []string{path}); err != nil || found != nil {
		t.Errorf("empty query: found %v, err %v", found, err)
	}
	if found, err := FindAllMatches(context.Background(), SearchOptions{}, "", "", "foo", nil); err != nil || found != nil {
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

	found, err := FindAllMatches(context.Background(), SearchOptions{CaseSensitive: true}, "*.go", "", "Foo", []string{dir})
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
