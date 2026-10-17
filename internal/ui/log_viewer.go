package ui

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/core/logfile"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

type LogSource interface {
	Progress() logfile.Progress
	ReadRows(int64, int) ([]logfile.Row, error)
	Previous(int64, int) int64
	Tail(int) int64
	LineNumber(int64) (int, bool)
	OffsetForLine(int) (int64, bool)
}

type LogViewerWidget struct {
	started  bool
	hscroll  HScrollbar
	maxWidth int
	BaseWidget
	Source                          LogSource
	OnClose                         func()
	OnNotice                        func(string)
	OnCopy                          func()
	OnFindNext                      func(int)
	OnClearFind                     func()
	Following                       bool
	LineNumbers                     bool
	TabSize                         int
	Cursor                          logfile.Position
	Anchor                          logfile.Position
	Selecting                       bool
	Matches                         []logfile.Match
	top                             int64
	left, gutter, textWidth, height int
	rows                            []logfile.Row
	focused, dragging               bool
	scrollbar                       Scrollbar
}

func NewLogViewer(source LogSource) *LogViewerWidget {
	return &LogViewerWidget{Source: source, LineNumbers: true, TabSize: 4}
}
func (v *LogViewerWidget) Focusable() bool   { return true }
func (v *LogViewerWidget) SetFocused(f bool) { v.focused = f }
func (v *LogViewerWidget) IsFocused() bool   { return v.focused }
func (v *LogViewerWidget) Close() {
	if v.OnClose != nil {
		v.OnClose()
		v.OnClose = nil
	}
}
func (v *LogViewerWidget) notice(s string) {
	if v.OnNotice != nil {
		v.OnNotice(s)
	}
}
func (v *LogViewerWidget) load() {
	rows, err := v.Source.ReadRows(v.top, max(v.height, 1))
	if err != nil {
		v.notice("Cannot read log: " + err.Error())
	}
	v.rows = rows
}
func (v *LogViewerWidget) RefreshProgress(reset bool) {
	if reset {
		v.left = 0
		v.top = 0
		v.maxWidth = 0
		v.Cursor = logfile.Position{}
		v.Selecting = false
		v.Matches = nil
	}
	if v.Following {
		left := v.left
		v.edge(true, false)
		v.left = left
	} else {
		v.load()
	}
}
func (v *LogViewerWidget) row(offset int64) logfile.Row {
	for _, r := range v.rows {
		if r.Offset == offset {
			return r
		}
	}
	rows, _ := v.Source.ReadRows(offset, 1)
	if len(rows) > 0 {
		return rows[0]
	}
	return logfile.Row{Offset: offset, Next: offset}
}
func (v *LogViewerWidget) selectMove(shift bool) {
	if shift && !v.Selecting {
		v.Anchor = v.Cursor
	}
	v.Selecting = shift
}
func (v *LogViewerWidget) edge(end, shift bool) {
	v.selectMove(shift)
	v.started = true
	v.Following = end && !shift
	v.left = 0
	if end {
		v.top = v.Source.Tail(max(v.height, 1))
		v.load()
		if len(v.rows) > 0 {
			r := v.rows[len(v.rows)-1]
			v.Cursor = logfile.Position{Offset: r.Offset, Col: utf8.RuneCountInString(r.Text)}
		}
	} else {
		v.top = 0
		v.Cursor = logfile.Position{}
		v.load()
	}
}
func (v *LogViewerWidget) MoveToFileEdge(end, shift bool) { v.edge(end, shift) }
func (v *LogViewerWidget) SelectAll() {
	v.Anchor = logfile.Position{}
	v.Selecting = true
	v.Cursor = logfile.Position{Offset: v.Source.Progress().Size}
}
func (v *LogViewerWidget) Selection() (logfile.Position, logfile.Position) {
	if v.Selecting {
		return v.Anchor, v.Cursor
	}
	row := v.row(v.Cursor.Offset)
	if row.Next < 0 {
		return logfile.Position{Offset: row.Offset}, logfile.Position{Offset: row.Offset, Col: utf8.RuneCountInString(row.Text)}
	}
	return logfile.Position{Offset: row.Offset}, logfile.Position{Offset: row.Next}
}
func (v *LogViewerWidget) GoToLine(line int) {
	off, ok := v.Source.OffsetForLine(max(line-1, 0))
	if !ok {
		v.notice(fmt.Sprintf("Line %d has not been indexed yet (%d lines indexed)", line, v.Source.Progress().Lines))
		return
	}
	v.Navigate(logfile.Match{Offset: off})
}
func (v *LogViewerWidget) Navigate(match logfile.Match) {
	v.Following = false
	v.Selecting = false
	v.Cursor = logfile.Position{Offset: match.Offset, Col: match.Col}
	v.top = v.Source.Previous(match.Offset, max(v.height/3, 0))
	v.load()
	if row := v.row(match.Offset); match.Col > utf8.RuneCountInString(row.Text) && row.Truncated {
		v.notice("Match is past the 1 MiB display limit for this line")
	}
	v.revealColumn()
}
func (v *LogViewerWidget) revealColumn() {
	row := v.row(v.Cursor.Offset)
	runes := []rune(row.Text)
	col := 0
	for _, r := range runes[:min(v.Cursor.Col, len(runes))] {
		if r == '\t' {
			col += max(v.TabSize, 1) - col%max(v.TabSize, 1)
		} else {
			col += textwidth.Rune(r)
		}
	}
	if col < v.left {
		v.left = col
	}
	if col >= v.left+max(v.textWidth, 1) {
		v.left = col - max(v.textWidth, 1) + 1
	}
}
func (v *LogViewerWidget) scroll(n int) {
	if n < 0 {
		v.top = v.Source.Previous(v.top, -n)
		v.Following = false
	} else {
		rows, _ := v.Source.ReadRows(v.top, n+1)
		if len(rows) > 0 {
			v.top = rows[len(rows)-1].Offset
		}
	}
	v.load()
	if n > 0 && len(v.rows) > 0 {
		last := v.rows[len(v.rows)-1]
		v.Following = last.Next >= v.Source.Progress().Size
	}
}
func (v *LogViewerWidget) move(n int, shift bool) {
	v.selectMove(shift)
	v.Following = false
	if n < 0 {
		v.Cursor.Offset = v.Source.Previous(v.Cursor.Offset, -n)
	} else {
		rows, _ := v.Source.ReadRows(v.Cursor.Offset, n+1)
		if len(rows) > 0 {
			v.Cursor.Offset = rows[len(rows)-1].Offset
		}
	}
	row := v.row(v.Cursor.Offset)
	v.Cursor.Col = min(v.Cursor.Col, utf8.RuneCountInString(row.Text))
	if len(v.rows) == 0 || v.Cursor.Offset < v.top {
		v.top = v.Cursor.Offset
		v.load()
	}
	if len(v.rows) > 0 && v.Cursor.Offset > v.rows[len(v.rows)-1].Offset {
		v.top = v.Source.Previous(v.Cursor.Offset, max(v.height-1, 0))
		v.load()
	}
	v.Following = row.Next >= v.Source.Progress().Size
	v.revealColumn()
}
func (v *LogViewerWidget) selected(offset int64, col int) bool {
	if !v.Selecting {
		return false
	}
	a, b := v.Anchor, v.Cursor
	if logfile.Before(b, a) {
		a, b = b, a
	}
	p := logfile.Position{Offset: offset, Col: col}
	return !logfile.Before(p, a) && logfile.Before(p, b)
}
func (v *LogViewerWidget) Render(surface Surface) {
	w, h := surface.Size()
	surface.Fill(term.Cell{Ch: ' ', Style: term.StyleDefault})
	p := v.Source.Progress()
	v.gutter = 0
	if v.LineNumbers {
		v.gutter = max(len(strconv.Itoa(p.Lines)), 2) + 2
	}
	v.textWidth = max(w-v.gutter-1, 1)
	v.height = max(h, 1)
	if v.Following {
		v.top = v.Source.Tail(v.height)
	}
	v.load()
	for _, row := range v.rows {
		width := 0
		for _, r := range row.Text {
			if r == '\t' {
				width += max(v.TabSize, 1) - width%max(v.TabSize, 1)
			} else {
				width += textwidth.Rune(r)
			}
		}
		v.maxWidth = max(v.maxWidth, width)
	}
	if v.maxWidth > v.textWidth {
		h = max(h-1, 1)
		v.height = h
		if v.Following {
			v.top = v.Source.Tail(h)
			v.load()
		}
	}
	if !v.started {
		v.started = true
		if len(v.rows) > 0 {
			v.Following = len(v.rows) <= h && v.rows[len(v.rows)-1].Next >= p.Size
		}
	}
	matchesByOffset := make(map[int64][]logfile.Match)
	for _, m := range v.Matches {
		matchesByOffset[m.Offset] = append(matchesByOffset[m.Offset], m)
	}
	base, known := v.Source.LineNumber(v.top)
	for y, row := range v.rows {
		if y >= h {
			break
		}
		if v.LineNumbers && known {
			surface.DrawText(0, y, fmt.Sprintf("%*d ", v.gutter-1, base+y+1), v.gutter, term.StyleLineNumber)
		}
		col := 0
		for i, r := range []rune(row.Text) {
			width := textwidth.Rune(r)
			if r == '\t' {
				width = max(v.TabSize, 1) - col%max(v.TabSize, 1)
			}
			x := v.gutter + col - v.left
			style := term.StyleDefault
			for _, match := range matchesByOffset[row.Offset] {
				if match.Offset == row.Offset && i >= match.Col && i < match.Col+match.Len {
					style = term.StyleSearchMatch
					break
				}
			}
			if v.selected(row.Offset, i) {
				style = term.StyleSelection
			}
			if x >= v.gutter && x < v.gutter+v.textWidth {
				if r == '\t' {
					width = surface.DrawText(x, y, strings.Repeat(" ", width), v.gutter+v.textWidth, style) - x
				} else if width > 0 {
					width = surface.DrawText(x, y, string(r), v.gutter+v.textWidth, style) - x
				}
			}
			col += width
			if col-v.left >= v.textWidth {
				break
			}
		}
		if row.Truncated {
			surface.DrawText(max(v.gutter, w-15), y, "… line cut off", w-1, term.StyleMuted)
		}
	}
	v.scrollbar.X = v.GetRect().X + w - 1
	v.scrollbar.Y = v.GetRect().Y
	v.scrollbar.Height = h
	// Byte positions keep the scrollbar usable before line indexing finishes.
	v.scrollbar.TotalItems = max(int(p.Size), h+1)
	v.scrollbar.TopItem = min(int(v.top), max(int(p.Size)-h, 0))
	v.scrollbar.Render(surface, w-1, 0)
	v.hscroll.X = v.GetRect().X + v.gutter
	v.hscroll.Y = v.GetRect().Y + h
	v.hscroll.Width = v.textWidth
	v.hscroll.TotalCols = v.maxWidth + 1
	v.hscroll.LeftCol = v.left
	v.hscroll.Render(surface, v.gutter, h)
}
func (v *LogViewerWidget) CursorPosition() (int, int, bool) {
	if !v.focused {
		return 0, 0, false
	}
	for y, row := range v.rows {
		if row.Offset != v.Cursor.Offset {
			continue
		}
		col := 0
		for _, r := range []rune(row.Text)[:min(v.Cursor.Col, utf8.RuneCountInString(row.Text))] {
			if r == '\t' {
				col += max(v.TabSize, 1) - col%max(v.TabSize, 1)
			} else {
				col += textwidth.Rune(r)
			}
		}
		x := col - v.left
		return v.GetRect().X + v.gutter + x, v.GetRect().Y + y, x >= 0 && x < v.textWidth
	}
	return 0, 0, false
}
func (v *LogViewerWidget) mousePosition(x, y int) logfile.Position {
	r := v.GetRect()
	y = min(max(y-r.Y, 0), max(len(v.rows)-1, 0))
	if len(v.rows) == 0 {
		return v.Cursor
	}
	row := v.rows[y]
	target := max(x-r.X-v.gutter+v.left, 0)
	visual, col := 0, 0
	for _, ch := range row.Text {
		width := textwidth.Rune(ch)
		if ch == '\t' {
			width = max(v.TabSize, 1) - visual%max(v.TabSize, 1)
		}
		if visual+width > target {
			break
		}
		visual += width
		col++
	}
	return logfile.Position{Offset: row.Offset, Col: col}
}
func (v *LogViewerWidget) OwnsPointerCapture() bool {
	return v.dragging || v.scrollbar.IsDragging() || v.hscroll.IsDragging()
}
func (v *LogViewerWidget) CancelPointerCapture() bool {
	owned := v.OwnsPointerCapture()
	v.dragging = false
	v.scrollbar.dragging = false
	v.hscroll.dragging = false
	return owned
}
func (v *LogViewerWidget) HandleEvent(ev tcell.Event) EventResult {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		if v.hscroll.Visible() {
			if left, ok := v.hscroll.HandleEvent(e); ok {
				v.left = left
				if v.hscroll.IsDragging() {
					return EventCaptured
				}
				return EventConsumed
			}
		}
		if top, ok := v.scrollbar.HandleEvent(e); ok {
			v.Following = false
			// Align an arbitrary byte position to a complete line boundary.
			v.top = v.Source.Previous(int64(top), 1)
			v.load()
			if v.scrollbar.IsDragging() {
				return EventCaptured
			}
			return EventConsumed
		}
		switch {
		case e.Buttons()&tcell.WheelUp != 0:
			v.scroll(-3)
		case e.Buttons()&tcell.WheelDown != 0:
			v.scroll(3)
		case e.Buttons()&tcell.WheelLeft != 0:
			v.left = max(v.left-4, 0)
		case e.Buttons()&tcell.WheelRight != 0:
			v.left = min(v.left+4, max(v.maxWidth-v.textWidth+1, 0))
		case e.Buttons()&tcell.Button1 != 0:
			x, y := e.Position()
			if !v.dragging {
				v.Anchor = v.mousePosition(x, y)
				v.dragging = true
			}
			if y < v.GetRect().Y {
				v.scroll(-1)
			} else if y >= v.GetRect().Y+v.height {
				v.scroll(1)
			}
			v.Cursor = v.mousePosition(x, y)
			v.Selecting = true
			v.Following = false
			return EventCaptured
		default:
			if v.dragging && v.Anchor == v.Cursor {
				v.Selecting = false
			}
			v.dragging = false
		}
		return EventConsumed
	case *tcell.EventKey:
		shift := e.Modifiers()&tcell.ModShift != 0
		ctrl := e.Modifiers()&tcell.ModCtrl != 0
		switch e.Key() {
		case tcell.KeyUp:
			v.move(-1, shift)
		case tcell.KeyDown:
			v.move(1, shift)
		case tcell.KeyPgUp:
			v.move(-max(v.height-1, 1), shift)
		case tcell.KeyPgDn:
			v.move(max(v.height-1, 1), shift)
		case tcell.KeyHome:
			if ctrl {
				v.edge(false, shift)
			} else {
				v.selectMove(shift)
				v.Cursor.Col = 0
				v.revealColumn()
			}
		case tcell.KeyEnd:
			if ctrl {
				v.edge(true, shift)
			} else {
				v.selectMove(shift)
				v.Cursor.Col = utf8.RuneCountInString(v.row(v.Cursor.Offset).Text)
				v.revealColumn()
			}
		case tcell.KeyLeft:
			v.selectMove(shift)
			v.Cursor.Col = max(v.Cursor.Col-1, 0)
			v.revealColumn()
		case tcell.KeyRight:
			v.selectMove(shift)
			v.Cursor.Col = min(v.Cursor.Col+1, utf8.RuneCountInString(v.row(v.Cursor.Offset).Text))
			v.revealColumn()
		case tcell.KeyEscape:
			v.Selecting = false
		default:
			return EventIgnored
		}
		return EventConsumed
	}
	return EventIgnored
}
