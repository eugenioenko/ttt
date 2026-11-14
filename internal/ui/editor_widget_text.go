package ui

import (
	"strings"
	"unicode"

	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

func wordBoundaryLeft(runes []rune, col int) int {
	pos := col - 1
	if pos >= len(runes) {
		pos = len(runes) - 1
	}
	if pos < 0 {
		return 0
	}
	if unicode.IsSpace(runes[pos]) {
		for pos > 0 && unicode.IsSpace(runes[pos-1]) {
			pos--
		}
	} else if isEditorIdentRune(runes[pos]) {
		for pos > 0 && isEditorIdentRune(runes[pos-1]) {
			pos--
		}
	} else {
		for pos > 0 && !isEditorIdentRune(runes[pos-1]) && !unicode.IsSpace(runes[pos-1]) {
			pos--
		}
	}
	return pos
}

func wordBoundaryRight(runes []rune, col int) int {
	pos := col
	if pos >= len(runes) {
		return len(runes)
	}
	if unicode.IsSpace(runes[pos]) {
		for pos < len(runes) && unicode.IsSpace(runes[pos]) {
			pos++
		}
	} else if isEditorIdentRune(runes[pos]) {
		for pos < len(runes) && isEditorIdentRune(runes[pos]) {
			pos++
		}
	} else {
		for pos < len(runes) && !isEditorIdentRune(runes[pos]) && !unicode.IsSpace(runes[pos]) {
			pos++
		}
	}
	return pos
}

// MoveToFileEdge puts the cursor at the start of the first line or the end
// of the last, extending the selection when shift is held. A diff gap hiding
// that line is revealed by the diff on its next render, not skipped.
func (e *EditorPaneWidget) MoveToFileEdge(end, shift bool) {
	if e.Buf == nil || len(e.Buf.Lines) == 0 {
		return
	}
	e.collapseMulti()
	e.ResetCursorGoal()
	e.startOrExtendSelection(shift)
	if end {
		e.Cursor.Line = len(e.Buf.Lines) - 1
		e.skipFoldUp()
		e.Cursor.Col = len([]rune(e.Buf.Lines[e.Cursor.Line]))
	} else {
		e.Cursor.Line, e.Cursor.Col = 0, 0
		e.skipFoldDown()
	}
	e.revealGapRow = e.DiffOverlay != nil
	e.scrollViewport()
}

// With multiple cursors shift is ignored and their selections are cleared,
// matching the arrow keys.
func (e *EditorPaneWidget) MoveToLineEdge(end, shift bool) {
	if e.Buf == nil || len(e.Buf.Lines) == 0 {
		return
	}
	e.ResetCursorGoal()
	e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line)
	if e.isMultiActive() {
		e.multiMoveAll(func(cs *multicursor.CursorState) {
			runes := []rune(e.Buf.Lines[cs.Line])
			if end {
				cs.Col = len(runes)
				return
			}
			firstNonSpace := 0
			for firstNonSpace < len(runes) && (runes[firstNonSpace] == ' ' || runes[firstNonSpace] == '\t') {
				firstNonSpace++
			}
			if cs.Col == firstNonSpace {
				cs.Col = 0
			} else {
				cs.Col = firstNonSpace
			}
		})
	} else {
		e.startOrExtendSelection(shift)
		if end {
			e.Cursor.Col = len([]rune(e.Buf.Lines[e.Cursor.Line]))
		} else {
			e.SmartHome()
		}
	}
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) MoveWordLeft(shift bool) {
	e.ResetCursorGoal()
	if e.isMultiActive() && !shift {
		e.multiMoveAll(func(cs *multicursor.CursorState) {
			if cs.Col == 0 {
				if cs.Line > 0 {
					cs.Line--
					cs.Col = len([]rune(e.Buf.Lines[cs.Line]))
				}
			} else {
				runes := []rune(e.Buf.Lines[cs.Line])
				cs.Col = wordBoundaryLeft(runes, cs.Col)
			}
		})
		e.scrollViewport()
		return
	}
	e.startOrExtendSelection(shift)
	e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line)
	if e.Cursor.Col == 0 {
		if e.Cursor.Line > 0 {
			e.Cursor.Line--
			e.Cursor.Col = len([]rune(e.Buf.Lines[e.Cursor.Line]))
		}
	} else {
		runes := []rune(e.Buf.Lines[e.Cursor.Line])
		e.Cursor.Col = wordBoundaryLeft(runes, e.Cursor.Col)
	}
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) MoveWordRight(shift bool) {
	e.ResetCursorGoal()
	if e.isMultiActive() && !shift {
		e.multiMoveAll(func(cs *multicursor.CursorState) {
			runes := []rune(e.Buf.Lines[cs.Line])
			if cs.Col >= len(runes) {
				if cs.Line < len(e.Buf.Lines)-1 {
					cs.Line++
					cs.Col = 0
				}
			} else {
				cs.Col = wordBoundaryRight(runes, cs.Col)
			}
		})
		e.scrollViewport()
		return
	}
	e.startOrExtendSelection(shift)
	e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line)
	runes := []rune(e.Buf.Lines[e.Cursor.Line])
	if e.Cursor.Col >= len(runes) {
		if e.Cursor.Line < len(e.Buf.Lines)-1 {
			e.Cursor.Line++
			e.Cursor.Col = 0
		}
	} else {
		e.Cursor.Col = wordBoundaryRight(runes, e.Cursor.Col)
	}
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) DeleteWordLeft() {
	if e.Selection != nil && e.Selection.Active {
		e.deleteSelection()
		e.clampCursor()
		e.scrollViewport()
		return
	}
	if e.Cursor.Col == 0 {
		return
	}
	e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line)
	runes := []rune(e.Buf.Lines[e.Cursor.Line])
	start := wordBoundaryLeft(runes, e.Cursor.Col)
	e.exec(&undo.DeleteSelectionCommand{
		StartLine: e.Cursor.Line, StartCol: start,
		EndLine: e.Cursor.Line, EndCol: e.Cursor.Col,
	})
	e.Cursor.Col = start
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) DeleteWordRight() {
	if e.Selection != nil && e.Selection.Active {
		e.deleteSelection()
		e.clampCursor()
		e.scrollViewport()
		return
	}
	e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line)
	runes := []rune(e.Buf.Lines[e.Cursor.Line])
	if e.Cursor.Col >= len(runes) {
		return
	}
	end := wordBoundaryRight(runes, e.Cursor.Col)
	e.exec(&undo.DeleteSelectionCommand{
		StartLine: e.Cursor.Line, StartCol: e.Cursor.Col,
		EndLine: e.Cursor.Line, EndCol: end,
	})
	e.clampCursor()
	e.scrollViewport()
}

// transformSelection replaces the selected text with the result of applying fn.
// It preserves the selection after transformation.
func (e *EditorPaneWidget) transformSelection(fn func(string) string) {
	if e.isMultiActive() {
		e.multiTransformSelections(fn)
		return
	}
	if e.Selection == nil || !e.Selection.Active {
		return
	}
	start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
	cmd, start, newEnd := e.transformRange(start, end, fn)
	if cmd == nil {
		return
	}

	if e.Undo != nil {
		e.Undo.BreakGroup()
	}
	e.exec(cmd)

	e.Selection.Start(start.Line, start.Col)
	e.Cursor.Line = newEnd.Line
	e.Cursor.Col = newEnd.Col
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) multiTransformSelections(fn func(string) string) {
	e.normalizeClipboardCursors()
	prevLines := len(e.Buf.Lines)
	var cmds []undo.EditCommand
	for i := len(e.Multi.Cursors) - 1; i >= 0; i-- {
		cs := &e.Multi.Cursors[i]
		if !cs.Sel.Active {
			continue
		}
		start, end := cs.Sel.Range(cs.Line, cs.Col)
		cmd, start, newEnd := e.transformRange(start, end, fn)
		if cmd == nil {
			continue
		}
		e.applyAutoClosingEdit(cmd)
		cmds = append(cmds, cmd)
		cs.Sel.Start(start.Line, start.Col)
		cs.Line, cs.Col = newEnd.Line, newEnd.Col

		// Later selections start at or after end, so only positions on
		// end.Line shift by columns.
		shift := func(line, col *int) {
			if *line == end.Line {
				*col += newEnd.Col - end.Col
			}
			if *line >= end.Line {
				*line += newEnd.Line - end.Line
			}
		}
		for j := i + 1; j < len(e.Multi.Cursors); j++ {
			later := &e.Multi.Cursors[j]
			shift(&later.Line, &later.Col)
			shift(&later.Sel.Anchor.Line, &later.Sel.Anchor.Col)
		}
	}
	if len(cmds) > 0 {
		if e.Undo != nil {
			e.Undo.BreakGroup()
			e.Undo.Push(&undo.BatchCommand{Commands: cmds})
		}
		e.bufferDirty = true
		if len(e.Buf.Lines) != prevLines {
			e.refreshFolds()
		}
	}
	e.syncFromMulti()
	e.scrollViewport()
}

// transformRange returns a nil command when fn leaves the text unchanged.
// The returned start is clamped to the line; newEnd is where the replaced text ends.
func (e *EditorPaneWidget) transformRange(start, end selection.Position, fn func(string) string) (*undo.ReplaceLinesCommand, selection.Position, selection.Position) {
	if start.Line < 0 || end.Line >= len(e.Buf.Lines) {
		return nil, start, end
	}
	sel := selection.Selection{Active: true, Anchor: start}
	text := sel.Text(e.Buf.Lines, end.Line, end.Col)
	if text == "" {
		return nil, start, end
	}
	transformed := fn(text)
	if transformed == text {
		return nil, start, end
	}

	oldLines := make([]string, end.Line-start.Line+1)
	copy(oldLines, e.Buf.Lines[start.Line:end.Line+1])

	tLines := strings.Split(transformed, "\n")
	firstOld := []rune(oldLines[0])
	startCol := min(start.Col, len(firstOld))
	prefix := string(firstOld[:startCol])
	lastOld := []rune(oldLines[len(oldLines)-1])
	suffix := ""
	if end.Col < len(lastOld) {
		suffix = string(lastOld[end.Col:])
	}

	newLines := make([]string, len(tLines))
	for i, tl := range tLines {
		switch {
		case len(tLines) == 1:
			newLines[i] = prefix + tl + suffix
		case i == 0:
			newLines[i] = prefix + tl
		case i == len(tLines)-1:
			newLines[i] = tl + suffix
		default:
			newLines[i] = tl
		}
	}

	newEnd := selection.Position{Line: start.Line + len(tLines) - 1}
	if len(tLines) == 1 {
		newEnd.Col = startCol + len([]rune(tLines[0]))
	} else {
		newEnd.Col = len([]rune(tLines[len(tLines)-1]))
	}
	cmd := &undo.ReplaceLinesCommand{
		Start:    start.Line,
		OldLines: oldLines,
		NewLines: newLines,
	}
	return cmd, selection.Position{Line: start.Line, Col: startCol}, newEnd
}

func (e *EditorPaneWidget) UpperCase() {
	e.transformSelection(strings.ToUpper)
}

func (e *EditorPaneWidget) LowerCase() {
	e.transformSelection(strings.ToLower)
}

func (e *EditorPaneWidget) TitleCase() {
	e.transformSelection(func(s string) string {
		runes := []rune(s)
		inWord := false
		for i, r := range runes {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				if !inWord {
					runes[i] = unicode.ToUpper(r)
					inWord = true
				} else {
					runes[i] = unicode.ToLower(r)
				}
			} else if !(inWord && r == '\'') {
				inWord = false
			}
		}
		return string(runes)
	})
}
