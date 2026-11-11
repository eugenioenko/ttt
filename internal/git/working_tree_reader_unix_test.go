//go:build linux || darwin || freebsd

package git

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestReleaseContractFIFOIsRejectedWithoutBlocking(t *testing.T) {
	dir := t.TempDir()
	if err := unix.Mkfifo(filepath.Join(dir, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := readWorkingTreeFileContext(ctx, dir, "pipe")
		done <- err
	}()
	select {
	case err := <-done:
		var pathErr *workingTreePathError
		if !errors.As(err, &pathErr) || pathErr.Kind != workingTreePathFIFO {
			t.Fatalf("FIFO error = %T %v", err, err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("FIFO read blocked")
	}
}

func TestReadWorkingTreeFileReturnsTypedSocketDirectoryAndDeviceErrors(t *testing.T) {
	dir := t.TempDir()
	listener, err := net.Listen("unix", filepath.Join(dir, "socket"))
	if err != nil {
		t.Skipf("unix sockets unavailable: %v", err)
	}
	defer listener.Close()
	_, err = readWorkingTreeFileContext(context.Background(), dir, "socket")
	var pathErr *workingTreePathError
	if !errors.As(err, &pathErr) || pathErr.Kind != workingTreePathSocket {
		t.Fatalf("socket error = %T %v", err, err)
	}
	if err := os.Mkdir(filepath.Join(dir, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = readWorkingTreeFileContext(context.Background(), dir, "directory")
	if !errors.As(err, &pathErr) || pathErr.Kind != workingTreePathDirectory {
		t.Fatalf("directory error = %T %v", err, err)
	}
	_, err = readWorkingTreeFileContext(context.Background(), string(filepath.Separator), filepath.Join("dev", "null"))
	if !errors.As(err, &pathErr) || pathErr.Kind != workingTreePathDevice {
		t.Fatalf("device error = %T %v", err, err)
	}
}

func TestReadWorkingTreeFileReturnsStableSymlinkMetadata(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(target, []byte("SECRET BYTES"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	file, err := readWorkingTreeFileContext(context.Background(), dir, "link")
	if err != nil || file.Kind != workingTreeFileSymlink || string(file.Content) != target || strings.Contains(string(file.Content), "SECRET BYTES") {
		t.Fatalf("symlink metadata=%+v err=%v", file, err)
	}
}

func TestReleaseContractSymlinkSwapCannotExposeTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "changing")
	temp := filepath.Join(dir, "replacement")
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("ordinary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secret, []byte("RACE SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stop atomic.Bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		for !stop.Load() {
			_ = os.Remove(temp)
			_ = os.WriteFile(temp, []byte("ordinary"), 0o644)
			_ = os.Rename(temp, path)
			_ = os.Remove(temp)
			_ = os.Symlink(secret, temp)
			_ = os.Rename(temp, path)
		}
	}()
	defer func() {
		stop.Store(true)
		<-done
	}()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		file, _ := readWorkingTreeFileContext(context.Background(), filepath.Dir(path), filepath.Base(path))
		if string(file.Content) == "RACE SECRET" {
			t.Fatal("symlink target bytes escaped")
		}
	}
}

func TestReadWorkingTreeFileIntermediateSymlinkSwapCannotExposeTarget(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	alias := filepath.Join(root, "alias")
	temp := filepath.Join(root, "replacement")
	outside := t.TempDir()
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "file"), []byte("ordinary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "file"), []byte("INTERMEDIATE SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	var stop atomic.Bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		for !stop.Load() {
			_ = os.Remove(temp)
			_ = os.Symlink("real", temp)
			_ = os.Rename(temp, alias)
			_ = os.Remove(temp)
			_ = os.Symlink(outside, temp)
			_ = os.Rename(temp, alias)
		}
	}()
	defer func() {
		stop.Store(true)
		<-done
	}()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		file, _ := readWorkingTreeFileContext(context.Background(), root, filepath.Join("alias", "file"))
		if string(file.Content) == "INTERMEDIATE SECRET" {
			t.Fatal("intermediate symlink target bytes escaped")
		}
	}
}

func TestReadWorkingTreeFileRejectsSymlinkedRootAndReadsRegularIntermediateComponents(t *testing.T) {
	parent := t.TempDir()
	realRoot := filepath.Join(parent, "real")
	if err := os.MkdirAll(filepath.Join(realRoot, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realRoot, "nested", "file"), []byte("ordinary"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := readWorkingTreeFileContext(context.Background(), realRoot, filepath.Join("nested", "file"))
	if err != nil || string(file.Content) != "ordinary" {
		t.Fatalf("regular intermediate read=%+v err=%v", file, err)
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(realRoot, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err = readWorkingTreeFileContext(context.Background(), alias, filepath.Join("nested", "file"))
	var pathErr *workingTreePathError
	if !errors.As(err, &pathErr) || pathErr.Kind != workingTreePathSymlinkComponent {
		t.Fatalf("symlinked root error = %T %v", err, err)
	}
}

func TestReadWorkingTreeFileAllowsSymlinkedAncestorsAboveRoot(t *testing.T) {
	parent := t.TempDir()
	realParent := filepath.Join(parent, "real-parent")
	realRoot := filepath.Join(realParent, "repo")
	if err := os.MkdirAll(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realRoot, "file"), []byte("ordinary"), 0o644); err != nil {
		t.Fatal(err)
	}
	aliasParent := filepath.Join(parent, "alias-parent")
	if err := os.Symlink(realParent, aliasParent); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	file, err := readWorkingTreeFileContext(context.Background(), filepath.Join(aliasParent, "repo"), "file")
	if err != nil || string(file.Content) != "ordinary" {
		t.Fatalf("symlinked ancestor read=%+v err=%v", file, err)
	}
}

func TestReadWorkingTreeLargeRegularFileHonorsCancellation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(64 << 20); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readWorkingTreeFileContext(ctx, dir, "large"); !errors.Is(err, context.Canceled) {
		t.Fatalf("large canceled read error = %v", err)
	}
}

func TestReadWorkingTreeFileClosesDescriptors(t *testing.T) {
	fdDir := "/proc/self/fd"
	before, err := os.ReadDir(fdDir)
	if err != nil {
		t.Skipf("descriptor inventory unavailable: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 200 {
		if _, err := readWorkingTreeFileContext(context.Background(), dir, "file"); err != nil {
			t.Fatal(err)
		}
	}
	runtime.GC()
	after, err := os.ReadDir(fdDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) > len(before)+2 {
		t.Fatalf("descriptor count grew from %d to %d", len(before), len(after))
	}
}
