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
	if err := os.Symlink("a.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("pipe", filepath.Join(dir, "pipelink")); err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, f := range listFilesWalkDir(dir, "") {
		got = append(got, f.Rel)
	}

	if len(got) != 2 || got[0] != "a.txt" || got[1] != "link" {
		t.Fatalf("got %v, want [a.txt link]", got)
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
