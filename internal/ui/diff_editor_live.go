package ui

import (
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
		left:          newDiffPane(),
		liveCursor:    -1,
	}
	d.attachHighlighters()
	d.rebuild()
	return d
}

func (d *DiffEditorWidget) Editable() bool { return d.editable }

func (d *DiffEditorWidget) SetLiveDiff(base []string, lines []diff.DiffLine) {
	d.oldLines = base
	d.full = lines
	d.rebuild()
}

func (d *DiffEditorWidget) bind(e *EditorPaneWidget) {
	if d.live != e {
		d.live = e
		d.liveCursor = -1
	}
	d.unified, d.right = e, e
	e.WordWrap = d.IsWrapped()
	e.Folds = nil
	e.SetDiffOverlay(d.liveOverlay())
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
	d.Lines, d.gapByLine, d.liveRows, d.liveGapLen, d.liveSpans = compactLiveDiff(d.full, d.contextMode != DiffContextFullFile, d.revealed)
	d.liveN = 0
	for _, dl := range d.full {
		if dl.Right.Kind != diff.Blank {
			d.liveN++
		}
	}
	d.hoveredGap = -1
	d.captured = nil

	d.leftBase = &DiffOverlay{Nums: []int{}, Gaps: map[int]int{}, Fillers: map[int]int{}}
	var lLines []string
	d.leftRows = nil
	for i, dl := range d.Lines {
		lLines, d.leftRows = appendDiffSide(d.leftBase, lLines, d.leftRows, dl.Left, i, d.gapByLine)
	}
	d.resetPane(d.left, lLines, d.leftBase)
	d.buildLiveOverlays()
	d.applyOverlayOptions()
	d.ClearSearch()
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

	d.liveUnified = &DiffOverlay{Kinds: kinds, Deleted: deleted, Gaps: gaps, Labels: labels, Hidden: hidden}
	d.rightBase = &DiffOverlay{Kinds: kinds, Fillers: map[int]int{}, Gaps: gaps, Labels: labels, Hidden: hidden}
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
	if d.live != nil {
		d.live.SetDiffOverlay(d.liveOverlay())
	}
	d.applyOverlayOptions()
}

// syncLive keeps the overlay roughly in place while the buffer is edited and
// the recomputed diff is still pending: rows below the edit shift with it.
func (d *DiffEditorWidget) syncLive() {
	e := d.live
	if e == nil {
		return
	}
	if n := len(e.Buf.Lines); n != d.liveN {
		delta := n - d.liveN
		at := e.Cursor.Line
		if d.liveCursor >= 0 {
			at = min(at, d.liveCursor)
		}
		for i, r := range d.liveRows {
			if r > at {
				d.liveRows[i] = max(r+delta, at)
			}
		}
		d.liveN = n
		d.buildLiveOverlays()
	}
	if d.liveCursor < 0 {
		d.placeInitialCursor()
		return
	}
	if e.Cursor.Line != d.liveCursor {
		d.liveCursor = e.Cursor.Line
		if gap, ok := d.hiddenCursorGap(); ok {
			d.expandContextGap(gap)
		}
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

func (d *DiffEditorWidget) handleLiveKey(ev *tcell.EventKey) EventResult {
	e := d.live
	d.focusLeft = false
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
