package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

const bashPrompt = "\x1b[36m/home/user/project\x1b[00m [main] ➜ "

// readlineRedraw is what bash sends after SIGWINCH: its cursor-up count comes
// from a prompt layout ttt cannot see, so the tests vary it.
func readlineRedraw(ups int, line string) string {
	return "\r\x1b[K" + strings.Repeat("\x1b[A", ups) + line
}

// feedOutput passes shell output to the emulator the way readLoop does.
func feedOutput(term *Terminal, s string) {
	out := []byte(s)
	if term.promptRedrawPending {
		out = term.takePromptRedraw(out)
	}
	term.term.Write(out)
}

func TestBashRedrawLeavesOnePrompt(t *testing.T) {
	term := newPromptTerminal(40, 10)
	feedOutput(term, "earlier output\r\n"+bashPrompt)

	for i, cols := range []int{25, 20, 15, 40} {
		term.resizeEmulator(cols, 10)
		feedOutput(term, readlineRedraw(i%3, bashPrompt))
	}

	screen := term.term.String()
	if n := strings.Count(screen, "/home/user"); n != 1 {
		t.Fatalf("%d prompts after resizing, want 1:\n%s", n, screen)
	}
	lines := strings.Split(screen, "\n")
	if lines[0] != "earlier output" || !strings.HasPrefix(lines[1], "/home/user") {
		t.Fatalf("prompt should follow the earlier output directly:\n%s", screen)
	}
}

func TestBashRedrawWithWrappedInput(t *testing.T) {
	term := newPromptTerminal(40, 10)
	input := "echo " + strings.Repeat("x", 30)
	feedOutput(term, "earlier output\r\n"+bashPrompt+input)

	term.resizeEmulator(20, 10)
	feedOutput(term, readlineRedraw(2, bashPrompt+input))

	screen := term.term.String()
	if n := strings.Count(screen, "/home/user"); n != 1 {
		t.Fatalf("%d prompts after resizing, want 1:\n%s", n, screen)
	}
	if !strings.HasPrefix(screen, "earlier output\n") {
		t.Fatalf("output above the prompt was overwritten:\n%s", screen)
	}
}

// readline repaints only the last line of a multi-line prompt.
func TestBashRedrawKeepsEarlierPromptLines(t *testing.T) {
	term := newPromptTerminal(40, 10)
	feedOutput(term, "first prompt line\r\n"+bashPrompt)

	term.resizeEmulator(20, 10)
	feedOutput(term, readlineRedraw(1, bashPrompt))

	screen := term.term.String()
	if n := strings.Count(screen, "first prompt line"); n != 1 {
		t.Fatalf("first prompt line appears %d times, want 1:\n%s", n, screen)
	}
	if n := strings.Count(screen, "/home/user"); n != 1 {
		t.Fatalf("%d prompts after resizing, want 1:\n%s", n, screen)
	}
}

func TestBashRedrawOnlyChecksFirstOutput(t *testing.T) {
	term := newPromptTerminal(40, 10)
	feedOutput(term, bashPrompt)

	term.resizeEmulator(20, 10)
	feedOutput(term, "job done\r\n")
	if term.promptRedrawPending {
		t.Fatal("redraw still pending after unrelated output")
	}
}

// A progress line redrawn by a running command after the resize matches no
// row above it, so it is written as is.
func TestBashRedrawLeavesProgressLinesAlone(t *testing.T) {
	term := newPromptTerminal(40, 10)
	feedOutput(term, "step one\r\ndownloading 49%")
	term.resizeEmulator(20, 10)

	progress := []byte("\r\x1b[K\x1b[Adownloading 50%")
	if got := term.takePromptRedraw(progress); string(got) != string(progress) {
		t.Fatalf("progress output rewritten as a prompt repaint: %q", got)
	}
}

func TestBashRedrawSkipsAltBuffer(t *testing.T) {
	term := newPromptTerminal(40, 10)
	feedOutput(term, bashPrompt+"\x1b[?1049h")
	term.resizeEmulator(20, 10)

	if term.promptRedrawPending {
		t.Fatal("redraw pending while a full-screen app is active")
	}
}

func TestFirstVisibleLine(t *testing.T) {
	got := firstVisibleLine([]byte("\x1b]0;title\a\x1b[36mdir\x1b[00m ➜ ls\r\nnext"))
	if got != "dir ➜ ls" {
		t.Fatalf("firstVisibleLine = %q, want %q", got, "dir ➜ ls")
	}
}

func TestBashPromptSurvivesResize(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash prompt repaint is Unix-only")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	prompt := "long-prompt-" + strings.Repeat("p", 40) + " $ "
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("PS1='"+prompt+"'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	term, err := New(bash, 80, 10, 0, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	term.Run()
	defer term.Close()

	waitForRawCount(t, term, "long-prompt-", 1)
	for i, cols := range []int{40, 30, 45, 80} {
		term.Resize(cols, 10)
		waitForRawCount(t, term, "long-prompt-", i+2)
	}

	var screen string
	term.Snapshot(func(x *xterm.Terminal) { screen = x.String() })
	if n := strings.Count(screen, "long-prompt-"); n != 1 {
		t.Fatalf("%d prompts after resizing, want 1:\n%s", n, screen)
	}
}

func waitForRawCount(t *testing.T, term *Terminal, want string, count int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Count(string(term.RawTail()), want) >= count {
			time.Sleep(50 * time.Millisecond)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("shell sent %q fewer than %d times; got %q", want, count, term.RawTail())
}
