package ui

import (
	"maps"
	"slices"
	"sort"
	"unsafe"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/diff"

	"github.com/gdamore/tcell/v3"
)

const liveDiffContext = 3

type splitPair struct{ l, r int }

// NewEditableDiffWidget shows a file's buffer against its base version. The
// editor pane that owns the buffer is bound by the editor group before each
// render or event, so editing, saving and the rest of the editor keep working.
func NewEditableDiffWidget(filePath string, base []string, lines []diff.DiffLine) *DiffEditorWidget {
	d := &DiffEditorWidget{
		FilePath:      filePath,
		editable:      true,
		oldLines:      base,
		full:          lines,
		contextMode:   DiffContextChangesOnly,
		contextLoaded: true,
		expandedGaps:  make(map[int]bool),
		pendingGap:    -1,
		hoveredGap:    -1,
		syntax:        true,
		signs:         true,
		signsColor:    true,
		left:          newDiffPane(),
		liveCursor:    -1,
		liveActive:    -1,
	}
	d.left.NoWrapMargin = false
	d.attachHighlighters()
	d.rebuild()
	return d
}

func (d *DiffEditorWidget) Editable() bool { return d.editable }

// SetLiveDiff takes a recomputed diff of the buffer as it was at version ver,
// with snap that version's text. Edits made since are replayed onto it.
func (d *DiffEditorWidget) SetLiveDiff(base []string, lines []diff.DiffLine, ver uint64, snap []string) {
	d.oldLines = base
	d.full = lines
	d.fullChanges = nil
	d.liveTouched = nil
	if d.liveBuf != nil {
		d.liveVer, d.liveSnap = ver, snap
		if snap == nil {
			d.liveVer, d.liveSnap = d.liveBuf.Version(), slices.Clone(d.liveBuf.Lines)
		}
	}
	d.ClearSearch()
	d.rebuild()
	if d.OnRecompute != nil {
		d.OnRecompute()
	}
}

func (d *DiffEditorWidget) bind(e *EditorPaneWidget) {
	d.live = e
	if d.liveBuf != e.Buf {
		d.liveBuf = e.Buf
		d.liveCursor = -1
		d.liveVer = e.Buf.Version()
		d.liveSnap = slices.Clone(e.Buf.Lines)
		d.fullChanges, d.liveTouched = nil, nil
		if len(e.Buf.Lines) != d.liveN {
			d.shiftStaleDiff(len(e.Buf.Lines) - d.liveN)
		}
	}
	d.unified, d.right = e, e
	e.WordWrap = d.IsWrapped()
	e.Folds = nil
	if o := d.liveOverlay(); e.DiffOverlay != o {
		e.SetDiffOverlay(o)
	}
	d.syncLive()
}

func (d *DiffEditorWidget) liveOverlay() *DiffOverlay {
	if d.IsUnified() {
		return d.liveUnified
	}
	return d.rightBase
}

// compactLiveDiff keeps changed rows and their context, folding each other
// run of unchanged rows into one gap row. right maps every row to its buffer
// line (-1 when blank on that side); a gap row maps to the first line it hides.
func compactLiveDiff(full []diff.DiffLine, compact bool, revealed [][2]int) (rows []diff.DiffLine, gaps map[int]int, right []int, gapLen map[int]int, spans [][2]int) {
	gaps, gapLen = make(map[int]int), make(map[int]int)
	keep := make([]bool, len(full))
	for i, dl := range full {
		if !compact {
			keep[i] = true
			continue
		}
		if dl.Left.Kind != diff.Context || dl.Right.Kind != diff.Context {
			for k := max(i-liveDiffContext, 0); k <= min(i+liveDiffContext, len(full)-1); k++ {
				keep[k] = true
			}
			continue
		}
		for _, span := range revealed {
			if dl.Left.Num >= span[0] && dl.Left.Num <= span[1] {
				keep[i] = true
				break
			}
		}
	}
	for i := 0; i < len(full); {
		if keep[i] {
			rows = append(rows, full[i])
			right = append(right, full[i].Right.Num-1)
			i++
			continue
		}
		j := i
		for j < len(full) && !keep[j] {
			j++
		}
		label := collapsedDistanceLabel(j - i)
		gaps[len(rows)] = len(spans)
		gapLen[len(rows)] = j - i
		spans = append(spans, [2]int{full[i].Left.Num, full[j-1].Left.Num})
		rows = append(rows, diff.DiffLine{
			Left:  diff.SideLine{Kind: diff.Collapsed, Text: label},
			Right: diff.SideLine{Kind: diff.Collapsed, Text: label},
		})
		right = append(right, full[i].Right.Num-1)
		i = j
	}
	return rows, gaps, right, gapLen, spans
}

func (d *DiffEditorWidget) rebuildLive() {
	if cs, _ := d.takeChanges(); len(cs) > 0 {
		for _, c := range cs {
			d.recordChange(c)
		}
	}
	d.Lines, d.gapByLine, d.liveRows, d.liveGapLen, d.liveSpans = compactLiveDiff(d.full, d.contextMode != DiffContextFullFile, d.revealed)
	d.liveN = 0
	for _, dl := range d.full {
		if dl.Right.Kind != diff.Blank {
			d.liveN++
		}
	}
	for _, c := range d.fullChanges {
		for i, r := range d.liveRows {
			if r >= 0 {
				d.liveRows[i] = shiftLine(r, c)
			}
		}
		d.liveN += c.Added - c.Removed
	}
	if d.liveBuf != nil {
		d.liveN = len(d.liveBuf.Lines)
	}
	d.hoveredGap = -1
	d.captured = nil

	d.syncDocSyntax()
	d.leftBase = &DiffOverlay{Nums: []int{}, Gaps: map[int]int{}, Fillers: map[int]int{}, Syntax: &d.docSyntax}
	var lLines []string
	d.leftRows = nil
	for i, dl := range d.Lines {
		lLines, d.leftRows = appendDiffSide(d.leftBase, lLines, d.leftRows, dl.Left, i, d.gapByLine)
	}
	d.resetPane(d.left, lLines, d.leftBase)
	d.buildLiveOverlays()
	d.applyOverlayOptions()
	if gap, ok := d.hiddenCursorGap(); ok {
		d.revealed = append(d.revealed, d.liveSpans[gap])
		d.rebuildLive()
	}
}

func (d *DiffEditorWidget) hiddenCursorGap() (int, bool) {
	if d.live == nil || d.liveCursor < 0 {
		return 0, false
	}
	line := d.live.Cursor.Line
	if _, _, ok := d.live.diffHiddenRange(line); !ok {
		return 0, false
	}
	return d.liveGapAt(line)
}

func (d *DiffEditorWidget) liveGapAt(line int) (int, bool) {
	for i, r := range d.liveRows {
		gap, ok := d.gapByLine[i]
		if ok && line >= r && line < r+d.liveGapLen[i] {
			return gap, true
		}
	}
	return 0, false
}

func (d *DiffEditorWidget) buildLiveOverlays() {
	n := d.liveN
	kinds := make([]diff.LineKind, n)
	for i := range kinds {
		kinds[i] = diff.Context
	}
	labels, gaps := map[int]string{}, map[int]int{}
	var hidden []diffHiddenRange
	for i, dl := range d.Lines {
		r := d.liveRows[i]
		if r < 0 || r >= n {
			continue
		}
		if gap, ok := d.gapByLine[i]; ok {
			kinds[r] = diff.Collapsed
			labels[r] = dl.Right.Text
			gaps[r] = gap
			if end := min(r+d.liveGapLen[i]-1, n-1); end > r {
				hidden = append(hidden, diffHiddenRange{start: r + 1, end: end})
			}
			continue
		}
		kinds[r] = dl.Right.Kind
	}

	d.tintTouched(kinds)

	deleted := make(map[int][]diff.SideLine)
	for i := 0; i < len(d.Lines); {
		if d.Lines[i].Left.Kind != diff.Deleted && d.Lines[i].Right.Kind != diff.Added {
			i++
			continue
		}
		end := i
		for end < len(d.Lines) && (d.Lines[end].Left.Kind == diff.Deleted || d.Lines[end].Right.Kind == diff.Added) {
			end++
		}
		anchor := -1
		for k := i; k < end && anchor < 0; k++ {
			if d.Lines[k].Right.Kind == diff.Added {
				anchor = d.liveRows[k]
			}
		}
		for k := end; k < len(d.Lines) && anchor < 0; k++ {
			anchor = d.liveRows[k]
		}
		if anchor < 0 {
			anchor = n
		}
		for k := i; k < end; k++ {
			if d.Lines[k].Left.Kind == diff.Deleted {
				deleted[anchor] = append(deleted[anchor], d.Lines[k].Left)
			}
		}
		i = end
	}

	d.liveUnified = &DiffOverlay{Kinds: kinds, Deleted: deleted, Gaps: gaps, Labels: labels, Hidden: hidden, DeletedActive: [2]int{-1, -1}, Syntax: &d.docSyntax}
	d.rightBase = &DiffOverlay{Kinds: slices.Clone(kinds), Fillers: map[int]int{}, Gaps: maps.Clone(gaps), Labels: maps.Clone(labels), Hidden: slices.Clone(hidden)}
	if d.liveBuf != nil {
		d.liveUnified.buf, d.liveUnified.ver = d.liveBuf, d.liveVer
		d.rightBase.buf, d.rightBase.ver = d.liveBuf, d.liveVer
	}
	d.alignContribs = nil
	d.pairs = d.pairs[:0]
	li := 0
	for i, dl := range d.Lines {
		l := -1
		if dl.Left.Kind != diff.Blank {
			l = li
			li++
		}
		d.pairs = append(d.pairs, splitPair{l: l, r: d.liveRows[i]})
	}
	if d.live != nil && d.live.Buf == d.liveBuf {
		d.live.SetDiffOverlay(d.liveOverlay())
	}
	d.applyOverlayOptions()
	d.applyLiveSearch()
}

func (d *DiffEditorWidget) syncLive() {
	e := d.live
	if e == nil {
		return
	}
	edited := d.trackLiveEdits()
	if d.liveCursor < 0 {
		d.placeInitialCursor()
		return
	}
	if edited || e.Cursor.Line != d.liveCursor {
		d.liveCursor = e.Cursor.Line
		if gap, ok := d.hiddenCursorGap(); ok {
			d.expandContextGap(gap)
		}
	}
}

// takeChanges reads the buffer edits made since liveVer. exact is false when
// the change log could not replay them, or one replaced the whole buffer; the
// edit is then found by comparing the text with liveSnap.
func (d *DiffEditorWidget) takeChanges() (cs []buffer.Change, ok bool) {
	buf := d.liveBuf
	if buf == nil || buf.Version() == d.liveVer {
		return nil, true
	}
	n, exact := len(d.liveSnap), true
	exact = buf.ChangesSince(d.liveVer, func(c buffer.Change) {
		if c.Start == 0 && c.Removed == n && n > 1 {
			exact = false
		}
		n += c.Added - c.Removed
		cs = append(cs, c)
	}) && exact
	if !exact {
		cs = nil
		if start, oldEnd, newEnd, changed := changedLineRange(d.liveSnap, buf.Lines); changed {
			cs = []buffer.Change{{Start: start, Removed: oldEnd - start, Added: newEnd - start}}
		}
		d.liveSnap = slices.Clone(buf.Lines)
	} else {
		d.liveSnap = spliceSnapshot(d.liveSnap, buf.Lines, cs)
	}
	d.liveVer = buf.Version()
	return cs, exact
}

func spliceSnapshot(snap, cur []string, cs []buffer.Change) []string {
	if len(cs) == 0 {
		return snap
	}
	a, b, delta := cs[0].Start, cs[0].Start+cs[0].Added, cs[0].Added-cs[0].Removed
	for _, c := range cs[1:] {
		bPre := max(b, c.Start+c.Removed)
		if b < c.Start {
			bPre = c.Start + c.Removed
		}
		a = min(a, c.Start)
		b = bPre + c.Added - c.Removed
		delta += c.Added - c.Removed
	}
	a = min(a, len(cur))
	b = min(b, len(cur))
	return slices.Replace(snap, a, min(b-delta, len(snap)), cur[a:b]...)
}

func (d *DiffEditorWidget) recordChange(c buffer.Change) {
	var touched [][2]int
	for _, t := range d.liveTouched {
		if left := [2]int{t[0], min(t[1], c.Start)}; left[0] < left[1] {
			touched = append(touched, left)
		}
		if right := [2]int{max(t[0], c.Start+c.Removed), t[1]}; right[0] < right[1] {
			delta := c.Added - c.Removed
			touched = append(touched, [2]int{right[0] + delta, right[1] + delta})
		}
	}
	if c.Added > 0 {
		touched = append(touched, [2]int{c.Start, c.Start + c.Added})
	}
	d.liveTouched = touched
	if c.Added != c.Removed {
		d.fullChanges = append(d.fullChanges, c)
	}
}

func editsGap(c buffer.Change, line, n int) bool {
	if c.Removed == 0 {
		return c.Start > line && c.Start < line+n
	}
	return c.Start < line+n && c.Start+c.Removed > line
}

// trackLiveEdits moves the diff with the edits made since the last sync, until
// the recomputed diff arrives.
func (d *DiffEditorWidget) trackLiveEdits() bool {
	if d.liveBuf == nil {
		return false
	}
	prevVer := d.liveVer
	cs, exact := d.takeChanges()
	if len(cs) == 0 {
		return false
	}
	d.liveExpand = d.liveExpand[:0]
	var window []int
	for _, c := range cs {
		for i, gap := range d.gapByLine {
			if editsGap(c, d.liveRows[i], d.liveGapLen[i]) {
				d.liveExpand = append(d.liveExpand, gap)
			}
		}
		window = d.shiftLive(c, window)
		d.recordChange(c)
	}
	d.liveN = len(d.liveBuf.Lines)
	if !exact || len(d.liveExpand) > 0 {
		for _, gap := range d.liveExpand {
			if gap >= 0 && gap < len(d.liveSpans) {
				d.revealed = append(d.revealed, d.liveSpans[gap])
			}
		}
		d.rebuildLive()
		return true
	}
	e := d.live
	if e == nil || e.Buf != d.liveBuf {
		return true
	}
	if !e.syncDiffOverlay() {
		d.buildLiveOverlays()
		return true
	}
	if d.IsUnified() || d.alignContribs == nil || d.alignVer != prevVer || d.right != e ||
		e.DiffOverlay != d.rightBase || d.left.DiffOverlay != d.leftBase {
		return true
	}
	d.realignLive(window)
	return true
}

// shiftLive adds to window the split rows whose wrapped height may change.
func (d *DiffEditorWidget) shiftLive(c buffer.Change, window []int) []int {
	inNew := func(r int) bool { return r >= c.Start && r < c.Start+max(c.Added, 1) }
	if c.Added == c.Removed {
		i := sort.Search(len(d.liveRows), func(i int) bool { return d.rowLineFrom(i) >= c.Start })
		for ; i < len(d.liveRows); i++ {
			r := d.liveRows[i]
			if r >= c.Start+c.Added {
				break
			}
			if inNew(r) {
				window = append(window, i)
			}
		}
		return window
	}
	from := sort.Search(len(d.liveRows), func(i int) bool { return d.rowLineFrom(i) >= c.Start })
	for i := from; i < len(d.liveRows); i++ {
		r := d.liveRows[i]
		if r < 0 {
			continue
		}
		nr := shiftLine(r, c)
		if (r >= c.Start && r < c.Start+c.Removed) || inNew(nr) {
			window = append(window, i)
		}
		d.liveRows[i] = nr
		if i < len(d.pairs) {
			d.pairs[i].r = nr
		}
	}
	for i := range d.alignContribs {
		if d.alignContribs[i].rAmt > 0 && d.alignContribs[i].rKey >= c.Start {
			d.alignContribs[i].rKey = shiftLine(d.alignContribs[i].rKey, c)
		}
	}
	for i := range d.liveRefs {
		if d.liveRefs[i].anchor >= 0 {
			d.liveRefs[i].anchor = shiftLine(d.liveRefs[i].anchor, c)
		}
	}
	if e := d.live; e != nil && e.Buf == d.liveBuf && len(e.SearchMatches) > 0 {
		for i := range e.SearchMatches {
			e.SearchMatches[i].Line = shiftLine(e.SearchMatches[i].Line, c)
		}
		e.buildSearchIndex()
	}
	return window
}

func (d *DiffEditorWidget) rowLineFrom(i int) int {
	for ; i < len(d.liveRows); i++ {
		if r := d.liveRows[i]; r >= 0 {
			return r
		}
	}
	return int(^uint(0) >> 1)
}

func (d *DiffEditorWidget) realignLive(window []int) {
	if len(window) > 0 {
		d.right.advanceLayouts()
		ll, rl := d.left.layout(), d.right.layout()
		slices.Sort(window)
		window = slices.Compact(window)
		next := func(i int, right bool) int {
			for ; i < len(d.pairs); i++ {
				if right && d.pairs[i].r >= 0 {
					return d.pairs[i].r
				}
				if !right && d.pairs[i].l >= 0 {
					return d.pairs[i].l
				}
			}
			if right {
				return len(d.right.Buf.Lines)
			}
			return len(d.left.Buf.Lines)
		}
		for _, i := range window {
			if i >= len(d.pairs) || i >= len(d.alignContribs) {
				continue
			}
			p := d.pairs[i]
			nc := pairContrib(p, alignSegs(d.left, ll, p.l), alignSegs(d.right, rl, p.r), next(i, false), next(i+1, false), next(i, true), next(i+1, true))
			oc := d.alignContribs[i]
			if nc == oc {
				continue
			}
			d.left.addFillers(oc.lKey, -oc.lAmt)
			d.left.addFillers(nc.lKey, nc.lAmt)
			d.right.addFillers(oc.rKey, -oc.rAmt)
			d.right.addFillers(nc.rKey, nc.rAmt)
			d.alignContribs[i] = nc
		}
	}
	d.alignVer = d.right.Buf.Version()
	d.alignKey = splitAlignKey{
		pairs: unsafe.SliceData(d.pairs), nPairs: len(d.pairs),
		left: d.leftBase, right: d.rightBase,
		lPane: paneAlignKeyOf(d.left), rPane: paneAlignKeyOf(d.right),
	}
}

// shiftStaleDiff handles a buffer whose edits since the diff are unknown:
// rows below the cursor move by the change in line count.
func (d *DiffEditorWidget) shiftStaleDiff(delta int) {
	at := d.live.Cursor.Line
	for i, r := range d.liveRows {
		if r > at {
			d.liveRows[i] = max(r+delta, at)
		}
	}
	d.liveN = len(d.live.Buf.Lines)
	d.buildLiveOverlays()
}

// changedLineRange reports the lines that differ between two versions of a
// buffer as [start, oldEnd) in before and [start, newEnd) in after.
func changedLineRange(before, after []string) (start, oldEnd, newEnd int, changed bool) {
	n := min(len(before), len(after))
	for start < n && before[start] == after[start] {
		start++
	}
	oldEnd, newEnd = len(before), len(after)
	for oldEnd > start && newEnd > start && before[oldEnd-1] == after[newEnd-1] {
		oldEnd--
		newEnd--
	}
	return start, oldEnd, newEnd, start != oldEnd || start != newEnd
}

func (d *DiffEditorWidget) tintTouched(kinds []diff.LineKind) {
	for _, t := range d.liveTouched {
		for line := max(t[0], 0); line < t[1] && line < len(kinds); line++ {
			if kinds[line] == diff.Context {
				kinds[line] = diff.Added
			}
		}
	}
}

func (d *DiffEditorWidget) revealCursorGap() {
	if d.live == nil {
		return
	}
	if gap, ok := d.live.DiffOverlay.Gaps[d.live.Cursor.Line]; ok {
		d.expandContextGap(gap)
	}
}

func (d *DiffEditorWidget) placeInitialCursor() {
	e := d.live
	line := e.Cursor.Line
	_, _, hidden := e.diffHiddenRange(line)
	_, onGap := e.DiffOverlay.label(line)
	if hidden || onGap {
		for i, r := range d.liveRows {
			if _, gap := d.gapByLine[i]; !gap && r >= 0 && d.Lines[i].Right.Kind != diff.Blank {
				e.Cursor.Line, e.Cursor.Col = e.Buf.ClampLine(r), 0
				e.Viewport.TopLine = 0
				break
			}
		}
	}
	d.liveCursor = e.Cursor.Line
}

func isNavigationKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight, tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyEscape:
		return true
	}
	return false
}

// keyPane is the pane keys move: the one last clicked in a split, the
// buffer's own pane in an editable diff.
func (d *DiffEditorWidget) keyPane() *EditorPaneWidget {
	if !d.editable {
		return d.lead()
	}
	if d.headFocused() {
		return d.left
	}
	return d.live
}

func (d *DiffEditorWidget) headFocused() bool {
	return d.editable && !d.IsUnified() && d.focusLeft
}

// handleLiveKey sends keys to the pane last clicked. The base pane is
// read-only, so typing there changes nothing.
func (d *DiffEditorWidget) handleLiveKey(ev *tcell.EventKey) EventResult {
	if d.headFocused() {
		d.left.HandleEvent(ev)
		return EventConsumed
	}
	e := d.live
	if gap, ok := e.DiffOverlay.Gaps[e.Cursor.Line]; ok && !isNavigationKey(ev) {
		d.expandContextGap(gap)
		if ev.Key() == tcell.KeyEnter && ev.Modifiers() == 0 {
			return EventConsumed
		}
	}
	return e.HandleEvent(ev)
}

// HeadSelection is the text selected in the read-only base pane of an editable
// split diff, when that pane was clicked last.
func (d *DiffEditorWidget) HeadSelection() (string, bool) {
	if !d.editable || d.IsUnified() || !d.focusLeft || !d.left.Selection.Active {
		return "", false
	}
	return d.CopySelection(), true
}
