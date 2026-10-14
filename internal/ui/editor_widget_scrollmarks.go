package ui

import (
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
)

var gitMarkStyle = map[diff.LineChangeKind]term.Style{
	diff.LineAdded:    term.StyleScrollMarkAdded,
	diff.LineModified: term.StyleScrollMarkModified,
	diff.LineDeleted:  term.StyleScrollMarkDeleted,
}

// gitMarkRank decides which change shows where several share a spot.
var gitMarkRank = map[diff.LineChangeKind]int{
	diff.LineAdded:    1,
	diff.LineModified: 2,
	diff.LineDeleted:  3,
}

// gitScrollMarks places marks by the row layout the scrollbar's TotalItems
// comes from, or they drift from the thumb. A pane showing a diff marks what
// its overlay draws, deleted phantom rows included; any other pane marks its
// git gutter changes.
func (e *EditorPaneWidget) gitScrollMarks(l *rowLayout) []ScrollMark {
	kindOf, n := e.lineChangeSource()
	var deleted map[int][]diff.SideLine
	if e.DiffOverlay != nil {
		deleted = e.DiffOverlay.Deleted
	}
	if n == 0 && len(deleted) == 0 {
		return nil
	}
	var marks []ScrollMark
	count := l.visibleCount()
	for i := 0; i <= count; i++ {
		line := l.visLine(i)
		if i == count {
			line = l.n()
		}
		row := l.startRow(line)
		if block := deleted[line]; len(block) > 0 {
			rows := 0
			for k := range block {
				rows += len(l.phantomSegments(line, k))
			}
			marks = appendGitScrollMark(marks, row, rows, diff.LineDeleted)
		}
		if i == count {
			break
		}
		// Lines hidden in a collapsed fold count toward its header line, so
		// changes inside a fold still show.
		end := l.n()
		if i+1 < count {
			end = l.visLine(i + 1)
		}
		kind := diff.LineUnchanged
		for hidden := line; hidden < min(end, n); hidden++ {
			if k := kindOf(hidden); gitMarkRank[k] > gitMarkRank[kind] {
				kind = k
			}
		}
		if kind != diff.LineUnchanged {
			marks = appendGitScrollMark(marks, row+l.phantomRows(line), l.textRows(line), kind)
		}
	}
	return marks
}

func (e *EditorPaneWidget) scrollMarksEnabled() bool {
	if e.DiffOverlay != nil {
		return e.DiffOverlay.ScrollMarks
	}
	return e.ScrollbarGitMarks
}

// lineChangeSource returns each buffer line's change kind and how many lines
// it covers, which lags behind the buffer until the next git diff.
func (e *EditorPaneWidget) lineChangeSource() (func(int) diff.LineChangeKind, int) {
	if o := e.DiffOverlay; o != nil {
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
func appendGitScrollMark(marks []ScrollMark, item, count int, kind diff.LineChangeKind) []ScrollMark {
	style := gitMarkStyle[kind]
	if len(marks) > 0 {
		last := &marks[len(marks)-1]
		if last.Style == style && last.Item+last.Count == item {
			last.Count += count
			return marks
		}
	}
	return append(marks, ScrollMark{Item: item, Count: count, Style: style, Rank: gitMarkRank[kind]})
}
