package multicursor

import (
	"cmp"
	"sort"

	"github.com/eugenioenko/ttt/internal/core/selection"
)

type CursorState struct {
	Line, Col int
	Sel       selection.Selection
}

type MultiCursor struct {
	Cursors []CursorState
	Primary int
	history []CursorState
}

func New(line, col int) *MultiCursor {
	return &MultiCursor{
		Cursors: []CursorState{{Line: line, Col: col}},
		Primary: 0,
	}
}

func (mc *MultiCursor) IsMulti() bool {
	return len(mc.Cursors) > 1
}

func (mc *MultiCursor) PrimaryCursor() CursorState {
	if mc.Primary >= 0 && mc.Primary < len(mc.Cursors) {
		return mc.Cursors[mc.Primary]
	}
	return CursorState{}
}

func (mc *MultiCursor) Add(line, col int) {
	for _, c := range mc.Cursors {
		if c.Line == line && c.Col == col {
			return
		}
	}
	cs := CursorState{Line: line, Col: col}
	mc.history = append(mc.history, cs)
	mc.Cursors = append(mc.Cursors, cs)
	mc.Sort()
}

func (mc *MultiCursor) AddWithSelection(line, col int, sel selection.Selection) {
	for _, c := range mc.Cursors {
		if c.Line == line && c.Col == col {
			return
		}
	}
	cs := CursorState{Line: line, Col: col, Sel: sel}
	mc.history = append(mc.history, cs)
	mc.Cursors = append(mc.Cursors, cs)
	mc.Sort()
}

func (mc *MultiCursor) RemoveLast() (CursorState, bool) {
	if len(mc.history) == 0 || len(mc.Cursors) <= 1 {
		return CursorState{}, false
	}
	last := mc.history[len(mc.history)-1]
	mc.history = mc.history[:len(mc.history)-1]
	for i, c := range mc.Cursors {
		if c.Line == last.Line && c.Col == last.Col {
			mc.Cursors = append(mc.Cursors[:i], mc.Cursors[i+1:]...)
			if mc.Primary >= len(mc.Cursors) {
				mc.Primary = len(mc.Cursors) - 1
			}
			return last, true
		}
	}
	return last, false
}

func (mc *MultiCursor) CollapseToSingle() {
	if len(mc.Cursors) == 0 {
		return
	}
	primary := mc.PrimaryCursor()
	primary.Sel.Clear()
	mc.Cursors = []CursorState{primary}
	mc.Primary = 0
	mc.history = nil
}

func (mc *MultiCursor) Sort() {
	if mc.Primary < 0 || mc.Primary >= len(mc.Cursors) {
		mc.Primary = 0
		return
	}
	p := mc.Cursors[mc.Primary]
	sort.SliceStable(mc.Cursors, func(i, j int) bool {
		a, b := mc.Cursors[i], mc.Cursors[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Col < b.Col
	})
	for i, c := range mc.Cursors {
		if c.Line == p.Line && c.Col == p.Col {
			mc.Primary = i
			break
		}
	}
}

func (mc *MultiCursor) NormalizePrimary() {
	c := &mc.Cursors[mc.Primary]
	if !c.Sel.Active {
		return
	}
	start, end := c.Sel.Range(c.Line, c.Col)
	c.Sel.Anchor = start
	c.Line = end.Line
	c.Col = end.Col
}

func (mc *MultiCursor) MergeSelections() {
	type cursorRange struct {
		start, end        selection.Position
		primary, reversed bool
	}
	ranges := make([]cursorRange, 0, len(mc.Cursors))
	for i, c := range mc.Cursors {
		start, end := selection.Position{Line: c.Line, Col: c.Col}, selection.Position{Line: c.Line, Col: c.Col}
		if c.Sel.Active {
			start, end = c.Sel.Range(c.Line, c.Col)
		}
		ranges = append(ranges, cursorRange{
			start: start, end: end, primary: i == mc.Primary,
			reversed: start != end && c.Line == start.Line && c.Col == start.Col,
		})
	}
	sort.SliceStable(ranges, func(i, j int) bool {
		if order := comparePosition(ranges[i].start, ranges[j].start); order != 0 {
			return order < 0
		}
		return comparePosition(ranges[i].end, ranges[j].end) > 0
	})
	merged := ranges[:0]
	for _, current := range ranges {
		if len(merged) > 0 {
			last := &merged[len(merged)-1]
			overlaps := comparePosition(current.start, last.end) < 0 ||
				(current.start == last.end && (current.start == current.end || last.start == last.end))
			if overlaps {
				if comparePosition(current.end, last.end) > 0 {
					last.end = current.end
				}
				if current.primary {
					last.primary, last.reversed = true, current.reversed
				}
				continue
			}
		}
		merged = append(merged, current)
	}
	if len(merged) < len(mc.Cursors) {
		mc.history = nil
	}
	mc.Cursors = mc.Cursors[:0]
	for i, r := range merged {
		c := CursorState{Line: r.end.Line, Col: r.end.Col}
		if r.start != r.end {
			c.Sel = selection.Selection{Active: true, Anchor: r.start}
			if r.reversed {
				c.Line, c.Col, c.Sel.Anchor = r.start.Line, r.start.Col, r.end
			}
		}
		if r.primary {
			mc.Primary = i
		}
		mc.Cursors = append(mc.Cursors, c)
	}
}

func comparePosition(a, b selection.Position) int {
	if order := cmp.Compare(a.Line, b.Line); order != 0 {
		return order
	}
	return cmp.Compare(a.Col, b.Col)
}

func (mc *MultiCursor) Deduplicate() {
	if len(mc.Cursors) <= 1 {
		return
	}
	mc.Sort()
	p := mc.Cursors[mc.Primary]
	unique := mc.Cursors[:1]
	for i := 1; i < len(mc.Cursors); i++ {
		prev := unique[len(unique)-1]
		if mc.Cursors[i].Line != prev.Line || mc.Cursors[i].Col != prev.Col {
			unique = append(unique, mc.Cursors[i])
		}
	}
	mc.Cursors = unique
	mc.Primary = 0
	for i, c := range mc.Cursors {
		if c.Line == p.Line && c.Col == p.Col {
			mc.Primary = i
			break
		}
	}
}
