package ui

import (
	"sort"

	"github.com/eugenioenko/ttt/internal/core/diff"
)

// liveSearchHit is a find match in an editable diff. match.Line is a row of
// the uncompacted diff, so a match inside a collapsed gap survives until the
// gap is revealed and the panes are rebuilt.
type liveSearchHit struct {
	match FindMatch
	right bool
}

type liveSearchRef struct {
	pane    *EditorPaneWidget
	paneIdx int
	anchor  int
	deleted int
}

func (d *DiffEditorWidget) searchRows() []diff.DiffLine {
	if d.editable {
		return d.full
	}
	return d.Lines
}

// setLiveSearch takes matches by row of LeftLines/RightLines. Unified mode
// keeps base-side matches only on removed lines, since every other base line
// is also a line of the file.
func (d *DiffEditorWidget) setLiveSearch(left, right []FindMatch) []FindMatch {
	d.SearchMatchesLeft, d.SearchMatchesRight = left, right
	var hits []liveSearchHit
	for _, m := range left {
		if m.Line < 0 || m.Line >= len(d.full) {
			continue
		}
		kind := d.full[m.Line].Left.Kind
		if kind == diff.Blank || (d.IsUnified() && kind != diff.Deleted) {
			continue
		}
		hits = append(hits, liveSearchHit{match: m})
	}
	for _, m := range right {
		if m.Line >= 0 && m.Line < len(d.full) && d.full[m.Line].Right.Kind != diff.Blank {
			hits = append(hits, liveSearchHit{match: m, right: true})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.match.Line != b.match.Line {
			return a.match.Line < b.match.Line
		}
		if a.right != b.right {
			return !a.right
		}
		return a.match.Col < b.match.Col
	})
	d.liveHits = hits
	d.liveActive = -1
	d.applyLiveSearch()
	merged := make([]FindMatch, len(hits))
	for i, h := range hits {
		merged[i] = h.match
	}
	return merged
}

func (d *DiffEditorWidget) clearLiveSearch() {
	d.liveHits = nil
	d.liveActive = -1
	d.liveRefs = nil
	if d.live != nil {
		d.live.SearchMatches = nil
		d.live.SearchActive = -1
		d.live.searchByLine = nil
	}
	if d.liveUnified != nil {
		d.liveUnified.DeletedMatches = nil
		d.liveUnified.DeletedActive = [2]int{-1, -1}
	}
}

// applyLiveSearch maps the hits onto the panes as they are now built.
func (d *DiffEditorWidget) applyLiveSearch() {
	if d.liveHits == nil {
		return
	}
	e := d.live
	d.left.SearchMatches = nil
	if e != nil {
		e.SearchMatches = nil
	}
	leftLine := make(map[int]int)
	if d.leftBase != nil {
		for line, num := range d.leftBase.Nums {
			if num > 0 {
				leftLine[num] = line
			}
		}
	}
	type deletedPos struct{ anchor, index int }
	deleted := make(map[int]deletedPos)
	if d.liveUnified != nil {
		d.liveUnified.DeletedMatches = make(map[int][]FindMatch)
		for anchor, block := range d.liveUnified.Deleted {
			for index, side := range block {
				deleted[side.Num] = deletedPos{anchor, index}
			}
		}
	}
	d.liveRefs = make([]liveSearchRef, len(d.liveHits))
	for i, h := range d.liveHits {
		dl := d.full[h.match.Line]
		ref := liveSearchRef{paneIdx: -1, anchor: -1, deleted: -1}
		switch {
		case h.right:
			if e != nil {
				ref.pane, ref.paneIdx = e, len(e.SearchMatches)
				e.SearchMatches = append(e.SearchMatches, FindMatch{Line: dl.Right.Num - 1, Col: h.match.Col, Len: h.match.Len})
			}
		case d.IsUnified():
			if pos, ok := deleted[dl.Left.Num]; ok && d.liveUnified != nil {
				ref.anchor, ref.deleted = pos.anchor, len(d.liveUnified.DeletedMatches[pos.anchor])
				d.liveUnified.DeletedMatches[pos.anchor] = append(d.liveUnified.DeletedMatches[pos.anchor], FindMatch{Line: pos.index, Col: h.match.Col, Len: h.match.Len})
			}
		default:
			if line, ok := leftLine[dl.Left.Num]; ok {
				ref.pane, ref.paneIdx = d.left, len(d.left.SearchMatches)
				d.left.SearchMatches = append(d.left.SearchMatches, FindMatch{Line: line, Col: h.match.Col, Len: h.match.Len})
			}
		}
		d.liveRefs[i] = ref
	}
	d.left.buildSearchIndex()
	if e != nil {
		e.buildSearchIndex()
	}
	d.setLiveActive(d.liveActive)
}

func (d *DiffEditorWidget) setLiveActive(index int) {
	d.liveActive = index
	d.left.SearchActive = -1
	if d.live != nil {
		d.live.SearchActive = -1
	}
	if d.liveUnified != nil {
		d.liveUnified.DeletedActive = [2]int{-1, -1}
	}
	d.searchActiveRight = false
	if index < 0 || index >= len(d.liveRefs) {
		return
	}
	ref := d.liveRefs[index]
	d.searchActiveRight = d.liveHits[index].right
	if ref.pane != nil {
		ref.pane.SearchActive = ref.paneIdx
	} else if ref.anchor >= 0 && d.liveUnified != nil {
		d.liveUnified.DeletedActive = [2]int{ref.anchor, ref.deleted}
	}
}

// scrollToLiveHit reveals the active hit, expanding the gap that hides it.
func (d *DiffEditorWidget) scrollToLiveHit(row int) {
	e := d.live
	if e == nil || row < 0 || row >= len(d.full) {
		return
	}
	dl := d.full[row]
	if dl.Left.Num > 0 {
		for _, span := range d.liveSpans {
			if dl.Left.Num >= span[0] && dl.Left.Num <= span[1] {
				d.revealed = append(d.revealed, span)
				d.rebuild()
				break
			}
		}
	}
	col := 0
	if d.liveActive >= 0 && d.liveActive < len(d.liveHits) {
		col = d.liveHits[d.liveActive].match.Col
	}
	switch {
	case d.searchActiveRight:
		d.focusLeft = false
		e.Selection.Clear()
		e.Cursor.Line, e.Cursor.Col = e.Buf.ClampLine(dl.Right.Num-1), col
		d.liveCursor = e.Cursor.Line
		e.scrollViewport()
	case d.IsUnified():
		ref := d.liveRefs[d.liveActive]
		if ref.anchor < 0 {
			return
		}
		l := e.layout()
		target := l.startRow(ref.anchor) + d.liveUnified.DeletedMatches[ref.anchor][ref.deleted].Line
		top, h := e.topRow(l), e.Viewport.Height
		if h <= 0 || target < top || target >= top+h {
			e.setTopRow(l, target-h/2)
		}
	default:
		ref := d.liveRefs[d.liveActive]
		if ref.pane != d.left {
			return
		}
		d.focusLeft = true
		d.left.Cursor.Line, d.left.Cursor.Col = d.left.SearchMatches[ref.paneIdx].Line, col
		d.left.scrollViewport()
	}
}
