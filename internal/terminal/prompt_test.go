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

func TestReadlineResizeKeepsOnePrompt(t *testing.T) {
	term := newPromptTerminal(60, 10)
	term.watchReadlinePrompt(func() bool { return true })
	const prompt = "…/enko/Documents/ttt [fix/1289-terminal-padding] ➜ "
	term.term.WriteString("earlier output\r\n\x1b[?2004h" + prompt)
	for _, cols := range []int{48, 35, 60, 25, 65} {
		term.resizeEmulator(cols, 10)
		term.term.WriteString("\r\x1b[K" + prompt)
		if screen := term.term.String(); strings.Count(screen, "…/enko") != 1 || !strings.Contains(screen, "earlier output") {
			t.Fatalf("resize to %d lost output or duplicated the prompt:\n%s", cols, screen)
		}
	}
}

func TestReadlineResizeClearsInputBelowCursor(t *testing.T) {
	term := newPromptTerminal(20, 10)
	term.watchReadlinePrompt(func() bool { return true })
	term.term.WriteString("\x1b[?2004hprompt> first\r\ninput tail\x1b[A")
	term.resizeEmulator(18, 10)
	if strings.Contains(term.term.String(), "input tail") {
		t.Fatalf("input below the cursor survived cleanup:\n%s", term.term.String())
	}
	term.term.WriteString("\r\x1b[Kprompt> first\r\ninput tail\x1b[A")
	if n := strings.Count(term.term.String(), "input tail"); n != 1 {
		t.Fatalf("input tail appears %d times:\n%s", n, term.term.String())
	}
}

func TestReadlineResizeKeepsMultilinePromptHeader(t *testing.T) {
	term := newPromptTerminal(60, 10)
	term.watchReadlinePrompt(func() bool { return true })
	const prompt = "a/rather/long/prompt/path/for/testing> "
	term.term.WriteString("output\r\n\x1b[?2004hheader\r\n" + prompt)
	for _, cols := range []int{25, 50, 20, 60} {
		term.resizeEmulator(cols, 10)
		term.term.WriteString("\r\x1b[K" + prompt)
		if screen := term.term.String(); strings.Count(screen, "header") != 1 || strings.Count(screen, "a/rather") != 1 || !strings.Contains(screen, "output") {
			t.Fatalf("multiline prompt damaged at width %d:\n%s", cols, screen)
		}
	}
}

func TestReadlineResizeAfterClearScreen(t *testing.T) {
	term := newPromptTerminal(60, 10)
	term.watchReadlinePrompt(func() bool { return true })
	const prompt = "a/rather/long/prompt/path/for/testing> "
	term.term.WriteString("output\r\n\x1b[?2004h" + prompt)
	term.term.WriteString("\x1b[H\x1b[2J" + prompt)
	for _, cols := range []int{25, 50, 20, 60} {
		term.resizeEmulator(cols, 10)
		term.term.WriteString("\r\x1b[K" + prompt)
		if n := strings.Count(term.term.String(), "a/rather"); n != 1 {
			t.Fatalf("%d prompts after clear and resize:\n%s", n, term.term.String())
		}
	}
}

func TestReadlineTrackingDoesNotClearPrograms(t *testing.T) {
	for _, foreground := range []bool{false, true} {
		term := newPromptTerminal(60, 10)
		term.watchReadlinePrompt(func() bool { return foreground })
		term.term.WriteString("\x1b[?2004hprompt> \x1b[?2004l\r\ncommand output")
		if !foreground {
			term.term.WriteString("\x1b[?2004h")
		}
		term.resizeEmulator(30, 10)
		if !strings.Contains(term.term.String(), "command output") || term.promptMarker != nil {
			t.Fatalf("command output treated as a prompt, shell foreground=%v:\n%s", foreground, term.term.String())
		}
	}
}
