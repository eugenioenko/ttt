package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/search"
)

func requireRipgrep(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep not installed")
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

	found, err := search.FindAllMatches(context.Background(), search.Options{Query: "foo"}, []string{path})
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
