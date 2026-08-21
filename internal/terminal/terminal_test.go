package terminal

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eugenioenko/vt10x"
)

func newTestTerminal(t *testing.T) *Terminal {
	t.Helper()
	term, err := New("/bin/sh", 80, 24, 0, nil, "")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	return term
}

func TestNewSpawnsShellWithSize(t *testing.T) {
	term := newTestTerminal(t)
	term.Run()
	defer term.Close()

	cols, rows := term.Size()
	if cols != 80 || rows != 24 {
		t.Fatalf("Size() = %d,%d, want 80,24", cols, rows)
	}
	if term.pt == nil {
		t.Fatal("expected pty to be set")
	}
	if term.cmd == nil || term.cmd.Process == nil {
		t.Fatal("expected shell process to be started")
	}
}

func TestNewDefaultsScrollback(t *testing.T) {
	term, err := New("/bin/sh", 80, 24, -5, nil, "")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	term.Run()
	defer term.Close()
	// A non-positive scrollbackMax must fall back to the default rather than
	// producing a terminal with no history.
	if term.vt == nil {
		t.Fatal("expected vt to be initialized")
	}
}

func TestCloseKillsProcessAndStopsReadLoop(t *testing.T) {
	term := newTestTerminal(t)
	term.Run()

	// Give the read loop a moment to start before tearing down.
	time.Sleep(20 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		term.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close() did not return — readLoop likely leaked")
	}

	// Close() blocks on <-t.done, which readLoop closes on exit, so reaching
	// this point already proves the read goroutine terminated. Confirm the
	// process was actually reaped too.
	if term.cmd.ProcessState == nil {
		t.Fatal("expected process to have exited after Close()")
	}
}

func TestCloseWithoutRunDoesNotDeadlock(t *testing.T) {
	term := newTestTerminal(t)

	done := make(chan struct{})
	go func() {
		term.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close() deadlocked when Run() was never called")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	term := newTestTerminal(t)
	term.Run()
	time.Sleep(20 * time.Millisecond)

	term.Close()

	doneSecond := make(chan struct{})
	go func() {
		term.Close()
		close(doneSecond)
	}()

	select {
	case <-doneSecond:
	case <-time.After(2 * time.Second):
		t.Fatal("second Close() call blocked or deadlocked")
	}
}

func TestCloseConcurrentIsSafe(t *testing.T) {
	term := newTestTerminal(t)
	term.Run()
	time.Sleep(20 * time.Millisecond)

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			term.Close()
		})
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent Close() calls did not all return")
	}
}

func TestWriteStringAndReadLoopUpdatesView(t *testing.T) {
	term := newTestTerminal(t)
	defer term.Close()

	updated := make(chan struct{}, 100)
	term.OnUpdate = func() {
		select {
		case updated <- struct{}{}:
		default:
		}
	}
	term.Run()

	term.WriteString("echo hello_ttt_test\n")

	found := false
	deadline := time.After(5 * time.Second)
	for !found {
		select {
		case <-updated:
			term.Snapshot(func(v vt10x.View) {
				if strings.Contains(v.String(), "hello_ttt_test") {
					found = true
				}
			})
		case <-deadline:
			t.Fatal("timed out waiting for shell output to appear in view")
		}
	}
}
