package terminal

import (
	"bytes"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gitpod-io/xterm-go"
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

func TestNewReturnsErrorAndClosesPtyForInvalidShell(t *testing.T) {
	term, err := New("/nonexistent/shell/path", 80, 24, 0, nil, "")
	if err == nil {
		t.Fatalf("expected New() to return an error, got terminal %+v", term)
	}
	if term != nil {
		t.Fatalf("expected New() to return a nil terminal on error, got %+v", term)
	}
}

func TestNewDefaultsScrollback(t *testing.T) {
	term, err := New("/bin/sh", 80, 24, -5, nil, "")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	updated := make(chan struct{}, 100)
	term.OnUpdate = func() {
		term.AckUpdate()
		select {
		case updated <- struct{}{}:
		default:
		}
	}
	term.Run()
	defer term.Close()

	// A non-positive scrollbackMax must fall back to the default rather than
	// producing a terminal with no history.
	// Output more lines than the viewport height (24 rows) and assert that lines
	// accumulate into scrollback.
	term.WriteString("for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30; do echo line $i; done\n")

	deadline := time.After(5 * time.Second)
	for term.ScrollbackLen() <= 0 {
		select {
		case <-updated:
		case <-deadline:
			t.Fatalf("timed out waiting for scrollback lines to appear: ScrollbackLen() = %d", term.ScrollbackLen())
		}
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
		term.AckUpdate()
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
			term.Snapshot(func(xt *xterm.Terminal) {
				if strings.Contains(xt.String(), "hello_ttt_test") {
					found = true
				}
			})
		case <-deadline:
			t.Fatal("timed out waiting for shell output to appear in view")
		}
	}
}

func TestRawTailCapturesWrittenBytes(t *testing.T) {
	term := newTestTerminal(t)
	defer term.Close()

	updated := make(chan struct{}, 100)
	term.OnUpdate = func() {
		term.AckUpdate()
		select {
		case updated <- struct{}{}:
		default:
		}
	}
	term.Run()

	term.WriteString("echo raw_tail_marker\n")

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-updated:
			if strings.Contains(string(term.RawTail()), "raw_tail_marker") {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for raw tail to capture shell output")
		}
	}
}

func TestRawTailCapsAtMaxAndKeepsMostRecent(t *testing.T) {
	term := &Terminal{}

	chunkSize := rawTailMax / 4
	for _, b := range []byte("abcde") {
		term.mu.Lock()
		term.appendRawTail(bytes.Repeat([]byte{b}, chunkSize))
		term.mu.Unlock()
	}

	got := term.RawTail()
	if len(got) != rawTailMax {
		t.Fatalf("RawTail() len = %d, want %d", len(got), rawTailMax)
	}
	if bytes.Contains(got, []byte{'a'}) {
		t.Error("expected oldest bytes ('a') to have been trimmed")
	}
	if !bytes.Contains(got, []byte{'e'}) {
		t.Error("expected most recent bytes ('e') to be present")
	}
}

func TestRawTailSingleWriteLargerThanMax(t *testing.T) {
	term := &Terminal{}

	big := append(bytes.Repeat([]byte{'x'}, rawTailMax), bytes.Repeat([]byte{'y'}, 10)...)
	term.mu.Lock()
	term.appendRawTail(big)
	term.mu.Unlock()

	got := term.RawTail()
	if len(got) != rawTailMax {
		t.Fatalf("RawTail() len = %d, want %d", len(got), rawTailMax)
	}
	if !bytes.HasSuffix(got, bytes.Repeat([]byte{'y'}, 10)) {
		t.Error("expected the tail to keep the end of an oversized single write")
	}
}

func TestPrimaryDeviceAttributesResponse(t *testing.T) {
	term, err := New("/bin/sh", 80, 24, 0, nil, "")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	updated := make(chan struct{}, 100)
	term.OnUpdate = func() {
		term.AckUpdate()
		select {
		case updated <- struct{}{}:
		default:
		}
	}

	term.Run()
	defer term.Close()

	term.WriteString("stty raw -echo; printf '\\033[0c'; exec cat\n")

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-updated:
			if strings.Contains(string(term.RawTail()), "\x1b[?1;2c") {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for DA1 response, got rawtail: %q", string(term.RawTail()))
		}
	}
}

// go-pty keeps its own copy of the slave fd open in this process, so the master
// never reports EOF when the child dies. Detecting exit therefore has to come
// from waiting on the process, not from the read loop erroring out.
func TestOnExitFiresWhenShellExits(t *testing.T) {
	term := newTestTerminal(t)
	exited := make(chan struct{})
	var once sync.Once
	term.OnExit = func() { once.Do(func() { close(exited) }) }
	term.Run()
	defer term.Close()

	term.WriteString("exit\n")

	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("OnExit never fired after the shell exited")
	}
	term.mu.Lock()
	exitedFlag := term.exited
	term.mu.Unlock()
	if !exitedFlag {
		t.Fatal("exited = false after the shell exited")
	}
}

// Close() tears the tab down on its own; OnExit firing there too would close it twice.
func TestOnExitDoesNotFireOnClose(t *testing.T) {
	term := newTestTerminal(t)
	var fired atomic.Bool
	term.OnExit = func() { fired.Store(true) }
	term.Run()

	term.Close()

	if fired.Load() {
		t.Fatal("OnExit fired on an explicit Close()")
	}
}

func TestOnUpdateCoalescesUntilAcknowledged(t *testing.T) {
	term, err := New("/bin/cat", 80, 24, 0, nil, "")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer term.Close()
	updates := make(chan struct{}, 100)
	term.OnUpdate = func() { updates <- struct{}{} }
	term.Run()

	// cat prints each line twice: the tty echo and cat's own output.
	waitForLine := func(text string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			count := 0
			term.Snapshot(func(xt *xterm.Terminal) {
				b := xt.Buffer()
				for y := 0; y < xt.Rows(); y++ {
					if strings.Contains(b.TranslateBufferLineToString(b.YBase+y, true, 0, xt.Cols()), text) {
						count++
					}
				}
			})
			if count == 2 {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatalf("timed out waiting for %q", text)
	}
	drain := func() int {
		time.Sleep(50 * time.Millisecond)
		n := 0
		for {
			select {
			case <-updates:
				n++
			default:
				return n
			}
		}
	}

	term.WriteString("first\n")
	waitForLine("first")
	term.WriteString("second\n")
	waitForLine("second")
	if n := drain(); n != 1 {
		t.Fatalf("got %d updates for several reads before AckUpdate, want 1", n)
	}

	term.AckUpdate()
	term.WriteString("third\n")
	waitForLine("third")
	if n := drain(); n != 1 {
		t.Fatalf("got %d updates after AckUpdate, want 1", n)
	}
}
