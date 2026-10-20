package widgets

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

// defaultInputMaxRows caps a wrapping input's height when MaxRows is unset.
const defaultInputMaxRows = 5

// inputRow is one visual row of a wrapping input: runes [start, end) of its
// text. A hard line break ends a row without being part of it.
type inputRow struct{ start, end int }

// wrapInputRows breaks runes into rows of at most width columns, at the last
// space that fits where there is one, and at every newline.
func wrapInputRows(runes []rune, width int) []inputRow {
	width = max(width, 1)
	var rows []inputRow
	start, used, lastSpace := 0, 0, -1
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\n' {
			rows = append(rows, inputRow{start, i})
			start, used, lastSpace = i+1, 0, -1
			continue
		}
		w := textwidth.Rune(r)
		if r == ' ' && used+w > width && i > start {
			// A space that doesn't fit ends the row it follows; it sits in
			// the column kept free for the cursor rather than starting a
			// row of its own.
			rows = append(rows, inputRow{start, i + 1})
			start, used, lastSpace = i+1, 0, -1
			continue
		}
		// The run carried past a space break can still leave no room for
		// this rune; the second pass breaks right before it.
		for used+w > width && i > start {
			end := i
			if lastSpace >= start {
				end = lastSpace + 1
			}
			rows = append(rows, inputRow{start, end})
			start, lastSpace = end, -1
			used = textwidth.Runes(runes[start:i])
		}
		if r == ' ' {
			lastSpace = i
		}
		used += w
	}
	return append(rows, inputRow{start, len(runes)})
}

// rowOf returns the index of the row holding rune position pos. At a soft
// break the position belongs to the row it starts.
func rowOf(rows []inputRow, pos int) int {
	for i := len(rows) - 1; i > 0; i-- {
		if pos >= rows[i].start {
			return i
		}
	}
	return 0
}

// posInRow returns the rune position col columns into rows[i]. Past the end
// of a row that wraps softly it stops on the row's last rune, since its end
// is where the next row starts.
func posInRow(runes []rune, rows []inputRow, i, col int) int {
	row := rows[i]
	used := 0
	for p := row.start; p < row.end; p++ {
		w := textwidth.Rune(runes[p])
		if col < used+w {
			return p
		}
		used += w
	}
	if i+1 < len(rows) && rows[i+1].start == row.end && row.end > row.start {
		return row.end - 1
	}
	return row.end
}

func (inp *InputWidget) maxRows() int {
	if inp.Config.MaxRows > 0 {
		return inp.Config.MaxRows
	}
	return defaultInputMaxRows
}

// wrapRows lays the text out at the width of the last render.
func (inp *InputWidget) wrapRows() []inputRow {
	return inp.rowsAt(inp.wrapWidth)
}

// rowsAt lays the text out in textW columns, keeping one free so the cursor
// fits after a full row.
func (inp *InputWidget) rowsAt(textW int) []inputRow {
	return wrapInputRows([]rune(inp.text), textW-1)
}

// visibleRows is how many rows a wrapping input shows in textW columns.
func (inp *InputWidget) visibleRows(textW int) int {
	return min(max(len(inp.rowsAt(textW)), max(1, inp.Config.MinRows)), inp.maxRows())
}

// HeightAtWidth is the input's height laid out w columns wide.
func (inp *InputWidget) HeightAtWidth(w int) int {
	if !inp.Config.Wrap {
		return 0
	}
	textW := w - inp.BoxOverheadW()
	if inp.Config.Bordered {
		textW -= 4
	} else {
		textW -= textwidth.String(inp.Config.Prefix)
	}
	if textW <= 0 {
		return 0
	}
	return inp.heightFor(inp.visibleRows(textW))
}

func (inp *InputWidget) renderWrapped(surface Surface, x, y, textW, rowsH int) {
	if textW <= 0 || rowsH <= 0 {
		return
	}
	inp.wrapWidth = textW
	style := inp.Config.Style
	if style == 0 {
		style = term.StyleInput
	}
	runes := []rune(inp.text)
	if len(runes) == 0 {
		inp.rowOffset = 0
	}
	if len(runes) == 0 && inp.Config.Placeholder != "" {
		inp.drawRunes(surface, x, y, textW, []rune(inp.Config.Placeholder), 0, func(int) term.Style {
			return term.StyleInputPlaceholder
		})
		for row := 1; row < rowsH; row++ {
			inp.drawRunes(surface, x, y+row, textW, nil, 0, func(int) term.Style { return style })
		}
		return
	}
	rows := inp.wrapRows()
	cur := rowOf(rows, inp.cursorPos)
	inp.rowOffset = min(inp.rowOffset, cur)
	if cur >= inp.rowOffset+rowsH {
		inp.rowOffset = cur - rowsH + 1
	}
	inp.rowOffset = max(min(inp.rowOffset, len(rows)-rowsH), 0)

	selLo, selHi := -1, -1
	if inp.hasSelection() {
		selLo, selHi = inp.selRange()
	}
	for row := range rowsH {
		ri := inp.rowOffset + row
		var line []rune
		if ri < len(rows) {
			line = runes[rows[ri].start:rows[ri].end]
		}
		start := 0
		if ri < len(rows) {
			start = rows[ri].start
		}
		inp.drawRunes(surface, x, y+row, textW, line, 0, func(i int) term.Style {
			if p := start + i; selLo >= 0 && i < len(line) && p >= selLo && p < selHi {
				return term.StyleSelection
			}
			return style
		})
	}
	inp.drawScrollMarks(surface, x, y, textW, rowsH, len(rows))
}

// drawScrollMarks marks rows scrolled out of view in the last column, which
// wrapRows keeps free for the cursor, except where the cursor is.
func (inp *InputWidget) drawScrollMarks(surface Surface, x, y, textW, rowsH, total int) {
	if textW < 3 {
		return
	}
	above, below := inp.rowOffset > 0, inp.rowOffset+rowsH < total
	col, row := inp.wrappedCursor()
	mark := func(r int, ch rune) {
		if inp.focused && row == r && col == textW-1 {
			return
		}
		surface.SetCell(x+textW-1, y+r, term.Cell{Ch: ch, Style: term.StyleInputPlaceholder})
	}
	switch {
	case above && below && rowsH == 1:
		mark(0, scrollMark('↕', ':'))
	default:
		if above {
			mark(0, scrollMark('↑', '^'))
		}
		if below {
			mark(rowsH-1, scrollMark('↓', 'v'))
		}
	}
}

// scrollMark returns arrow, or fallback where the terminal draws arrow two
// columns wide.
func scrollMark(arrow, fallback rune) rune {
	if textwidth.Rune(arrow) == 1 {
		return arrow
	}
	return fallback
}

// wrappedCursor returns the cursor's row and column relative to the first
// visible row.
func (inp *InputWidget) wrappedCursor() (col, row int) {
	runes := []rune(inp.text)
	rows := inp.wrapRows()
	r := rowOf(rows, inp.cursorPos)
	return textwidth.Runes(runes[rows[r].start:inp.cursorPos]), r - inp.rowOffset
}

// wrappedPosAt maps a click col columns into the text, on the visible row
// row, to a rune position.
func (inp *InputWidget) wrappedPosAt(col, row int) int {
	rows := inp.wrapRows()
	r := min(max(inp.rowOffset+row, 0), len(rows)-1)
	return posInRow([]rune(inp.text), rows, r, max(col, 0))
}

// handleWrappedKey handles the keys that move between rows or break lines.
// It reports false for keys the single-row handling covers.
func (inp *InputWidget) handleWrappedKey(ev *tcell.EventKey) (EventResult, bool) {
	mods := ev.Modifiers()
	shift := mods&tcell.ModShift != 0
	switch ev.Key() {
	case tcell.KeyEnter:
		if !inp.Config.Multiline && mods&(tcell.ModShift|tcell.ModAlt) == 0 {
			return EventIgnored, false
		}
		inp.insertRunes([]rune{'\n'})
		return EventConsumed, true
	case tcell.KeyCtrlJ:
		inp.insertRunes([]rune{'\n'})
		return EventConsumed, true
	case tcell.KeyRune:
		// Legacy terminals send Alt+Enter as ESC CR, which tcell reports as
		// Alt+Ctrl+M.
		if mods&(tcell.ModAlt|tcell.ModCtrl) == tcell.ModAlt|tcell.ModCtrl && strings.EqualFold(ev.Str(), "m") {
			inp.insertRunes([]rune{'\n'})
			return EventConsumed, true
		}
		return EventIgnored, false
	case tcell.KeyUp, tcell.KeyDown:
		if mods&^tcell.ModShift != 0 {
			return EventIgnored, false
		}
		runes := []rune(inp.text)
		rows := inp.wrapRows()
		r := rowOf(rows, inp.cursorPos)
		next := r - 1
		if ev.Key() == tcell.KeyDown {
			next = r + 1
		}
		if next < 0 || next >= len(rows) {
			return EventIgnored, true
		}
		if inp.goalCol < 0 {
			inp.goalCol = textwidth.Runes(runes[rows[r].start:inp.cursorPos])
		}
		inp.moveCursor(posInRow(runes, rows, next, inp.goalCol), shift)
		return EventConsumed, true
	case tcell.KeyHome, tcell.KeyEnd:
		if mods&tcell.ModCtrl != 0 {
			pos := 0
			if ev.Key() == tcell.KeyEnd {
				pos = len([]rune(inp.text))
			}
			inp.moveCursor(pos, shift)
			return EventConsumed, true
		}
		rows := inp.wrapRows()
		r := rowOf(rows, inp.cursorPos)
		pos := rows[r].start
		if ev.Key() == tcell.KeyEnd {
			pos = posInRow([]rune(inp.text), rows, r, inp.wrapWidth)
		}
		inp.moveCursor(pos, shift)
		return EventConsumed, true
	}
	return EventIgnored, false
}

func (inp *InputWidget) moveCursor(pos int, extend bool) {
	if extend {
		inp.startSel()
	}
	inp.cursorPos = pos
	if extend {
		inp.selEnd = pos
	} else {
		inp.clearSel()
	}
}
