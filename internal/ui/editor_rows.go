package ui

import (
	"slices"
	"sort"
	"unsafe"

	"github.com/eugenioenko/ttt/internal/core/buffer"
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

// rowLayoutKey identifies what a cached layout was built from. The buffer
// version covers every edit; the line-slice identity catches a slice assigned
// to Lines without going through the buffer.
type rowLayoutKey struct {
	buf        *buffer.Buffer
	lines      *string
	n          int
	version    uint64
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
	e.syncDiffOverlay()
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
		buf:        e.Buf,
		lines:      unsafe.SliceData(e.Buf.Lines),
		n:          n,
		version:    e.Buf.Version(),
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
	for _, c := range e.layoutCache {
		if c != nil && c.advance(key, e.Buf) {
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
		if i >= 0 && i < len(prev) && prev[i].rows > 0 && len(prev[i].text) == len(text) && unsafe.StringData(prev[i].text) == unsafe.StringData(text) {
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

func sameInputs(a, b rowLayoutKey) bool {
	a.lines, a.n, a.version = nil, 0, 0
	b.lines, b.n, b.version = nil, 0, 0
	return a == b
}

// advance brings an ensured layout up to date with the buffer edits made
// since it was built, re-measuring only the edited lines. It reports false
// when the edits cannot be replayed and the layout must be rebuilt.
func (l *rowLayout) advance(key rowLayoutKey, buf *buffer.Buffer) bool {
	if l.starts == nil || l.visible != nil || key.visible != nil || key.version <= l.key.version || !sameInputs(l.key, key) {
		return false
	}
	n := len(l.lines)
	lo, hi := n, 0
	ok := true
	replayed := buf.ChangesSince(l.key.version, func(c buffer.Change) {
		if !ok {
			return
		}
		if c.Start < 0 || c.Start+c.Removed > n {
			ok = false
			return
		}
		lo, hi = spliceRange(lo, hi, c)
		if c.Added != c.Removed {
			l.spliceStarts(c)
			if l.wrap {
				l.memo = slices.Replace(l.memo, c.Start, c.Start+c.Removed, make([]lineRowsMemo, c.Added)...)
			}
		}
		n += c.Added - c.Removed
	})
	if !replayed || !ok || n != len(buf.Lines) {
		l.starts, l.memo = nil, nil
		return false
	}
	l.lines = buf.Lines
	l.key = key
	l.restart(lo, min(hi, n))
	return true
}

// spliceRange grows the edited range [lo, hi) of earlier changes to cover c,
// moving it to the line numbers after c. A deletion dirties the line that
// takes the removed lines' place, whose row count is no longer known.
func spliceRange(lo, hi int, c buffer.Change) (int, int) {
	end := c.Start + max(c.Added, 1)
	if lo >= hi {
		return c.Start, end
	}
	move := func(p int) int {
		switch {
		case p <= c.Start:
			return p
		case p >= c.Start+c.Removed:
			return p + c.Added - c.Removed
		}
		return c.Start + c.Added
	}
	return min(move(lo), c.Start), max(move(hi), end)
}

// spliceStarts replaces the row starts of the removed lines with placeholders
// for the added ones. The starts after the edit keep their old values, so the
// differences between them, the row counts of unedited lines, stay valid.
func (l *rowLayout) spliceStarts(c buffer.Change) {
	s := c.Start
	switch {
	case c.Added == 0:
		l.starts = slices.Delete(l.starts, s+1, s+c.Removed+1)
	case c.Removed == 0:
		fill := make([]int, c.Added)
		fill[c.Added-1] = l.starts[s]
		l.starts = slices.Insert(l.starts, s+1, fill...)
	default:
		l.starts = slices.Replace(l.starts, s+1, s+c.Removed, make([]int, c.Added-1)...)
	}
}

// restart re-measures the lines in [lo, hi) and shifts the starts after them
// by the change in their rows.
func (l *rowLayout) restart(lo, hi int) {
	n := l.n()
	hi = min(hi, n)
	lo = min(lo, hi)
	oldHi := l.starts[hi]
	for i := lo; i < hi; i++ {
		rows := l.phantomRows(i) + 1
		if l.wrap && !l.hasLabel(i) {
			rows += l.lineRows(i, l.memo, 0) - 1
		}
		l.starts[i+1] = l.starts[i] + rows
	}
	if delta := l.starts[hi] - oldHi; delta != 0 {
		tail := l.starts[hi+1:]
		for i := range tail {
			tail[i] += delta
		}
	}
	l.endRow = l.starts[n] + l.phantomRows(n)
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
	return l.appendRows(make([]editorRow, 0, h), topLine, offset, h)
}

func (l *rowLayout) appendRows(out []editorRow, topLine, offset, h int) []editorRow {
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
