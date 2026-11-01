//go:build !windows

package terminal

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestBashPromptSurvivesRepeatedPTYResizes(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	const marker = "\x1b]0;resize-ready\a"
	const prompt = `\[\e[36m\]…/enko/Documents/ttt \[\e[33m\][fix/1289-terminal-padding]\[\e[0m\] ➜ \[\e]0;resize-ready\a\]`
	dev, err := New(bash, 60, 10, 1000, []string{"PS1=" + prompt, "PROMPT_COMMAND=printf 'kept-output\\n'", "HISTFILE=/dev/null"}, t.TempDir(), "--noprofile", "--norc", "-i")
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	updates := make(chan struct{}, 1)
	dev.OnUpdate = func() {
		dev.AckUpdate()
		select {
		case updates <- struct{}{}:
		default:
		}
	}
	dev.Run()
	waitPrompt := func(previous int) {
		t.Helper()
		timeout := time.NewTimer(3 * time.Second)
		defer timeout.Stop()
		for {
			raw := dev.RawTail()
			if bytes.Count(raw, []byte(marker)) > previous && bytes.HasSuffix(raw, []byte(marker)) {
				return
			}
			select {
			case <-updates:
			case <-timeout.C:
				t.Fatalf("Bash did not complete its prompt redraw: %q", dev.RawTail())
			}
		}
	}
	waitPrompt(0)
	widths := make([]int, 0, 55)
	for cols := 59; cols >= 35; cols-- {
		widths = append(widths, cols)
	}
	for cols := 36; cols <= 65; cols++ {
		widths = append(widths, cols)
	}
	for _, cols := range widths {
		previous := bytes.Count(dev.RawTail(), []byte(marker))
		dev.Resize(cols, 10)
		waitPrompt(previous)
		dev.mu.Lock()
		var text strings.Builder
		buf := dev.term.NormalBuffer()
		for i := 0; i < buf.Lines.Length(); i++ {
			text.WriteString(buf.Lines.Get(i).TranslateToString(true, 0, -1))
		}
		screen := text.String()
		scrollback := buf.YBase
		dev.mu.Unlock()
		if strings.Count(screen, "…/enko") != 1 || strings.Count(screen, "kept-output") != 1 || scrollback != 0 {
			t.Fatalf("resize to %d duplicated a prompt, lost output, or scrolled: %q (scrollback %d)", cols, screen, scrollback)
		}
	}
}
