package ui

import (
	"time"

	"github.com/gdamore/tcell/v3"
)

func (e *EditorPaneWidget) handleMouse(mev *tcell.EventMouse) EventResult {
	btn := mev.Buttons()

	if newTop, result := e.scrollbar.HandleEvent(mev); result != EventIgnored {
		if e.Folds != nil && e.Folds.HasCollapsedFolds() {
			e.Viewport.TopLine = e.Folds.VisibleToBuffer(newTop)
		} else {
			e.Viewport.TopLine = newTop
		}
		return result
	}
	if newLeft, result := e.hscrollbar.HandleEvent(mev); result != EventIgnored {
		e.Viewport.LeftCol = newLeft
		e.clampLeftCol()
		return result
	}

	mod := mev.Modifiers()
	if btn&tcell.WheelUp != 0 {
		if mod&tcell.ModShift != 0 {
			e.Viewport.LeftCol -= 4
			e.clampLeftCol()
		} else {
			e.scrollUp(3)
		}
		return EventConsumed
	}
	if btn&tcell.WheelDown != 0 {
		if mod&tcell.ModShift != 0 {
			e.Viewport.LeftCol += 4
			e.clampLeftCol()
		} else {
			e.scrollDown(3)
		}
		return EventConsumed
	}
	if btn&tcell.WheelLeft != 0 {
		e.Viewport.LeftCol -= 4
		e.clampLeftCol()
		return EventConsumed
	}
	if btn&tcell.WheelRight != 0 {
		e.Viewport.LeftCol += 4
		e.clampLeftCol()
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
			e.mouseDownX = mx
			e.mouseDownY = my

			if e.isMultiActive() {
				e.collapseMulti()
			}

			now := time.Now().UnixMilli()
			if now-e.lastClickTime < DoubleClickMs && line == e.lastClickLine && col == e.lastClickCol {
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
		if mx == e.mouseDownX && my == e.mouseDownY && inGutter {
			bufLine := e.screenToBufferLine(my - r.Y)
			if e.Folds != nil && e.Folds.FoldAt(bufLine) != nil {
				e.Folds.Toggle(bufLine)
				return EventConsumed
			}
		}
		if e.Selection != nil && e.Selection.Active {
			if e.Selection.Anchor.Line == e.Cursor.Line && e.Selection.Anchor.Col == e.Cursor.Col {
				e.Selection.Clear()
			}
		}
	}
	return EventIgnored
}

func (e *EditorPaneWidget) mouseToPos(r Rect, mx, my int) (line, col int) {
	if len(e.Buf.Lines) == 0 {
		return 0, 0
	}
	gutterW := e.GutterWidth()
	screenY := my - r.Y

	if e.WordWrap && e.wrapMap != nil && screenY >= 0 && screenY < len(e.wrapMap) {
		entry := e.wrapMap[screenY]
		line = entry.bufLine
		if line >= len(e.Buf.Lines) {
			line = len(e.Buf.Lines) - 1
		}
		segVisCol := mx - r.X - gutterW
		if segVisCol < 0 {
			segVisCol = 0
		}
		segLeftCol := bufColToVisualCol(e.Buf.Lines[line], entry.startCol, e.resolveTabSize())
		col = visualColToBufCol(e.Buf.Lines[line], segLeftCol+segVisCol, e.resolveTabSize())
	} else {
		line = e.screenToBufferLine(screenY)
		visCol := mx - r.X - gutterW + e.Viewport.LeftCol
		if visCol < 0 {
			visCol = 0
		}
		if line < 0 {
			line = 0
		}
		if line >= len(e.Buf.Lines) {
			line = len(e.Buf.Lines) - 1
		}
		col = visualColToBufCol(e.Buf.Lines[line], visCol, e.resolveTabSize())
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
