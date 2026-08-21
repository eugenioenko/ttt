package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/eugenioenko/vt10x"
	"github.com/gdamore/tcell/v3"
)

func newUpdateChan(term *terminal.Terminal) chan struct{} {
	updated := make(chan struct{}, 100)
	term.OnUpdate = func() {
		select {
		case updated <- struct{}{}:
		default:
		}
	}
	return updated
}

func waitFor(t *testing.T, updated chan struct{}, cond func() bool) {
	t.Helper()
	if cond() {
		return
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-updated:
			if cond() {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for condition")
		}
	}
}

// newRawMouseLoopbackTerminal spawns the default shell, then drives it into
// raw mode ("stty raw -echo") and execs cat in its place. A real full-screen
// TUI always puts its PTY into raw/cbreak mode as one of the first things it
// does — plain cat never does, so the PTY's line discipline would otherwise
// stay in canonical (cooked) mode and buffer every write until a newline,
// which would silently swallow the newline-less escape sequences this test
// needs to round-trip (both the DECSET mode-enabling sequence and the SGR
// mouse bytes TerminalWidget forwards). Once raw, cat echoes stdin to stdout
// byte-for-byte and unbuffered, so anything TerminalWidget writes to the PTY
// comes straight back through the read loop into RawTail() unmodified.
func newRawMouseLoopbackTerminal(t *testing.T) (*terminal.Terminal, chan struct{}) {
	t.Helper()
	term, err := terminal.New("", 80, 24, 0, nil, "")
	if err != nil {
		t.Fatalf("terminal.New() error: %v", err)
	}
	term.Run()

	updated := newUpdateChan(term)
	term.WriteString("stty raw -echo; printf RAWOK; exec cat\n")
	waitFor(t, updated, func() bool {
		return strings.Contains(string(term.RawTail()), "RAWOK")
	})
	return term, updated
}

func enableSGRMouseMode(t *testing.T, term *terminal.Terminal, updated chan struct{}) {
	t.Helper()
	term.WriteString("\x1b[?1006h\x1b[?1000h")
	waitFor(t, updated, func() bool {
		return term.Mode()&vt10x.ModeMouseSgr != 0 && term.Mode()&vt10x.ModeMouseButton != 0
	})
}

func TestTerminalWidget_ForwardsSGRMouseClickAndRelease(t *testing.T) {
	term, updated := newRawMouseLoopbackTerminal(t)
	defer term.Close()
	enableSGRMouseMode(t, term, updated)

	tw := NewTerminalWidget(term, &TerminalColorPalette{})
	tw.SetRect(Rect{X: 0, Y: 0, W: 80, H: 24})

	press := tcell.NewEventMouse(5, 3, tcell.Button1, tcell.ModNone)
	if result := tw.HandleEvent(press); result != EventCaptured {
		t.Fatalf("HandleEvent(press) = %v, want EventCaptured", result)
	}
	if tw.hasSelection {
		t.Error("expected no local selection to start while mouse reporting is enabled")
	}

	waitFor(t, updated, func() bool {
		return strings.Contains(string(term.RawTail()), "\x1b[<0;6;4M")
	})

	release := tcell.NewEventMouse(5, 3, tcell.ButtonNone, tcell.ModNone)
	if result := tw.HandleEvent(release); result != EventConsumed {
		t.Fatalf("HandleEvent(release) = %v, want EventConsumed", result)
	}

	waitFor(t, updated, func() bool {
		return strings.Contains(string(term.RawTail()), "\x1b[<0;6;4m")
	})

	if tw.mouseButtonHeld != -1 {
		t.Errorf("mouseButtonHeld = %d after release, want -1", tw.mouseButtonHeld)
	}
}

func TestTerminalWidget_ForwardsSGRWheel(t *testing.T) {
	term, updated := newRawMouseLoopbackTerminal(t)
	defer term.Close()
	enableSGRMouseMode(t, term, updated)

	tw := NewTerminalWidget(term, &TerminalColorPalette{})
	tw.SetRect(Rect{X: 0, Y: 0, W: 80, H: 24})

	wheel := tcell.NewEventMouse(10, 10, tcell.WheelUp, tcell.ModNone)
	if result := tw.HandleEvent(wheel); result != EventConsumed {
		t.Fatalf("HandleEvent(wheel) = %v, want EventConsumed", result)
	}
	if tw.scrollOffset != 0 {
		t.Errorf("scrollOffset = %d, want 0 — wheel should forward to the app, not scroll locally", tw.scrollOffset)
	}

	waitFor(t, updated, func() bool {
		return strings.Contains(string(term.RawTail()), "\x1b[<64;11;11M")
	})
}

func TestTerminalWidget_CtrlClickStaysLocalEvenWithMouseReporting(t *testing.T) {
	term, updated := newRawMouseLoopbackTerminal(t)
	defer term.Close()
	enableSGRMouseMode(t, term, updated)

	tw := NewTerminalWidget(term, &TerminalColorPalette{})
	tw.SetRect(Rect{X: 0, Y: 0, W: 80, H: 24})

	press := tcell.NewEventMouse(5, 3, tcell.Button1, tcell.ModCtrl)
	if result := tw.HandleEvent(press); result != EventCaptured {
		t.Fatalf("HandleEvent(ctrl+press) = %v, want EventCaptured", result)
	}
	if !tw.hasSelection {
		t.Error("expected ctrl+click with no link to start local selection even though mouse reporting is enabled")
	}
}

func TestTerminalWidget_WheelScrollsLocallyWithoutMouseReporting(t *testing.T) {
	term, err := terminal.New("/bin/cat", 20, 5, 1000, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	term.Run()

	for i := 0; i < 10; i++ {
		term.WriteString("line\n")
	}
	deadline := time.Now().Add(2 * time.Second)
	for term.ScrollbackLen() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if term.ScrollbackLen() == 0 {
		t.Fatalf("setup: expected live ScrollbackLen() > 0, got %d", term.ScrollbackLen())
	}

	tw := NewTerminalWidget(term, &TerminalColorPalette{})
	tw.SetRect(Rect{X: 0, Y: 0, W: 20, H: 5})

	wheel := tcell.NewEventMouse(10, 3, tcell.WheelUp, tcell.ModNone)
	if result := tw.HandleEvent(wheel); result != EventConsumed {
		t.Fatalf("HandleEvent(wheel) = %v, want EventConsumed", result)
	}
	if tw.scrollOffset == 0 {
		t.Error("expected wheel to scroll locally when the app has not enabled mouse reporting")
	}
}
