package ui

import "github.com/eugenioenko/ttt/internal/textwidth"

// wrapGoal is valid only while the cursor sits where the last vertical move
// left it, so any other cursor move or edit resets it.
type wrapGoal struct {
	x, line, col int
	version      uint64
	ok           bool
}

func (e *EditorPaneWidget) moveCursorWrapped(delta int) {
	x := -1
	if g := e.wrapGoal; g.ok && g.line == e.Cursor.Line && g.col == e.Cursor.Col && g.version == e.Buf.Version() {
		x = g.x
	}
	line, col, x := e.wrappedRowMove(e.Cursor.Line, e.Cursor.Col, delta, x)
	e.Cursor.Line, e.Cursor.Col = line, col
	e.wrapGoal = wrapGoal{x: x, line: line, col: col, version: e.Buf.Version(), ok: true}
}

// A negative x aims for the screen column (line, col) is drawn at.
func (e *EditorPaneWidget) wrappedRowMove(line, col, delta, x int) (int, int, int) {
	l := e.layout()
	from, screenCol := l.rowOf(line, col)
	if x < 0 {
		x = screenCol
	}
	r, ok := l.textRowToward(from, delta)
	if !ok {
		return line, col, x
	}
	return r.bufLine, l.colAtX(r, x), x
}

// textRowToward steps past phantom rows away from from, then back toward it
// when the layout edge is reached first.
func (l *rowLayout) textRowToward(from, delta int) (editorRow, bool) {
	total := l.total()
	if delta == 0 || total == 0 {
		return editorRow{}, false
	}
	dir := 1
	if delta < 0 {
		dir = -1
	}
	target := min(max(from+delta, 0), total-1)
	for abs := target; abs >= 0 && abs < total; abs += dir {
		if r, ok := l.textRowAt(abs); ok {
			return r, abs != from
		}
	}
	for abs := target - dir; abs != from && abs >= 0 && abs < total; abs -= dir {
		if r, ok := l.textRowAt(abs); ok {
			return r, true
		}
	}
	return editorRow{}, false
}

func (l *rowLayout) textRowAt(abs int) (editorRow, bool) {
	r := l.rowInBlock(l.rowToTop(abs))
	return r, !r.isPhantom() && r.bufLine >= 0 && r.bufLine < l.n()
}

// colAtX is the column of the rune drawn at screen column x of row r. Past the
// end of a row that continues on the next one it is the row's last rune, since
// the column after it is drawn at the start of the next row.
func (l *rowLayout) colAtX(r editorRow, x int) int {
	runes := []rune(l.lines[r.bufLine])
	end := len(runes)
	segs := l.segments(r.bufLine)
	for i, s := range segs {
		if s == r.startCol && i+1 < len(segs) {
			end = segs[i+1] - 1
			break
		}
	}
	screenCol := 0
	for c := r.startCol; c < end; c++ {
		w := textwidth.Rune(runes[c])
		if runes[c] == '\t' {
			w = ((screenCol/l.tabW)+1)*l.tabW - screenCol
		}
		if screenCol+w > x {
			return c
		}
		screenCol += w
	}
	return end
}
