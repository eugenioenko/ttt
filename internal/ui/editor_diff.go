package ui

import (
	"sort"
	"strconv"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

// DiffOverlay decorates an editor buffer with a diff against an older
// version: Kinds is indexed by buffer line, Deleted holds removed old lines
// and Fillers blank alignment rows, both keyed by the buffer line they precede
// (len(Kinds) for end of file). A block's deleted rows come before its fillers.
type DiffOverlay struct {
	Kinds   []diff.LineKind
	Deleted map[int][]diff.SideLine
	Fillers map[int]int

	// Nums switches the gutter to the diff layout (line number plus change
	// marker); 0 leaves a row unnumbered. Read-only diffs number their
	// synthetic buffers from the source files this way.
	Nums          []int
	Gaps          map[int]int
	HoveredGap    int
	HighContrast  bool
	EmphasizeGaps bool

	// Labels draws a gap label in place of a buffer line's text; Hidden holds
	// the buffer lines folded away behind such a gap line, so a real buffer can
	// show changes only without touching the user's folds.
	Labels  map[int]string
	Hidden  []diffHiddenRange
	visible []int
	visN    int
}

type diffHiddenRange struct{ start, end int }

func (o *DiffOverlay) label(line int) (string, bool) {
	if o == nil {
		return "", false
	}
	s, ok := o.Labels[line]
	return s, ok
}

func (o *DiffOverlay) hasHidden() bool { return o != nil && len(o.Hidden) > 0 }

func (o *DiffOverlay) hidden(line int) bool {
	if o == nil {
		return false
	}
	i := sort.Search(len(o.Hidden), func(i int) bool { return o.Hidden[i].end >= line })
	return i < len(o.Hidden) && o.Hidden[i].start <= line
}

func (o *DiffOverlay) visibleLines(n int) []int {
	if o.visible != nil && o.visN == n {
		return o.visible
	}
	vis := make([]int, 0, n)
	for line := 0; line < n; line++ {
		if !o.hidden(line) {
			vis = append(vis, line)
		}
	}
	o.visible, o.visN = vis, n
	return vis
}

func (o *DiffOverlay) kind(line int) diff.LineKind {
	if o == nil || line < 0 || line >= len(o.Kinds) {
		return diff.Context
	}
	return o.Kinds[line]
}

func (o *DiffOverlay) deletedLine(row editorRow) (diff.SideLine, bool) {
	if o == nil {
		return diff.SideLine{}, false
	}
	block := o.Deleted[row.bufLine]
	if row.phantom < 0 || row.phantom >= len(block) {
		return diff.SideLine{}, false
	}
	return block[row.phantom], true
}

func (o *DiffOverlay) diffGutter() bool { return o != nil && o.Nums != nil }

func (o *DiffOverlay) num(line int) int {
	if o == nil || line < 0 || line >= len(o.Nums) {
		return 0
	}
	return o.Nums[line]
}

func (o *DiffOverlay) gapHovered(line int) bool {
	if o == nil {
		return false
	}
	gap, ok := o.Gaps[line]
	return ok && gap == o.HoveredGap
}

func (o *DiffOverlay) gutterWidth() int {
	maxNum := 0
	for _, n := range o.Nums {
		maxNum = max(maxNum, n)
	}
	return len(strconv.Itoa(maxNum)) + 3
}

// diffLineFg overrides syntax colors: collapsed separators are muted labels
// and high contrast paints changed text with its semantic color. full reports
// that the style also owns the row background.
func (e *EditorPaneWidget) diffLineFg(line int) (style term.Style, override, full bool) {
	o := e.DiffOverlay
	kind := o.kind(line)
	if kind == diff.Collapsed {
		row := collapsedDiffRowStyle(kind, o.EmphasizeGaps, o.gapHovered(line))
		if row != term.StyleDefault {
			return row, true, true
		}
		return term.StyleMuted, true, false
	}
	if o != nil && o.HighContrast && (kind == diff.Added || kind == diff.Deleted) {
		return diffKindForeground(kind, true), true, false
	}
	return 0, false, false
}

func (e *EditorPaneWidget) renderDiffGutterRow(surface Surface, y, gutterW, line int, continuation bool) {
	o := e.DiffOverlay
	side := diff.SideLine{}
	if !continuation && line < len(e.Buf.Lines) {
		side = diff.SideLine{Num: o.num(line), Kind: o.kind(line)}
	}
	renderDiffGutterWithCollapsedStyle(surface, 0, y, gutterW, side, collapsedDiffGutterStyle(side.Kind, o.EmphasizeGaps, o.gapHovered(line)))
}

func (e *EditorPaneWidget) SetDiffOverlay(o *DiffOverlay) {
	e.DiffOverlay = o
	e.phantoms = nil
	if o == nil {
		return
	}
	e.phantoms = make(map[int]int, len(o.Deleted)+len(o.Fillers))
	for anchor, block := range o.Deleted {
		e.phantoms[anchor] += len(block)
	}
	for anchor, n := range o.Fillers {
		e.phantoms[anchor] += n
	}
	for anchor, n := range e.phantoms {
		if n == 0 {
			delete(e.phantoms, anchor)
		}
	}
}

func (e *EditorPaneWidget) diffLineBg(line int) term.Style {
	switch e.DiffOverlay.kind(line) {
	case diff.Added:
		return term.StyleDiffAdded
	case diff.Deleted:
		return term.StyleDiffDeleted
	case diff.Collapsed:
		return collapsedDiffRowStyle(diff.Collapsed, e.DiffOverlay.EmphasizeGaps, e.DiffOverlay.gapHovered(line))
	}
	return 0
}

func (e *EditorPaneWidget) renderGapRow(surface Surface, y, gutterW, editorW, line int, label string) {
	o := e.DiffOverlay
	hovered := o.gapHovered(line)
	rowStyle := collapsedDiffRowStyle(diff.Collapsed, o.EmphasizeGaps, hovered)
	if gutterW > 0 {
		renderDiffGutterWithCollapsedStyle(surface, 0, y, gutterW, diff.SideLine{Kind: diff.Collapsed}, collapsedDiffGutterStyle(diff.Collapsed, o.EmphasizeGaps, hovered))
	}
	fg := rowStyle
	if fg == term.StyleDefault {
		fg = term.StyleMuted
	}
	x := 0
	for _, ch := range label {
		w := textwidth.Rune(ch)
		if x+w > editorW {
			break
		}
		surface.SetCell(gutterW+x, y, term.Cell{Ch: ch, Style: fg, BgStyle: rowStyle})
		x += w
	}
	for ; x < editorW; x++ {
		surface.SetCell(gutterW+x, y, term.Cell{Ch: ' ', Style: fg, BgStyle: rowStyle})
	}
}

func (e *EditorPaneWidget) renderPhantomRow(surface Surface, y, gutterW, editorW int, row editorRow) {
	old, ok := e.DiffOverlay.deletedLine(row)
	if !ok {
		for x := 0; x < gutterW+editorW; x++ {
			surface.SetCell(x, y, term.Cell{Ch: ' ', Style: term.StyleLineNumber})
		}
		return
	}
	if gutterW > 0 {
		padded := []rune(e.gutterNumber("", gutterW))
		if e.LineNumbers && old.Num > 0 {
			padded = []rune(e.gutterNumber(strconv.Itoa(old.Num), gutterW))
		}
		for i := 0; i < gutterW; i++ {
			ch := ' '
			if i < len(padded) {
				ch = padded[i]
			}
			surface.SetCell(i, y, term.Cell{Ch: ch, Style: term.StyleLineNumber, BgStyle: term.StyleDiffDeleted})
		}
		surface.SetCell(0, y, term.Cell{Ch: '−', Style: term.StyleGutterDeleted, BgStyle: term.StyleDiffDeleted})
	}
	var spans []highlight.Span
	if e.Highlighter != nil && old.Text != "" {
		spans = e.Highlighter.HighlightLine(old.Text)
	}
	leftCol := e.Viewport.LeftCol
	if e.WordWrap {
		leftCol = 0
	}
	cells := e.renderLineToScreen([]rune(old.Text), spans, false, nil, e.resolveTabSize(), leftCol, editorW)
	for x := 0; x < editorW; x++ {
		surface.SetCell(gutterW+x, y, term.Cell{Ch: cells[x].ch, Style: cells[x].style, BgStyle: term.StyleDiffDeleted})
	}
}
