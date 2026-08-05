package ui

import (
	"strconv"
	"strings"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/highlight"
	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

func (e *EditorPaneWidget) Render(surface Surface) {
	w, h := surface.Size()

	totalLines := len(e.Buf.Lines)
	gutterW := e.GutterWidth()

	maxLineW := e.computeMaxLineWidth()
	tabW := e.resolveTabSize()

	editorW := w - gutterW
	showHScrollbar := !e.WordWrap && maxLineW > editorW
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

	if e.WordWrap {
		visibleCount = totalVisualLines(e.Buf.Lines, editorW, tabW)
	}

	showScrollbar := visibleCount > h
	if showScrollbar {
		editorW--
	}
	if editorW < 1 {
		editorW = 1
	}
	if e.WordWrap && editorW > 4 {
		switch e.GutterStyle {
		case "minimal":
			editorW--
		case "extended":
			editorW -= 3
		default:
			editorW -= 2
		}
	}

	e.Viewport.Width = editorW
	e.Viewport.Height = h

	if e.WordWrap {
		e.Viewport.LeftCol = 0
		visibleCount = totalVisualLines(e.Buf.Lines, editorW, tabW)
		showScrollbar = visibleCount > h
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

	if e.WordWrap {
		e.wrapMap = buildWrapMap(e.Buf.Lines, e.Viewport.TopLine, e.wrapTopOffset, h, editorW, tabW)
	} else {
		e.wrapMap = nil
	}

	var allSpans [][]highlight.Span
	if e.Highlighter != nil {
		allSpans = e.Highlighter.HighlightAll(e.Buf.Lines)
	}

	for y := 0; y < h; y++ {
		var lineIdx int
		var segStartCol int
		var isWrapContinuation bool

		if e.WordWrap && e.wrapMap != nil {
			entry := e.wrapMap[y]
			lineIdx = entry.bufLine
			segStartCol = entry.startCol
			isWrapContinuation = segStartCol > 0
		} else {
			lineIdx = e.screenToBufferLine(y)
			segStartCol = 0
		}

		if gutterW > 0 {
			gutterStyle := term.StyleLineNumber
			if lineIdx < totalLines && lineIdx == e.Cursor.Line {
				gutterStyle = term.StyleActiveLine
			}
			var padded string
			if !e.LineNumbers || (e.WordWrap && isWrapContinuation) {
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
			if e.Folds != nil && !e.WordWrap && lineIdx < totalLines && !isWrapContinuation {
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
			if lineIdx < totalLines && lineIdx < len(e.LineChanges) && !isWrapContinuation {
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
		}

		if lineIdx < totalLines {
			line := []rune(e.Buf.Lines[lineIdx])
			var syntaxSpans []highlight.Span
			if allSpans != nil && lineIdx < len(allSpans) {
				syntaxSpans = allSpans[lineIdx]
			}

			isCollapsedLine := e.Folds != nil && e.Folds.IsCollapsed(lineIdx)
			var annRunes []rune
			if isCollapsedLine && !isWrapContinuation {
				annRunes = []rune(" ⋯")
			}

			var leftCol int
			if e.WordWrap {
				leftCol = bufColToVisualCol(e.Buf.Lines[lineIdx], segStartCol, tabW)
			} else {
				leftCol = e.Viewport.LeftCol
			}
			screenCells := e.renderLineToScreen(line, syntaxSpans, isCollapsedLine, annRunes, tabW, leftCol, editorW)
			var lineBrackets []bracketColorEntry
			if bracketColors != nil && lineIdx < len(bracketColors) {
				lineBrackets = bracketColors[lineIdx]
			}
			for x := 0; x < editorW; x++ {
				colIdx := screenCells[x].bufCol
				ch := screenCells[x].ch
				style := screenCells[x].style

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
						isCursorLine = lineIdx == e.Cursor.Line
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
		if e.WordWrap {
			e.scrollbar.TotalItems = visibleCount + h - 1
			curTopVisRow, _ := bufferPosToWrapScreenPos(e.Buf.Lines, e.Viewport.TopLine, 0, editorW, tabW)
			curTopVisRow += e.wrapTopOffset
			e.scrollbar.TopItem = curTopVisRow
		} else if foldsActive {
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
	if e.WordWrap {
		curVisRow, curScreenCol := bufferPosToWrapScreenPos(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col, editorW, tabW)
		topVisRow, _ := bufferPosToWrapScreenPos(e.Buf.Lines, e.Viewport.TopLine, 0, editorW, tabW)
		topVisRow += e.wrapTopOffset
		e.CursorX = curScreenCol + gutterW + r.X
		e.CursorY = curVisRow - topVisRow + r.Y
	} else {
		e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line)
		cursorVisCol := bufColToVisualCol(e.Buf.Lines[e.Cursor.Line], e.Cursor.Col, tabW)
		e.CursorX = cursorVisCol - e.Viewport.LeftCol + gutterW + r.X
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

func (e *EditorPaneWidget) renderLineToScreen(line []rune, spans []highlight.Span, collapsed bool, ann []rune, tabW, leftCol, width int) []screenCell {
	cells := make([]screenCell, width)
	for i := range cells {
		cells[i] = screenCell{ch: ' ', style: term.StyleDefault, bufCol: -1}
	}
	visCol := 0
	lineLen := len(line)
	for bufCol := 0; bufCol < lineLen; bufCol++ {
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
