package ui

import (
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"

	"github.com/gdamore/tcell/v3"
)

const maxBracketColorLines = 10_000

type Bookmark struct {
	Icon  rune
	Style term.Style
}

type EditorPaneWidget struct {
	BaseWidget
	lineScratch             []screenCell
	runeScratch             []rune
	Buf                     *buffer.Buffer
	Cursor                  *cursor.Cursor
	Viewport                *view.Viewport
	hScrollPending          bool
	Undo                    *undo.UndoStack
	Selection               *selection.Selection
	CursorX                 int
	CursorY                 int
	TabSize                 int
	UseTabs                 bool
	LineNumbers             bool
	GutterStyle             string
	FoldChevronCollapsed    rune
	FoldChevronExpanded     rune
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
	PostDragAutoScrollTick  func(generation uint64)
	dragPointerX            int
	dragPointerY            int
	autoScrollTimer         *time.Timer
	autoScrollGeneration    uint64
	maxWidthSeen            int
	searchByLine            map[int][]int
	diagByLine              map[int][]int
	LineChanges             []diff.LineChangeKind
	Bookmarks               map[int]Bookmark
	OnBookmarkChange        func(line int, action string, b Bookmark)
	OnGutterClick           func(line int) bool
	ReadOnly                bool
	Passive                 bool
	Embedded                bool
	NoWrapMargin            bool
	wrapCols                int
	overlayGen              uint64
	layoutCache             [4]*rowLayout
	layoutNext              int
	bracketColorCache       bracketColorMap
	bracketColorDirty       bool
	bracketMatchCache       bracketMatch
	bracketGen              int
	rowMap                  []editorRow
	rowMapTop               int
	rowMapOffset            int
	topRowOffset            int
	topOffsetLine           int
	phantoms                map[int]int
	DiffOverlay             *DiffOverlay
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
	if e.DiffOverlay.diffGutter() {
		return e.DiffOverlay.gutterWidth()
	}
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

func (e *EditorPaneWidget) bookmarkColumn() int {
	if e.GutterStyle == "extended" {
		return 1
	}
	return 0
}

func (e *EditorPaneWidget) validBookmarkLine(line int) bool {
	return e.Buf != nil && line >= 0 && line < len(e.Buf.Lines)
}

func (e *EditorPaneWidget) SetBookmark(line int, icon rune, style term.Style) {
	if !e.validBookmarkLine(line) {
		return
	}
	if e.Bookmarks == nil {
		e.Bookmarks = make(map[int]Bookmark)
	}
	b := Bookmark{Icon: icon, Style: style}
	e.Bookmarks[line] = b
	e.fireBookmarkChange(line, "set", b)
}

func (e *EditorPaneWidget) GetBookmark(line int) (Bookmark, bool) {
	b, ok := e.Bookmarks[line]
	return b, ok
}

func (e *EditorPaneWidget) RemoveBookmark(line int) {
	if _, ok := e.Bookmarks[line]; !ok {
		return
	}
	delete(e.Bookmarks, line)
	e.fireBookmarkChange(line, "remove", Bookmark{})
}

func (e *EditorPaneWidget) SetAllBookmarks(items map[int]Bookmark) {
	for line := range items {
		if !e.validBookmarkLine(line) {
			delete(items, line)
		}
	}
	e.Bookmarks = items
	e.fireBookmarkChange(-1, "set_all", Bookmark{})
}

func (e *EditorPaneWidget) GetAllBookmarks() map[int]Bookmark {
	return e.Bookmarks
}

func (e *EditorPaneWidget) ClearBookmarks() {
	e.Bookmarks = nil
	e.fireBookmarkChange(-1, "clear", Bookmark{})
}

func (e *EditorPaneWidget) fireBookmarkChange(line int, action string, b Bookmark) {
	if e.OnBookmarkChange != nil {
		e.OnBookmarkChange(line, action, b)
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
	return e.rowAt(y).bufLine
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
	if undo.IsNoop(cmd) {
		return
	}
	if e.Undo != nil {
		e.Undo.Push(cmd)
	}
	e.markBufferDirty()
	if e.Folds != nil && len(e.Buf.Lines) != prevLines {
		e.Folds.SetRanges(fold.ComputeIndentRanges(e.Buf.Lines))
	}
}

func (e *EditorPaneWidget) ExecCommand(cmd undo.EditCommand) { e.exec(cmd) }

func (e *EditorPaneWidget) markBufferDirty() {
	e.bufferDirty = true
}

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

func (e *EditorPaneWidget) diffHiddenRange(line int) (gapLine, end int, ok bool) {
	o := e.DiffOverlay
	if !o.hasHidden() {
		return 0, 0, false
	}
	i := sort.Search(len(o.Hidden), func(i int) bool { return o.Hidden[i].end >= line })
	if i < len(o.Hidden) && o.Hidden[i].start <= line {
		return o.Hidden[i].start - 1, o.Hidden[i].end, true
	}
	return 0, 0, false
}

func (e *EditorPaneWidget) clampCursorCol() {
	if lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line])); e.Cursor.Col > lineLen {
		e.Cursor.Col = lineLen
	}
}

func (e *EditorPaneWidget) skipHiddenLineDown() {
	if _, end, ok := e.diffHiddenRange(e.Cursor.Line); ok {
		e.Cursor.Line = e.Buf.ClampLine(end + 1)
		e.clampCursorCol()
	}
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
	if gapLine, _, ok := e.diffHiddenRange(e.Cursor.Line); ok {
		e.Cursor.Line = e.Buf.ClampLine(gapLine)
		e.clampCursorCol()
	}
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
	l := e.layout()
	curRow, _ := l.rowOf(e.Cursor.Line, e.Cursor.Col)
	top := e.topRow(l)
	if curRow < top {
		e.setTopRow(l, curRow)
	}
	if curRow >= top+e.Viewport.Height {
		e.setTopRow(l, curRow-e.Viewport.Height+1)
	}
	if e.WordWrap {
		e.Viewport.LeftCol = 0
		return
	}
	e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line)
	if e.Viewport.Width <= 0 {
		e.hScrollPending = true
		return
	}
	visCol := bufColToVisualCol(e.Buf.Lines[e.Cursor.Line], e.Cursor.Col, e.resolveTabSize())
	if visCol < e.Viewport.LeftCol {
		e.Viewport.LeftCol = visCol
	}
	if visCol >= e.Viewport.LeftCol+e.Viewport.Width {
		e.Viewport.LeftCol = visCol - e.Viewport.Width + 1
	}
}

func (e *EditorPaneWidget) scrollUp(n int) {
	l := e.layout()
	e.setTopRow(l, e.topRow(l)-n)
}

func (e *EditorPaneWidget) scrollDown(n int) {
	l := e.layout()
	newTop := e.topRow(l) + n
	if total := l.total(); newTop >= total {
		newTop = total - 1
	}
	e.setTopRow(l, newTop)
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
