package ui

import (
	"github.com/eugenioenko/ttt/internal/core/diff"
)

// setSource replaces what a host-driven diff shows. The host owns fetching
// and gap state; the widget only projects and draws it.
func (d *DiffEditorWidget) setSource(fd diff.FileDiff, oldLines, newLines []string, loaded, full bool, expanded map[int]bool) {
	d.fileDiff = fd
	d.oldLines, d.newLines = oldLines, newLines
	d.contextLoaded = loaded
	d.extended = full && loaded
	d.contextMode = DiffContextChangesOnly
	if d.extended {
		d.contextMode = DiffContextFullFile
	}
	if expanded == nil {
		expanded = make(map[int]bool)
	}
	d.expandedGaps = expanded
	d.rebuild()
}

func (d *DiffEditorWidget) setEmbedded() {
	for _, p := range d.panes() {
		p.Embedded = true
	}
	d.deferPanes = true
}

// embedRows lays the diff out at width w for a host that stacks several
// diffs in one scroll area, and returns how many screen rows it takes.
func (d *DiffEditorWidget) embedRows(w int) int {
	wrap := d.IsWrapped()
	if !wrap {
		if d.IsUnified() {
			return max(len(d.unifiedRows), 1)
		}
		return max(len(d.pairs), 1)
	}
	d.ensurePanes()
	for _, p := range d.panes() {
		p.WordWrap = wrap
	}
	if d.IsUnified() {
		d.unified.Viewport.Width = d.unified.embeddedTextWidth(w)
		return d.unified.layout().total()
	}
	divider := (w - 1) / 2
	if divider < 1 {
		d.right.Viewport.Width = d.right.embeddedTextWidth(w)
		return d.right.layout().total()
	}
	d.left.wrapCols, d.right.wrapCols = 0, 0
	lw, rw := d.left.embeddedTextWidth(divider), d.right.embeddedTextWidth(w-divider-1)
	if wrap {
		c := min(lw, rw)
		lw, rw = c, c
		d.left.wrapCols, d.right.wrapCols = c, c
	}
	d.left.Viewport.Width, d.right.Viewport.Width = lw, rw
	d.alignSplit()
	return max(d.left.layout().total(), d.right.layout().total())
}

// renderEmbedded draws rows [top, top+rect.H) of the diff laid out by
// embedRows into surface, which covers rect on screen.
func (d *DiffEditorWidget) renderEmbedded(surface Surface, rect Rect, top, leftCol int) {
	d.ensurePanes()
	d.SetRect(rect)
	for _, p := range d.panes() {
		p.hScrollPending = false
		p.Viewport.LeftCol = leftCol
	}
	lead := d.lead()
	if !d.IsUnified() && (rect.W-1)/2 < 1 {
		lead = d.right
	}
	lead.setTopRow(lead.layout(), top)
	d.Render(surface)
}

func (d *DiffEditorWidget) gapAtPoint(x, y int) (int, bool) {
	d.ensurePanes()
	p := d.paneAt(x, y)
	if p == nil {
		return 0, false
	}
	return d.gapAt(p, y)
}

func (d *DiffEditorWidget) setHoveredGap(gap int) bool {
	if d.hoveredGap == gap {
		return false
	}
	d.hoveredGap = gap
	d.applyOverlayOptions()
	return true
}

func (d *DiffEditorWidget) hasSelection() bool {
	for _, p := range d.panes() {
		if p.Selection.Active {
			return true
		}
	}
	return false
}

func (d *DiffEditorWidget) maxTextWidth() int {
	w := 0
	for _, dl := range d.Lines {
		w = max(w, diffLineVisualWidth(dl.Left.Text), diffLineVisualWidth(dl.Right.Text))
	}
	return w
}

func (d *DiffEditorWidget) maxLineNumber() int {
	n := 0
	for _, dl := range d.Lines {
		n = max(n, dl.Left.Num, dl.Right.Num)
	}
	return n
}

type diffSelectionMark struct {
	pane                 int
	anchorRow, anchorCol int
	anchorRight          bool
	curRow, curCol       int
	curRight             bool
}

func (d *DiffEditorWidget) paneIndex(p *EditorPaneWidget) int {
	switch p {
	case d.unified:
		return 0
	case d.left:
		return 1
	}
	return 2
}

func (d *DiffEditorWidget) paneByIndex(i int) *EditorPaneWidget {
	switch i {
	case 0:
		return d.unified
	case 1:
		return d.left
	}
	return d.right
}

// captureSelection records the active selection by diff row, so it survives
// a rebuild that renumbers the synthetic buffers.
func (d *DiffEditorWidget) captureSelection() (diffSelectionMark, bool) {
	for _, p := range d.panes() {
		if !p.Selection.Active {
			continue
		}
		m := diffSelectionMark{pane: d.paneIndex(p), anchorCol: p.Selection.Anchor.Col, curCol: p.Cursor.Col}
		m.anchorRow, m.anchorRight = d.diffRowForBufLine(p, p.Selection.Anchor.Line)
		m.curRow, m.curRight = d.diffRowForBufLine(p, p.Cursor.Line)
		return m, true
	}
	return diffSelectionMark{}, false
}

func (d *DiffEditorWidget) restoreSelection(m diffSelectionMark) bool {
	d.ensurePanes()
	p := d.paneByIndex(m.pane)
	line := func(row int, right bool) int {
		if p == d.unified {
			return d.unifiedIndex(row, right)
		}
		rows := d.paneRows(p)
		for i, r := range rows {
			if r == row {
				return i
			}
		}
		return -1
	}
	anchor, cur := line(m.anchorRow, m.anchorRight), line(m.curRow, m.curRight)
	if anchor < 0 || cur < 0 {
		return false
	}
	p.Selection.Start(anchor, m.anchorCol)
	p.Cursor.Line, p.Cursor.Col = cur, m.curCol
	if !d.IsUnified() {
		d.focusLeft = p == d.left
	}
	return true
}
