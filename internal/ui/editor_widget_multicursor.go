package ui

import (
	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

func (e *EditorPaneWidget) ensureMulti() {
	if e.Multi == nil {
		e.Multi = multicursor.New(e.Cursor.Line, e.Cursor.Col)
		if e.Selection != nil && e.Selection.Active {
			e.Multi.Cursors[0].Sel = *e.Selection
		}
	}
}

func (e *EditorPaneWidget) syncFromMulti() {
	if e.Multi == nil || len(e.Multi.Cursors) == 0 {
		return
	}
	p := e.Multi.PrimaryCursor()
	e.Cursor.Line = p.Line
	e.Cursor.Col = p.Col
	if e.Selection != nil {
		*e.Selection = p.Sel
	}
}

func (e *EditorPaneWidget) syncToMulti() {
	if e.Multi == nil || len(e.Multi.Cursors) == 0 {
		return
	}
	c := &e.Multi.Cursors[e.Multi.Primary]
	c.Line = e.Cursor.Line
	c.Col = e.Cursor.Col
	if e.Selection != nil {
		c.Sel = *e.Selection
	}
}

func (e *EditorPaneWidget) isMultiActive() bool {
	return e.Multi != nil && e.Multi.IsMulti()
}

func (e *EditorPaneWidget) collapseMulti() {
	if e.Multi == nil {
		return
	}
	e.Multi.CollapseToSingle()
	e.syncFromMulti()
	e.Multi = nil
	e.multiSearchWord = ""
}

func (e *EditorPaneWidget) multiExecRune(r rune) {
	e.syncToMulti()
	var cmds []undo.EditCommand
	for i := len(e.Multi.Cursors) - 1; i >= 0; i-- {
		cs := &e.Multi.Cursors[i]
		if cs.Sel.Active {
			start, end := cs.Sel.Range(cs.Line, cs.Col)
			delCmd := &undo.DeleteSelectionCommand{
				StartLine: start.Line, StartCol: start.Col,
				EndLine: end.Line, EndCol: end.Col,
			}
			delCmd.Apply(e.Buf)
			cmds = append(cmds, delCmd)
			cs.Line = start.Line
			cs.Col = start.Col
			cs.Sel.Clear()
			e.adjustLaterCursors(i, start, end)
		}
		insertCol := cs.Col
		cmd := &undo.InsertRuneCommand{Line: cs.Line, Col: insertCol, Rune: r}
		cmd.Apply(e.Buf)
		cmds = append(cmds, cmd)
		cs.Col++
		e.shiftLaterCursors(i, cs.Line, insertCol, 1)
	}
	if e.Undo != nil {
		e.Undo.Push(&undo.BatchCommand{Commands: cmds})
	}
	e.markBufferDirty()
	e.syncFromMulti()
}

// multiExecTab inserts one indent unit at every cursor, mirroring how a typed
// character is applied across cursors.
func (e *EditorPaneWidget) multiExecTab() {
	e.syncToMulti()
	indent := e.indentUnit()
	n := len([]rune(indent))
	var cmds []undo.EditCommand
	for i := len(e.Multi.Cursors) - 1; i >= 0; i-- {
		cs := &e.Multi.Cursors[i]
		if cs.Sel.Active {
			start, end := cs.Sel.Range(cs.Line, cs.Col)
			delCmd := &undo.DeleteSelectionCommand{
				StartLine: start.Line, StartCol: start.Col,
				EndLine: end.Line, EndCol: end.Col,
			}
			delCmd.Apply(e.Buf)
			cmds = append(cmds, delCmd)
			cs.Line = start.Line
			cs.Col = start.Col
			cs.Sel.Clear()
			e.adjustLaterCursors(i, start, end)
		}
		insertCol := cs.Col
		cmd := &undo.InsertStringCommand{Line: cs.Line, Col: insertCol, Text: indent}
		cmd.Apply(e.Buf)
		cmds = append(cmds, cmd)
		cs.Col += n
		e.shiftLaterCursors(i, cs.Line, insertCol, n)
	}
	if e.Undo != nil {
		e.Undo.Push(&undo.BatchCommand{Commands: cmds})
	}
	e.markBufferDirty()
	e.syncFromMulti()
}

func (e *EditorPaneWidget) multiExecBackspace() {
	e.syncToMulti()
	var cmds []undo.EditCommand
	for i := len(e.Multi.Cursors) - 1; i >= 0; i-- {
		cs := &e.Multi.Cursors[i]
		if cs.Sel.Active {
			start, end := cs.Sel.Range(cs.Line, cs.Col)
			delCmd := &undo.DeleteSelectionCommand{
				StartLine: start.Line, StartCol: start.Col,
				EndLine: end.Line, EndCol: end.Col,
			}
			delCmd.Apply(e.Buf)
			cmds = append(cmds, delCmd)
			cs.Line = start.Line
			cs.Col = start.Col
			cs.Sel.Clear()
			e.adjustLaterCursors(i, start, end)
			continue
		}
		cs.Line = e.Buf.ClampLine(cs.Line)
		if cs.Col > 0 {
			cmd := &undo.DeleteRuneCommand{Line: cs.Line, Col: cs.Col - 1}
			cmd.Apply(e.Buf)
			cmds = append(cmds, cmd)
			cs.Col--
			e.shiftLaterCursors(i, cs.Line, cs.Col, -1)
		} else if cs.Line > 0 {
			prevLen := len([]rune(e.Buf.Lines[cs.Line-1]))
			cmd := &undo.JoinLineCommand{Line: cs.Line}
			cmd.Apply(e.Buf)
			cmds = append(cmds, cmd)
			e.shiftLaterLines(i, cs.Line, -1)
			cs.Line--
			cs.Col = prevLen
		}
	}
	if len(cmds) > 0 {
		if e.Undo != nil {
			e.Undo.Push(&undo.BatchCommand{Commands: cmds})
		}
		e.markBufferDirty()
	}
	e.Multi.Deduplicate()
	e.syncFromMulti()
}

func (e *EditorPaneWidget) multiExecDelete() {
	e.syncToMulti()
	var cmds []undo.EditCommand
	for i := len(e.Multi.Cursors) - 1; i >= 0; i-- {
		cs := &e.Multi.Cursors[i]
		if cs.Sel.Active {
			start, end := cs.Sel.Range(cs.Line, cs.Col)
			delCmd := &undo.DeleteSelectionCommand{
				StartLine: start.Line, StartCol: start.Col,
				EndLine: end.Line, EndCol: end.Col,
			}
			delCmd.Apply(e.Buf)
			cmds = append(cmds, delCmd)
			cs.Line = start.Line
			cs.Col = start.Col
			cs.Sel.Clear()
			e.adjustLaterCursors(i, start, end)
			continue
		}
		cs.Line = e.Buf.ClampLine(cs.Line)
		lineLen := len([]rune(e.Buf.Lines[cs.Line]))
		if cs.Col < lineLen {
			cmd := &undo.DeleteRuneCommand{Line: cs.Line, Col: cs.Col}
			cmd.Apply(e.Buf)
			cmds = append(cmds, cmd)
			e.shiftLaterCursors(i, cs.Line, cs.Col, -1)
		} else if cs.Line < len(e.Buf.Lines)-1 {
			cmd := &undo.JoinLineCommand{Line: cs.Line + 1}
			cmd.Apply(e.Buf)
			cmds = append(cmds, cmd)
			e.shiftLaterLines(i, cs.Line+1, -1)
		}
	}
	if len(cmds) > 0 {
		if e.Undo != nil {
			e.Undo.Push(&undo.BatchCommand{Commands: cmds})
		}
		e.markBufferDirty()
	}
	e.Multi.Deduplicate()
	e.syncFromMulti()
}

func (e *EditorPaneWidget) multiExecEnter() {
	e.syncToMulti()
	var cmds []undo.EditCommand
	for i := len(e.Multi.Cursors) - 1; i >= 0; i-- {
		cs := &e.Multi.Cursors[i]
		if cs.Sel.Active {
			start, end := cs.Sel.Range(cs.Line, cs.Col)
			delCmd := &undo.DeleteSelectionCommand{
				StartLine: start.Line, StartCol: start.Col,
				EndLine: end.Line, EndCol: end.Col,
			}
			delCmd.Apply(e.Buf)
			cmds = append(cmds, delCmd)
			cs.Line = start.Line
			cs.Col = start.Col
			cs.Sel.Clear()
			e.adjustLaterCursors(i, start, end)
		}
		cs.Line = e.Buf.ClampLine(cs.Line)
		indent := ""
		if e.AutoIndent {
			indent = leadingWhitespace(e.Buf.Lines[cs.Line])
		}
		cmd := &undo.SplitLineCommand{Line: cs.Line, Col: cs.Col}
		cmd.Apply(e.Buf)
		cmds = append(cmds, cmd)
		e.shiftLaterLines(i, cs.Line, 1)
		cs.Line++
		cs.Col = 0
		if len(indent) > 0 {
			indCmd := &undo.InsertStringCommand{Line: cs.Line, Col: 0, Text: indent}
			indCmd.Apply(e.Buf)
			cmds = append(cmds, indCmd)
			cs.Col = len([]rune(indent))
		}
	}
	if e.Undo != nil {
		e.Undo.Push(&undo.BatchCommand{Commands: cmds})
	}
	e.markBufferDirty()
	e.syncFromMulti()
}

func (e *EditorPaneWidget) adjustLaterCursors(editedIdx int, start, end selection.Position) {
	for j := editedIdx + 1; j < len(e.Multi.Cursors); j++ {
		cs := &e.Multi.Cursors[j]
		if start.Line == end.Line {
			if cs.Line == start.Line && cs.Col >= end.Col {
				cs.Col -= end.Col - start.Col
			}
		} else {
			if cs.Line == end.Line {
				cs.Col = start.Col + (cs.Col - end.Col)
				cs.Line = start.Line
			} else if cs.Line > end.Line {
				cs.Line -= end.Line - start.Line
			}
		}
	}
}

func (e *EditorPaneWidget) shiftLaterCursors(editedIdx, line, col, delta int) {
	for j := editedIdx + 1; j < len(e.Multi.Cursors); j++ {
		cs := &e.Multi.Cursors[j]
		if cs.Line == line && cs.Col >= col {
			cs.Col += delta
			if cs.Col < 0 {
				cs.Col = 0
			}
		}
	}
}

func (e *EditorPaneWidget) shiftLaterLines(editedIdx, fromLine, delta int) {
	for j := editedIdx + 1; j < len(e.Multi.Cursors); j++ {
		if e.Multi.Cursors[j].Line >= fromLine {
			e.Multi.Cursors[j].Line += delta
		}
	}
}

func (e *EditorPaneWidget) multiMoveAll(moveFn func(cs *multicursor.CursorState)) {
	e.syncToMulti()
	for i := range e.Multi.Cursors {
		e.Multi.Cursors[i].Sel.Clear()
		e.Multi.Cursors[i].Line = e.Buf.ClampLine(e.Multi.Cursors[i].Line)
		moveFn(&e.Multi.Cursors[i])
		e.Multi.Cursors[i].Line = e.Buf.ClampLine(e.Multi.Cursors[i].Line)
	}
	e.Multi.Deduplicate()
	e.syncFromMulti()
}

func (e *EditorPaneWidget) SelectNextOccurrence() {
	if e.Selection == nil {
		return
	}
	word := ""
	if e.Selection.Active {
		word = e.Selection.Text(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col)
	}
	if word == "" {
		e.selectWord(e.Cursor.Line, e.Cursor.Col)
		if e.Selection.Active {
			e.multiSearchWord = e.Selection.Text(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col)
		}
		e.ensureMulti()
		e.syncToMulti()
		return
	}
	if e.multiSearchWord == "" {
		e.multiSearchWord = word
	}
	e.ensureMulti()
	e.syncToMulti()
	e.Multi.NormalizePrimary()
	e.syncFromMulti()

	searchWord := e.multiSearchWord
	lastCursor := e.Multi.Cursors[len(e.Multi.Cursors)-1]
	startLine := lastCursor.Line
	startCol := lastCursor.Col

	for line := startLine; line < len(e.Buf.Lines)+startLine; line++ {
		l := line % len(e.Buf.Lines)
		runes := []rune(e.Buf.Lines[l])
		searchRunes := []rune(searchWord)
		fromCol := 0
		if l == startLine {
			fromCol = startCol
		}
		for col := fromCol; col <= len(runes)-len(searchRunes); col++ {
			if string(runes[col:col+len(searchRunes)]) == searchWord {
				already := false
				for _, c := range e.Multi.Cursors {
					s, end := c.Sel.Range(c.Line, c.Col)
					if s.Line == l && s.Col == col && end.Col == col+len(searchRunes) {
						already = true
						break
					}
				}
				if already {
					continue
				}
				sel := selection.Selection{Active: true, Anchor: selection.Position{Line: l, Col: col}}
				e.Multi.AddWithSelection(l, col+len(searchRunes), sel)
				e.syncFromMulti()
				e.scrollViewport()
				return
			}
		}
	}
}

func (e *EditorPaneWidget) SelectAllOccurrences() {
	if e.Selection == nil {
		return
	}
	word := ""
	if e.Selection.Active {
		word = e.Selection.Text(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col)
	}
	if word == "" {
		e.selectWord(e.Cursor.Line, e.Cursor.Col)
		if e.Selection.Active {
			word = e.Selection.Text(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col)
		}
	}
	if word == "" {
		return
	}
	e.multiSearchWord = word
	e.ensureMulti()
	e.syncToMulti()

	searchRunes := []rune(word)
	for line := 0; line < len(e.Buf.Lines); line++ {
		runes := []rune(e.Buf.Lines[line])
		for col := 0; col <= len(runes)-len(searchRunes); col++ {
			if string(runes[col:col+len(searchRunes)]) == word {
				sel := selection.Selection{Active: true, Anchor: selection.Position{Line: line, Col: col}}
				e.Multi.AddWithSelection(line, col+len(searchRunes), sel)
			}
		}
	}
	e.syncFromMulti()
}

func (e *EditorPaneWidget) UndoLastCursor() {
	if e.Multi == nil || !e.Multi.IsMulti() {
		return
	}
	e.Multi.RemoveLast()
	if !e.Multi.IsMulti() {
		e.syncFromMulti()
		e.Multi = nil
		e.multiSearchWord = ""
	} else {
		e.syncFromMulti()
	}
	e.scrollViewport()
}

func (e *EditorPaneWidget) SplitSelectionToLines() {
	if e.Selection == nil || !e.Selection.Active {
		return
	}
	start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
	if start.Line == end.Line {
		return
	}
	if start.Line >= len(e.Buf.Lines) || end.Line >= len(e.Buf.Lines) {
		e.Selection.Clear()
		return
	}
	// If the selection ends at column 0 of the last line,
	// exclude that line (cursor sits at the start, nothing selected there)
	if end.Col == 0 && end.Line > start.Line {
		end.Line--
		end.Col = len([]rune(e.Buf.Lines[end.Line]))
	}
	if start.Line == end.Line {
		e.Selection.Clear()
		return
	}
	e.ensureMulti()
	e.syncToMulti()
	// Place the primary cursor at the end of the first selected line
	firstLineLen := len([]rune(e.Buf.Lines[start.Line]))
	col := firstLineLen
	e.Multi.Cursors[e.Multi.Primary] = multicursor.CursorState{
		Line: start.Line,
		Col:  col,
	}
	// Add a cursor at the end of each subsequent line in the selection
	for line := start.Line + 1; line <= end.Line; line++ {
		lineLen := len([]rune(e.Buf.Lines[line]))
		c := lineLen
		if line == end.Line && end.Col < c {
			c = end.Col
		}
		e.Multi.Add(line, c)
	}
	e.Selection.Clear()
	// Clear selections on all cursors
	for i := range e.Multi.Cursors {
		e.Multi.Cursors[i].Sel.Clear()
	}
	e.syncFromMulti()
	e.scrollViewport()
}
