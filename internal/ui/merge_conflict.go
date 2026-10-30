package ui

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/core/undo"
)

const conflictActions = "[Current] [Incoming] [Both]"

type mergeConflict struct {
	start, separator, end int
	base                  int
}

func conflictAt(lines []string, header int) (mergeConflict, bool) {
	if header < 0 || header >= len(lines) || !strings.HasPrefix(strings.TrimSpace(lines[header]), "<<<<<<<") {
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

func (e *EditorPaneWidget) conflictActionLine(line int) bool {
	_, ok := conflictAt(e.Buf.Lines, line)
	return ok
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
	e.exec(&undo.ReplaceLinesCommand{
		Start: c.start, OldLines: append([]string(nil), e.Buf.Lines[c.start:c.end+1]...), NewLines: replacement,
	})
	return true
}

func conflictActionAt(column int) int {
	for i, bounds := range [][2]int{{0, 8}, {10, 19}, {21, 26}} {
		if column >= bounds[0] && column <= bounds[1] {
			return i
		}
	}
	return -1
}
