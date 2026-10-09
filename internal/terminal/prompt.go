package terminal

import (
	"bytes"
	"strings"

	xterm "github.com/eugenioenko/xterm-go"
)

// Shells that mark their prompt with OSC 133 (fish does natively) redraw it
// on SIGWINCH by moving the cursor up as many rows as the prompt had and
// repainting. Reflow changes that row count, so the repaint lands on the wrong
// row and leaves a copy of the old prompt above the new one on every resize.
// Blanking the prompt rows before the reflow keeps their count, the same
// thing kitty and Ghostty do for a prompt that redraws itself.
//
// bash marks nothing, and readline moves up by a row count from a prompt
// layout it never updates for the new width. Its repaint, the first output
// after a resize, is fixed up instead: the old prompt is found by the
// repaint's own text and the repaint is moved to start there.

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
		if line == nil {
			continue
		}
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

func (t *Terminal) takePromptRedraw(p []byte) []byte {
	t.promptRedrawPending = false
	rest, ok := bytes.CutPrefix(p, []byte("\r\x1b[K"))
	if !ok || t.term.IsAltBufferActive() {
		return p
	}
	for {
		next, up := bytes.CutPrefix(rest, []byte("\x1b[A"))
		if !up {
			break
		}
		rest = next
	}
	text := firstVisibleLine(rest)
	if len(text) < 2 {
		return p
	}
	buf := t.term.NormalBuffer()
	cursor := buf.YBase + buf.Y
	for row := cursor; row >= buf.YBase; row-- {
		line := lineAt(buf, row)
		if line == nil {
			continue
		}
		old := line.TranslateToString(true, 0, -1)
		if len(old) < 2 || !(strings.HasPrefix(text, old) || strings.HasPrefix(old, text)) {
			continue
		}
		last := cursor
		for next := lineAt(buf, last+1); next != nil && next.IsWrapped; next = lineAt(buf, last+1) {
			last++
		}
		attr := xterm.DefaultAttrData()
		for r := row; r <= last; r++ {
			if line := lineAt(buf, r); line != nil {
				line.ReplaceCells(0, line.Len, buf.GetNullCell(&attr), false)
				line.IsWrapped = false
			}
		}
		buf.Y, buf.X = row-buf.YBase, 0
		return rest
	}
	return p
}

func lineAt(buf *xterm.Buffer, row int) *xterm.BufferLine {
	if row < 0 || row >= buf.Lines.Length() {
		return nil
	}
	return buf.Lines.Get(row)
}

// firstVisibleLine returns the printable text of p up to its first line break.
func firstVisibleLine(p []byte) string {
	var out []byte
	for i := 0; i < len(p); i++ {
		switch c := p[i]; {
		case c == '\r' || c == '\n':
			return string(out)
		case c == 0x1b && i+1 < len(p) && p[i+1] == '[':
			for i += 2; i < len(p) && (p[i] < 0x40 || p[i] > 0x7e); i++ {
			}
		case c == 0x1b && i+1 < len(p) && p[i+1] == ']':
			for i += 2; i < len(p) && p[i] != 0x07 && !(p[i] == 0x1b && i+1 < len(p) && p[i+1] == '\\'); i++ {
			}
			if i < len(p) && p[i] == 0x1b {
				i++
			}
		case c == 0x1b:
			i++
		case c >= 0x20:
			out = append(out, c)
		}
	}
	return string(out)
}
