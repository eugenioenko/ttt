//go:build linux || darwin || freebsd

package app

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCopyTreeRejectsUnsupportedEntry(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "folder")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "ok.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(src, "pipe"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "folder copy")

	if err := copyTree(src, dst); err == nil {
		t.Fatal("unsupported entry was silently skipped")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("partial destination was left behind: %v", err)
	}
}
