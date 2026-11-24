package terminal

import (
	"errors"
	"testing"

	"github.com/aymanbagabas/go-pty"
)

type resizeFailurePty struct {
	pty.Pty
	err   error
	calls int
}

func (p *resizeFailurePty) Resize(_, _ int) error {
	p.calls++
	return p.err
}

func TestFailedPtyResizePreservesEmulatorAndCanRetry(t *testing.T) {
	term := newPromptTerminal(60, 10)
	term.term.WriteString(fishPrompt)
	pt := &resizeFailurePty{err: errors.New("resize failed")}
	term.pt = pt
	before := term.term.String()
	term.Resize(30, 10)
	if cols, rows := term.Size(); cols != 60 || rows != 10 || term.term.String() != before {
		t.Fatal("failed PTY resize changed emulator geometry or prompt")
	}
	pt.err = nil
	term.Resize(30, 10)
	if cols, rows := term.Size(); cols != 30 || rows != 10 || pt.calls != 2 {
		t.Fatalf("resize retry failed: %dx%d, calls=%d", cols, rows, pt.calls)
	}
	term.Resize(30, 10)
	if pt.calls != 2 {
		t.Fatal("unchanged size retried the PTY resize")
	}
}

func TestExitedTerminalCanReflowWithoutPty(t *testing.T) {
	term := newPromptTerminal(60, 10)
	term.exited = true
	pt := &resizeFailurePty{err: errors.New("closed PTY")}
	term.pt = pt
	term.Resize(30, 10)
	if cols, rows := term.Size(); cols != 30 || rows != 10 || pt.calls != 0 {
		t.Fatalf("exited terminal did not reflow independently: %dx%d, calls=%d", cols, rows, pt.calls)
	}
}
