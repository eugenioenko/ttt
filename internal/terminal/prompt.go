package terminal

import (
	"strings"

	xterm "github.com/eugenioenko/xterm-go"
)

// Shells redraw their prompt on SIGWINCH by moving the cursor up as many rows
// as the prompt had and repainting. Reflow changes that row count, so the
// repaint lands on the wrong row and leaves a copy of the old prompt above the
// new one on every resize. Blanking the prompt rows before the reflow keeps
// their count, the same thing kitty and Ghostty do for a prompt that redraws
// itself.
//
// OSC 133 shells (fish) mark every repaint, so their marker is single-use.
// Bash's Readline marks nothing: its prompt is found through bracketed paste,
// which Readline enables before PS1 and disables before running a command, and
// the marker survives resizes. promptEnd tracks the lowest row the edit reached,
// so input below the cursor is cleared too.
//
// Readline starts its SIGWINCH repaint with a CR, an erase and cursor-up moves
// counted from the layout it drew at an older width, so they land on the wrong
// row. After a resize the marker holds the row the repaint must start on, and
// those moves are replaced by a jump there.

func (t *Terminal) watchPromptMarks() {
	t.term.RegisterOscHandler(133, xterm.NewOscStringHandler(func(data string) bool {
		t.notePromptMark(data)
		return false
	}))
}

func (t *Terminal) watchReadlinePrompt(shellOwnsForeground func() bool) {
	for _, final := range []byte{'h', 'l'} {
		t.term.RegisterCsiHandler(xterm.FunctionIdentifier{Prefix: '?', Final: final}, func(params *xterm.Params) bool {
			for _, mode := range params.Params[:params.Length] {
				if mode != 2004 {
					continue
				}
				t.dropPromptMarker()
				// Other programs use bracketed paste without a prompt.
				if final == 'h' && shellOwnsForeground() {
					t.markPromptAtCursor(true)
				}
			}
			return false
		})
	}
	t.term.OnWriteParsed(func() {
		if t.promptRepaint && t.term.NormalBuffer().X != 0 {
			t.promptRepaint = false
		}
		t.trackReadlineEnd()
	})
	t.term.OnLineFeed(t.trackReadlineEnd)
	t.term.RegisterCsiHandler(xterm.FunctionIdentifier{Final: 'A'}, func(*xterm.Params) bool {
		buf := t.term.NormalBuffer()
		if t.promptRepaint && buf.X == 0 && !t.term.IsAltBufferActive() && t.promptMarker != nil && !t.promptMarker.IsDisposed {
			buf.Y = max(t.promptMarker.Line-buf.YBase, 0)
			return true
		}
		t.trackReadlineEnd()
		return false
	})
	// Record the cursor before Readline moves it back up within one write.
	for _, final := range []byte{'F', 'H', 'f', 'd'} {
		t.term.RegisterCsiHandler(xterm.FunctionIdentifier{Final: final}, func(*xterm.Params) bool {
			t.trackReadlineEnd()
			return false
		})
	}
	t.term.RegisterCsiHandler(xterm.FunctionIdentifier{Final: 'J'}, func(params *xterm.Params) bool {
		if params.Params[0] == 2 && t.promptReadline {
			t.dropPromptMarker()
			t.markPromptAtCursor(true)
		}
		return false
	})
}

func (t *Terminal) trackReadlineEnd() {
	if !t.promptReadline || t.term.IsAltBufferActive() {
		return
	}
	buf := t.term.NormalBuffer()
	row := buf.YBase + buf.Y
	// Clearing the screen disposes the marker set just before it.
	if t.promptMarker == nil || t.promptMarker.IsDisposed {
		first := row
		for first > 0 && buf.Lines.Get(first).IsWrapped {
			first--
		}
		t.promptMarker = buf.AddMarker(first)
		t.promptCol = 0
	}
	if t.promptEnd != nil && !t.promptEnd.IsDisposed {
		if t.promptEnd.Line >= row {
			return
		}
		t.promptEnd.Dispose()
	}
	t.promptEnd = buf.AddMarker(row)
}

func (t *Terminal) notePromptMark(data string) {
	kind, opts, _ := strings.Cut(data, ";")
	switch kind {
	case "A":
		t.dropPromptMarker()
		if !strings.Contains(";"+opts+";", ";redraw=0;") {
			t.markPromptAtCursor(false)
		}
	case "C", "D":
		t.dropPromptMarker()
	}
}

func (t *Terminal) markPromptAtCursor(readline bool) {
	if t.term.IsAltBufferActive() {
		return
	}
	buf := t.term.NormalBuffer()
	t.promptMarker = buf.AddMarker(buf.YBase + buf.Y)
	t.promptCol = buf.X
	t.promptReadline = readline
}

func (t *Terminal) dropPromptMarker() {
	if t.promptMarker != nil {
		t.promptMarker.Dispose()
		t.promptMarker = nil
	}
	t.promptReadline = false
	t.promptRedrawn = false
	t.promptRepaint = false
	if t.promptEnd != nil {
		t.promptEnd.Dispose()
		t.promptEnd = nil
	}
}

func (t *Terminal) clearPromptForRedraw() {
	marker := t.promptMarker
	if marker == nil || marker.IsDisposed || t.term.IsAltBufferActive() {
		return
	}
	if !t.promptReadline {
		t.promptMarker = nil
		defer marker.Dispose()
	}
	buf := t.term.NormalBuffer()
	cursor := buf.YBase + buf.Y
	if marker.Line < 0 || marker.Line > cursor || cursor-marker.Line >= t.rows {
		return
	}
	first, last := marker.Line, cursor
	if t.promptReadline {
		// Before the first repaint, Readline redraws from the cursor's logical
		// line; explicit PS1 newlines above it stay. Its repaints insert their
		// own line breaks, so afterwards the marker is the only reliable start.
		if !t.promptRedrawn {
			for first = cursor; first > marker.Line && buf.Lines.Get(first).IsWrapped; first-- {
			}
		}
		for last+1 < buf.Lines.Length() && buf.Lines.Get(last+1).IsWrapped {
			last++
		}
		if t.promptEnd != nil && !t.promptEnd.IsDisposed {
			last = max(last, t.promptEnd.Line)
		}
	}
	t.clearRows(first, last, marker.Line)
	if t.promptReadline {
		if first != marker.Line {
			marker.Dispose()
			t.promptMarker = buf.AddMarker(first)
		}
		t.promptCol = 0
		if t.promptEnd != nil {
			t.promptEnd.Dispose()
			t.promptEnd = nil
		}
		t.promptRedrawn = true
		t.promptRepaint = true
		buf.Y = first - buf.YBase
		buf.X = 0
	}
}

func (t *Terminal) clearRows(first, last, markerLine int) {
	buf := t.term.NormalBuffer()
	attr := xterm.DefaultAttrData()
	for row := first; row <= last && row < buf.Lines.Length(); row++ {
		line := buf.Lines.Get(row)
		// A prompt can start after output that lacked a final newline: keep
		// that output, and the first row's wrap flag that belongs to it.
		start := 0
		if row == markerLine {
			start = t.promptCol
		}
		line.ReplaceCells(start, line.Len, buf.GetNullCell(&attr), false)
		if start == 0 {
			line.IsWrapped = false
		}
	}
}
