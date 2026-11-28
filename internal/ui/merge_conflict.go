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
	if !strings.HasPrefix(strings.TrimSpace(lines[header]), "<<<<<<<") {
		return gitConflictAt(lines, header, gitMarkerWidth(lines[header]))
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
		if strings.HasPrefix(line[i:], "<<<<<<<") {
			return true
		}
		return i == 0 && line[i] == '<' && gitMarkerWidth(line) != 0
	}
	return false
}

// conflictMarker reports whether line is any of the conflict marker lines.
func conflictMarker(line string) bool {
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case ' ', '\t':
			continue
		}
		rest := line[i:]
		return (i == 0 && gitMarkerWidth(rest) != 0) || strings.HasPrefix(rest, "<<<<<<<") || strings.HasPrefix(rest, "|||||||") ||
			strings.HasPrefix(rest, "=======") || strings.HasPrefix(rest, ">>>>>>>")
	}
	return false
}

// scanConflicts returns the complete conflict blocks by header line, and
// whether any header line lacked a complete block.
func scanConflicts(lines []string) (map[int]mergeConflict, bool) {
	var conflicts map[int]mergeConflict
	incomplete := false
	for i := 0; i < len(lines); i++ {
		if !conflictHeader(lines[i]) {
			continue
		}
		c, ok := conflictAt(lines, i)
		if !ok {
			incomplete = true
			continue
		}
		if conflicts == nil {
			conflicts = make(map[int]mergeConflict, 1)
		}
		conflicts[i] = c
		i = c.end
	}
	return conflicts, incomplete
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
		e.conflicts, e.conflictIncomplete = scanConflicts(lines)
		e.conflictGen++
	}
	e.conflictValid, e.conflictVer, e.conflictData = true, ver, data
	return e.conflicts
}

// conflictInsertedSince reports whether an edit since the last scan added a
// line that can be a conflict marker. Each change records positions from the
// buffer state just before it was applied, so an inserted range is moved
// through every later splice before its lines are examined. Once a scan found
// an incomplete header, any marker can complete it, so every marker line
// counts; otherwise only a new header can introduce a conflict.
func (e *EditorPaneWidget) conflictInsertedSince() bool {
	if !e.conflictValid {
		return true
	}
	var changes []buffer.Change
	if !e.Buf.ChangesSince(e.conflictVer, func(c buffer.Change) { changes = append(changes, c) }) {
		return true
	}
	lines := e.Buf.Lines
	for i, c := range changes {
		if c.Added <= 0 {
			continue
		}
		start, end, ok := mapInsertedRange(changes[i+1:], c.Start, c.Start+c.Added)
		if !ok {
			return true
		}
		for line := max(start, 0); line < min(end, len(lines)); line++ {
			if conflictHeader(lines[line]) || (e.conflictIncomplete && conflictMarker(lines[line])) {
				return true
			}
		}
	}
	return false
}

// mapInsertedRange moves an inserted line range through later changes, which
// hold positions from the buffer state just before they were applied. It
// returns ok false when a later change only partly overlaps the range: the
// surviving lines cannot be mapped without a scan.
func mapInsertedRange(later []buffer.Change, start, end int) (int, int, bool) {
	for _, c := range later {
		if c.Removed == 0 {
			switch {
			case c.Start <= start:
				start += c.Added
				end += c.Added
			case c.Start < end:
				end += c.Added
			}
			continue
		}
		removedEnd := c.Start + c.Removed
		switch {
		case removedEnd <= start:
			delta := c.Added - c.Removed
			start += delta
			end += delta
		case c.Start >= end:
			// The change lands after the range; it does not move it.
		case c.Start <= start && removedEnd >= end:
			// The whole range was replaced; its lines are gone, and the
			// replacement's own inserted lines carry the check.
			return 0, 0, true
		default:
			return 0, 0, false
		}
	}
	return start, end, true
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
	if len(replacement) == 0 && c.start == 0 && c.end == len(e.Buf.Lines)-1 {
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

// conflictActionLayout is one candidate arrangement of the action buttons.
type conflictActionLayout struct {
	labels  [3]string
	gap     int
	bracket bool
}

func (l conflictActionLayout) spans() [3]conflictActionSpan {
	var spans [3]conflictActionSpan
	x := 0
	for i, label := range l.labels {
		if i > 0 {
			x += l.gap
		}
		text := label
		if l.bracket {
			text = "[" + label + "]"
		}
		spans[i] = conflictActionSpan{start: x, end: x + textwidth.String(text), action: i, text: text}
		x = spans[i].end
	}
	return spans
}

func firstRune(s string) string {
	for _, r := range s {
		return string(r)
	}
	return "?"
}

// conflictActionSpans lays out the three buttons in width terminal columns.
// The full labels are kept whenever they fit; a narrower pane falls back to
// shorter labels and then to bare initials, so every action stays on screen
// and none needs horizontal scrolling to be clicked.
func conflictActionSpans(width int) [3]conflictActionSpan {
	short := [3]string{
		i18n.Text("conflict.current"),
		i18n.Text("conflict.incoming"),
		i18n.Text("conflict.both"),
	}
	initials := [3]string{firstRune(short[0]), firstRune(short[1]), firstRune(short[2])}
	layouts := [4]conflictActionLayout{
		{labels: [3]string{
			i18n.Text("conflict.acceptCurrent"),
			i18n.Text("conflict.acceptIncoming"),
			i18n.Text("conflict.acceptBoth"),
		}, gap: 2, bracket: true},
		{labels: short, gap: 2, bracket: true},
		{labels: initials, gap: 1, bracket: true},
		{labels: initials},
	}
	for _, layout := range layouts {
		spans := layout.spans()
		if spans[2].end <= width {
			return spans
		}
	}
	return layouts[len(layouts)-1].spans()
}

// conflictActionAt returns the button at column, counted from the editor text.
func conflictActionAt(width, column int) int {
	for _, span := range conflictActionSpans(width) {
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
	return conflictActionAt(e.Viewport.Width, x)
}

func (e *EditorPaneWidget) renderConflictRow(surface Surface, y, gutterW, editorW, line int) {
	for x := 0; x < gutterW; x++ {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: term.StyleLineNumber})
	}
	end := gutterW + editorW
	for x := gutterW; x < end; x++ {
		surface.SetCell(x, y, term.Cell{Ch: ' '})
	}
	for i, span := range conflictActionSpans(editorW) {
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
