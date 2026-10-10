package ui

import (
	"sort"
	"unsafe"

	"github.com/eugenioenko/ttt/internal/core/diff"
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
	visible  []int
	phantoms map[int]int
	labels   map[int]string
	deleted  map[int][]diff.SideLine

	key    rowLayoutKey
	starts []int
	memo   []lineRowsMemo
	endRow int
}

// rowLayoutKey identifies what a cached layout was built from. Edits made
// through the editor bump editGen; the line-slice identity catches buffers
// replaced wholesale.
type rowLayoutKey struct {
	lines      *string
	n          int
	editGen    uint64
	overlay    *DiffOverlay
	overlayGen uint64
	visible    *int
	visN       int
	wrap       bool
	width      int
	tabW       int
}

type lineRowsMemo struct {
	text string
	rows int
}

func (e *EditorPaneWidget) rowLayout(width int) *rowLayout {
	if width < 1 {
		width = 1
	}
	n := len(e.Buf.Lines)
	var visible []int
	folds, hidden := e.hasFolds(), e.DiffOverlay.hasHidden()
	switch {
	case folds && hidden:
		for _, line := range e.Folds.VisibleLines(n) {
			if !e.DiffOverlay.hidden(line) {
				visible = append(visible, line)
			}
		}
		if visible == nil {
			visible = []int{}
		}
	case folds:
		visible = e.Folds.VisibleLines(n)
	case hidden:
		visible = e.DiffOverlay.visibleLines(n)
	}
	key := rowLayoutKey{
		lines:      unsafe.SliceData(e.Buf.Lines),
		n:          n,
		editGen:    e.editGen,
		overlay:    e.DiffOverlay,
		overlayGen: e.overlayGen,
		visible:    unsafe.SliceData(visible),
		visN:       len(visible),
		wrap:       e.WordWrap,
		tabW:       e.resolveTabSize(),
	}
	if e.WordWrap {
		key.width = width
	}
	for _, c := range e.layoutCache {
		if c != nil && c.key == key {
			return c
		}
	}
	l := &rowLayout{
		lines:   e.Buf.Lines,
		wrap:    e.WordWrap,
		width:   width,
		tabW:    key.tabW,
		visible: visible,
		key:     key,
	}
	if len(e.phantoms) > 0 {
		l.phantoms = e.phantoms
	}
	if e.DiffOverlay != nil {
		l.labels = e.DiffOverlay.Labels
		l.deleted = e.DiffOverlay.Deleted
	}
	slot := 0
	for i, c := range e.layoutCache {
		if c == nil {
			slot = i
			break
		}
		if c.key.wrap == key.wrap && c.key.width == key.width && c.key.tabW == key.tabW {
			l.memo = c.memo
			slot = i
			break
		}
		if i == len(e.layoutCache)-1 {
			slot = e.layoutNext
			e.layoutNext = (e.layoutNext + 1) % len(e.layoutCache)
		}
	}
	e.layoutCache[slot] = l
	return l
}

// lineRows is how many rows a wrapped line takes, reusing the count from the
// previous layout when the line's text is unchanged; a line shifted by an
// insert or delete is found at its old index offset by the line delta.
func (l *rowLayout) lineRows(line int, prev []lineRowsMemo, delta int) int {
	text := l.lines[line]
	for _, i := range [2]int{line, line - delta} {
		if i >= 0 && i < len(prev) && len(prev[i].text) == len(text) && unsafe.StringData(prev[i].text) == unsafe.StringData(text) {
			l.memo[line] = prev[i]
			return prev[i].rows
		}
	}
	rows := len(wrapLineSegments([]rune(text), l.width, l.tabW))
	l.memo[line] = lineRowsMemo{text: text, rows: rows}
	return rows
}

// textRows is how many rows a buffer line's text wraps into, phantoms aside.
func (l *rowLayout) textRows(line int) int {
	if !l.wrap || line < 0 || line >= l.n() || l.hasLabel(line) {
		return 1
	}
	l.ensure()
	if m := l.memo[line]; m.rows > 0 {
		return m.rows
	}
	return len(l.segments(line))
}

func (l *rowLayout) ensure() {
	if l.starts != nil {
		return
	}
	prev := l.memo
	if l.wrap {
		l.memo = make([]lineRowsMemo, l.n())
	}
	delta := l.n() - len(prev)
	count := l.visibleCount()
	starts := make([]int, count+1)
	row := 0
	for i := 0; i < count; i++ {
		starts[i] = row
		line := l.visLine(i)
		row += l.phantomRows(line)
		switch {
		case !l.wrap:
			row++
		case l.hasLabel(line):
			row++
		default:
			row += l.lineRows(line, prev, delta)
		}
	}
	starts[count] = row
	l.starts = starts
	l.endRow = row + l.phantomRows(l.n())
}

func (e *EditorPaneWidget) layout() *rowLayout {
	return e.rowLayout(e.Viewport.Width)
}

func (l *rowLayout) n() int { return len(l.lines) }

func (l *rowLayout) identity() bool { return !l.wrap && l.phantoms == nil }

func (l *rowLayout) hasLabel(line int) bool {
	_, ok := l.labels[line]
	return ok
}

func (l *rowLayout) segments(line int) []int {
	if l.hasLabel(line) {
		return singleSegment
	}
	if l.wrap && line >= 0 && line < l.n() {
		return wrapLineSegments([]rune(l.lines[line]), l.width, l.tabW)
	}
	return singleSegment
}

// phantomSegments is where each wrapped row of the k-th phantom line anchored
// at line starts; deleted lines wrap like buffer lines, fillers never do.
func (l *rowLayout) phantomSegments(line, k int) []int {
	if !l.wrap {
		return singleSegment
	}
	block := l.deleted[line]
	if k >= len(block) {
		return singleSegment
	}
	return wrapLineSegments([]rune(block[k].Text), l.width, l.tabW)
}

func (l *rowLayout) phantomRows(line int) int {
	n := l.phantoms[line]
	if !l.wrap || n == 0 {
		return n
	}
	rows := 0
	for k := 0; k < n; k++ {
		rows += len(l.phantomSegments(line, k))
	}
	return rows
}

func (l *rowLayout) blockRows(line int) int {
	rows := l.phantomRows(line)
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
	i := sort.SearchInts(l.visible, line)
	if i < len(l.visible) && l.visible[i] == line {
		return i
	}
	if i > 0 {
		return i - 1
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
	if l.visible != nil && line < l.n() && len(l.visible) > 0 {
		return l.visible[l.visIndex(line)]
	}
	return line
}

func (l *rowLayout) startRow(line int) int {
	if l.identity() {
		return l.visIndex(line)
	}
	if line > l.n() {
		return l.startRow(l.n()) + l.phantomRows(l.n()) + line - l.n() - 1
	}
	l.ensure()
	return l.starts[min(l.visIndex(line), l.visibleCount())]
}

func (l *rowLayout) total() int {
	if l.identity() {
		return l.visibleCount()
	}
	l.ensure()
	return l.endRow
}

func (l *rowLayout) rowOf(line, col int) (row, screenCol int) {
	line = l.visibleLine(line)
	if line >= l.n() {
		return l.total(), 0
	}
	row = l.startRow(line) + l.phantomRows(line)
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
	l.ensure()
	count := l.visibleCount()
	if i := sort.Search(count, func(i int) bool { return l.starts[i+1] > abs }); i < count {
		return l.visLine(i), abs - l.starts[i]
	}
	acc := l.starts[count]
	last := l.lastVisibleLine()
	if abs < acc+l.phantomRows(l.n()) {
		return last, abs - l.startRow(last)
	}
	return last, 0
}

func (l *rowLayout) maxOffset(line int) int {
	rows := l.blockRows(line)
	if line == l.lastVisibleLine() {
		rows += l.phantomRows(l.n())
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
			for _, seg := range l.phantomSegments(line, k) {
				if emit(editorRow{bufLine: line, startCol: seg, phantom: k}) {
					return true
				}
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

func (l *rowLayout) phantomAt(line, offset int) (editorRow, bool) {
	for k := 0; k < l.phantoms[line]; k++ {
		segs := l.phantomSegments(line, k)
		if offset < len(segs) {
			return editorRow{bufLine: line, startCol: segs[offset], phantom: k}, true
		}
		offset -= len(segs)
	}
	return editorRow{}, false
}

func (l *rowLayout) rowInBlock(line, offset int) editorRow {
	ph := l.phantomRows(line)
	if offset < ph {
		r, _ := l.phantomAt(line, offset)
		return r
	}
	segs := l.segments(line)
	idx := offset - ph
	if idx >= len(segs) {
		if line == l.lastVisibleLine() {
			if r, ok := l.phantomAt(l.n(), idx-len(segs)); ok {
				return r
			}
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
