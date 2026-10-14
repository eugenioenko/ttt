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
// comes from, or they drift from the thumb.
func (e *EditorPaneWidget) gitScrollMarks(l *rowLayout) []ScrollMark {
	changes := e.LineChanges
	if len(changes) == 0 {
		return nil
	}
	var marks []ScrollMark
	count := l.visibleCount()
	for i := 0; i < count; i++ {
		line := l.visLine(i)
		// Lines hidden in a collapsed fold count toward its header line, so
		// changes inside a fold still show. Changes lag behind the buffer
		// until the next git diff.
		end := l.n()
		if i+1 < count {
			end = l.visLine(i + 1)
		}
		kind := diff.LineUnchanged
		for hidden := line; hidden < min(end, len(changes)); hidden++ {
			if gitMarkRank[changes[hidden]] > gitMarkRank[kind] {
				kind = changes[hidden]
			}
		}
		if kind != diff.LineUnchanged {
			marks = appendGitScrollMark(marks, l.startRow(line)+l.phantomRows(line), l.textRows(line), kind)
		}
	}
	return marks
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
