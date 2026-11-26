package ui

import (
	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

func (g *EditorGroupWidget) newUndoStack() *undo.UndoStack {
	return &undo.UndoStack{DeleteCursorStart: g.UndoDeleteCursorStart}
}

func (g *EditorGroupWidget) ApplyUndoDeleteCursorStart(v bool) {
	for i := range g.tabs {
		if g.tabs[i].Undo != nil {
			g.tabs[i].Undo.DeleteCursorStart = v
		}
	}
}

func (g *EditorGroupWidget) undoRedoPostProcess(version uint64, reveal bool) {
	if g.Editor.Folds != nil {
		g.Editor.remapFolds(version)
		g.Editor.refreshFolds()
		g.Editor.ExpandFoldContaining(g.Editor.Cursor.Line)
	}
	g.Editor.bufferDirty = true
	if reveal {
		g.Editor.scrollViewport()
	}
}

func (e *EditorPaneWidget) applyUndoCursor(pos *undo.CursorPos, sel selection.Selection) {
	if pos == nil {
		return
	}
	e.Cursor.Line, e.Cursor.Col = pos.Line, pos.Col
	if sel.Active && e.Selection != nil {
		*e.Selection = sel
	}
}

func (g *EditorGroupWidget) Undo() {
	t := g.activeTab()
	if t == nil || g.editableKeyEditor() == nil {
		return
	}
	// Undo has no concept of e.Multi; collapsing first avoids stranding
	// secondary cursors at stale offsets that corrupt on the next
	// multicursor keystroke (BUG-008).
	if g.IsEditorActive() && g.Editor.isMultiActive() {
		g.Editor.collapseMulti()
	}
	if t.Undo != nil {
		version := t.Buf.Version()
		pos, sel := t.Undo.UndoWithSelection(t.Buf)
		g.Editor.applyUndoCursor(pos, sel)
		if t.Undo.AtSavePoint() && !t.Buf.LineEndingChanged() {
			t.Buf.Dirty = false
		}
		g.undoRedoPostProcess(version, pos != nil)
	}
}

func (g *EditorGroupWidget) Redo() {
	t := g.activeTab()
	if t == nil || g.editableKeyEditor() == nil {
		return
	}
	if g.IsEditorActive() && g.Editor.isMultiActive() {
		g.Editor.collapseMulti()
	}
	if t.Undo != nil {
		version := t.Buf.Version()
		pos, sel := t.Undo.RedoWithSelection(t.Buf)
		g.Editor.applyUndoCursor(pos, sel)
		if t.Undo.AtSavePoint() && !t.Buf.LineEndingChanged() {
			t.Buf.Dirty = false
		}
		g.undoRedoPostProcess(version, pos != nil)
	}
}

func (g *EditorGroupWidget) SelectAll() {
	if sel := g.ActiveTextSelection(); sel != nil {
		sel.SelectAllText()
		return
	}
	e := g.keyEditor()
	if e == nil || e.Selection == nil {
		return
	}
	e.collapseMulti()
	e.ResetCursorGoal()
	g.saveMultiState()
	e.Selection.Start(0, 0)
	lastLine := len(e.Buf.Lines) - 1
	e.Cursor.Line = lastLine
	e.Cursor.Col = len([]rune(e.Buf.Lines[lastLine]))
}

func (g *EditorGroupWidget) ActiveCursor() (line, col int) {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return 0, 0
	}
	return t.Cur.Line, t.Cur.Col
}

func (g *EditorGroupWidget) ActiveSelection() (active bool, sl, sc, el, ec int) {
	t := g.activeTab()
	if t == nil || t.Content != nil || t.Sel == nil || !t.Sel.Active {
		return false, 0, 0, 0, 0
	}
	start, end := t.Sel.Range(t.Cur.Line, t.Cur.Col)
	return true, start.Line, start.Col, end.Line, end.Col
}

// PlaceCursor moves the cursor to a 1-based line and column, clamping both to
// the buffer and expanding any fold hiding the line. It reports whether an
// editor was active. Unlike GoToLineCol it never touches the viewport, so it is
// safe before the first render: a scroll computed against a zero-height
// viewport leaves the tab mis-framed for as long as it stays open.
func (g *EditorGroupWidget) PlaceCursor(line, col int) bool {
	if !g.IsEditorActive() {
		return false
	}
	g.Editor.ResetCursorGoal()
	if line < 1 {
		line = 1
	}
	if line > len(g.Editor.Buf.Lines) {
		line = len(g.Editor.Buf.Lines)
	}
	bufLine := line - 1
	g.Editor.ExpandFoldContaining(bufLine)
	g.Editor.Cursor.Line = bufLine
	g.Editor.Cursor.Col = 0
	if col > 1 {
		lineLen := len([]rune(g.Editor.Buf.Lines[bufLine]))
		c := min(col-1, lineLen)
		g.Editor.Cursor.Col = c
	}
	return true
}

func (g *EditorGroupWidget) GoToLine(line int) {
	if !g.PlaceCursor(line, 0) {
		return
	}
	bufLine := g.Editor.Cursor.Line
	h := g.Editor.Viewport.Height
	if h <= 0 {
		r := g.GetRect()
		h = r.H - 3
		if h > 0 {
			g.Editor.Viewport.Height = h
		}
	}
	if h > 0 {
		margin := h / 3
		top := max(bufLine-margin, 0)
		g.Editor.Viewport.TopLine = top
	}
	g.Editor.scrollViewport()
}

// GoToLineCol moves the cursor to a 1-based line and column. The column is
// clamped to the line's rune length; col 0 or 1 leaves the cursor at the
// start of the line, matching GoToLine.
func (g *EditorGroupWidget) GoToLineCol(line, col int) {
	g.GoToLine(line)
	if !g.IsEditorActive() || col <= 1 {
		return
	}
	lineLen := len([]rune(g.Editor.Buf.Lines[g.Editor.Cursor.Line]))
	c := min(col-1, lineLen)
	g.Editor.Cursor.Col = c
	g.Editor.scrollViewport()
}

func (g *EditorGroupWidget) ScrollToCursor() {
	if g.IsEditorActive() {
		g.Editor.scrollViewport()
	}
}

func (g *EditorGroupWidget) MoveLineUp() {
	if e := g.editableKeyEditor(); e != nil {
		e.MoveLineUp()
	}
}

func (g *EditorGroupWidget) MoveLineDown() {
	if e := g.editableKeyEditor(); e != nil {
		e.MoveLineDown()
	}
}

func (g *EditorGroupWidget) DuplicateLine() {
	if e := g.editableKeyEditor(); e != nil {
		e.DuplicateLine()
	}
}

func (g *EditorGroupWidget) DeleteLine() {
	if e := g.editableKeyEditor(); e != nil {
		e.DeleteLine()
	}
}

func (g *EditorGroupWidget) JoinLines() {
	if e := g.editableKeyEditor(); e != nil {
		e.JoinLines()
	}
}

func (g *EditorGroupWidget) InsertLineBelow() {
	if e := g.editableKeyEditor(); e != nil {
		e.InsertLineBelow()
	}
}

func (g *EditorGroupWidget) InsertLineAbove() {
	if e := g.editableKeyEditor(); e != nil {
		e.InsertLineAbove()
	}
}

func (g *EditorGroupWidget) ToggleLineComment() {
	if e := g.editableKeyEditor(); e != nil {
		e.ToggleLineComment()
	}
}

func (g *EditorGroupWidget) SortLinesAsc() {
	if e := g.editableKeyEditor(); e != nil {
		e.SortLinesAsc()
	}
}

func (g *EditorGroupWidget) SortLinesDesc() {
	if e := g.editableKeyEditor(); e != nil {
		e.SortLinesDesc()
	}
}

func (g *EditorGroupWidget) ReverseLines() {
	if e := g.editableKeyEditor(); e != nil {
		e.ReverseLines()
	}
}

func (g *EditorGroupWidget) UniqueLines() {
	if e := g.editableKeyEditor(); e != nil {
		e.UniqueLines()
	}
}

func (g *EditorGroupWidget) TrimTrailingWhitespaceLines() {
	if e := g.editableKeyEditor(); e != nil {
		e.TrimTrailingWhitespaceLines()
	}
}

func (g *EditorGroupWidget) MoveToFileEdge(end, shift bool) {
	if e := g.keyEditor(); e != nil {
		e.MoveToFileEdge(end, shift)
	} else if d := g.ActiveDiffWidget(); d != nil {
		d.KeyPane().MoveToFileEdge(end, shift)
	}
}

func (g *EditorGroupWidget) MoveToLineEdge(end, shift bool) {
	if e := g.keyEditor(); e != nil {
		e.MoveToLineEdge(end, shift)
	} else if d := g.ActiveDiffWidget(); d != nil {
		d.KeyPane().MoveToLineEdge(end, shift)
	}
}

func (g *EditorGroupWidget) MoveWordLeft(shift bool) {
	if e := g.keyEditor(); e != nil {
		e.MoveWordLeft(shift)
	}
}

func (g *EditorGroupWidget) MoveWordRight(shift bool) {
	if e := g.keyEditor(); e != nil {
		e.MoveWordRight(shift)
	}
}

func (g *EditorGroupWidget) ScrollLines(delta int) {
	if e := g.keyEditor(); e != nil {
		e.ScrollRows(delta)
	}
}

func (g *EditorGroupWidget) ScrollViewportLines(delta int) {
	if e := g.keyEditor(); e != nil {
		if delta < 0 {
			e.scrollUp(-delta)
		} else {
			e.scrollDown(delta)
		}
	}
}

func (g *EditorGroupWidget) ScrollHalfPage(direction int) {
	if e := g.keyEditor(); e != nil {
		e.ScrollRows(direction * max(1, e.Viewport.Height/2))
	}
}

func (g *EditorGroupWidget) DeleteWordLeft() {
	if e := g.editableKeyEditor(); e != nil {
		e.DeleteWordLeft()
	}
}

func (g *EditorGroupWidget) DeleteWordRight() {
	if e := g.editableKeyEditor(); e != nil {
		e.DeleteWordRight()
	}
}

func (g *EditorGroupWidget) AddCursorAbove() {
	g.addCursorVertical(-1)
}

func (g *EditorGroupWidget) AddCursorBelow() {
	g.addCursorVertical(1)
}

func (g *EditorGroupWidget) addCursorVertical(delta int) {
	if e := g.keyEditor(); e != nil {
		e.addCursorVertical(delta)
		g.saveMultiState()
	}
}

func (g *EditorGroupWidget) SelectNextOccurrence() {
	if e := g.keyEditor(); e != nil {
		e.SelectNextOccurrence()
		g.saveMultiState()
	}
}

func (g *EditorGroupWidget) SelectAllOccurrences() {
	if e := g.keyEditor(); e != nil {
		e.SelectAllOccurrences()
		g.saveMultiState()
	}
}

func (g *EditorGroupWidget) UndoLastCursor() {
	if e := g.keyEditor(); e != nil {
		e.UndoLastCursor()
		g.saveMultiState()
	}
}

func (g *EditorGroupWidget) SplitSelectionToLines() {
	if e := g.keyEditor(); e != nil {
		e.SplitSelectionToLines()
		g.saveMultiState()
	}
}

func (g *EditorGroupWidget) UpperCase() {
	if e := g.editableKeyEditor(); e != nil {
		e.UpperCase()
	}
}

func (g *EditorGroupWidget) LowerCase() {
	if e := g.editableKeyEditor(); e != nil {
		e.LowerCase()
	}
}

func (g *EditorGroupWidget) TitleCase() {
	if e := g.editableKeyEditor(); e != nil {
		e.TitleCase()
	}
}

func (g *EditorGroupWidget) GoToMatchingBracket() {
	if e := g.keyEditor(); e != nil {
		e.GoToMatchingBracket()
	}
}

func (g *EditorGroupWidget) IsMultiCursorActive() bool {
	return g.IsEditorActive() && g.Editor.isMultiActive()
}

func (g *EditorGroupWidget) MultiCursorCount() int {
	if g.IsEditorActive() && g.Editor.Multi != nil {
		return len(g.Editor.Multi.Cursors)
	}
	return 1
}

func (g *EditorGroupWidget) CollapseMultiCursor() {
	if g.IsEditorActive() {
		g.Editor.collapseMulti()
	}
}

func (g *EditorGroupWidget) AddCursor(line, col int) {
	if !g.IsEditorActive() {
		return
	}
	g.Editor.ensureMulti()
	g.Editor.syncToMulti()
	g.Editor.Multi.Add(line, col)
	g.Editor.syncFromMulti()
	g.saveMultiState()
}

func (g *EditorGroupWidget) ToggleCursor(line, col int) {
	if !g.IsEditorActive() {
		return
	}
	g.Editor.ensureMulti()
	g.Editor.syncToMulti()
	g.Editor.Multi.Toggle(line, col)
	if !g.Editor.Multi.IsMulti() {
		g.Editor.syncFromMulti()
		g.Editor.Multi = nil
	} else {
		g.Editor.syncFromMulti()
	}
	g.saveMultiState()
}

func (g *EditorGroupWidget) GetCursors() []multicursor.CursorState {
	if !g.IsEditorActive() || g.Editor.Multi == nil {
		return nil
	}
	return g.Editor.Multi.Cursors
}
