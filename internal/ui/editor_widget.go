package ui

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/highlight"
	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"

	"github.com/gdamore/tcell/v3"
)

const maxBracketColorLines = 10_000

type EditorPaneWidget struct {
	BaseWidget
	Buf                     *buffer.Buffer
	Cursor                  *cursor.Cursor
	Viewport                *view.Viewport
	Undo                    *undo.UndoStack
	Selection               *selection.Selection
	CursorX                 int
	CursorY                 int
	TabSize                 int
	UseTabs                 bool
	LineNumbers             bool
	GutterStyle             string
	WordWrap                bool
	AutoDedent              bool
	AutoIndent              bool
	BracketPairColorization bool
	BracketColorStyles      []term.Style
	Highlighter             *highlight.Highlighter
	SearchQuery             string
	SearchMatches           []FindMatch
	SearchActive            int
	lastClickTime           int64
	lastClickLine           int
	lastClickCol            int
	clickCount              int
	mouseDown               bool
	scrollbar               Scrollbar
	hscrollbar              HScrollbar
	Diagnostics             []Diagnostic
	Folds                   *fold.State
	OnChange                func()
	bufferDirty             bool
	Multi                   *multicursor.MultiCursor
	multiSearchWord         string
	gutterHover             bool
	gutterHoverLine         int
	mouseDownX, mouseDownY  int
	maxWidthSeen            int
	cachedVisibleLines      []int
	searchByLine            map[int][]int
	diagByLine              map[int][]int
	LineChanges             []diff.LineChangeKind
	ReadOnly                bool
	bracketColorCache       bracketColorMap
	bracketColorDirty       bool
	bracketMatchCache       bracketMatch
	bracketGen              int
	wrapMap                 []wrapEntry
	wrapTopOffset           int
}

func NewEditorPaneWidget(buf *buffer.Buffer, cur *cursor.Cursor, vp *view.Viewport) *EditorPaneWidget {
	return &EditorPaneWidget{
		Buf:               buf,
		Cursor:            cur,
		Viewport:          vp,
		AutoDedent:        true,
		AutoIndent:        true,
		bracketColorDirty: true,
	}
}

func (e *EditorPaneWidget) InvalidateBracketColors() {
	e.bracketColorDirty = true
	e.bracketGen++
}

func (e *EditorPaneWidget) Focusable() bool { return true }

func (e *EditorPaneWidget) GutterWidth() int {
	if !e.LineNumbers {
		if e.GutterStyle != "minimal" {
			return 1
		}
		return 0
	}
	digits := len(strconv.Itoa(len(e.Buf.Lines)))
	if digits < 2 {
		digits = 2
	}
	switch e.GutterStyle {
	case "minimal":
		return digits + 1
	case "extended":
		return digits + 5
	default:
		return digits + 3
	}
}

func (e *EditorPaneWidget) computeMaxLineWidth() int {
	tabW := e.resolveTabSize()
	maxW := 0
	topLine := e.Viewport.TopLine
	botLine := topLine + e.Viewport.Height
	if botLine > len(e.Buf.Lines) {
		botLine = len(e.Buf.Lines)
	}
	for i := topLine; i < botLine; i++ {
		lw := bufColToVisualCol(e.Buf.Lines[i], len([]rune(e.Buf.Lines[i])), tabW)
		if lw > maxW {
			maxW = lw
		}
	}
	if maxW > e.maxWidthSeen {
		e.maxWidthSeen = maxW
	}
	return e.maxWidthSeen
}

func (e *EditorPaneWidget) clampLeftCol() {
	editorW := e.Viewport.Width
	maxW := e.computeMaxLineWidth()
	max := maxW - editorW
	if max < 0 {
		max = 0
	}
	if e.Viewport.LeftCol > max {
		e.Viewport.LeftCol = max
	}
	if e.Viewport.LeftCol < 0 {
		e.Viewport.LeftCol = 0
	}
}

func (e *EditorPaneWidget) buildSearchIndex() {
	e.searchByLine = make(map[int][]int, len(e.SearchMatches))
	for i, m := range e.SearchMatches {
		e.searchByLine[m.Line] = append(e.searchByLine[m.Line], i)
	}
}

func (e *EditorPaneWidget) buildDiagIndex() {
	e.diagByLine = make(map[int][]int)
	for i, d := range e.Diagnostics {
		for line := d.StartLine; line <= d.EndLine; line++ {
			e.diagByLine[line] = append(e.diagByLine[line], i)
		}
	}
}

func (e *EditorPaneWidget) hasFolds() bool {
	return !e.WordWrap && e.Folds != nil && e.Folds.HasCollapsedFolds()
}

func (e *EditorPaneWidget) ensureTopLineVisible() {
	if e.Folds == nil {
		return
	}
	if r := e.Folds.ContainingFold(e.Viewport.TopLine); r != nil {
		e.Viewport.TopLine = r.StartLine
	}
}

func (e *EditorPaneWidget) screenToBufferLine(y int) int {
	if e.cachedVisibleLines == nil || e.Folds == nil {
		return e.Viewport.TopLine + y
	}
	topVis := e.Folds.BufferToVisible(e.Viewport.TopLine)
	if topVis < 0 {
		topVis = 0
	}
	idx := topVis + y
	if idx < 0 {
		return 0
	}
	if idx >= len(e.cachedVisibleLines) {
		return len(e.Buf.Lines)
	}
	return e.cachedVisibleLines[idx]
}

func (e *EditorPaneWidget) DiagnosticAt(line, col int) *Diagnostic {
	for i := range e.Diagnostics {
		d := &e.Diagnostics[i]
		if line < d.StartLine || line > d.EndLine {
			continue
		}
		if line == d.StartLine && col < d.StartCol {
			continue
		}
		if line == d.EndLine && col >= d.EndCol {
			continue
		}
		return d
	}
	return nil
}

func (e *EditorPaneWidget) exec(cmd undo.EditCommand) {
	prevLines := len(e.Buf.Lines)
	cmd.Apply(e.Buf)
	if e.Undo != nil {
		e.Undo.Push(cmd)
	}
	e.bufferDirty = true
	if e.Folds != nil && len(e.Buf.Lines) != prevLines {
		e.Folds.SetRanges(fold.ComputeIndentRanges(e.Buf.Lines))
	}
}

func (e *EditorPaneWidget) ExecCommand(cmd undo.EditCommand) { e.exec(cmd) }

func (e *EditorPaneWidget) FlushOnChange() {
	if e.bufferDirty {
		e.bufferDirty = false
		e.maxWidthSeen = 0
		e.bracketColorDirty = true
		e.bracketGen++
		if e.Highlighter != nil {
			e.Highlighter.ClearCache()
		}
		if e.OnChange != nil {
			e.OnChange()
		}
	}
}

func (e *EditorPaneWidget) deleteSelection() {
	if e.Selection == nil || !e.Selection.Active {
		return
	}
	if e.Undo != nil {
		e.Undo.BreakGroup()
	}
	start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
	cmd := &undo.DeleteSelectionCommand{
		StartLine: start.Line, StartCol: start.Col,
		EndLine: end.Line, EndCol: end.Col,
	}
	e.exec(cmd)
	e.Cursor.Line = start.Line
	e.Cursor.Col = start.Col
	e.Selection.Clear()
}

func (e *EditorPaneWidget) replaceSelection(cmds ...undo.EditCommand) {
	if e.Selection == nil || !e.Selection.Active {
		return
	}
	start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
	all := make([]undo.EditCommand, 0, 1+len(cmds))
	all = append(all, &undo.DeleteSelectionCommand{
		StartLine: start.Line, StartCol: start.Col,
		EndLine: end.Line, EndCol: end.Col,
	})
	all = append(all, cmds...)
	if e.Undo != nil {
		e.Undo.BreakGroup()
	}
	e.exec(&undo.BatchCommand{Commands: all})
	if e.Undo != nil {
		e.Undo.ContinueGroup()
	}
	e.Cursor.Line = start.Line
	e.Cursor.Col = start.Col
	e.Selection.Clear()
}

func (e *EditorPaneWidget) startOrExtendSelection(shift bool) {
	if e.Selection == nil {
		return
	}
	if shift {
		if !e.Selection.Active {
			e.Selection.Start(e.Cursor.Line, e.Cursor.Col)
		}
	} else {
		e.Selection.Clear()
	}
}

func (e *EditorPaneWidget) pasteText(text string) {
	if e.Selection != nil && e.Selection.Active {
		e.deleteSelection()
	}
	e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line)
	lines := strings.Split(text, "\n")
	if len(lines) == 1 {
		e.exec(&undo.InsertStringCommand{Line: e.Cursor.Line, Col: e.Cursor.Col, Text: lines[0]})
		e.Cursor.Col += len([]rune(lines[0]))
	} else {
		currentLine := []rune(e.Buf.Lines[e.Cursor.Line])
		col := e.Cursor.Col
		if col > len(currentLine) {
			col = len(currentLine)
		}
		suffix := string(currentLine[col:])
		e.exec(&undo.PasteCommand{
			Line:   e.Cursor.Line,
			Col:    col,
			Text:   text,
			Suffix: suffix,
		})
		e.Cursor.Line += len(lines) - 1
		e.Cursor.Col = len([]rune(lines[len(lines)-1]))
	}
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) HandleEvent(ev tcell.Event) EventResult {
	if mev, ok := ev.(*tcell.EventMouse); ok {
		return e.handleMouse(mev)
	}
	if kev, ok := ev.(*tcell.EventKey); ok {
		return e.handleKey(kev)
	}
	return EventIgnored
}

func (e *EditorPaneWidget) selectWord(line, col int) {
	if e.Selection == nil || line < 0 || line >= len(e.Buf.Lines) {
		return
	}
	runes := []rune(e.Buf.Lines[line])
	if len(runes) == 0 {
		return
	}
	if col >= len(runes) {
		col = len(runes) - 1
	}
	isWord := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
	}
	start, end := col, col
	if isWord(runes[col]) {
		for start > 0 && isWord(runes[start-1]) {
			start--
		}
		for end < len(runes)-1 && isWord(runes[end+1]) {
			end++
		}
	}
	end++
	e.Selection.Start(line, start)
	e.Cursor.Line = line
	e.Cursor.Col = end
}

func (e *EditorPaneWidget) selectLine(line int) {
	if e.Selection == nil {
		return
	}
	e.Selection.Start(line, 0)
	if line < len(e.Buf.Lines)-1 {
		e.Cursor.Line = line + 1
		e.Cursor.Col = 0
	} else {
		e.Cursor.Line = line
		e.Cursor.Col = len([]rune(e.Buf.Lines[line]))
	}
}

func (e *EditorPaneWidget) resolveTabSize() int {
	if e.TabSize > 0 {
		return e.TabSize
	}
	return 4
}

func (e *EditorPaneWidget) indentUnit() string {
	if e.UseTabs {
		return "\t"
	}
	return strings.Repeat(" ", e.resolveTabSize())
}

func (e *EditorPaneWidget) dedentForCloser() {
	runes := []rune(e.Buf.Lines[e.Cursor.Line])
	col := e.Cursor.Col
	if col > len(runes) {
		col = len(runes)
	}
	for i := 0; i < col; i++ {
		if runes[i] != ' ' && runes[i] != '\t' {
			return
		}
	}
	remove := leadingIndentWidth(e.Buf.Lines[e.Cursor.Line], e.resolveTabSize())
	if remove <= 0 || remove > col {
		return
	}
	e.exec(&undo.DeleteSelectionCommand{
		StartLine: e.Cursor.Line, StartCol: 0,
		EndLine: e.Cursor.Line, EndCol: remove,
	})
	e.Cursor.Col -= remove
	if e.Cursor.Col < 0 {
		e.Cursor.Col = 0
	}
}

func (e *EditorPaneWidget) clampCursor() {
	e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line)
	if e.Cursor.Col < 0 {
		e.Cursor.Col = 0
	}
}

func (e *EditorPaneWidget) skipHiddenLineDown() {
	if e.Folds == nil {
		return
	}
	if r := e.Folds.ContainingFold(e.Cursor.Line); r != nil {
		e.Cursor.Line = e.Buf.ClampLine(r.EndLine + 1)
		lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
		if e.Cursor.Col > lineLen {
			e.Cursor.Col = lineLen
		}
	}
}

func (e *EditorPaneWidget) skipHiddenLineUp() {
	if e.Folds == nil {
		return
	}
	if r := e.Folds.ContainingFold(e.Cursor.Line); r != nil {
		e.Cursor.Line = r.StartLine
		lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
		if e.Cursor.Col > lineLen {
			e.Cursor.Col = lineLen
		}
	}
}

func (e *EditorPaneWidget) EnsureCursorVisible() {
	e.skipHiddenLineDown()
	e.scrollViewport()
}

func (e *EditorPaneWidget) ExpandFoldContaining(line int) {
	if e.Folds != nil {
		if r := e.Folds.ContainingFold(line); r != nil {
			e.Folds.Expand(r.StartLine)
		}
	}
}

func (e *EditorPaneWidget) expandFoldAtCursor() {
	if e.Folds == nil {
		return
	}
	if e.Folds.IsCollapsed(e.Cursor.Line) {
		e.Folds.Expand(e.Cursor.Line)
	}
}

func (e *EditorPaneWidget) scrollViewport() {
	if e.WordWrap {
		e.scrollViewportWrap()
		return
	}
	if e.Folds != nil && e.Folds.HasCollapsedFolds() {
		curVis := e.Folds.BufferToVisible(e.Cursor.Line)
		topVis := e.Folds.BufferToVisible(e.Viewport.TopLine)
		if curVis < 0 {
			curVis = 0
		}
		if topVis < 0 {
			topVis = 0
		}
		if curVis < topVis {
			e.Viewport.TopLine = e.Folds.VisibleToBuffer(curVis)
		}
		if curVis >= topVis+e.Viewport.Height {
			newTopVis := curVis - e.Viewport.Height + 1
			e.Viewport.TopLine = e.Folds.VisibleToBuffer(newTopVis)
		}
	} else {
		if e.Cursor.Line < e.Viewport.TopLine {
			e.Viewport.TopLine = e.Cursor.Line
		}
		if e.Cursor.Line >= e.Viewport.TopLine+e.Viewport.Height {
			e.Viewport.TopLine = e.Cursor.Line - e.Viewport.Height + 1
		}
	}
	e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line)
	visCol := bufColToVisualCol(e.Buf.Lines[e.Cursor.Line], e.Cursor.Col, e.resolveTabSize())
	if visCol < e.Viewport.LeftCol {
		e.Viewport.LeftCol = visCol
	}
	if visCol >= e.Viewport.LeftCol+e.Viewport.Width {
		e.Viewport.LeftCol = visCol - e.Viewport.Width + 1
	}
}

func (e *EditorPaneWidget) scrollViewportWrap() {
	e.Viewport.LeftCol = 0
	tabW := e.resolveTabSize()
	width := e.Viewport.Width
	if width < 1 {
		width = 1
	}

	curVisRow, _ := bufferPosToWrapScreenPos(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col, width, tabW)
	topVisRow, _ := bufferPosToWrapScreenPos(e.Buf.Lines, e.Viewport.TopLine, 0, width, tabW)
	topVisRow += e.wrapTopOffset

	if curVisRow < topVisRow {
		e.Viewport.TopLine, e.wrapTopOffset = wrapVisualRowToTopLine(e.Buf.Lines, curVisRow, width, tabW)
	}
	if curVisRow >= topVisRow+e.Viewport.Height {
		newTop := curVisRow - e.Viewport.Height + 1
		e.Viewport.TopLine, e.wrapTopOffset = wrapVisualRowToTopLine(e.Buf.Lines, newTop, width, tabW)
	}
}

func (e *EditorPaneWidget) scrollUp(n int) {
	if e.WordWrap {
		tabW := e.resolveTabSize()
		width := e.Viewport.Width
		if width < 1 {
			width = 1
		}
		topVisRow, _ := bufferPosToWrapScreenPos(e.Buf.Lines, e.Viewport.TopLine, 0, width, tabW)
		topVisRow += e.wrapTopOffset
		newTop := topVisRow - n
		if newTop < 0 {
			newTop = 0
		}
		e.Viewport.TopLine, e.wrapTopOffset = wrapVisualRowToTopLine(e.Buf.Lines, newTop, width, tabW)
		return
	}
	if e.Folds != nil && e.Folds.HasCollapsedFolds() {
		topVis := e.Folds.BufferToVisible(e.Viewport.TopLine)
		newVis := topVis - n
		if newVis < 0 {
			newVis = 0
		}
		e.Viewport.TopLine = e.Folds.VisibleToBuffer(newVis)
	} else {
		e.Viewport.TopLine -= n
		if e.Viewport.TopLine < 0 {
			e.Viewport.TopLine = 0
		}
	}
}

func (e *EditorPaneWidget) scrollDown(n int) {
	if e.WordWrap {
		tabW := e.resolveTabSize()
		width := e.Viewport.Width
		if width < 1 {
			width = 1
		}
		topVisRow, _ := bufferPosToWrapScreenPos(e.Buf.Lines, e.Viewport.TopLine, 0, width, tabW)
		topVisRow += e.wrapTopOffset
		totalVis := totalVisualLines(e.Buf.Lines, width, tabW)
		newTop := topVisRow + n
		if newTop >= totalVis {
			newTop = totalVis - 1
		}
		if newTop < 0 {
			newTop = 0
		}
		e.Viewport.TopLine, e.wrapTopOffset = wrapVisualRowToTopLine(e.Buf.Lines, newTop, width, tabW)
		return
	}
	if e.Folds != nil && e.Folds.HasCollapsedFolds() {
		totalLines := len(e.Buf.Lines)
		visCount := e.Folds.VisibleLineCount(totalLines)
		topVis := e.Folds.BufferToVisible(e.Viewport.TopLine)
		newVis := topVis + n
		if newVis >= visCount {
			newVis = visCount - 1
		}
		if newVis < 0 {
			newVis = 0
		}
		e.Viewport.TopLine = e.Folds.VisibleToBuffer(newVis)
	} else {
		max := len(e.Buf.Lines) - 1
		if max < 0 {
			max = 0
		}
		e.Viewport.TopLine += n
		if e.Viewport.TopLine > max {
			e.Viewport.TopLine = max
		}
	}
}

func (e *EditorPaneWidget) SmartHome() {
	runes := []rune(e.Buf.Lines[e.Cursor.Line])
	firstNonSpace := 0
	for firstNonSpace < len(runes) && (runes[firstNonSpace] == ' ' || runes[firstNonSpace] == '\t') {
		firstNonSpace++
	}
	if e.Cursor.Col == firstNonSpace {
		e.Cursor.Col = 0
	} else {
		e.Cursor.Col = firstNonSpace
	}
	e.clampCursor()
	e.scrollViewport()
}
