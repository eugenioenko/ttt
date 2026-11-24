package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
)

// TestTerminalCtrlClickFileLink drives the real pipeline: a PTY running
// /bin/cat echoes a file:line:col reference, Ctrl+hover makes Render detect
// and underline the link, and Ctrl+click opens the file at that position.
func TestTerminalCtrlClickFileLink(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	target := filepath.Join(h.dir, "target.txt")
	requireNoError(t, os.WriteFile(target, []byte("one\ntwo\nthree four five\nsix\n"), 0644))

	// cat echoes whatever is written to the PTY, giving deterministic output.
	h.app.Settings.Terminal.Shell = "/bin/cat"
	h.exec("terminal.toggle")
	if len(h.app.Terminals) != 1 {
		t.Fatalf("expected 1 terminal after toggle, got %d", len(h.app.Terminals))
	}
	term := h.app.Terminals[0].Term
	defer term.Close()

	const link = "target.txt:3:6"
	term.WriteString("see " + link + " end\n")

	// Poll until the echoed text is rendered and the screen has settled
	// (the tty echo and cat's own output both arrived).
	deadline := time.Now().Add(3 * time.Second)
	var prev string
	settled := false
	for time.Now().Before(deadline) {
		time.Sleep(30 * time.Millisecond)
		h.redraw()
		cur := h.screenText()
		if strings.Contains(cur, link) && cur == prev {
			settled = true
			break
		}
		prev = cur
	}
	if !settled {
		t.Fatalf("terminal output never settled, screen:\n%s", h.screenText())
	}

	// Locate the link on screen.
	lx, ly := -1, -1
	for y, line := range strings.Split(h.screenText(), "\n") {
		if x := strings.Index(line, link); x >= 0 {
			lx, ly = x, y
			break
		}
	}
	if lx < 0 {
		t.Fatalf("link text not found on screen:\n%s", h.screenText())
	}

	// Ctrl+hover, then render a frame: link detection runs during Render
	// while Ctrl is held, populating the link cache and underlining the span.
	h.app.Root.HandleEvent(tcell.NewEventMouse(lx, ly, tcell.ButtonNone, tcell.ModCtrl))
	h.redraw()

	_, style, _ := h.screen.Get(lx, ly)
	if style.GetUnderlineStyle() == tcell.UnderlineStyleNone {
		t.Errorf("expected underlined link cell at (%d,%d) while ctrl is held", lx, ly)
	}

	// Ctrl+click the link.
	h.app.Root.HandleEvent(tcell.NewEventMouse(lx, ly, tcell.Button1, tcell.ModCtrl))
	h.app.Root.HandleEvent(tcell.NewEventMouse(lx, ly, tcell.ButtonNone, tcell.ModNone))
	h.redraw()

	if got := h.app.EditorGroup.ActiveFilePath(); got != target {
		t.Fatalf("active file = %q, want %q", got, target)
	}
	line, col := h.app.EditorGroup.ActiveCursor()
	if line != 2 {
		t.Errorf("cursor line = %d, want 2 (0-based line 3)", line)
	}
	if col != 5 {
		t.Errorf("cursor col = %d, want 5 (0-based col 6)", col)
	}
}
