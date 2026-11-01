package terminal

import (
	"strings"

	xterm "github.com/eugenioenko/xterm-go"
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

func (t *Terminal) watchReadlinePrompt(shellOwnsForeground func() bool) {
	// Bash's Readline enables bracketed paste before PS1 and disables it before
	// executing commands. Other programs can use this mode without a prompt.
	for _, final := range []byte{'h', 'l'} {
		t.term.RegisterCsiHandler(xterm.FunctionIdentifier{Prefix: '?', Final: final}, func(params *xterm.Params) bool {
			for _, mode := range params.Params[:params.Length] {
				if mode != 2004 {
					continue
				}
				t.dropPromptMarker()
				if final == 'h' && !t.term.IsAltBufferActive() && shellOwnsForeground() {
					buf := t.term.NormalBuffer()
					t.promptMarker = buf.AddMarker(buf.YBase + buf.Y)
					t.promptCol = buf.X
					t.promptReadline = true
				}
			}
			return false
		})
	}
	t.term.OnWriteParsed(t.trackReadlineEnd)
	t.term.OnLineFeed(t.trackReadlineEnd)
	for _, final := range []byte{'A', 'F', 'H', 'f', 'd'} {
		t.term.RegisterCsiHandler(xterm.FunctionIdentifier{Final: final}, func(*xterm.Params) bool {
			t.trackReadlineEnd()
			return false
		})
	}
	t.term.RegisterCsiHandler(xterm.FunctionIdentifier{Final: 'J'}, func(params *xterm.Params) bool {
		if params.Params[0] == 2 && t.promptReadline && !t.term.IsAltBufferActive() {
			t.dropPromptMarker()
			buf := t.term.NormalBuffer()
			t.promptMarker = buf.AddMarker(buf.YBase + buf.Y)
			t.promptCol = buf.X
			t.promptReadline = true
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
	if t.promptMarker == nil || t.promptMarker.IsDisposed {
		first := row
		for first > 0 && buf.Lines.Get(first).IsWrapped {
			first--
		}
		t.promptMarker = buf.AddMarker(first)
		t.promptCol = 0
	}
	if t.promptEnd != nil && !t.promptEnd.IsDisposed && t.promptEnd.Line >= row {
		return
	}
	if t.promptEnd != nil {
		t.promptEnd.Dispose()
	}
	t.promptEnd = buf.AddMarker(row)
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
	t.promptReadline = false
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
		// Readline repaints from the current logical line, but does not emit a
		// fresh prompt marker on resize. Explicit PS1 newlines belong above it.
		first = cursor
		for first > marker.Line && buf.Lines.Get(first).IsWrapped {
			first--
		}
		for last+1 < buf.Lines.Length() && buf.Lines.Get(last+1).IsWrapped {
			last++
		}
		if t.promptEnd != nil && !t.promptEnd.IsDisposed {
			last = max(last, t.promptEnd.Line)
		}
		buf.Y = first - buf.YBase
		buf.X = 0
	}
	attr := xterm.DefaultAttrData()
	for row := first; row <= last && row < buf.Lines.Length(); row++ {
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
