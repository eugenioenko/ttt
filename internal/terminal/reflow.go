package terminal

import (
	xterm "github.com/gitpod-io/xterm-go"
)

// trimForReflow drops the oldest scrollback lines that a narrowing resize would
// otherwise reflow past the buffer's capacity.
//
// xterm-go's reflowSmaller rearrange loop (buffer.go:446) walks its write index
// downwards once per inserted line without a lower bound, so when the reflowed
// lines do not fit in the circular list the index goes negative: CircularList.Set
// then panics with "index out of range [-1]" on a list whose start index is 0 and
// silently overwrites an unrelated slot on one that has wrapped. Keeping the
// projected line count within capacity is the only way to avoid both from here.
func trimForReflow(b *xterm.Buffer, oldCols, oldRows, newCols, newRows int) {
	if b == nil || !b.HasScrollback() || newCols >= oldCols || newCols < 1 {
		return
	}
	lines := b.Lines
	if lines == nil || lines.Length() == 0 {
		return
	}

	budget := lines.MaxLength() - oldRows + newRows
	if budget < 1 {
		return
	}

	// A destination row holds newCols cells, or newCols-1 when a wide character
	// cannot straddle the wrap, so the narrower divisor is the safe projection.
	perRow := newCols
	if perRow > 1 {
		perRow--
	}

	trimTo := 0
	projected := 0
	for i := lines.Length() - 1; i >= 0; {
		start := i
		for start > 0 && lines.Get(start).IsWrapped {
			start--
		}

		content := (i-start)*oldCols + lines.Get(i).GetTrimmedLength()
		rows := max((content+perRow-1)/perRow, 1)
		if projected+rows > budget {
			trimTo = i + 1
			break
		}

		projected += rows
		trimTo = start
		i = start - 1
	}

	if trimTo > b.YBase {
		trimTo = b.YBase
	}
	if trimTo <= 0 {
		return
	}

	lines.TrimStart(trimTo)
	b.YBase = max(b.YBase-trimTo, 0)
	b.YDisp = max(b.YDisp-trimTo, 0)
	b.SavedState.Y = max(b.SavedState.Y-trimTo, 0)
}
