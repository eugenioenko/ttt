//go:build !windows

package workspace

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
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
	for _, f := range listFilesWalkDir(context.Background(), dir, "", FileVisibility{}) {
		got = append(got, f.Rel)
	}

	if len(got) != 2 || got[0] != "a.txt" || got[1] != "link" {
		t.Fatalf("got %v, want [a.txt link]", got)
	}
}

func TestListFilesCancelsRunningCommandWithoutFallback(t *testing.T) {
	bin := t.TempDir()
	dir := t.TempDir()
	ready := filepath.Join(dir, "started")
	fallback := filepath.Join(dir, "fallback")
	t.Setenv("TTT_LIST_READY", ready)
	t.Setenv("TTT_LIST_FALLBACK", fallback)
	for name, script := range map[string]string{
		"rg":  "#!/bin/sh\nprintf ready > \"$TTT_LIST_READY\"\nwhile :; do :; done\n",
		"git": "#!/bin/sh\nprintf fallback > \"$TTT_LIST_FALLBACK\"\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan []File, 1)
	go func() { done <- ListFiles(ctx, []string{dir}, FileVisibility{}) }()
	deadline := time.After(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatal("file listing command never started")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	select {
	case files := <-done:
		if len(files) != 0 {
			t.Fatalf("cancelled scan returned files: %v", files)
		}
	case <-deadline:
		t.Fatal("file listing did not stop after cancellation")
	}
	if _, err := os.Stat(fallback); !os.IsNotExist(err) {
		t.Fatalf("cancelled listing attempted fallback: %v", err)
	}
}
