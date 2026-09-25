package terminal

import (
	"strings"

	xterm "github.com/gitpod-io/xterm-go"
)

// Shells that mark their prompt with OSC 133 (fish does natively) redraw it
// on SIGWINCH by moving the cursor up as many rows as the prompt had and
// repainting. Reflow changes that row count, so the repaint lands on the wrong
// row and leaves a copy of the old prompt above the new one on every resize.
// Blanking the prompt rows before the reflow keeps their count, the same
// thing kitty and Ghostty do for a prompt that redraws itself.

func (t *Terminal) watchPromptMarks() {
	t.term.RegisterOscHandler(133, xterm.NewOscStringHandler(func(data string) bool {
		t.notePromptMark(data)
		return false
	}))
}

func (t *Terminal) notePromptMark(data string) {
	kind, opts, _ := strings.Cut(data, ";")
	switch kind {
	case "A":
		t.dropPromptMarker()
		if t.term.IsAltBufferActive() || strings.Contains(";"+opts+";", ";redraw=0;") {
			return
		}
		buf := t.term.NormalBuffer()
		t.promptMarker = buf.AddMarker(buf.YBase + buf.Y)
		t.promptCol = buf.X
	case "C", "D":
		t.dropPromptMarker()
	}
}

func (t *Terminal) dropPromptMarker() {
	if t.promptMarker != nil {
		t.promptMarker.Dispose()
		t.promptMarker = nil
	}
}

// clearPromptForRedraw blanks the prompt the shell is about to repaint. The
// marker is dropped either way: the repaint sets a fresh one.
func (t *Terminal) clearPromptForRedraw() {
	marker := t.promptMarker
	t.promptMarker = nil
	if marker == nil || marker.IsDisposed || t.term.IsAltBufferActive() {
		return
	}
	defer marker.Dispose()
	buf := t.term.NormalBuffer()
	cursor := buf.YBase + buf.Y
	if marker.Line < 0 || marker.Line > cursor || cursor-marker.Line >= t.rows {
		return
	}
	attr := xterm.DefaultAttrData()
	for row := marker.Line; row <= cursor && row < buf.Lines.Length(); row++ {
		line := buf.Lines.Get(row)
		// A prompt can start after output that lacked a final newline: keep
		// that output, and the first row's wrap flag that belongs to it.
		start := 0
		if row == marker.Line {
			start = t.promptCol
		}
		line.ReplaceCells(start, line.Len, buf.GetNullCell(&attr), false)
		if start == 0 {
			line.IsWrapped = false
		}
	}
}
