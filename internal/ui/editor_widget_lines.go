package ui

import (
	"sort"
	"strings"

	"github.com/eugenioenko/ttt/internal/core/undo"
)

func (e *EditorPaneWidget) MoveLineUp() {
	if e.isMultiActive() {
		e.moveLinesMulti(-1)
		return
	}
	if startLine, endLine, ok := e.selectedLineRange(); ok {
		if startLine <= 0 {
			return
		}
		for line := startLine; line <= endLine; line++ {
			e.exec(&undo.SwapLineCommand{Line1: line, Line2: line - 1})
		}
		e.Cursor.Line--
		e.Selection.Anchor.Line--
	} else {
		if e.Cursor.Line <= 0 {
			return
		}
		e.exec(&undo.SwapLineCommand{Line1: e.Cursor.Line, Line2: e.Cursor.Line - 1})
		e.Cursor.Line--
	}
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) MoveLineDown() {
	if e.isMultiActive() {
		e.moveLinesMulti(1)
		return
	}
	if startLine, endLine, ok := e.selectedLineRange(); ok {
		if endLine >= len(e.Buf.Lines)-1 {
			return
		}
		for line := endLine; line >= startLine; line-- {
			e.exec(&undo.SwapLineCommand{Line1: line, Line2: line + 1})
		}
		e.Cursor.Line++
		e.Selection.Anchor.Line++
	} else {
		if e.Cursor.Line >= len(e.Buf.Lines)-1 {
			return
		}
		e.exec(&undo.SwapLineCommand{Line1: e.Cursor.Line, Line2: e.Cursor.Line + 1})
		e.Cursor.Line++
	}
	e.clampCursor()
	e.scrollViewport()
}

// moveLinesMulti shifts every buffer line touched by a cursor (or its
// selection) by delta (-1 or +1) and moves all cursors with it, so each
// cursor stays on its own text. The whole move is a no-op if any touched
// line would cross a buffer edge. Without this, line commands leave
// e.Multi.Cursors at stale offsets and the next keystroke corrupts (BUG-005).
func (e *EditorPaneWidget) moveLinesMulti(delta int) {
	e.syncToMulti()

	touched := make(map[int]bool)
	for _, cs := range e.Multi.Cursors {
		lo, hi := cs.Line, cs.Line
		if cs.Sel.Active {
			s, en := cs.Sel.Range(cs.Line, cs.Col)
			lo, hi = s.Line, en.Line
			if en.Col == 0 && hi > lo {
				hi--
			}
		}
		for l := lo; l <= hi; l++ {
			touched[e.Buf.ClampLine(l)] = true
		}
	}
	if len(touched) == 0 {
		return
	}
	lines := make([]int, 0, len(touched))
	for l := range touched {
		lines = append(lines, l)
	}
	sort.Ints(lines)

	lastReal := len(e.Buf.Lines) - 1
	if lastReal > 0 && e.Buf.Lines[lastReal] == "" {
		lastReal--
	}
	if delta < 0 && lines[0] <= 0 {
		return
	}
	if delta > 0 && lines[len(lines)-1] >= lastReal {
		return
	}

	// One BatchCommand so a single undo reverses the whole multicursor move,
	// mirroring the multiExec* handlers.
	var cmds []undo.EditCommand
	if delta < 0 {
		for _, l := range lines {
			cmd := &undo.SwapLineCommand{Line1: l, Line2: l - 1}
			cmd.Apply(e.Buf)
			cmds = append(cmds, cmd)
		}
	} else {
		for i := len(lines) - 1; i >= 0; i-- {
			cmd := &undo.SwapLineCommand{Line1: lines[i], Line2: lines[i] + 1}
			cmd.Apply(e.Buf)
			cmds = append(cmds, cmd)
		}
	}
	if e.Undo != nil {
		e.Undo.Push(&undo.BatchCommand{Commands: cmds})
	}
	e.bufferDirty = true

	for i := range e.Multi.Cursors {
		cs := &e.Multi.Cursors[i]
		cs.Line = e.Buf.ClampLine(cs.Line + delta)
		if cs.Sel.Active {
			cs.Sel.Anchor.Line += delta
		}
	}
	e.Multi.Deduplicate()
	e.syncFromMulti()
	e.clampCursor()
	e.scrollViewport()
}

// collapseMultiForLineOp drops multi-cursor mode before a line command that
// has no meaningful multi-cursor semantics (duplicate/delete/join/sort a
// line at N cursors is ambiguous). Leaving e.Multi.Cursors in place while
// such a command reshapes the buffer strands them at stale offsets and the
// next keystroke corrupts (BUG-005).
func (e *EditorPaneWidget) collapseMultiForLineOp() {
	if e.isMultiActive() {
		e.collapseMulti()
	}
}

func (e *EditorPaneWidget) DuplicateLine() {
	e.collapseMultiForLineOp()
	startLine, endLine, hasSel := e.selectedLineRange()
	if !hasSel {
		startLine = e.Buf.ClampLine(e.Cursor.Line)
		endLine = startLine
	}
	texts := e.copyLines(startLine, endLine)
	for i, text := range texts {
		e.exec(&undo.InsertLineCommand{Idx: endLine + 1 + i, Text: text})
	}
	blockSize := endLine - startLine + 1
	e.Cursor.Line += blockSize
	if hasSel {
		e.Selection.Anchor.Line += blockSize
	}
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) DeleteLine() {
	e.collapseMultiForLineOp()
	startLine, endLine, hasSel := e.selectedLineRange()
	if !hasSel {
		startLine = e.Buf.ClampLine(e.Cursor.Line)
		endLine = startLine
	}
	for i := endLine; i >= startLine; i-- {
		if len(e.Buf.Lines) <= 1 {
			e.exec(&undo.DeleteSelectionCommand{
				StartLine: 0, StartCol: 0,
				EndLine: 0, EndCol: len([]rune(e.Buf.Lines[0])),
			})
			break
		}
		e.exec(&undo.DeleteLineCommand{Idx: i})
	}
	if hasSel {
		e.Selection.Active = false
	}
	e.Cursor.Line = e.Buf.ClampLine(startLine)
	e.Cursor.Col = 0
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) JoinLines() {
	e.collapseMultiForLineOp()
	if e.Undo != nil {
		e.Undo.BreakGroup()
	}
	hasSel := e.Selection != nil && e.Selection.Active
	if hasSel {
		start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
		endLine := end.Line
		if end.Col == 0 && endLine > start.Line {
			endLine--
		}
		// Join all lines in the selection range from the first line
		for endLine > start.Line {
			cmd := &undo.JoinNextLineCommand{Line: start.Line}
			e.exec(cmd)
			endLine--
		}
		e.Selection.Active = false
		e.Cursor.Line = start.Line
		lineLen := len([]rune(e.Buf.Lines[start.Line]))
		if e.Cursor.Col > lineLen {
			e.Cursor.Col = lineLen
		}
	} else {
		if e.Cursor.Line >= len(e.Buf.Lines)-1 {
			return
		}
		cmd := &undo.JoinNextLineCommand{Line: e.Cursor.Line}
		e.exec(cmd)
		e.Cursor.Col = cmd.JoinCol
	}
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) InsertLineBelow() {
	e.exec(&undo.InsertLineCommand{Idx: e.Cursor.Line + 1, Text: ""})
	e.Cursor.Line++
	e.Cursor.Col = 0
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) InsertLineAbove() {
	e.exec(&undo.InsertLineCommand{Idx: e.Cursor.Line, Text: ""})
	e.Cursor.Col = 0
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) commentPrefix() string {
	prefix := "//"
	if e.Highlighter != nil {
		lang := strings.ToLower(e.Highlighter.Language())
		switch lang {
		case "python", "ruby", "bash", "shell", "yaml", "toml":
			prefix = "#"
		case "lua", "sql":
			prefix = "--"
		case "html", "xml":
			prefix = "<!--"
		}
	}
	return prefix
}

func (e *EditorPaneWidget) ToggleLineComment() {
	e.collapseMultiForLineOp()
	prefix := e.commentPrefix()

	startLine, endLine := e.Cursor.Line, e.Cursor.Line
	if e.Selection != nil && e.Selection.Active {
		start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
		startLine = start.Line
		endLine = end.Line
		if end.Col == 0 && endLine > startLine {
			endLine--
		}
	}
	startLine = e.Buf.ClampLine(startLine)
	endLine = e.Buf.ClampLine(endLine)

	allCommented := true
	for l := startLine; l <= endLine; l++ {
		trimmed := strings.TrimLeft(e.Buf.Lines[l], " \t")
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, prefix) {
			allCommented = false
			break
		}
	}

	var cmds []undo.EditCommand
	cursorDelta := 0

	for l := startLine; l <= endLine; l++ {
		runes := []rune(e.Buf.Lines[l])
		trimmed := strings.TrimLeft(string(runes), " \t")
		if trimmed == "" {
			continue
		}
		indent := len(runes) - len([]rune(trimmed))

		if allCommented {
			removeLen := len([]rune(prefix))
			if indent+removeLen < len(runes) && runes[indent+removeLen] == ' ' {
				removeLen++
			}
			cmd := &undo.DeleteSelectionCommand{
				StartLine: l, StartCol: indent,
				EndLine: l, EndCol: indent + removeLen,
			}
			cmd.Apply(e.Buf)
			cmds = append(cmds, cmd)
			if l == e.Cursor.Line {
				cursorDelta = -removeLen
			}
		} else {
			cmd := &undo.InsertStringCommand{Line: l, Col: indent, Text: prefix + " "}
			cmd.Apply(e.Buf)
			cmds = append(cmds, cmd)
			if l == e.Cursor.Line {
				cursorDelta = len([]rune(prefix)) + 1
			}
		}
	}

	if len(cmds) > 0 && e.Undo != nil {
		e.Undo.Push(&undo.BatchCommand{Commands: cmds})
		e.bufferDirty = true
	}

	e.Cursor.Col += cursorDelta
	if e.Cursor.Col < 0 {
		e.Cursor.Col = 0
	}
	e.clampCursor()
	e.scrollViewport()
}

// selectedLineRange returns the start and end line indices for the current
// selection applying the col-0 convention, and true. If no selection is
// active it returns (0, 0, false).
func (e *EditorPaneWidget) selectedLineRange() (int, int, bool) {
	if e.Selection != nil && e.Selection.Active {
		start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
		endLine := end.Line
		if end.Col == 0 && endLine > start.Line {
			endLine--
		}
		return e.Buf.ClampLine(start.Line), e.Buf.ClampLine(endLine), true
	}
	return 0, 0, false
}

// lineRange returns the start and end line indices for the current selection,
// or the full buffer range if no selection is active.
func (e *EditorPaneWidget) lineRange() (int, int) {
	if start, end, ok := e.selectedLineRange(); ok {
		return start, end
	}
	end := len(e.Buf.Lines) - 1
	if end > 0 && e.Buf.Lines[end] == "" {
		end--
	}
	return 0, end
}

// copyLines returns a copy of the buffer lines in the given range (inclusive).
func (e *EditorPaneWidget) copyLines(start, end int) []string {
	lines := make([]string, end-start+1)
	copy(lines, e.Buf.Lines[start:end+1])
	return lines
}

func (e *EditorPaneWidget) SortLinesAsc() {
	e.collapseMultiForLineOp()
	start, end := e.lineRange()
	old := e.copyLines(start, end)
	sorted := make([]string, len(old))
	copy(sorted, old)
	sort.Strings(sorted)
	e.exec(&undo.ReplaceLinesCommand{Start: start, OldLines: old, NewLines: sorted})
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) SortLinesDesc() {
	e.collapseMultiForLineOp()
	start, end := e.lineRange()
	old := e.copyLines(start, end)
	sorted := make([]string, len(old))
	copy(sorted, old)
	sort.Sort(sort.Reverse(sort.StringSlice(sorted)))
	e.exec(&undo.ReplaceLinesCommand{Start: start, OldLines: old, NewLines: sorted})
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) ReverseLines() {
	e.collapseMultiForLineOp()
	start, end := e.lineRange()
	old := e.copyLines(start, end)
	reversed := make([]string, len(old))
	for i, line := range old {
		reversed[len(old)-1-i] = line
	}
	e.exec(&undo.ReplaceLinesCommand{Start: start, OldLines: old, NewLines: reversed})
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) UniqueLines() {
	e.collapseMultiForLineOp()
	start, end := e.lineRange()
	old := e.copyLines(start, end)
	seen := make(map[string]bool)
	var unique []string
	for _, line := range old {
		if !seen[line] {
			seen[line] = true
			unique = append(unique, line)
		}
	}
	e.exec(&undo.ReplaceLinesCommand{Start: start, OldLines: old, NewLines: unique})
	e.clampCursor()
	e.scrollViewport()
}
