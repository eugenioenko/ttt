package ui

import (
	"slices"
	"unsafe"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/diff"
)

// shiftLine moves a line number across an edit. A line inside the replaced
// range lands on the last line that replaced it, or on the line that took its
// place when nothing did.
func shiftLine(r int, c buffer.Change) int {
	switch {
	case r >= c.Start+c.Removed:
		return r + c.Added - c.Removed
	case r >= c.Start:
		return min(r, max(c.Start+c.Added-1, c.Start))
	}
	return r
}

// shiftKeys moves the keys of m across c in place. Keys that land on the same
// line are merged in the order of their old lines.
func shiftKeys[V any](m map[int]V, c buffer.Change, merge func(a, b V) V) {
	if len(m) == 0 || c.Added == c.Removed {
		return
	}
	var region, tail []int
	for k := range m {
		switch {
		case k >= c.Start+c.Removed:
			tail = append(tail, k)
		case k >= c.Start:
			region = append(region, k)
		}
	}
	if len(region) == 0 && len(tail) == 0 {
		return
	}
	slices.Sort(region)
	moved := make([]V, 0, len(region)+len(tail))
	for _, k := range region {
		moved = append(moved, m[k])
	}
	for _, k := range tail {
		moved = append(moved, m[k])
	}
	for _, k := range region {
		delete(m, k)
	}
	for _, k := range tail {
		delete(m, k)
	}
	put := func(k int, v V) {
		if old, ok := m[k]; ok {
			v = merge(old, v)
		}
		m[k] = v
	}
	for i, k := range region {
		put(shiftLine(k, c), moved[i])
	}
	for i, k := range tail {
		put(shiftLine(k, c), moved[len(region)+i])
	}
}

func sumInts(a, b int) int { return a + b }

func keepLast[V any](_, b V) V { return b }

// splice moves the overlay across one buffer edit. Edited and inserted lines
// are marked added; everything else keeps its decoration on the line it was on.
func (o *DiffOverlay) splice(c buffer.Change) {
	if c.Added == c.Removed {
		for line := c.Start; line < c.Start+c.Added && line < len(o.Kinds); line++ {
			if o.Kinds[line] == diff.Context {
				o.Kinds[line] = diff.Added
			}
		}
		return
	}
	if len(o.Kinds) > 0 {
		s := min(c.Start, len(o.Kinds))
		end := min(c.Start+c.Removed, len(o.Kinds))
		added := make([]diff.LineKind, c.Added)
		for i := range added {
			added[i] = diff.Added
		}
		o.Kinds = slices.Replace(o.Kinds, s, end, added...)
	}
	shiftKeys(o.Deleted, c, func(a, b []diff.SideLine) []diff.SideLine { return append(slices.Clip(a), b...) })
	shiftKeys(o.Fillers, c, sumInts)
	shiftKeys(o.Gaps, c, keepLast[int])
	shiftKeys(o.Labels, c, keepLast[string])
	shiftKeys(o.DeletedMatches, c, func(a, b []FindMatch) []FindMatch { return append(slices.Clip(a), b...) })
	if o.DeletedActive[0] >= 0 {
		o.DeletedActive[0] = shiftLine(o.DeletedActive[0], c)
	}
	for i, h := range o.Hidden {
		o.Hidden[i] = diffHiddenRange{start: shiftLine(h.start, c), end: shiftLine(h.end, c)}
	}
	o.visible = nil
}

// syncDiffOverlay replays the buffer edits made since the overlay was last
// brought up to date. When they can no longer be replayed the overlay stays
// where it is; its owner rebuilds it from the next diff.
func (e *EditorPaneWidget) syncDiffOverlay() bool {
	o := e.DiffOverlay
	if o == nil || o.buf != e.Buf || o.ver == e.Buf.Version() {
		return o == nil || !o.stale
	}
	if !e.Buf.ChangesSince(o.ver, func(c buffer.Change) {
		o.splice(c)
		shiftKeys(e.phantoms, c, sumInts)
	}) {
		o.stale = true
	}
	o.ver = e.Buf.Version()
	return !o.stale
}

func (e *EditorPaneWidget) addFillers(anchor, delta int) {
	o := e.DiffOverlay
	if o == nil || delta == 0 {
		return
	}
	if o.Fillers == nil {
		o.Fillers = map[int]int{}
	}
	if o.Fillers[anchor] += delta; o.Fillers[anchor] == 0 {
		delete(o.Fillers, anchor)
	}
	patchable := len(e.phantoms) > 0
	if e.phantoms == nil {
		e.phantoms = map[int]int{}
	}
	if e.phantoms[anchor] += delta; e.phantoms[anchor] == 0 {
		delete(e.phantoms, anchor)
	}
	e.rowMap = nil
	ver := e.Buf.Version()
	for i, l := range e.layoutCache {
		if l == nil {
			continue
		}
		if !patchable || l.phantoms == nil || l.visible != nil || l.key.buf != e.Buf || l.key.version != ver ||
			l.key.overlay != o || l.key.overlayGen != e.overlayGen {
			e.layoutCache[i] = nil
			continue
		}
		if l.starts != nil {
			at := min(anchor, l.n())
			l.restart(at, at+1)
		}
	}
}

// advanceLayouts lets addFillers patch every cached layout rather than drop
// the ones behind the buffer.
func (e *EditorPaneWidget) advanceLayouts() {
	e.syncDiffOverlay()
	ver := e.Buf.Version()
	for i, l := range e.layoutCache {
		if l == nil || l.key.version == ver {
			continue
		}
		key := l.key
		key.lines, key.n, key.version = unsafe.SliceData(e.Buf.Lines), len(e.Buf.Lines), ver
		if l.key.buf != e.Buf || !l.advance(key, e.Buf) {
			e.layoutCache[i] = nil
		}
	}
}
