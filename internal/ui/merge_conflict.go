package ui

import (
	"strings"
	"unsafe"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

// mergeConflict locates one conflict block by the line indexes of its markers.
// base is -1 unless the block carries a "|||||||" base section.
type mergeConflict struct {
	start, separator, end int
	base                  int
}

func conflictAt(lines []string, header int) (mergeConflict, bool) {
	if header < 0 || header >= len(lines) || !conflictHeader(lines[header]) {
		return mergeConflict{}, false
	}
	c := mergeConflict{start: header, separator: -1, end: -1, base: -1}
	for i := header + 1; i < len(lines); i++ {
		marker := strings.TrimSpace(lines[i])
		switch {
		case c.separator < 0 && strings.HasPrefix(marker, "|||||||"):
			c.base = i
		case c.separator < 0 && strings.HasPrefix(marker, "======="):
			c.separator = i
		case c.separator >= 0 && strings.HasPrefix(marker, ">>>>>>>"):
			c.end = i
			return c, true
		}
	}
	return mergeConflict{}, false
}

func conflictHeader(line string) bool {
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case ' ', '\t':
			continue
		}
		return strings.HasPrefix(line[i:], "<<<<<<<")
	}
	return false
}

func scanConflicts(lines []string) map[int]mergeConflict {
	var conflicts map[int]mergeConflict
	for i := 0; i < len(lines); i++ {
		if !conflictHeader(lines[i]) {
			continue
		}
		c, ok := conflictAt(lines, i)
		if !ok {
			continue
		}
		if conflicts == nil {
			conflicts = make(map[int]mergeConflict, 1)
		}
		conflicts[i] = c
		i = c.end
	}
	return conflicts
}

// conflictBlocks returns the buffer's conflicts by header line. A buffer with
// no conflicts is not rescanned on every edit: only the lines inserted since
// the last scan can introduce a marker. A buffer that does hold conflicts is
// rescanned, because any edit can break or move one.
func (e *EditorPaneWidget) conflictBlocks() map[int]mergeConflict {
	if e.ReadOnly || e.Buf == nil {
		return nil
	}
	lines := e.Buf.Lines
	ver, data := e.Buf.Version(), unsafe.SliceData(lines)
	if e.conflictValid && e.conflictVer == ver && e.conflictData == data {
		return e.conflicts
	}
	// A grown or replaced line slice says nothing when the version moved on:
	// the recorded changes already cover every edited line.
	if !e.conflictValid || (e.conflictVer == ver && e.conflictData != data) || len(e.conflicts) > 0 || e.conflictInsertedSince() {
		e.conflicts = scanConflicts(lines)
		e.conflictGen++
	}
	e.conflictValid, e.conflictVer, e.conflictData = true, ver, data
	return e.conflicts
}

// conflictInsertedSince reports whether an edit since the last scan added a
// line that can be a conflict header. Changes are replayed into final line
// numbers, so only the inserted lines are examined.
func (e *EditorPaneWidget) conflictInsertedSince() bool {
	if !e.conflictValid {
		return true
	}
	var changes []buffer.Change
	if !e.Buf.ChangesSince(e.conflictVer, func(c buffer.Change) { changes = append(changes, c) }) {
		return true
	}
	lines := e.Buf.Lines
	after := 0
	for i := len(changes) - 1; i >= 0; i-- {
		c := changes[i]
		start := c.Start + after
		for line := max(start, 0); line < min(start+c.Added, len(lines)); line++ {
			if conflictHeader(lines[line]) {
				return true
			}
		}
		after += c.Added - c.Removed
	}
	return false
}

func (e *EditorPaneWidget) resolveConflict(line, choice int) bool {
	c, ok := conflictAt(e.Buf.Lines, line)
	if !ok || choice < 0 || choice > 2 {
		return false
	}
	currentEnd := c.separator
	if c.base >= 0 {
		currentEnd = c.base
	}
	var replacement []string
	switch choice {
	case 0:
		replacement = append(replacement, e.Buf.Lines[c.start+1:currentEnd]...)
	case 1:
		replacement = append(replacement, e.Buf.Lines[c.separator+1:c.end]...)
	case 2:
		replacement = append(replacement, e.Buf.Lines[c.start+1:currentEnd]...)
		replacement = append(replacement, e.Buf.Lines[c.separator+1:c.end]...)
	}
	// A buffer always holds one line, even when the chosen side is empty.
	if len(replacement) == 0 && c.end == len(e.Buf.Lines)-1 {
		replacement = []string{""}
	}
	e.collapseMulti()
	e.conflictHover = [2]int{-1, -1}
	if e.Selection != nil {
		e.Selection.Clear()
	}
	if e.Undo != nil {
		e.Undo.BreakGroup()
	}
	e.exec(&undo.ReplaceLinesCommand{
		Start: c.start, OldLines: append([]string(nil), e.Buf.Lines[c.start:c.end+1]...), NewLines: replacement,
	})
	e.Cursor.Line = c.start
	e.Cursor.Col = 0
	e.clampCursor()
	e.scrollViewport()
	return true
}

// conflictActionSpan is one button of a conflict's action row, in terminal
// columns from the start of the editor text.
type conflictActionSpan struct {
	start, end int
	action     int
	text       string
}

func conflictActionSpans() [3]conflictActionSpan {
	labels := [3]string{
		i18n.Text("conflict.acceptCurrent"),
		i18n.Text("conflict.acceptIncoming"),
		i18n.Text("conflict.acceptBoth"),
	}
	var spans [3]conflictActionSpan
	x := 0
	for i, label := range labels {
		if i > 0 {
			x += 2
		}
		text := "[" + label + "]"
		spans[i] = conflictActionSpan{start: x, end: x + textwidth.String(text), action: i, text: text}
		x = spans[i].end
	}
	return spans
}

// conflictActionAt returns the button at column, counted from the editor text.
func conflictActionAt(column int) int {
	for _, span := range conflictActionSpans() {
		if column >= span.start && column < span.end {
			return span.action
		}
	}
	return -1
}

func (e *EditorPaneWidget) conflictActionAt(row editorRow, x int) int {
	if !row.conflict {
		return -1
	}
	return conflictActionAt(x)
}

func (e *EditorPaneWidget) renderConflictRow(surface Surface, y, gutterW, editorW, line int) {
	for x := 0; x < gutterW; x++ {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: term.StyleLineNumber})
	}
	end := gutterW + editorW
	for x := gutterW; x < end; x++ {
		surface.SetCell(x, y, term.Cell{Ch: ' '})
	}
	for i, span := range conflictActionSpans() {
		x := gutterW + span.start
		if x >= end {
			break
		}
		style := term.StyleButton
		if e.conflictHover == [2]int{line, i} {
			style = term.StyleButtonFocused
		}
		surface.DrawText(x, y, span.text, end, style)
	}
}
