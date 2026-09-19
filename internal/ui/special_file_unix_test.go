//go:build !windows

package ui

import (
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

	files := listFilesWalkDir(dir, "")

	if len(files) != 1 || files[0].Rel != "a.txt" {
		t.Fatalf("got %+v, want only a.txt", files)
	}
}

func TestOpenFileRefusesNamedPipe(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	g := NewEditorGroupWidget(nil, 4, true, "relative")
	var reported string
	g.OnError = func(msg string) { reported = msg }
	before := len(g.tabs)

	done := make(chan struct{})
	go func() {
		g.OpenFile(pipe)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("OpenFile blocked on a named pipe")
	}
	if reported == "" {
		t.Error("expected an error to be reported")
	}
	if len(g.tabs) != before {
		t.Errorf("got %d tabs, want %d", len(g.tabs), before)
	}
}
