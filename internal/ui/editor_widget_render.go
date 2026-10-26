package ui

import (
	"github.com/eugenioenko/ttt/internal/icons"
	"strconv"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

func (e *EditorPaneWidget) Render(surface Surface) {
	e.syncDiffOverlay()
	if e.Highlighter != nil && e.Buf != nil {
		e.Highlighter.SyncBuffer(e.Buf)
	}
	w, h := surface.Size()

	totalLines := len(e.Buf.Lines)
	gutterW := e.GutterWidth()

	maxLineW := e.computeMaxLineWidth()
	tabW := e.resolveTabSize()

	editorW := w - gutterW
	showHScrollbar := !e.Embedded && !e.WordWrap && maxLineW > editorW
	if showHScrollbar {
		h--
	}

	if e.hasFolds() {
		e.ensureTopLineVisible()
	}
	// Note rows count toward the scrollbar, so wrap them before deciding it;
	// they wrap again below if the scrollbar narrows the text.
	if e.DiffOverlay.wrapNotes(e.wrapTextWidth(max(editorW, 1)), false) {
		e.SetDiffOverlay(e.DiffOverlay)
	}
	visibleCount := e.rowLayout(e.wrapTextWidth(max(editorW, 1))).total()

	showScrollbar := !e.Embedded && visibleCount > h
	if showScrollbar {
		editorW--
	}
	editorW = e.wrapTextWidth(max(editorW, 1))

	if e.Viewport.Width != editorW {
		e.wrapGoal = wrapGoal{}
		if e.WordWrap && !e.Embedded && e.WordWrapColumn > 0 && e.Viewport.Width > 0 {
			previous := e.layout()
			row, _ := previous.rowOf(e.Cursor.Line, e.Cursor.Col)
			top := e.topRow(previous)
			e.hScrollPending = e.hScrollPending || (row >= top && row < top+e.Viewport.Height)
		}
	}
	e.Viewport.Width = editorW
	e.Viewport.Height = h
	if e.DiffOverlay.wrapNotes(editorW, false) {
		e.SetDiffOverlay(e.DiffOverlay)
	}
	if e.hScrollPending {
		e.hScrollPending = false
		e.scrollViewport()
	}

	layout := e.layout()
	if e.WordWrap {
		e.Viewport.LeftCol = 0
		visibleCount = layout.total()
		showScrollbar = !e.Embedded && visibleCount > h
	}

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
	hasMatch = hasMatch && !e.Passive

	if e.Viewport.TopLine < 0 {
		e.Viewport.TopLine = 0
	}

	var bracketColors bracketColorMap
	if e.BracketPairColorization && len(e.Buf.Lines) <= maxBracketColorLines {
		if e.bracketColorDirty {
			e.bracketColorCache = e.computeBracketColors()
			e.bracketColorDirty = false
		}
		bracketColors = e.bracketColorCache
	}

	if off := e.topOffset(); off > 0 && off >= layout.maxOffset(e.Viewport.TopLine) {
		e.topRowOffset = 0
	}
	e.rowMap = layout.appendRows(e.rowMap[:0], e.Viewport.TopLine, e.topOffset(), h)
	e.rowMapTop = e.Viewport.TopLine
	e.rowMapOffset = e.topOffset()
	topRow := e.topRow(layout)

	// A pane narrowed to a shared wrap width leaves columns past its text
	// that the row loop does not draw.
	textEnd := w
	if showScrollbar {
		textEnd--
	}
	for y := 0; y < h; y++ {
		for x := gutterW + editorW; x < textEnd; x++ {
			surface.SetCell(x, y, term.Cell{Ch: ' '})
		}
	}

	for y := 0; y < h; y++ {
		row := e.rowMap[y]
		lineIdx := row.bufLine
		segStartCol := row.startCol
		isWrapContinuation := segStartCol > 0

		if row.isPhantom() {
			e.renderPhantomRow(surface, y, gutterW, editorW, row, layout)
			continue
		}
		if label, ok := e.DiffOverlay.label(lineIdx); ok && lineIdx < totalLines && !isWrapContinuation {
			e.renderGapRow(surface, y, gutterW, editorW, lineIdx, label)
			continue
		}

		if gutterW > 0 && e.DiffOverlay.diffGutter() {
			e.renderDiffGutterRow(surface, y, gutterW, lineIdx, isWrapContinuation)
		} else if gutterW > 0 {
			gutterStyle := term.StyleLineNumber
			if lineIdx < totalLines && lineIdx == e.Cursor.Line && !e.Passive {
				gutterStyle = term.StyleActiveLine
			}
			bookmarkBg := gutterStyle
			diffKind := diff.Context
			if lineIdx < totalLines && !isWrapContinuation {
				diffKind = e.DiffOverlay.kind(lineIdx)
			}
			if e.DiffOverlay != nil && (diffKind == diff.Added || diffKind == diff.Deleted) {
				num := 0
				if e.LineNumbers {
					num = lineIdx + 1
				}
				if bg := e.renderEditorDiffGutter(surface, y, gutterW, num, diffKind, gutterStyle).bg; bg != 0 {
					bookmarkBg = bg
				}
			} else if !e.LineNumbers || (e.WordWrap && isWrapContinuation) {
				for i := 0; i < gutterW; i++ {
					surface.SetCell(i, y, term.Cell{Ch: ' ', Style: gutterStyle})
				}
			} else {
				num := 0
				if lineIdx < totalLines {
					num = lineIdx + 1
				}
				e.drawGutterNumber(surface, y, gutterW, num, term.Cell{Style: gutterStyle})
			}
			if e.Folds != nil && !e.WordWrap && lineIdx < totalLines && !isWrapContinuation {
				if fr := e.Folds.FoldAt(lineIdx); fr != nil {
					chevronCol := gutterW - 2
					collapsedCh, expandedCh := e.FoldChevronCollapsed, e.FoldChevronExpanded
					if collapsedCh == 0 {
						collapsedCh = icons.Rune(icons.ChevronRight)
					}
					if expandedCh == 0 {
						expandedCh = icons.Rune(icons.ChevronDown)
					}
					if e.GutterStyle == "minimal" {
						chevronCol = gutterW - 1
					}
					if e.Folds.IsCollapsed(lineIdx) {
						surface.SetCell(chevronCol, y, term.Cell{Ch: collapsedCh, Style: gutterStyle})
					} else if e.gutterHover {
						surface.SetCell(chevronCol, y, term.Cell{Ch: expandedCh, Style: gutterStyle})
					}
				}
			}
			if !e.DiffOverlay.isDiff() && lineIdx < totalLines && lineIdx < len(e.LineChanges) && !isWrapContinuation {
				change := e.LineChanges[lineIdx]
				if change != diff.LineUnchanged {
					var ch rune
					var style term.Style
					switch change {
					case diff.LineAdded:
						ch = '▎'
						style = term.StyleGutterAdded
					case diff.LineModified:
						ch = '▎'
						style = term.StyleGutterModified
					case diff.LineDeleted:
						ch = '▾'
						style = term.StyleGutterDeleted
					}
					surface.SetCell(0, y, term.Cell{Ch: ch, Style: style})
				}
			}
			if len(e.Bookmarks) > 0 && lineIdx < totalLines && !isWrapContinuation {
				if b, ok := e.Bookmarks[lineIdx]; ok {
					surface.SetCell(e.bookmarkColumn(), y, term.Cell{Ch: b.Icon, Style: b.Style, BgStyle: bookmarkBg})
				}
			}
		}

		if lineIdx < totalLines {
			e.runeScratch = e.runeScratch[:0]
			for _, r := range e.Buf.Lines[lineIdx] {
				e.runeScratch = append(e.runeScratch, r)
			}
			line := e.runeScratch
			var syntaxSpans []highlight.Span
			diffFg, diffFgOverride, diffFgFull := e.diffLineFg(lineIdx)
			if diffFgOverride {
				syntaxSpans = []highlight.Span{{Start: 0, End: len(line), Style: diffFg}}
			} else if whole, ok := e.DiffOverlay.syntaxAt(lineIdx, e.Buf.Lines[lineIdx]); ok {
				syntaxSpans = whole
			} else if e.Highlighter != nil {
				syntaxSpans = e.Highlighter.HighlightLineAt(e.Buf.Lines, lineIdx)
			}

			isCollapsedLine := e.Folds != nil && e.Folds.IsCollapsed(lineIdx)
			var annRunes []rune
			if isCollapsedLine && !isWrapContinuation {
				annRunes = []rune(" ⋯")
			}

			startCol, leftCol := 0, e.Viewport.LeftCol
			if e.WordWrap {
				startCol, leftCol = segStartCol, 0
			}
			diffBg := e.diffLineBg(lineIdx)
			var lineSpans []ColSpan
			if e.DiffOverlay != nil {
				lineSpans = e.DiffOverlay.Spans[lineIdx]
			}
			screenCells := e.renderLineSegmentToScreen(line, syntaxSpans, isCollapsedLine, annRunes, startCol, tabW, leftCol, editorW)
			var lineBrackets []bracketColorEntry
			if bracketColors != nil && lineIdx < len(bracketColors) {
				lineBrackets = bracketColors[lineIdx]
			}
			for x := 0; x < editorW; x++ {
				colIdx := screenCells[x].bufCol
				ch := screenCells[x].ch
				style := screenCells[x].style
				if diffFgFull {
					style = diffFg
				}

				for _, bc := range lineBrackets {
					if bc.col == colIdx {
						style = bc.style
						break
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
						isCursorLine = lineIdx == e.Cursor.Line && !e.Passive
					}
					if isCursorLine && !isSearchHighlight {
						bgStyle = term.StyleActiveLine
					}
				}
				inText := colIdx >= 0 && colIdx < len(line)
				hide := false
				textOnly := diffBg == term.StyleReverse || diffBg == term.StyleHide
				if diffBg != 0 && !inAnySel && !isSearchHighlight && (bgStyle == 0 || bgStyle == term.StyleActiveLine) && (!textOnly || inText) {
					if diffBg == term.StyleHide {
						hide = true
					} else {
						bgStyle = diffBg
					}
				}
				if len(lineSpans) > 0 && inText && !inAnySel && !isSearchHighlight {
					if spanBg := spanStyle(lineSpans, colIdx); spanBg != 0 {
						hide = spanBg == term.StyleHide
						if !hide {
							bgStyle = spanBg
						}
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
				cell := term.Cell{Ch: ch, Style: style, BgStyle: bgStyle, UlStyle: ulStyle}
				if bgStyle == term.StyleReverse {
					cell.BgStyle, cell.Attrs = 0, term.CellAttrReverse
				}
				if hide {
					cell.Attrs |= term.CellAttrHide
				}
				surface.SetCell(gutterW+x, y, cell)
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
		e.scrollbar.TotalItems = visibleCount + h - 1
		e.scrollbar.TopItem = topRow
		e.scrollbar.Marks = nil
		if e.scrollMarksEnabled() {
			e.scrollbar.Marks = e.gitScrollMarks(layout)
		}
		e.scrollbar.LegacyGlyphs = e.LegacyScrollbarGlyphs
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
	if !e.WordWrap {
		e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line)
	}
	curRow, curScreenCol := layout.rowOf(e.Cursor.Line, e.Cursor.Col)
	if e.WordWrap {
		e.CursorX = curScreenCol + gutterW + r.X
	} else {
		cursorVisCol := bufColToVisualCol(e.Buf.Lines[e.Cursor.Line], e.Cursor.Col, tabW)
		e.CursorX = cursorVisCol - e.Viewport.LeftCol + gutterW + r.X
	}
	e.CursorY = curRow - topRow + r.Y
}

// embeddedTextWidth is the text width Render gives an Embedded pane drawn at
// width w, so a host can lay the pane out before drawing it.
func (e *EditorPaneWidget) embeddedTextWidth(w int) int {
	return e.wrapTextWidth(max(w-e.GutterWidth(), 1))
}

func (e *EditorPaneWidget) wrapTextWidth(editorW int) int {
	if !e.WordWrap {
		return editorW
	}
	w := editorW
	if !e.NoWrapMargin && editorW > 4 {
		switch e.GutterStyle {
		case "minimal":
			w--
		case "extended":
			w -= 3
		default:
			w -= 2
		}
	}
	if e.wrapCols > 0 && w > e.wrapCols {
		w = e.wrapCols
	}
	if e.WordWrapColumn > 0 {
		w = min(w, e.WordWrapColumn)
	}
	return w
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
		if d.Style != 0 {
			return d.Style
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

type screenCell struct {
	ch     rune
	style  term.Style
	bufCol int
}

// The returned slice is reused by the next call.
func (e *EditorPaneWidget) renderLineToScreen(line []rune, spans []highlight.Span, collapsed bool, ann []rune, tabW, leftCol, width int) []screenCell {
	return e.renderLineSegmentToScreen(line, spans, collapsed, ann, 0, tabW, leftCol, width)
}

func (e *EditorPaneWidget) renderLineSegmentToScreen(line []rune, spans []highlight.Span, collapsed bool, ann []rune, startCol, tabW, leftCol, width int) []screenCell {
	if cap(e.lineScratch) < width {
		e.lineScratch = make([]screenCell, width)
	}
	cells := e.lineScratch[:width]
	for i := range cells {
		cells[i] = screenCell{ch: ' ', style: term.StyleDefault, bufCol: -1}
	}
	visCol := 0
	lineLen := len(line)
	for bufCol := startCol; bufCol < lineLen; bufCol++ {
		ch := line[bufCol]
		style := term.StyleDefault
		for _, sp := range spans {
			if bufCol >= sp.Start && bufCol < sp.End {
				style = sp.Style
				break
			}
		}
		if ch == '\t' {
			nextStop := ((visCol / tabW) + 1) * tabW
			for visCol < nextStop {
				sx := visCol - leftCol
				if sx >= 0 && sx < width {
					cells[sx] = screenCell{ch: ' ', style: style, bufCol: bufCol}
				}
				visCol++
			}
		} else {
			cw := textwidth.Rune(ch)
			sx := visCol - leftCol
			if sx >= 0 && sx < width {
				if cw > 1 && sx == width-1 {
					// The rune's second column falls outside the pane; drawing
					// it would bleed over the scrollbar or border, so pad.
					cells[sx] = screenCell{ch: ' ', style: style, bufCol: bufCol}
				} else {
					cells[sx] = screenCell{ch: ch, style: style, bufCol: bufCol}
				}
			}
			// A fullwidth rune's second column keeps the blank the cell array
			// was initialised with. The terminal never draws that cell — the
			// rune itself covers it — so it needs no content of its own.
			visCol += cw
		}
		if visCol-leftCol >= width {
			break
		}
	}
	if collapsed && len(ann) > 0 {
		for i, ch := range ann {
			sx := visCol + i - leftCol
			if sx >= 0 && sx < width {
				cells[sx] = screenCell{ch: ch, style: term.StyleLineNumber, bufCol: lineLen + i}
			}
		}
	}
	return cells
}

// drawGutterNumber writes the gutter's number column for num (0 leaves it
// blank) without building a string.
func (e *EditorPaneWidget) drawGutterNumber(surface Surface, y, gutterW, num int, cell term.Cell) {
	var buf [20]byte
	digits := buf[:0]
	if num > 0 {
		digits = strconv.AppendInt(digits, int64(num), 10)
	}
	lead, fixed, trail := 1, 3, 2
	switch e.GutterStyle {
	case "minimal":
		lead, fixed, trail = 0, 1, 1
	case "extended":
		lead, fixed, trail = 2, 5, 3
	}
	x := 0
	put := func(ch rune) {
		cell.Ch = ch
		surface.SetCell(x, y, cell)
		x++
	}
	for range lead + max(gutterW-fixed-len(digits), 0) {
		put(' ')
	}
	for _, d := range digits {
		put(rune(d))
	}
	for range trail {
		put(' ')
	}
}
