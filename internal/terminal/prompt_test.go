package terminal

import (
	"strings"
	"testing"

	xterm "github.com/eugenioenko/xterm-go"
)

const fishPrompt = "\x1b]133;A;click_events=1\x1b\\/home/user/a/rather/long/project/path\r\n❯ \x1b]133;B\x1b\\"

func newPromptTerminal(cols, rows int) *Terminal {
	t := &Terminal{cols: cols, rows: rows}
	t.term = xterm.New(xterm.WithCols(cols), xterm.WithRows(rows), xterm.WithScrollback(100))
	t.watchPromptMarks()
	return t
}

// fish repaints after SIGWINCH by moving up to the first prompt row and
// clearing from there; a reflowed prompt used to survive that as a copy.
func TestResizeLeavesOnePromptAfterShellRedraw(t *testing.T) {
	term := newPromptTerminal(60, 10)
	term.term.WriteString("earlier output\r\n" + fishPrompt)

	for _, cols := range []int{20, 50, 15, 60} {
		term.resizeEmulator(cols, 10)
		term.term.WriteString("\r\x1b[A\x1b[J" + fishPrompt)
	}

	screen := term.term.String()
	if n := strings.Count(screen, "/home/user"); n != 1 {
		t.Fatalf("%d prompts after resizing, want 1:\n%s", n, screen)
	}
	if !strings.Contains(screen, "earlier output") {
		t.Fatalf("output above the prompt was lost:\n%s", screen)
	}
}

func TestResizeKeepsRunningCommandOutput(t *testing.T) {
	term := newPromptTerminal(60, 10)
	term.term.WriteString(fishPrompt + "ls\r\n\x1b]133;C\x1b\\file-one file-two")

	term.resizeEmulator(30, 10)

	if !strings.Contains(term.term.String(), "file-one") {
		t.Fatalf("command output cleared on resize:\n%s", term.term.String())
	}
}

// Layout passes resize terminals to the size they already have; no SIGWINCH
// follows, so clearing the prompt then would leave it blank.
func TestResizeToSameSizeKeepsPrompt(t *testing.T) {
	term := newPromptTerminal(60, 10)
	term.term.WriteString(fishPrompt)

	if term.resizeEmulator(60, 10) {
		t.Fatal("resize to the same size reported a change")
	}
	if !strings.Contains(term.term.String(), "/home/user") {
		t.Fatalf("prompt cleared by a same-size resize:\n%s", term.term.String())
	}
}

// A command whose output lacks a final newline leaves the next prompt on the
// same row; blanking the prompt must not take that output with it.
func TestResizeKeepsOutputBeforeSameRowPrompt(t *testing.T) {
	term := newPromptTerminal(60, 10)
	term.term.WriteString("partial" + fishPrompt)

	term.resizeEmulator(40, 10)

	screen := term.term.String()
	if !strings.Contains(screen, "partial") {
		t.Fatalf("output before the prompt was cleared:\n%s", screen)
	}
	if strings.Contains(screen, "/home/user") {
		t.Fatalf("prompt not cleared:\n%s", screen)
	}
}

// Captured from bash with ttt's shell integration, after the prompt was first
// drawn wrapped: each repaint moves up from Readline's layout at an older width.
func TestResizeKeepsOutputAboveIntegratedBashPrompt(t *testing.T) {
	term := newPromptTerminal(46, 12)
	const (
		prompt  = "\x1b]133;A\a\x1b[36m…/enko/Documents/ttt\x1b[00m \x1b[33m[fix/readline-bash-prompt-resize]\x1b[00m ➜ "
		repaint = "\r\x1b[K\x1b[A\x1b]133;A\a\x1b[36m…/enko/Documents/ttt\x1b[00m \x1b[33m[fix/readline-bash-prompt-resize\r\n\r]\x1b[00m ➜ "
	)
	term.term.WriteString("$ echo RESIZE_READY\r\n\x1b]133;C\aRESIZE_READY\r\n\x1b[?2004h" + prompt)
	for _, cols := range []int{45, 43, 41, 39, 38, 40, 44, 50, 60, 30} {
		term.resizeEmulator(cols, 12)
		term.term.WriteString(repaint)
		screen := term.term.String()
		if strings.Count(screen, "enko/Documents") != 1 || !strings.Contains(screen, "RESIZE_READY\n") {
			t.Fatalf("resize to %d lost output or duplicated the prompt:\n%s", cols, screen)
		}
	}
}
