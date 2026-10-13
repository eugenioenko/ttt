package highlight

import (
	"math"
	"unsafe"

	"github.com/eugenioenko/ttt/internal/core/buffer"
)

// SyncBuffer brings the document up to buf. The edits since the last sync are
// replayed as one splice, so typing in a large file does not hand the
// document every line; a full update happens on the first sync, for another
// buffer, or when the buffer's change log no longer covers the edits.
func (h *Highlighter) SyncBuffer(buf *buffer.Buffer) {
	version, lines := buf.Version(), unsafe.SliceData(buf.Lines)
	if !h.dirty && h.buf == buf && h.version == version && h.lines == lines {
		return
	}
	if !h.dirty && h.buf == buf && h.version != version {
		if start, removed, added, ok := mergedEdit(buf, h.version, h.doc.Len()); ok &&
			h.doc.ReplaceLines(start, start+removed, buf.Lines[start:start+added]) == nil {
			h.version, h.lines, h.wantLine = version, lines, 0
			return
		}
	}
	h.doc.SetLines(buf.Lines)
	h.buf, h.version, h.lines, h.dirty, h.wantLine = buf, version, lines, false, 0
}

// mergedEdit folds the changes made to buf after version into one splice of a
// document holding oldLen lines: the lines before every change's start and the
// lines after every change's added range are unchanged.
func mergedEdit(buf *buffer.Buffer, version uint64, oldLen int) (start, removed, added int, ok bool) {
	n, start, suffix, valid := oldLen, math.MaxInt, math.MaxInt, true
	replayed := buf.ChangesSince(version, func(c buffer.Change) {
		if c.Start < 0 || c.Start+c.Removed > n {
			valid = false
			return
		}
		n += c.Added - c.Removed
		start = min(start, c.Start)
		suffix = min(suffix, n-c.Start-c.Added)
	})
	if !replayed || !valid || n != len(buf.Lines) || start == math.MaxInt {
		return 0, 0, 0, false
	}
	suffix = min(suffix, oldLen-start, n-start)
	return start, oldLen - suffix - start, n - suffix - start, true
}
