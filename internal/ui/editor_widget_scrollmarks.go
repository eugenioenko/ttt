package ui

import (
	"maps"
	"slices"
	"unsafe"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
)

var gitMarkStyle = [...]term.Style{
	diff.LineAdded:    term.StyleScrollMarkAdded,
	diff.LineModified: term.StyleScrollMarkModified,
	diff.LineDeleted:  term.StyleScrollMarkDeleted,
}

// gitMarkRank decides which change shows where several share a spot.
var gitMarkRank = [...]int{
	diff.LineAdded:    1,
	diff.LineModified: 2,
	diff.LineDeleted:  3,
}

type gitChangeRun struct {
	start, end int
	kind       diff.LineChangeKind
}

// gitChangeIndexKey identifies the change source an index was built from. A
// diff overlay's kinds and deleted blocks move in place with buffer edits, so
// its key carries the buffer version; gutter changes are replaced, never
// edited, so theirs does not and typing keeps the index.
type gitChangeIndexKey struct {
	changes *diff.LineChangeKind
	overlay *DiffOverlay
	kinds   *diff.LineKind
	n       int
	version uint64
}

// gitChangeIndex holds a pane's changed lines as runs and its deleted blocks'
// anchors in order, so placing marks costs the number of changes rather than
// the number of lines.
type gitChangeIndex struct {
	key     gitChangeIndexKey
	runs    []gitChangeRun
	deleted []int
}

func (e *EditorPaneWidget) gitChangeIndex() *gitChangeIndex {
	key := gitChangeIndexKey{changes: unsafe.SliceData(e.LineChanges), n: len(e.LineChanges)}
	if o := e.DiffOverlay; o.isDiff() {
		key = gitChangeIndexKey{overlay: o, kinds: unsafe.SliceData(o.Kinds), n: len(o.Kinds), version: e.Buf.Version()}
	}
	if e.gitChanges.key == key {
		return &e.gitChanges
	}
	idx := gitChangeIndex{key: key}
	kindOf, n := e.lineChangeSource()
	for line := 0; line < n; {
		kind := kindOf(line)
		if kind == diff.LineUnchanged {
			line++
			continue
		}
		start := line
		for line++; line < n && kindOf(line) == kind; line++ {
		}
		idx.runs = append(idx.runs, gitChangeRun{start: start, end: line, kind: kind})
	}
	if o := e.DiffOverlay; o != nil {
		for anchor, block := range o.Deleted {
			if len(block) > 0 {
				idx.deleted = append(idx.deleted, anchor)
			}
		}
		slices.Sort(idx.deleted)
	}
	e.gitChanges = idx
	return &e.gitChanges
}

// gitScrollMarks places marks by the row layout the scrollbar's TotalItems
// comes from, or they drift from the thumb. A pane showing a diff marks what
// its overlay draws, deleted phantom rows included; any other pane marks its
// git gutter changes.
func (e *EditorPaneWidget) gitScrollMarks(l *rowLayout) []ScrollMark {
	idx := e.gitChangeIndex()
	if len(idx.runs) == 0 && len(idx.deleted) == 0 {
		return nil
	}
	// A new file is one change from top to bottom, which marks nothing the
	// gutter does not already show.
	if len(idx.runs) == 1 && len(idx.deleted) == 0 && idx.runs[0].start == 0 && idx.runs[0].end == idx.key.n {
		return nil
	}
	// A run is marked on its text rows only, so it is cut where phantom rows
	// sit between its lines; fillers move without a buffer edit, so the cuts
	// are taken from the layout on every frame.
	var cuts []int
	if l.phantoms != nil {
		cuts = slices.Sorted(maps.Keys(l.phantoms))
	}
	n, d, c := l.n(), 0, 0
	// Hidden lines before the first visible one have no header to count toward.
	top := 0
	if l.visible != nil {
		top = n
		if len(l.visible) > 0 {
			top = l.visible[0]
		}
	}
	var marks []ScrollMark
	for _, r := range idx.runs {
		end := min(r.end, n)
		for start := max(r.start, top); start < end; {
			for c < len(cuts) && cuts[c] <= start {
				c++
			}
			stop := end
			if c < len(cuts) && cuts[c] < end {
				stop = cuts[c]
			}
			// Lines hidden in a collapsed fold count toward its header line,
			// so changes inside a fold still show.
			first, last := l.visibleLine(start), l.visibleLine(stop-1)
			for ; d < len(idx.deleted) && idx.deleted[d] <= first; d++ {
				marks = appendDeletedScrollMark(marks, l, idx.deleted[d])
			}
			item := l.startRow(first) + l.phantomRows(first)
			marks = appendGitScrollMark(marks, item, l.startRow(last)+l.phantomRows(last)+l.textRows(last)-item, r.kind)
			start = stop
		}
	}
	for ; d < len(idx.deleted); d++ {
		marks = appendDeletedScrollMark(marks, l, idx.deleted[d])
	}
	return marks
}

func appendDeletedScrollMark(marks []ScrollMark, l *rowLayout, line int) []ScrollMark {
	if line > l.n() {
		return marks
	}
	if _, visible := slices.BinarySearch(l.visible, line); l.visible != nil && line < l.n() && !visible {
		return marks
	}
	rows := 0
	for k := range l.deleted[line] {
		rows += len(l.phantomSegments(line, k))
	}
	return appendGitScrollMark(marks, l.startRow(line), rows, diff.LineDeleted)
}

func (e *EditorPaneWidget) scrollMarksEnabled() bool {
	if e.DiffOverlay.isDiff() {
		return e.DiffOverlay.ScrollMarks
	}
	return e.ScrollbarGitMarks
}

// lineChangeSource returns each buffer line's change kind and how many lines
// it covers, which lags behind the buffer until the next git diff.
func (e *EditorPaneWidget) lineChangeSource() (func(int) diff.LineChangeKind, int) {
	if o := e.DiffOverlay; o.isDiff() {
		return func(line int) diff.LineChangeKind {
			switch o.Kinds[line] {
			case diff.Added:
				return diff.LineAdded
			case diff.Deleted:
				return diff.LineDeleted
			}
			return diff.LineUnchanged
		}, len(o.Kinds)
	}
	changes := e.LineChanges
	return func(line int) diff.LineChangeKind { return changes[line] }, len(changes)
}

// appendGitScrollMark extends the previous mark when the run continues it.
// Runs folded into the same header line overlap on its rows, which keep the
// higher ranked change.
func appendGitScrollMark(marks []ScrollMark, item, count int, kind diff.LineChangeKind) []ScrollMark {
	style, rank := gitMarkStyle[kind], gitMarkRank[kind]
	if len(marks) > 0 {
		last := &marks[len(marks)-1]
		if overlap := last.Item + last.Count - item; overlap > 0 {
			if rank <= last.Rank {
				item, count = item+overlap, count-overlap
				if count <= 0 {
					return marks
				}
			} else if last.Count = item - last.Item; last.Count <= 0 {
				return appendGitScrollMark(marks[:len(marks)-1], item, count, kind)
			}
		}
		if last.Style == style && last.Item+last.Count == item {
			last.Count += count
			return marks
		}
	}
	return append(marks, ScrollMark{Item: item, Count: count, Style: style, Rank: rank})
}
