//go:build !windows

package ui

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestOpenFileRefusesNamedPipe(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	g := NewEditorGroupWidget(nil, 4, true, "relative")
	var reported string
	g.OnError = func(msg string) { reported = msg }
	before := len(g.tabs)

	done := make(chan error, 1)
	go func() {
		done <- g.OpenFile(pipe)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("OpenFile succeeded for a named pipe")
		}
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
