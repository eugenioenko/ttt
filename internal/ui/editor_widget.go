package ui

import (
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/highlight"
	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

type EditorPaneWidget struct {
	BaseWidget
	Buf                *buffer.Buffer
	Cursor             *cursor.Cursor
	Viewport           *view.Viewport
	Undo               *undo.UndoStack
	Selection          *selection.Selection
	CursorX            int
	CursorY            int
	TabSize            int
	LineNumbers        bool
	GutterStyle        string
	Highlighter        *highlight.Highlighter
	SearchQuery        string
	SearchMatches      []FindMatch
	SearchActive       int
	lastClickTime      int64
	lastClickLine      int
	lastClickCol       int
	clickCount         int
	mouseDown          bool
	scrollbar          Scrollbar
	hscrollbar         HScrollbar
	Diagnostics        []Diagnostic
	Folds              *fold.State
	OnChange           func()
	Multi              *multicursor.MultiCursor
	multiSearchWord    string
	maxLineWidth       int
	maxLineWidthDirty  bool
	gutterHover        bool
	gutterHoverLine    int
	cachedVisibleLines []int
	searchByLine       map[int][]int
	diagByLine         map[int][]int
	Bookmarks          map[int]bool
}

func NewEditorPaneWidget(buf *buffer.Buffer, cur *cursor.Cursor, vp *view.Viewport) *EditorPaneWidget {
	return &EditorPaneWidget{
		Buf:      buf,
		Cursor:   cur,
		Viewport: vp,
	}
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
	if !e.maxLineWidthDirty && e.maxLineWidth > 0 {
		return e.maxLineWidth
	}
	maxW := 0
	for _, line := range e.Buf.Lines {
		if lw := len([]rune(line)); lw > maxW {
			maxW = lw
		}
	}
	e.maxLineWidth = maxW
	e.maxLineWidthDirty = false
	return maxW
}

func (e *EditorPaneWidget) InvalidateMaxLineWidth() {
	e.maxLineWidthDirty = true
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
	return e.Folds != nil && e.Folds.HasCollapsedFolds()
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
	if e.cachedVisibleLines == nil {
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

func (e *EditorPaneWidget) Render(surface *RenderSurface) {
	w, h := surface.Size()

	totalLines := len(e.Buf.Lines)
	gutterW := e.GutterWidth()

	maxLineW := e.computeMaxLineWidth()

	editorW := w - gutterW
	showHScrollbar := maxLineW > editorW
	if showHScrollbar {
		h--
	}

	foldsActive := e.hasFolds()
	if foldsActive {
		e.ensureTopLineVisible()
		e.cachedVisibleLines = e.Folds.VisibleLines(totalLines)
	} else {
		e.cachedVisibleLines = nil
	}

	visibleCount := totalLines
	if foldsActive {
		visibleCount = len(e.cachedVisibleLines)
	}

	showScrollbar := visibleCount > h
	if showScrollbar {
		editorW--
	}
	if editorW < 1 {
		editorW = 1
	}

	e.Viewport.Width = editorW
	e.Viewport.Height = h

	sel := e.Selection
	hasSel := sel != nil && sel.Active

	multiActive := e.isMultiActive()
	var allCursors []multicursor.CursorState
	if multiActive {
		e.syncToMulti()
		allCursors = e.Multi.Cursors
	}

	hasSearch := len(e.SearchMatches) > 0

	matchLine, matchCol, hasMatch := e.findMatchingBracket()

	if e.Viewport.TopLine < 0 {
		e.Viewport.TopLine = 0
	}
	for y := 0; y < h; y++ {
		lineIdx := e.screenToBufferLine(y)

		if gutterW > 0 {
			gutterStyle := term.StyleLineNumber
			if lineIdx < totalLines && lineIdx == e.Cursor.Line {
				gutterStyle = term.StyleActiveLine
			}
			var padded string
			if !e.LineNumbers {
				padded = strings.Repeat(" ", gutterW)
			} else {
				numStr := ""
				if lineIdx < totalLines {
					numStr = strconv.Itoa(lineIdx + 1)
				}
				switch e.GutterStyle {
				case "minimal":
					padded = strings.Repeat(" ", gutterW-1-len(numStr)) + numStr + " "
				case "extended":
					padded = "  " + strings.Repeat(" ", gutterW-5-len(numStr)) + numStr + "   "
				default:
					padded = " " + strings.Repeat(" ", gutterW-3-len(numStr)) + numStr + "  "
				}
			}
			for i, ch := range padded {
				surface.SetCell(i, y, term.Cell{Ch: ch, Style: gutterStyle})
			}
			if e.Folds != nil && lineIdx < totalLines {
				if fr := e.Folds.FoldAt(lineIdx); fr != nil {
					chevronCol := gutterW - 2
					collapsedCh := '▶'
					expandedCh := '▼'
					if e.GutterStyle == "minimal" {
						chevronCol = gutterW - 1
						collapsedCh = '▸'
						expandedCh = '▾'
					}
					if e.Folds.IsCollapsed(lineIdx) {
						surface.SetCell(chevronCol, y, term.Cell{Ch: collapsedCh, Style: gutterStyle})
					} else if e.gutterHover {
						surface.SetCell(chevronCol, y, term.Cell{Ch: expandedCh, Style: gutterStyle})
					}
				}
			}
			if lineIdx < totalLines && e.Bookmarks != nil && e.Bookmarks[lineIdx] {
				surface.SetCell(0, y, term.Cell{Ch: '●', Style: term.StyleBookmark})
			}
		}

		if lineIdx < totalLines {
			line := []rune(e.Buf.Lines[lineIdx])
			var syntaxSpans []highlight.Span
			if e.Highlighter != nil {
				syntaxSpans = e.Highlighter.HighlightLine(e.Buf.Lines[lineIdx])
			}

			isCollapsedLine := e.Folds != nil && e.Folds.IsCollapsed(lineIdx)
			var annRunes []rune
			if isCollapsedLine {
				annRunes = []rune(" ⋯")
			}

			lineEnd := len(line)
			for x := 0; x < editorW; x++ {
				colIdx := e.Viewport.LeftCol + x
				ch := ' '
				style := term.StyleDefault

				if isCollapsedLine && colIdx >= lineEnd && len(annRunes) > 0 {
					annIdx := colIdx - lineEnd
					if annIdx < len(annRunes) {
						ch = annRunes[annIdx]
						style = term.StyleLineNumber
					}
				} else if colIdx < len(line) {
					ch = line[colIdx]
					for _, sp := range syntaxSpans {
						if colIdx >= sp.Start && colIdx < sp.End {
							style = sp.Style
							break
						}
					}
				}

				if hasSearch {
					if indices, ok := e.searchByLine[lineIdx]; ok {
						for _, mi := range indices {
							m := e.SearchMatches[mi]
							if colIdx >= m.Col && colIdx < m.Col+m.Len {
								if mi == e.SearchActive {
									style = term.StyleSearchActive
								} else {
									style = term.StyleSearchMatch
								}
								break
							}
						}
					}
				}
				isSearchHighlight := style == term.StyleSearchActive || style == term.StyleSearchMatch
				bgStyle := term.Style(0)
				inAnySel := false
				if multiActive {
					for _, mc := range allCursors {
						if mc.Sel.Active && mc.Sel.Contains(lineIdx, colIdx, mc.Line, mc.Col) {
							bgStyle = term.StyleSelection
							inAnySel = true
							break
						}
					}
				} else if hasSel && sel.Contains(lineIdx, colIdx, e.Cursor.Line, e.Cursor.Col) {
					bgStyle = term.StyleSelection
					inAnySel = true
				}
				if !inAnySel {
					isCursorLine := false
					if multiActive {
						for _, mc := range allCursors {
							if mc.Line == lineIdx {
								isCursorLine = true
								break
							}
						}
					} else {
						isCursorLine = lineIdx == e.Cursor.Line && !hasSel
					}
					if isCursorLine && !isSearchHighlight {
						bgStyle = term.StyleActiveLine
					}
				}
				if hasMatch && ((lineIdx == e.Cursor.Line && colIdx == e.Cursor.Col) ||
					(lineIdx == matchLine && colIdx == matchCol)) {
					bgStyle = term.StyleBracketMatch
				}
				if multiActive {
					for _, mc := range allCursors {
						if mc.Line == lineIdx && mc.Col == colIdx {
							style = term.StyleSelection
							bgStyle = 0
							break
						}
					}
				}
				ulStyle := e.diagStyleAt(lineIdx, colIdx)
				surface.SetCell(gutterW+x, y, term.Cell{Ch: ch, Style: style, BgStyle: bgStyle, UlStyle: ulStyle})
			}
		} else {
			for x := 0; x < editorW; x++ {
				surface.SetCell(gutterW+x, y, term.Cell{Ch: ' '})
			}
		}
	}

	scrollbarCol := w - 1
	if showHScrollbar {
		scrollbarCol = w - 1
	}
	if showScrollbar {
		r := e.GetRect()
		e.scrollbar.X = r.X + scrollbarCol
		e.scrollbar.Y = r.Y
		e.scrollbar.Height = h
		if foldsActive {
			e.scrollbar.TotalItems = visibleCount + h - 1
			topVis := e.Folds.BufferToVisible(e.Viewport.TopLine)
			if topVis < 0 {
				topVis = 0
			}
			e.scrollbar.TopItem = topVis
		} else {
			e.scrollbar.TotalItems = totalLines + h - 1
			e.scrollbar.TopItem = e.Viewport.TopLine
		}
		e.scrollbar.Render(surface, scrollbarCol, 0)
	}

	if showHScrollbar {
		r := e.GetRect()
		trackW := editorW
		e.hscrollbar.X = r.X + gutterW
		e.hscrollbar.Y = r.Y + h
		e.hscrollbar.Width = trackW
		e.hscrollbar.TotalCols = maxLineW
		e.hscrollbar.LeftCol = e.Viewport.LeftCol
		e.hscrollbar.Render(surface, gutterW, h)
	}

	r := e.GetRect()
	e.CursorX = e.Cursor.Col - e.Viewport.LeftCol + gutterW + r.X
	if foldsActive {
		curVis := e.Folds.BufferToVisible(e.Cursor.Line)
		topVis := e.Folds.BufferToVisible(e.Viewport.TopLine)
		if curVis >= 0 && topVis >= 0 {
			e.CursorY = curVis - topVis + r.Y
		} else {
			e.CursorY = e.Cursor.Line - e.Viewport.TopLine + r.Y
		}
	} else {
		e.CursorY = e.Cursor.Line - e.Viewport.TopLine + r.Y
	}
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

func (e *EditorPaneWidget) diagStyleAt(line, col int) term.Style {
	indices, ok := e.diagByLine[line]
	if !ok {
		return 0
	}
	for _, i := range indices {
		d := e.Diagnostics[i]
		if line == d.StartLine && col < d.StartCol {
			continue
		}
		if line == d.EndLine && col >= d.EndCol {
			continue
		}
		switch d.Severity {
		case DiagError:
			return term.StyleDiagError
		case DiagWarning:
			return term.StyleDiagWarning
		case DiagInformation:
			return term.StyleDiagInfo
		case DiagHint:
			return term.StyleDiagHint
		default:
			return term.StyleDiagError
		}
	}
	return 0
}

func (e *EditorPaneWidget) exec(cmd undo.EditCommand) {
	prevLines := len(e.Buf.Lines)
	cmd.Apply(e.Buf)
	if e.Undo != nil {
		e.Undo.Push(cmd)
	}
	e.maxLineWidthDirty = true
	if e.Folds != nil && len(e.Buf.Lines) != prevLines {
		e.Folds.SetRanges(fold.ComputeIndentRanges(e.Buf.Lines))
	}
	if e.OnChange != nil {
		e.OnChange()
	}
}

func (e *EditorPaneWidget) ExecCommand(cmd undo.EditCommand) { e.exec(cmd) }

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
		btn := mev.Buttons()

		if newTop, consumed := e.scrollbar.HandleEvent(ev); consumed {
			if e.Folds != nil && e.Folds.HasCollapsedFolds() {
				e.Viewport.TopLine = e.Folds.VisibleToBuffer(newTop)
			} else {
				e.Viewport.TopLine = newTop
			}
			if e.scrollbar.IsDragging() {
				return EventCaptured
			}
			return EventConsumed
		}
		if e.scrollbar.IsDragging() {
			return EventCaptured
		}
		if newLeft, consumed := e.hscrollbar.HandleEvent(ev); consumed {
			e.Viewport.LeftCol = newLeft
			if e.hscrollbar.IsDragging() {
				return EventCaptured
			}
			return EventConsumed
		}
		if e.hscrollbar.IsDragging() {
			return EventCaptured
		}

		mod := mev.Modifiers()
		if btn&tcell.WheelUp != 0 {
			if mod&tcell.ModShift != 0 {
				e.Viewport.LeftCol -= 4
				if e.Viewport.LeftCol < 0 {
					e.Viewport.LeftCol = 0
				}
			} else {
				e.scrollUp(3)
			}
			return EventConsumed
		}
		if btn&tcell.WheelDown != 0 {
			if mod&tcell.ModShift != 0 {
				e.Viewport.LeftCol += 4
			} else {
				e.scrollDown(3)
			}
			return EventConsumed
		}
		if btn&tcell.WheelLeft != 0 {
			e.Viewport.LeftCol -= 4
			if e.Viewport.LeftCol < 0 {
				e.Viewport.LeftCol = 0
			}
			return EventConsumed
		}
		if btn&tcell.WheelRight != 0 {
			e.Viewport.LeftCol += 4
			return EventConsumed
		}

		r := e.GetRect()
		mx, my := mev.Position()
		gutterW := e.GutterWidth()
		inGutter := gutterW > 0 && mx >= r.X && mx < r.X+gutterW

		if btn == tcell.ButtonNone && !e.mouseDown {
			prevHover := e.gutterHover
			prevLine := e.gutterHoverLine
			if inGutter {
				screenY := my - r.Y
				bufLine := e.screenToBufferLine(screenY)
				e.gutterHover = true
				e.gutterHoverLine = bufLine
			} else {
				e.gutterHover = false
			}
			if e.gutterHover != prevHover || e.gutterHoverLine != prevLine {
				return EventConsumed
			}
		}

		if btn&tcell.Button1 != 0 && inGutter && !e.mouseDown {
			screenY := my - r.Y
			bufLine := e.screenToBufferLine(screenY)
			if e.Folds != nil {
				if e.Folds.FoldAt(bufLine) != nil {
					e.Folds.Toggle(bufLine)
					return EventConsumed
				}
			}
		}

		if btn&tcell.Button1 != 0 {
			if e.Undo != nil {
				e.Undo.BreakGroup()
			}
			line, col := e.mouseToPos(r, mx, my)

			isAlt := mev.Modifiers()&tcell.ModAlt != 0
			if isAlt && !e.mouseDown {
				e.ensureMulti()
				e.syncToMulti()
				e.Multi.Add(line, col)
				e.syncFromMulti()
				e.scrollViewport()
				return EventCaptured
			}

			if !e.mouseDown {
				e.mouseDown = true

				if e.isMultiActive() {
					e.collapseMulti()
				}

				now := time.Now().UnixMilli()
				if now-e.lastClickTime < 400 && line == e.lastClickLine && col == e.lastClickCol {
					e.clickCount++
				} else {
					e.clickCount = 1
				}
				e.lastClickTime = now
				e.lastClickLine = line
				e.lastClickCol = col

				switch e.clickCount {
				case 2:
					e.selectWord(line, col)
				case 3:
					e.selectLine(line)
					e.clickCount = 0
				default:
					if e.Selection != nil {
						e.Selection.Clear()
						e.Selection.Start(line, col)
					}
					e.Cursor.Line = line
					e.Cursor.Col = col
				}
			} else {
				e.Cursor.Line = line
				e.Cursor.Col = col
			}
			e.scrollViewport()
			return EventCaptured
		}
		if btn == tcell.ButtonNone && e.mouseDown {
			e.mouseDown = false
			if e.Selection != nil && e.Selection.Active {
				if e.Selection.Anchor.Line == e.Cursor.Line && e.Selection.Anchor.Col == e.Cursor.Col {
					e.Selection.Clear()
				}
			}
		}
		return EventIgnored
	}

	kev, ok := ev.(*tcell.EventKey)
	if !ok {
		return EventIgnored
	}

	switch kev.Key() {
	case tcell.KeyRune, tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete:
	default:
		if e.Undo != nil {
			e.Undo.BreakGroup()
		}
	}

	shift := kev.Modifiers()&tcell.ModShift != 0
	hasSel := e.Selection != nil && e.Selection.Active

	multi := e.isMultiActive()

	switch kev.Key() {
	case tcell.KeyUp:
		if multi {
			e.multiMoveAll(func(cs *multicursor.CursorState) {
				if cs.Line > 0 {
					cs.Line--
					if e.Folds != nil {
						if r := e.Folds.ContainingFold(cs.Line); r != nil {
							cs.Line = r.StartLine
						}
					}
					lineLen := len([]rune(e.Buf.Lines[cs.Line]))
					if cs.Col > lineLen {
						cs.Col = lineLen
					}
				}
			})
		} else {
			e.startOrExtendSelection(shift)
			if e.Cursor.Line > 0 {
				e.Cursor.Line--
				e.skipHiddenLineUp()
				lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
				if e.Cursor.Col > lineLen {
					e.Cursor.Col = lineLen
				}
			}
		}
	case tcell.KeyDown:
		if multi {
			e.multiMoveAll(func(cs *multicursor.CursorState) {
				if cs.Line < len(e.Buf.Lines)-1 {
					cs.Line++
					if e.Folds != nil {
						if r := e.Folds.ContainingFold(cs.Line); r != nil {
							cs.Line = r.EndLine + 1
							if cs.Line >= len(e.Buf.Lines) {
								cs.Line = len(e.Buf.Lines) - 1
							}
						}
					}
					lineLen := len([]rune(e.Buf.Lines[cs.Line]))
					if cs.Col > lineLen {
						cs.Col = lineLen
					}
				}
			})
		} else {
			e.startOrExtendSelection(shift)
			if e.Cursor.Line < len(e.Buf.Lines)-1 {
				e.Cursor.Line++
				e.skipHiddenLineDown()
				lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
				if e.Cursor.Col > lineLen {
					e.Cursor.Col = lineLen
				}
			}
		}
	case tcell.KeyLeft:
		if multi {
			e.multiMoveAll(func(cs *multicursor.CursorState) {
				if cs.Col > 0 {
					cs.Col--
				} else if cs.Line > 0 {
					cs.Line--
					cs.Col = len([]rune(e.Buf.Lines[cs.Line]))
				}
			})
		} else {
			e.startOrExtendSelection(shift)
			if e.Cursor.Col > 0 {
				e.Cursor.Col--
			} else if e.Cursor.Line > 0 {
				e.Cursor.Line--
				e.Cursor.Col = len([]rune(e.Buf.Lines[e.Cursor.Line]))
			}
		}
	case tcell.KeyRight:
		if multi {
			e.multiMoveAll(func(cs *multicursor.CursorState) {
				lineLen := len([]rune(e.Buf.Lines[cs.Line]))
				if cs.Col < lineLen {
					cs.Col++
				} else if cs.Line < len(e.Buf.Lines)-1 {
					cs.Line++
					cs.Col = 0
				}
			})
		} else {
			e.startOrExtendSelection(shift)
			lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
			if e.Cursor.Col < lineLen {
				e.Cursor.Col++
			} else if e.Cursor.Line < len(e.Buf.Lines)-1 {
				e.Cursor.Line++
				e.Cursor.Col = 0
			}
		}
	case tcell.KeyHome:
		if multi {
			e.multiMoveAll(func(cs *multicursor.CursorState) {
				runes := []rune(e.Buf.Lines[cs.Line])
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
			e.SmartHome()
		}
	case tcell.KeyEnd:
		if multi {
			e.multiMoveAll(func(cs *multicursor.CursorState) {
				cs.Col = len([]rune(e.Buf.Lines[cs.Line]))
			})
		} else {
			e.startOrExtendSelection(shift)
			e.Cursor.Col = len([]rune(e.Buf.Lines[e.Cursor.Line]))
		}
	case tcell.KeyPgUp:
		e.startOrExtendSelection(shift)
		e.Cursor.Line -= e.Viewport.Height
		if e.Cursor.Line < 0 {
			e.Cursor.Line = 0
		}
		e.skipHiddenLineUp()
		lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
		if e.Cursor.Col > lineLen {
			e.Cursor.Col = lineLen
		}
	case tcell.KeyPgDn:
		e.startOrExtendSelection(shift)
		e.Cursor.Line += e.Viewport.Height
		if e.Cursor.Line >= len(e.Buf.Lines) {
			e.Cursor.Line = len(e.Buf.Lines) - 1
		}
		e.skipHiddenLineDown()
		lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
		if e.Cursor.Col > lineLen {
			e.Cursor.Col = lineLen
		}
	case tcell.KeyEnter:
		e.expandFoldAtCursor()
		if multi {
			e.multiExecEnter()
		} else {
			if hasSel {
				e.deleteSelection()
			}
			col := e.Cursor.Col
			if col < 0 {
				col = 0
			}
			lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
			if col > lineLen {
				col = lineLen
			}
			line := e.Buf.Lines[e.Cursor.Line]
			indent := leadingWhitespace(line)
			runes := []rune(line)
			charBefore := ' '
			if col > 0 && col <= len(runes) {
				charBefore = runes[col-1]
			}
			charAfter := ' '
			if col < len(runes) {
				charAfter = runes[col]
			}
			extraIndent := charBefore == '{' || charBefore == '(' || charBefore == '[' || charBefore == ':'
			e.exec(&undo.SplitLineCommand{Line: e.Cursor.Line, Col: col})
			e.Cursor.Line++
			e.Cursor.Col = 0
			tabSize := e.TabSize
			if tabSize <= 0 {
				tabSize = 4
			}
			newIndent := indent
			if extraIndent {
				newIndent += strings.Repeat(" ", tabSize)
			}
			if len(newIndent) > 0 {
				e.exec(&undo.InsertStringCommand{Line: e.Cursor.Line, Col: 0, Text: newIndent})
				e.Cursor.Col = len([]rune(newIndent))
			}
			if extraIndent && (charAfter == '}' || charAfter == ')' || charAfter == ']') {
				e.exec(&undo.SplitLineCommand{Line: e.Cursor.Line, Col: e.Cursor.Col})
				e.exec(&undo.InsertStringCommand{Line: e.Cursor.Line + 1, Col: 0, Text: indent})
			}
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		e.expandFoldAtCursor()
		if multi {
			e.multiExecBackspace()
		} else {
			if hasSel {
				e.deleteSelection()
			} else if e.Cursor.Col > 0 {
				runes := []rune(e.Buf.Lines[e.Cursor.Line])
				if e.Cursor.Col > len(runes) {
					e.Cursor.Col = len(runes)
				}
				inLeadingWhitespace := true
				for i := 0; i < e.Cursor.Col && i < len(runes); i++ {
					if runes[i] != ' ' && runes[i] != '\t' {
						inLeadingWhitespace = false
						break
					}
				}
				tabSize := e.TabSize
				if tabSize <= 0 {
					tabSize = 4
				}
				if inLeadingWhitespace && e.Cursor.Col > 1 && runes[e.Cursor.Col-1] == ' ' {
					target := ((e.Cursor.Col - 1) / tabSize) * tabSize
					if target == e.Cursor.Col {
						target -= tabSize
					}
					if target < 0 {
						target = 0
					}
					e.exec(&undo.DeleteSelectionCommand{
						StartLine: e.Cursor.Line, StartCol: target,
						EndLine: e.Cursor.Line, EndCol: e.Cursor.Col,
					})
					e.Cursor.Col = target
				} else {
					e.exec(&undo.DeleteRuneCommand{Line: e.Cursor.Line, Col: e.Cursor.Col - 1})
					e.Cursor.Col--
				}
			} else if e.Cursor.Line > 0 {
				cmd := &undo.JoinLineCommand{Line: e.Cursor.Line}
				e.exec(cmd)
				e.Cursor.Line--
				e.Cursor.Col = cmd.PrevLen
			}
		}
	case tcell.KeyDelete:
		e.expandFoldAtCursor()
		if multi {
			e.multiExecDelete()
		} else {
			if hasSel {
				e.deleteSelection()
			} else {
				lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
				if e.Cursor.Col < lineLen {
					e.exec(&undo.DeleteRuneCommand{Line: e.Cursor.Line, Col: e.Cursor.Col})
				} else if e.Cursor.Line < len(e.Buf.Lines)-1 {
					e.exec(&undo.JoinLineCommand{Line: e.Cursor.Line + 1})
				}
			}
		}
	case tcell.KeyRune:
		if kev.Modifiers() == 0 {
			e.expandFoldAtCursor()
			r := kev.Rune()
			if r != 0 {
				if multi {
					e.multiExecRune(r)
				} else if hasSel {
					if closing, ok := autoPairs[r]; ok {
						e.deleteSelection()
						e.exec(&undo.InsertRuneCommand{Line: e.Cursor.Line, Col: e.Cursor.Col, Rune: r})
						e.Cursor.Col++
						e.exec(&undo.InsertRuneCommand{Line: e.Cursor.Line, Col: e.Cursor.Col, Rune: closing})
					} else {
						e.deleteSelection()
						e.exec(&undo.InsertRuneCommand{Line: e.Cursor.Line, Col: e.Cursor.Col, Rune: r})
						e.Cursor.Col++
					}
				} else if closing, skip := autoCloseSkip[r]; skip && e.charAtCursor() == r {
					_ = closing
					e.Cursor.Col++
				} else if closing, ok := autoPairs[r]; ok {
					e.exec(&undo.InsertRuneCommand{Line: e.Cursor.Line, Col: e.Cursor.Col, Rune: r})
					e.Cursor.Col++
					e.exec(&undo.InsertRuneCommand{Line: e.Cursor.Line, Col: e.Cursor.Col, Rune: closing})
				} else {
					e.exec(&undo.InsertRuneCommand{Line: e.Cursor.Line, Col: e.Cursor.Col, Rune: r})
					e.Cursor.Col++
				}
			}
		} else {
			return EventIgnored
		}
	case tcell.KeyBacktab:
		tabSize := e.TabSize
		if tabSize <= 0 {
			tabSize = 4
		}
		if hasSel {
			start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
			for line := start.Line; line <= end.Line; line++ {
				runes := []rune(e.Buf.Lines[line])
				remove := 0
				for remove < tabSize && remove < len(runes) && runes[remove] == ' ' {
					remove++
				}
				if remove > 0 {
					e.exec(&undo.DeleteSelectionCommand{
						StartLine: line, StartCol: 0,
						EndLine: line, EndCol: remove,
					})
				}
			}
		} else {
			runes := []rune(e.Buf.Lines[e.Cursor.Line])
			remove := 0
			for remove < tabSize && remove < len(runes) && runes[remove] == ' ' {
				remove++
			}
			if remove > 0 {
				e.exec(&undo.DeleteSelectionCommand{
					StartLine: e.Cursor.Line, StartCol: 0,
					EndLine: e.Cursor.Line, EndCol: remove,
				})
				e.Cursor.Col -= remove
				if e.Cursor.Col < 0 {
					e.Cursor.Col = 0
				}
			}
		}
	case tcell.KeyTab:
		tabSize := e.TabSize
		if tabSize <= 0 {
			tabSize = 4
		}
		if hasSel {
			start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
			indent := strings.Repeat(" ", tabSize)
			for line := start.Line; line <= end.Line; line++ {
				e.exec(&undo.InsertStringCommand{Line: line, Col: 0, Text: indent})
			}
		} else {
			e.exec(&undo.InsertStringCommand{Line: e.Cursor.Line, Col: e.Cursor.Col, Text: strings.Repeat(" ", tabSize)})
			e.Cursor.Col += tabSize
		}
	default:
		return EventIgnored
	}

	e.clampCursor()
	e.scrollViewport()
	return EventConsumed
}

func (e *EditorPaneWidget) mouseToPos(r Rect, mx, my int) (line, col int) {
	if len(e.Buf.Lines) == 0 {
		return 0, 0
	}
	gutterW := e.GutterWidth()
	screenY := my - r.Y
	line = e.screenToBufferLine(screenY)
	col = mx - r.X - gutterW + e.Viewport.LeftCol
	if col < 0 {
		col = 0
	}
	if line < 0 {
		line = 0
	}
	if line >= len(e.Buf.Lines) {
		line = len(e.Buf.Lines) - 1
	}
	lineLen := len([]rune(e.Buf.Lines[line]))
	if col > lineLen {
		col = lineLen
	}
	return
}

func (e *EditorPaneWidget) selectWord(line, col int) {
	if e.Selection == nil {
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

func leadingWhitespace(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return s[:i]
		}
	}
	return s
}

var autoPairs = map[rune]rune{
	'(': ')', '{': '}', '[': ']',
	'"': '"', '\'': '\'', '`': '`',
}

var autoCloseSkip = map[rune]bool{
	')': true, '}': true, ']': true,
	'"': true, '\'': true, '`': true,
}

func (e *EditorPaneWidget) charAtCursor() rune {
	if e.Cursor.Line >= len(e.Buf.Lines) {
		return 0
	}
	runes := []rune(e.Buf.Lines[e.Cursor.Line])
	if e.Cursor.Col >= len(runes) {
		return 0
	}
	return runes[e.Cursor.Col]
}

var bracketPairs = map[rune]rune{
	'(': ')', ')': '(',
	'[': ']', ']': '[',
	'{': '}', '}': '{',
}

var closingBrackets = map[rune]bool{')': true, ']': true, '}': true}

func (e *EditorPaneWidget) findMatchingBracket() (int, int, bool) {
	if e.Cursor.Line < 0 || e.Cursor.Line >= len(e.Buf.Lines) {
		return 0, 0, false
	}
	runes := []rune(e.Buf.Lines[e.Cursor.Line])
	if e.Cursor.Col < 0 || e.Cursor.Col >= len(runes) {
		return 0, 0, false
	}
	ch := runes[e.Cursor.Col]
	match, ok := bracketPairs[ch]
	if !ok {
		return 0, 0, false
	}
	dir := 1
	if closingBrackets[ch] {
		dir = -1
	}
	depth := 1
	line, col := e.Cursor.Line, e.Cursor.Col
	for {
		col += dir
		lr := []rune(e.Buf.Lines[line])
		if col < 0 {
			line--
			if line < 0 {
				return 0, 0, false
			}
			lr = []rune(e.Buf.Lines[line])
			col = len(lr) - 1
			if col < 0 {
				col = 0
				continue
			}
		} else if col >= len(lr) {
			line++
			if line >= len(e.Buf.Lines) {
				return 0, 0, false
			}
			col = 0
			lr = []rune(e.Buf.Lines[line])
			if len(lr) == 0 {
				continue
			}
		}
		c := lr[col]
		if c == ch {
			depth++
		} else if c == match {
			depth--
			if depth == 0 {
				return line, col, true
			}
		}
	}
}

func (e *EditorPaneWidget) clampCursor() {
	if e.Cursor.Line < 0 {
		e.Cursor.Line = 0
	}
	if e.Cursor.Line >= len(e.Buf.Lines) {
		e.Cursor.Line = len(e.Buf.Lines) - 1
	}
	if e.Cursor.Col < 0 {
		e.Cursor.Col = 0
	}
}

func (e *EditorPaneWidget) skipHiddenLineDown() {
	if e.Folds == nil {
		return
	}
	if r := e.Folds.ContainingFold(e.Cursor.Line); r != nil {
		e.Cursor.Line = r.EndLine + 1
		if e.Cursor.Line >= len(e.Buf.Lines) {
			e.Cursor.Line = len(e.Buf.Lines) - 1
		}
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
	if e.Cursor.Col < e.Viewport.LeftCol {
		e.Viewport.LeftCol = e.Cursor.Col
	}
	if e.Cursor.Col >= e.Viewport.LeftCol+e.Viewport.Width {
		e.Viewport.LeftCol = e.Cursor.Col - e.Viewport.Width + 1
	}
}

func isEditorIdentRune(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func (e *EditorPaneWidget) scrollUp(n int) {
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

func (e *EditorPaneWidget) MoveLineUp() {
	hasSel := e.Selection != nil && e.Selection.Active
	if hasSel {
		start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
		if start.Line <= 0 {
			return
		}
		for line := start.Line; line <= end.Line; line++ {
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
	hasSel := e.Selection != nil && e.Selection.Active
	if hasSel {
		start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
		if end.Line >= len(e.Buf.Lines)-1 {
			return
		}
		for line := end.Line; line >= start.Line; line-- {
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

func (e *EditorPaneWidget) DuplicateLine() {
	text := e.Buf.Lines[e.Cursor.Line]
	e.exec(&undo.InsertLineCommand{Idx: e.Cursor.Line + 1, Text: text})
	e.Cursor.Line++
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) DeleteLine() {
	if len(e.Buf.Lines) <= 1 {
		e.exec(&undo.DeleteSelectionCommand{
			StartLine: 0, StartCol: 0,
			EndLine: 0, EndCol: len([]rune(e.Buf.Lines[0])),
		})
		e.Cursor.Col = 0
	} else {
		e.exec(&undo.DeleteLineCommand{Idx: e.Cursor.Line})
		if e.Cursor.Line >= len(e.Buf.Lines) {
			e.Cursor.Line = len(e.Buf.Lines) - 1
		}
	}
	lineLen := len([]rune(e.Buf.Lines[e.Cursor.Line]))
	if e.Cursor.Col > lineLen {
		e.Cursor.Col = lineLen
	}
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) JoinLines() {
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
		e.maxLineWidthDirty = true
		if e.OnChange != nil {
			e.OnChange()
		}
	}

	e.Cursor.Col += cursorDelta
	if e.Cursor.Col < 0 {
		e.Cursor.Col = 0
	}
	e.clampCursor()
	e.scrollViewport()
}

// lineRange returns the start and end line indices for the current selection,
// or the full buffer range if no selection is active.
func (e *EditorPaneWidget) lineRange() (int, int) {
	if e.Selection != nil && e.Selection.Active {
		start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)
		endLine := end.Line
		if end.Col == 0 && endLine > start.Line {
			endLine--
		}
		return start.Line, endLine
	}
	return 0, len(e.Buf.Lines) - 1
}

// copyLines returns a copy of the buffer lines in the given range (inclusive).
func (e *EditorPaneWidget) copyLines(start, end int) []string {
	lines := make([]string, end-start+1)
	copy(lines, e.Buf.Lines[start:end+1])
	return lines
}

func (e *EditorPaneWidget) SortLinesAsc() {
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
	if e.Cursor.Line >= len(e.Buf.Lines) {
		e.Cursor.Line = len(e.Buf.Lines) - 1
	}
	e.clampCursor()
	e.scrollViewport()
}

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

func (e *EditorPaneWidget) MoveWordLeft(shift bool) {
	e.startOrExtendSelection(shift)
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
	e.startOrExtendSelection(shift)
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
	if e.Cursor.Col == 0 {
		return
	}
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
	e.syncFromMulti()
	if e.OnChange != nil {
		e.OnChange()
	}
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
		if e.OnChange != nil {
			e.OnChange()
		}
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
		if e.OnChange != nil {
			e.OnChange()
		}
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
		cmd := &undo.SplitLineCommand{Line: cs.Line, Col: cs.Col}
		cmd.Apply(e.Buf)
		cmds = append(cmds, cmd)
		e.shiftLaterLines(i, cs.Line, 1)
		cs.Line++
		cs.Col = 0
	}
	if e.Undo != nil {
		e.Undo.Push(&undo.BatchCommand{Commands: cmds})
	}
	e.syncFromMulti()
	if e.OnChange != nil {
		e.OnChange()
	}
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
		moveFn(&e.Multi.Cursors[i])
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

// transformSelection replaces the selected text with the result of applying fn.
// It preserves the selection after transformation.
func (e *EditorPaneWidget) transformSelection(fn func(string) string) {
	if e.Selection == nil || !e.Selection.Active {
		return
	}
	text := e.Selection.Text(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col)
	if text == "" {
		return
	}
	transformed := fn(text)
	if transformed == text {
		return
	}

	start, end := e.Selection.Range(e.Cursor.Line, e.Cursor.Col)

	if e.Undo != nil {
		e.Undo.BreakGroup()
	}

	// Delete the selection
	delCmd := &undo.DeleteSelectionCommand{
		StartLine: start.Line, StartCol: start.Col,
		EndLine: end.Line, EndCol: end.Col,
	}
	e.exec(delCmd)

	// Insert the transformed text
	tLines := strings.Split(transformed, "\n")
	if len(tLines) == 1 {
		e.exec(&undo.InsertStringCommand{Line: start.Line, Col: start.Col, Text: tLines[0]})
	} else {
		currentLine := []rune(e.Buf.Lines[start.Line])
		col := start.Col
		if col > len(currentLine) {
			col = len(currentLine)
		}
		suffix := string(currentLine[col:])
		e.exec(&undo.PasteCommand{
			Line:   start.Line,
			Col:    col,
			Text:   transformed,
			Suffix: suffix,
		})
	}

	// Restore the selection so the user sees the transformed range
	newEndLine := start.Line + len(tLines) - 1
	var newEndCol int
	if len(tLines) == 1 {
		newEndCol = start.Col + len([]rune(tLines[0]))
	} else {
		newEndCol = len([]rune(tLines[len(tLines)-1]))
	}

	e.Selection.Start(start.Line, start.Col)
	e.Cursor.Line = newEndLine
	e.Cursor.Col = newEndCol
	e.clampCursor()
	e.scrollViewport()
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

func (e *EditorPaneWidget) ToggleBookmark() {
	if e.Bookmarks == nil {
		e.Bookmarks = make(map[int]bool)
	}
	line := e.Cursor.Line
	if e.Bookmarks[line] {
		delete(e.Bookmarks, line)
	} else {
		e.Bookmarks[line] = true
	}
}

func (e *EditorPaneWidget) sortedBookmarks() []int {
	if len(e.Bookmarks) == 0 {
		return nil
	}
	lines := make([]int, 0, len(e.Bookmarks))
	for line := range e.Bookmarks {
		lines = append(lines, line)
	}
	sort.Ints(lines)
	return lines
}

func (e *EditorPaneWidget) NextBookmark() {
	sorted := e.sortedBookmarks()
	if len(sorted) == 0 {
		return
	}
	cur := e.Cursor.Line
	for _, line := range sorted {
		if line > cur {
			e.Cursor.Line = line
			e.Cursor.Col = 0
			e.clampCursor()
			e.scrollViewport()
			return
		}
	}
	// Wrap around to first bookmark
	e.Cursor.Line = sorted[0]
	e.Cursor.Col = 0
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) PrevBookmark() {
	sorted := e.sortedBookmarks()
	if len(sorted) == 0 {
		return
	}
	cur := e.Cursor.Line
	for i := len(sorted) - 1; i >= 0; i-- {
		if sorted[i] < cur {
			e.Cursor.Line = sorted[i]
			e.Cursor.Col = 0
			e.clampCursor()
			e.scrollViewport()
			return
		}
	}
	// Wrap around to last bookmark
	e.Cursor.Line = sorted[len(sorted)-1]
	e.Cursor.Col = 0
	e.clampCursor()
	e.scrollViewport()
}

func (e *EditorPaneWidget) ClearBookmarks() {
	e.Bookmarks = nil
}

func (e *EditorPaneWidget) HasBookmarks() bool {
	return len(e.Bookmarks) > 0
}
