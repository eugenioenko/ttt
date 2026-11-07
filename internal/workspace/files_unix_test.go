//go:build !windows

package workspace

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestListFilesWalkDirSkipsSpecialFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	if err := os.Symlink("a.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("pipe", filepath.Join(dir, "pipelink")); err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, f := range listFilesWalkDir(dir, "", FileVisibility{}) {
		got = append(got, f.Rel)
	}

	if len(got) != 2 || got[0] != "a.txt" || got[1] != "link" {
		t.Fatalf("got %v, want [a.txt link]", got)
	}
}
