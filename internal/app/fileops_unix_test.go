//go:build linux || darwin || freebsd

package app

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
)

func TestCopyTreeRejectsUnsupportedEntry(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "folder")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(src, "a-locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "child.txt"), []byte("child"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	pipe := filepath.Join(src, "z-pipe")
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "folder copy")

	err := copyTree(src, dst)
	var unsupported unsupportedFileTypeError
	if !errors.As(err, &unsupported) || unsupported.path != pipe {
		t.Fatalf("error = %v, want unsupported type for %q", err, pipe)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("partial destination was left behind: %v", err)
	}
}

func TestFileOpDuplicateUnsupportedEntryUsesCatalogMessage(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(a.CloseTasks)

	dir := t.TempDir()
	src := filepath.Join(dir, "folder")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	pipe := filepath.Join(src, "pipe")
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Fatal(err)
	}

	a.FileOpDuplicate(src, func() {})
	want := i18n.Format("files.unsupportedType", i18n.Args{"path": pipe})
	if got := a.Status.Notification; got != want {
		t.Errorf("notification = %q, want %q", got, want)
	}
}
