package image

import (
	"bytes"
	"io"
)

// Layer diffs each frame against the previous one: an unchanged frame writes zero bytes and never re-transmits image data.
type Layer struct {
	sources     map[uint64]*Source
	transmitted map[uint64]bool
	prev        map[uint32]Placement
	prevOrder   []uint32
	cur         map[uint32]Placement
	order       []uint32
	autoID      uint32
	invalidated bool
	forget      []uint64
	// Locks are re-asserted on every Commit: tcell drops them all on resize.
	lock func(r Rect, locked bool)
}

func NewLayer() *Layer {
	return &Layer{}
}

func (l *Layer) SetLockFunc(f func(r Rect, locked bool)) {
	l.lock = f
}

func (l *Layer) ensureInit() {
	if l.sources == nil {
		l.sources = make(map[uint64]*Source)
	}
	if l.transmitted == nil {
		l.transmitted = make(map[uint64]bool)
	}
	if l.prev == nil {
		l.prev = make(map[uint32]Placement)
	}
}

func (l *Layer) AddSource(s *Source) {
	l.ensureInit()
	l.sources[s.ID] = s
}

// Forget frees a source's pixel data, in-process and in the terminal, on the next Commit.
func (l *Layer) Forget(id uint64) {
	l.forget = append(l.forget, id)
}

func (l *Layer) Begin() {
	l.cur = make(map[uint32]Placement)
	l.order = nil
	l.autoID = 0
}

// A zero PlacementID is assigned sequentially within the frame; tracking a placement across frames needs a stable explicit id, since the diff keys on PlacementID.
func (l *Layer) Place(p Placement) {
	if l.cur == nil {
		l.cur = make(map[uint32]Placement)
	}
	if p.PlacementID == 0 {
		l.autoID++
		p.PlacementID = l.autoID
	}
	if _, ok := l.cur[p.PlacementID]; !ok {
		l.order = append(l.order, p.PlacementID)
	}
	l.cur[p.PlacementID] = p
}

func (l *Layer) Invalidate() {
	l.invalidated = true
}

func (l *Layer) Commit(w io.Writer) error {
	l.ensureInit()
	if l.cur == nil {
		l.cur = make(map[uint32]Placement)
	}
	var out bytes.Buffer
	var firstErr error
	fail := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	for _, id := range l.forget {
		if l.transmitted[id] {
			fail(DeleteImage(&out, id))
		}
		delete(l.transmitted, id)
		delete(l.sources, id)
	}
	l.forget = nil
	for _, id := range l.prevOrder {
		if _, ok := l.cur[id]; !ok {
			p := l.prev[id]
			fail(DeletePlacement(&out, p.SourceID, id))
			if l.lock != nil {
				l.lock(p.Cell, false)
			}
		}
	}
	for _, id := range l.order {
		p := l.cur[id]
		old, ok := l.prev[id]
		changed := !ok || !Equal(old, p)
		if l.invalidated || changed {
			if changed && ok {
				fail(DeletePlacement(&out, old.SourceID, id))
				if l.lock != nil {
					l.lock(old.Cell, false)
				}
			}
			if src, known := l.sources[p.SourceID]; known && !l.transmitted[p.SourceID] {
				if err := Transmit(&out, src); err != nil {
					fail(err)
					continue
				}
				l.transmitted[p.SourceID] = true
			}
			fail(Place(&out, p))
		}
	}

	l.prev = l.cur
	l.prevOrder = l.order
	l.cur = nil
	l.order = nil
	l.invalidated = false
	if l.lock != nil {
		for _, id := range l.prevOrder {
			l.lock(l.prev[id].Cell, true)
		}
	}
	// One write per frame: tcell flushes its own frame from another goroutine on resize.
	if out.Len() > 0 {
		if _, err := w.Write(out.Bytes()); err != nil {
			fail(err)
		}
	}
	return firstErr
}

// Releases locks without emitting bytes; Commit still deletes vanished placements on the wire.
func (l *Layer) UnlockAll() {
	if l.lock == nil {
		return
	}
	l.ensureInit()
	for _, id := range l.prevOrder {
		l.lock(l.prev[id].Cell, false)
	}
}

func (l *Layer) Close(w io.Writer) error {
	l.ensureInit()
	var firstErr error
	for _, id := range l.prevOrder {
		if err := DeletePlacement(w, l.prev[id].SourceID, id); err != nil && firstErr == nil {
			firstErr = err
		}
		if l.lock != nil {
			l.lock(l.prev[id].Cell, false)
		}
	}
	for id := range l.transmitted {
		if err := DeleteImage(w, id); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	l.prev = make(map[uint32]Placement)
	l.prevOrder = nil
	l.cur = nil
	l.order = nil
	l.transmitted = make(map[uint64]bool)
	l.sources = make(map[uint64]*Source)
	l.invalidated = false
	return firstErr
}
