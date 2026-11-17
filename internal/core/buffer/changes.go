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
	// overflow keeps the changes evicted from ring while a version is pinned,
	// oldest first at overflowBase, so a pinned batch longer than the ring can
	// still be replayed in full.
	overflow     []Change
	overflowBase uint64
	pins         []uint64
}

// Version increases with every change to Lines made through the Buffer's
// methods.
func (b *Buffer) Version() uint64 { return b.log.version }

// ChangePin keeps the changes made after a version replayable until released.
type ChangePin struct {
	log     *changeLog
	version uint64
}

// PinChanges keeps the changes made after version replayable until Release is
// called, even when there are more of them than the ring holds. A caller that
// applies a long batch in several steps and then replays it must pin the
// version it started from before the first step.
func (b *Buffer) PinChanges(version uint64) ChangePin {
	b.log.pins = append(b.log.pins, version)
	return ChangePin{log: &b.log, version: version}
}

// Release drops the retained changes no remaining pin needs.
func (p ChangePin) Release() { p.log.release(p.version) }

func (l *changeLog) release(version uint64) {
	for i := len(l.pins) - 1; i >= 0; i-- {
		if l.pins[i] == version {
			l.pins = append(l.pins[:i], l.pins[i+1:]...)
			break
		}
	}
	if len(l.pins) == 0 {
		l.overflow, l.overflowBase = nil, 0
		return
	}
	l.trimOverflow()
}

// trimOverflow drops the entries the remaining pins no longer need.
func (l *changeLog) trimOverflow() {
	from := l.pinnedFrom()
	for len(l.overflow) > 0 && l.overflowBase <= from {
		l.overflow = l.overflow[1:]
		l.overflowBase++
	}
	if len(l.overflow) == 0 {
		l.overflowBase = 0
	}
}

func (l *changeLog) pinnedFrom() uint64 {
	from := l.version
	for _, v := range l.pins {
		if v < from {
			from = v
		}
	}
	return from
}

// OldestReplayVersion returns the oldest version ChangesSince can still replay
// from; changes made before it have left the log.
func (b *Buffer) OldestReplayVersion() uint64 {
	oldest := uint64(0)
	if b.log.version > changeLogSize {
		oldest = b.log.version - changeLogSize
	}
	if len(b.log.pins) > 0 {
		oldest = min(oldest, b.log.pinnedFrom())
	}
	return oldest
}

// ChangesSince calls fn, oldest first, for each change made after version v.
// It returns false when v is too old to replay; the caller must then treat the
// whole buffer as changed.
func (b *Buffer) ChangesSince(v uint64, fn func(Change)) bool {
	cur := b.log.version
	if v > cur || v < b.OldestReplayVersion() {
		return false
	}
	for ver := v + 1; ver <= cur; ver++ {
		if len(b.log.overflow) > 0 && ver >= b.log.overflowBase {
			if off := ver - b.log.overflowBase; off < uint64(len(b.log.overflow)) {
				fn(b.log.overflow[off])
				continue
			}
		}
		fn(b.log.ring[(ver-1)%changeLogSize])
	}
	return true
}

func (b *Buffer) record(c Change) {
	b.log.version++
	idx := (b.log.version - 1) % changeLogSize
	if b.log.version > changeLogSize && len(b.log.pins) > 0 {
		evicted := b.log.version - changeLogSize
		if len(b.log.overflow) == 0 {
			b.log.overflowBase = evicted
		}
		b.log.overflow = append(b.log.overflow, b.log.ring[idx])
	}
	b.log.ring[idx] = c
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
