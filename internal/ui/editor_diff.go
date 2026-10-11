package ui

import (
	"strconv"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

// DiffOverlay decorates an editor buffer with a diff. Kinds is indexed by
// buffer line. Deleted holds removed old lines and Fillers blank alignment
// rows, both keyed by the buffer line they precede (len(Kinds) for end of
// file); a block's deleted rows come before its fillers.
type DiffOverlay struct {
	Kinds   []diff.LineKind
	Deleted map[int][]diff.SideLine
	Fillers map[int]int

	// Nums numbers buffer lines from the diff's source files instead of by
	// buffer position; 0 leaves a line unnumbered.
	Nums []int

	// Gaps maps a collapsed gap line to its gap index and Labels holds the
	// text drawn in its place.
	Gaps   map[int]int
	Labels map[int]string
}

// DiffSide is one pane of a split diff: its buffer lines and their overlay.
type DiffSide struct {
	Lines   []string
	Overlay *DiffOverlay
}

// NewDiffOverlay projects full-file diff lines onto the new file. A change
// block shows all of its removals before its first addition, like a unified
// diff.
func NewDiffOverlay(lines []diff.DiffLine) *DiffOverlay {
	o := &DiffOverlay{Deleted: make(map[int][]diff.SideLine)}
	for i := 0; i < len(lines); {
		dl := lines[i]
		if dl.Left.Kind != diff.Deleted && dl.Right.Kind != diff.Added {
			if dl.Right.Kind != diff.Blank {
				o.Kinds = append(o.Kinds, diff.Context)
			}
			i++
			continue
		}
		anchor := len(o.Kinds)
		end := i
		for end < len(lines) && (lines[end].Left.Kind == diff.Deleted || lines[end].Right.Kind == diff.Added) {
			if lines[end].Left.Kind == diff.Deleted {
				o.Deleted[anchor] = append(o.Deleted[anchor], lines[end].Left)
			}
			end++
		}
		for ; i < end; i++ {
			if lines[i].Right.Kind == diff.Added {
				o.Kinds = append(o.Kinds, diff.Added)
			}
		}
	}
	return o
}

// NewSplitDiffOverlays builds both panes of a split diff. Each diff row yields
// exactly one row per side, a filler where that side has no line, so the
// panes stay in step without word wrap. Collapsed rows become gap lines on
// both sides.
func NewSplitDiffOverlays(lines []diff.DiffLine) (left, right DiffSide) {
	left = newDiffSide(len(lines))
	right = newDiffSide(len(lines))
	gap := 0
	for _, dl := range lines {
		left.add(dl.Left, gap)
		right.add(dl.Right, gap)
		if dl.Left.Kind == diff.Collapsed {
			gap++
		}
	}
	return left, right
}

func newDiffSide(n int) DiffSide {
	return DiffSide{
		Lines: make([]string, 0, n),
		Overlay: &DiffOverlay{
			Kinds:   make([]diff.LineKind, 0, n),
			Nums:    make([]int, 0, n),
			Fillers: make(map[int]int),
			Gaps:    make(map[int]int),
			Labels:  make(map[int]string),
		},
	}
}

func (s *DiffSide) add(side diff.SideLine, gap int) {
	o := s.Overlay
	line := len(s.Lines)
	switch side.Kind {
	case diff.Blank:
		o.Fillers[line]++
		return
	case diff.Collapsed:
		o.Gaps[line] = gap
		o.Labels[line] = side.Text
	}
	s.Lines = append(s.Lines, side.Text)
	o.Kinds = append(o.Kinds, side.Kind)
	o.Nums = append(o.Nums, side.Num)
}

func (o *DiffOverlay) kind(line int) diff.LineKind {
	if o == nil || line < 0 || line >= len(o.Kinds) {
		return diff.Context
	}
	return o.Kinds[line]
}

func (o *DiffOverlay) num(line int) (int, bool) {
	if o == nil || o.Nums == nil {
		return 0, false
	}
	if line < 0 || line >= len(o.Nums) {
		return 0, true
	}
	return o.Nums[line], true
}

func (o *DiffOverlay) label(line int) (string, bool) {
	if o == nil {
		return "", false
	}
	s, ok := o.Labels[line]
	return s, ok
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

// maxNum is the widest line number the overlay draws, so the gutter can fit
// numbers from a source file longer than the buffer.
func (o *DiffOverlay) maxNum() int {
	if o == nil {
		return 0
	}
	n := 0
	for _, v := range o.Nums {
		n = max(n, v)
	}
	for _, block := range o.Deleted {
		for _, l := range block {
			n = max(n, l.Num)
		}
	}
	return n
}

func (e *EditorPaneWidget) SetDiffOverlay(o *DiffOverlay) {
	e.DiffOverlay = o
	e.diffNumDigits = diffGutterDigits(o)
	if o == nil {
		e.setPhantoms(nil)
		return
	}
	phantoms := make(map[int]int, len(o.Deleted)+len(o.Fillers))
	for anchor, block := range o.Deleted {
		phantoms[anchor] += len(block)
	}
	for anchor, n := range o.Fillers {
		phantoms[anchor] += n
	}
	for anchor, n := range phantoms {
		if n == 0 {
			delete(phantoms, anchor)
		}
	}
	if len(phantoms) == 0 {
		phantoms = nil
	}
	e.setPhantoms(phantoms)
}

func (e *EditorPaneWidget) diffLineBg(line int) term.Style {
	switch e.DiffOverlay.kind(line) {
	case diff.Added:
		return term.StyleDiffAdded
	case diff.Deleted:
		return term.StyleDiffDeleted
	}
	return 0
}

func (e *EditorPaneWidget) renderGapRow(surface Surface, y, gutterW, editorW int, label string) {
	for x := 0; x < gutterW; x++ {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: term.StyleLineNumber})
	}
	x := 0
	for _, ch := range label {
		w := textwidth.Rune(ch)
		if x+w > editorW {
			break
		}
		surface.SetCell(gutterW+x, y, term.Cell{Ch: ch, Style: term.StyleMuted})
		x += w
	}
	for ; x < editorW; x++ {
		surface.SetCell(gutterW+x, y, term.Cell{Ch: ' ', Style: term.StyleMuted})
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
		num := 0
		if e.LineNumbers && row.startCol == 0 {
			num = old.Num
		}
		e.drawGutterNumber(surface, y, gutterW, num, term.Cell{Style: term.StyleGutterDeleted, BgStyle: term.StyleDiffDeleted})
	}
	var spans []highlight.Span
	if e.Highlighter != nil && old.Text != "" {
		spans = e.Highlighter.HighlightLine(old.Text)
	}
	tabW := e.resolveTabSize()
	leftCol := e.Viewport.LeftCol
	if e.WordWrap {
		leftCol = bufColToVisualCol(old.Text, row.startCol, tabW)
	}
	cells := e.renderLineToScreen([]rune(old.Text), spans, false, nil, tabW, leftCol, editorW)
	for x := 0; x < editorW; x++ {
		surface.SetCell(gutterW+x, y, term.Cell{Ch: cells[x].ch, Style: cells[x].style, BgStyle: term.StyleDiffDeleted})
	}
}

func diffGutterDigits(o *DiffOverlay) int {
	if n := o.maxNum(); n > 0 {
		return len(strconv.Itoa(n))
	}
	return 0
}
