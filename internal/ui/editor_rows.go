package ui

import (
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

type editorRow struct {
	bufLine  int
	startCol int
	phantom  int
}

func (r editorRow) isPhantom() bool { return r.phantom >= 0 }

var singleSegment = []int{0}

// rowLayout maps buffer lines to screen rows. Each visible buffer line owns a
// block of rows: the phantom rows anchored before it, then its wrap segments.
// Phantom rows anchored at len(lines) follow the last line.
type rowLayout struct {
	lines    []string
	wrap     bool
	width    int
	tabW     int
	folds    *fold.State
	visible  []int
	phantoms map[int]int
}

func (e *EditorPaneWidget) rowLayout(width int) *rowLayout {
	if width < 1 {
		width = 1
	}
	l := &rowLayout{
		lines: e.Buf.Lines,
		wrap:  e.WordWrap,
		width: width,
		tabW:  e.resolveTabSize(),
	}
	if e.hasFolds() {
		l.folds = e.Folds
		l.visible = e.Folds.VisibleLines(len(e.Buf.Lines))
	}
	if len(e.phantoms) > 0 {
		l.phantoms = e.phantoms
	}
	return l
}

func (e *EditorPaneWidget) layout() *rowLayout {
	return e.rowLayout(e.Viewport.Width)
}

func (l *rowLayout) n() int { return len(l.lines) }

func (l *rowLayout) identity() bool { return !l.wrap && l.phantoms == nil }

func (l *rowLayout) segments(line int) []int {
	if l.wrap && line >= 0 && line < l.n() {
		return wrapLineSegments([]rune(l.lines[line]), l.width, l.tabW)
	}
	return singleSegment
}

func (l *rowLayout) blockRows(line int) int {
	rows := l.phantoms[line]
	if line < l.n() {
		rows += len(l.segments(line))
	}
	return rows
}

func (l *rowLayout) visibleCount() int {
	if l.visible == nil {
		return l.n()
	}
	return len(l.visible)
}

func (l *rowLayout) visIndex(line int) int {
	if l.visible == nil {
		return line
	}
	if line >= l.n() {
		return len(l.visible) + line - l.n()
	}
	if v := l.folds.BufferToVisible(line); v >= 0 {
		return v
	}
	if r := l.folds.ContainingFold(line); r != nil {
		return l.folds.BufferToVisible(r.StartLine)
	}
	return 0
}

func (l *rowLayout) visLine(i int) int {
	if l.visible == nil {
		return i
	}
	if i < len(l.visible) {
		return l.visible[i]
	}
	return l.n() + i - len(l.visible)
}

func (l *rowLayout) visibleLine(line int) int {
	if l.folds != nil && line < l.n() {
		if r := l.folds.ContainingFold(line); r != nil {
			return r.StartLine
		}
	}
	return line
}

func (l *rowLayout) startRow(line int) int {
	if l.identity() {
		return l.visIndex(line)
	}
	if line > l.n() {
		return l.startRow(l.n()) + l.phantoms[l.n()] + line - l.n() - 1
	}
	row := 0
	end := l.visIndex(line)
	for i := 0; i < end && i < l.visibleCount(); i++ {
		row += l.blockRows(l.visLine(i))
	}
	return row
}

func (l *rowLayout) total() int {
	if l.identity() {
		return l.visibleCount()
	}
	return l.startRow(l.n()) + l.phantoms[l.n()]
}

func (l *rowLayout) rowOf(line, col int) (row, screenCol int) {
	line = l.visibleLine(line)
	if line >= l.n() {
		return l.total(), 0
	}
	row = l.startRow(line) + l.phantoms[line]
	if !l.wrap {
		return row, 0
	}
	runes := []rune(l.lines[line])
	segs := l.segments(line)
	segIdx := 0
	for i := len(segs) - 1; i >= 0; i-- {
		if col >= segs[i] {
			segIdx = i
			break
		}
	}
	for i := segs[segIdx]; i < col && i < len(runes); i++ {
		if runes[i] == '\t' {
			screenCol = ((screenCol / l.tabW) + 1) * l.tabW
		} else {
			screenCol += textwidth.Rune(runes[i])
		}
	}
	return row + segIdx, screenCol
}

func (l *rowLayout) lastVisibleLine() int {
	if l.visibleCount() == 0 {
		return 0
	}
	return l.visLine(l.visibleCount() - 1)
}

func (l *rowLayout) rowToTop(abs int) (line, offset int) {
	if abs <= 0 || l.n() == 0 {
		return 0, 0
	}
	if l.identity() {
		if abs >= l.visibleCount() {
			abs = l.visibleCount() - 1
		}
		return l.visLine(abs), 0
	}
	acc := 0
	for i := 0; i < l.visibleCount(); i++ {
		ln := l.visLine(i)
		rows := l.blockRows(ln)
		if acc+rows > abs {
			return ln, abs - acc
		}
		acc += rows
	}
	last := l.lastVisibleLine()
	if abs < acc+l.phantoms[l.n()] {
		return last, abs - l.startRow(last)
	}
	return last, 0
}

func (l *rowLayout) maxOffset(line int) int {
	rows := l.blockRows(line)
	if line == l.lastVisibleLine() {
		rows += l.phantoms[l.n()]
	}
	return rows
}

func (l *rowLayout) rows(topLine, offset, h int) []editorRow {
	out := make([]editorRow, 0, h)
	if h <= 0 {
		return out
	}
	if topLine < 0 {
		topLine = 0
	}
	if offset < 0 || offset >= l.maxOffset(topLine) {
		offset = 0
	}
	skip := offset
	emit := func(r editorRow) bool {
		if skip > 0 {
			skip--
			return false
		}
		out = append(out, r)
		return len(out) >= h
	}
	emitBlock := func(line int) bool {
		for k := 0; k < l.phantoms[line]; k++ {
			if emit(editorRow{bufLine: line, phantom: k}) {
				return true
			}
		}
		if line >= l.n() {
			return false
		}
		for _, seg := range l.segments(line) {
			if emit(editorRow{bufLine: line, startCol: seg, phantom: -1}) {
				return true
			}
		}
		return false
	}
	for i := l.visIndex(topLine); i < l.visibleCount(); i++ {
		if emitBlock(l.visLine(i)) {
			return out
		}
	}
	if emitBlock(l.n()) {
		return out
	}
	past := l.n()
	if topLine > past {
		past = topLine
	}
	for len(out) < h {
		out = append(out, editorRow{bufLine: past, phantom: -1})
		past++
	}
	return out
}

func (l *rowLayout) rowInBlock(line, offset int) editorRow {
	ph := l.phantoms[line]
	if offset < ph {
		return editorRow{bufLine: line, phantom: offset}
	}
	segs := l.segments(line)
	idx := offset - ph
	if idx >= len(segs) {
		if line == l.lastVisibleLine() && idx-len(segs) < l.phantoms[l.n()] {
			return editorRow{bufLine: l.n(), phantom: idx - len(segs)}
		}
		idx = len(segs) - 1
	}
	return editorRow{bufLine: line, startCol: segs[idx], phantom: -1}
}

func (e *EditorPaneWidget) topOffset() int {
	if e.topOffsetLine != e.Viewport.TopLine {
		return 0
	}
	return e.topRowOffset
}

func (e *EditorPaneWidget) setTopRow(l *rowLayout, abs int) {
	if abs < 0 {
		abs = 0
	}
	e.Viewport.TopLine, e.topRowOffset = l.rowToTop(abs)
	e.topOffsetLine = e.Viewport.TopLine
}

func (e *EditorPaneWidget) topRow(l *rowLayout) int {
	return l.startRow(e.Viewport.TopLine) + e.topOffset()
}

func (e *EditorPaneWidget) rowAt(y int) editorRow {
	if y >= 0 && y < len(e.rowMap) && e.rowMapTop == e.Viewport.TopLine && e.rowMapOffset == e.topOffset() {
		return e.rowMap[y]
	}
	l := e.layout()
	if l.identity() && l.visible == nil {
		return editorRow{bufLine: e.Viewport.TopLine + y, phantom: -1}
	}
	abs := e.topRow(l) + y
	if abs < 0 {
		abs = 0
	}
	if abs >= l.total() {
		return editorRow{bufLine: l.n() + abs - l.total(), phantom: -1}
	}
	line, off := l.rowToTop(abs)
	return l.rowInBlock(line, off)
}
