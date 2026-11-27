package ui

import (
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

type cursorGoalContext struct {
	buffer          *buffer.Buffer
	version         uint64
	tabWidth, width int
	wrap, words     bool
}

func (e *EditorPaneWidget) ResetCursorGoal() {
	e.Cursor.Goal.Reset()
	if e.Multi != nil {
		for i := range e.Multi.Cursors {
			e.Multi.Cursors[i].Goal.Reset()
		}
	}
}

func (e *EditorPaneWidget) goalColumn(line, col int, goal *cursor.Goal) int {
	l := e.layout()
	context := cursorGoalContext{
		buffer: e.Buf, version: e.Buf.Version(),
		tabWidth: l.tabW, width: l.key.width, wrap: l.wrap, words: l.words,
	}
	if e.goalContext != context {
		e.ResetCursorGoal()
		e.goalContext = context
	}
	x := -1
	if !e.WordWrap {
		x = bufColToVisualCol(e.Buf.Lines[line], col, l.tabW)
	}
	return goal.Column(line, col, x)
}

func (e *EditorPaneWidget) moveCursorRows(delta int) {
	x := e.goalColumn(e.Cursor.Line, e.Cursor.Col, &e.Cursor.Goal)
	line, col, x := e.wrappedRowMove(e.Cursor.Line, e.Cursor.Col, delta, x)
	e.Cursor.Line, e.Cursor.Col = line, col
	e.Cursor.Goal.Remember(line, col, x)
}

func (e *EditorPaneWidget) moveCursorUnwrapped(delta int) {
	x := e.goalColumn(e.Cursor.Line, e.Cursor.Col, &e.Cursor.Goal)
	e.Cursor.Line = e.Buf.ClampLine(e.Cursor.Line + delta)
	if delta < 0 {
		e.skipHiddenLineUp()
	} else {
		e.skipHiddenLineDown()
	}
	e.Cursor.Col = visualColToBufCol(e.Buf.Lines[e.Cursor.Line], x, e.resolveTabSize())
	e.Cursor.Goal.Remember(e.Cursor.Line, e.Cursor.Col, x)
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
