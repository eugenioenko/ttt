package buffer

import "slices"

// Change records one edit as a splice: lines [Start, Start+Removed) of the
// previous version were replaced by lines [Start, Start+Added).
type Change struct {
	Start, Removed, Added int
}

const changeLogSize = 64

type changeLog struct {
	version uint64
	ring    [changeLogSize]Change
}

// Version increases with every change to Lines made through the Buffer's
// methods.
func (b *Buffer) Version() uint64 { return b.log.version }

// ChangesSince calls fn, oldest first, for each change made after version v.
// It returns false when v is too old to replay; the caller must then treat the
// whole buffer as changed.
func (b *Buffer) ChangesSince(v uint64, fn func(Change)) bool {
	cur := b.log.version
	if v > cur || cur-v > changeLogSize {
		return false
	}
	for ver := v + 1; ver <= cur; ver++ {
		fn(b.log.ring[(ver-1)%changeLogSize])
	}
	return true
}

func (b *Buffer) record(c Change) {
	b.log.version++
	b.log.ring[(b.log.version-1)%changeLogSize] = c
}

// SetLine replaces the text of one line.
func (b *Buffer) SetLine(i int, text string) {
	if i < 0 || i >= len(b.Lines) {
		return
	}
	b.Lines[i] = text
	b.record(Change{Start: i, Removed: 1, Added: 1})
}

// Splice replaces removed lines starting at start with lines.
func (b *Buffer) Splice(start, removed int, lines ...string) {
	n := len(b.Lines)
	if start < 0 || start > n {
		return
	}
	removed = max(0, min(removed, n-start))
	if removed == len(lines) {
		copy(b.Lines[start:], lines)
	} else {
		tail := n - start - removed
		newN := start + len(lines) + tail
		if newN > cap(b.Lines) {
			grown := make([]string, newN, newN+newN/4)
			copy(grown, b.Lines[:start])
			copy(grown[start+len(lines):], b.Lines[start+removed:])
			b.Lines = grown
		} else {
			// lines may alias the region the tail moves over.
			lines = slices.Clone(lines)
			old := b.Lines
			b.Lines = b.Lines[:newN]
			copy(b.Lines[start+len(lines):], old[start+removed:n])
			if newN < n {
				clear(old[newN:n])
			}
		}
		copy(b.Lines[start:], lines)
	}
	b.record(Change{Start: start, Removed: removed, Added: len(lines)})
}

// SetLines replaces the whole content.
func (b *Buffer) SetLines(lines []string) {
	old := len(b.Lines)
	b.Lines = lines
	b.record(Change{Start: 0, Removed: old, Added: len(lines)})
}
