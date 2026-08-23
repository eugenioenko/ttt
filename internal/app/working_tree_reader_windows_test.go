//go:build windows

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsWorkingTreeReaderClassifiesRegularMissingAndDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := readWorkingTreeFileContext(context.Background(), root, "file")
	if err != nil || !file.Exists || file.Kind != workingTreeFileRegular || string(file.Content) != "content" {
		t.Fatalf("regular file=%+v err=%v", file, err)
	}
	missing, err := readWorkingTreeFileContext(context.Background(), root, "missing")
	if err != nil || missing.Exists || missing.Kind != workingTreeFileMissing {
		t.Fatalf("missing file=%+v err=%v", missing, err)
	}
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = readWorkingTreeFileContext(context.Background(), root, "directory")
	var pathErr *workingTreePathError
	if !errors.As(err, &pathErr) || pathErr.Kind != workingTreePathDirectory {
		t.Fatalf("directory error=%T %v", err, err)
	}
}

func TestWindowsWorkingTreeReaderRejectsIntermediateReparsePoint(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "file"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "alias")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := readWorkingTreeFileContext(context.Background(), root, filepath.Join("alias", "file"))
	var pathErr *workingTreePathError
	if !errors.As(err, &pathErr) || pathErr.Kind != workingTreePathSymlinkComponent {
		t.Fatalf("intermediate reparse error=%T %v", err, err)
	}
}

func TestWindowsWorkingTreeReaderRejectsReparseRootAndReturnsLinkMetadata(t *testing.T) {
	parent := t.TempDir()
	realRoot := filepath.Join(parent, "real")
	if err := os.Mkdir(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realRoot, "file"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(realRoot, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := readWorkingTreeFileContext(context.Background(), alias, "file")
	var pathErr *workingTreePathError
	if !errors.As(err, &pathErr) || pathErr.Kind != workingTreePathSymlinkComponent {
		t.Fatalf("reparse root error=%T %v", err, err)
	}

	link := filepath.Join(realRoot, "link")
	if err := os.Symlink("file", link); err != nil {
		t.Skipf("file symlinks unavailable: %v", err)
	}
	file, err := readWorkingTreeFileContext(context.Background(), realRoot, "link")
	if err != nil || file.Kind != workingTreeFileSymlink || string(file.Content) != "file" {
		t.Fatalf("link metadata=%+v err=%v", file, err)
	}
}

func TestWindowsWorkingTreeReaderHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := readWorkingTreeFileContext(ctx, t.TempDir(), "file")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read error=%v", err)
	}
}
