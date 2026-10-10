package ui

import (
	"fmt"
	"sort"
	"strconv"
	"unsafe"

	"github.com/eugenioenko/ttt/internal/core/buffer"
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
	Signs         bool
	SignsColor    bool
	MinGutter     int

	// DeletedMatches holds find matches on deleted rows, keyed like Deleted;
	// a match's Line is the row's index in its block. DeletedActive is the
	// anchor and index into DeletedMatches of the active match, or -1s.
	DeletedMatches map[int][]FindMatch
	DeletedActive  [2]int

	// Labels draws a gap label in place of a buffer line's text; Hidden holds
	// the buffer lines folded away behind such a gap line, so a real buffer can
	// show changes only without touching the user's folds.
	Labels  map[int]string
	Hidden  []diffHiddenRange
	visible []int
	visN    int

	// Syntax highlights lines taken from the diff's source files within those
	// whole files, so a line inside a multi-line comment or string keeps its
	// state. NewSide marks a Nums line as taken from the new file; deleted
	// phantom rows always come from the old one.
	Syntax  *diffSyntax
	NewSide []bool

	// buf and ver are the buffer and version the overlay's line numbers refer
	// to. The pane replays later edits of that buffer onto the overlay before
	// using it, so it moves with the text until its owner rebuilds it.
	buf   *buffer.Buffer
	ver   uint64
	stale bool

	maxNum, numsN int
}

type diffHiddenRange struct{ start, end int }

type diffSyntax struct {
	old, new diffSideSyntax
}

// diffSideSyntax creates its highlighter on first use; path is empty when
// syntax highlighting is off.
type diffSideSyntax struct {
	path  string
	hl    *highlight.Highlighter
	lines []string
}

func (s *diffSideSyntax) reset(path string) {
	s.path, s.hl = path, nil
}

func (s *diffSideSyntax) set(lines []string) {
	if s.hl != nil && (len(s.lines) != len(lines) || unsafe.SliceData(s.lines) != unsafe.SliceData(lines)) {
		s.hl.ClearCache()
	}
	s.lines = lines
}

func (s *diffSideSyntax) spans(num int, text string) ([]highlight.Span, bool) {
	if s.path == "" || num < 1 || num > len(s.lines) || s.lines[num-1] != text {
		return nil, false
	}
	if s.hl == nil {
		if s.hl = highlight.New(s.path); s.hl == nil {
			s.path = ""
			return nil, false
		}
	}
	return s.hl.HighlightLineAt(s.lines, num-1), true
}

func (o *DiffOverlay) syntaxAt(line int, text string) ([]highlight.Span, bool) {
	if o == nil || o.Syntax == nil || line < 0 || line >= len(o.Nums) {
		return nil, false
	}
	if line < len(o.NewSide) && o.NewSide[line] {
		return o.Syntax.new.spans(o.Nums[line], text)
	}
	return o.Syntax.old.spans(o.Nums[line], text)
}

func (o *DiffOverlay) deletedSyntax(old diff.SideLine) ([]highlight.Span, bool) {
	if o == nil || o.Syntax == nil {
		return nil, false
	}
	return o.Syntax.old.spans(old.Num, old.Text)
}

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
	var vis []int
	next := 0
	for _, h := range o.Hidden {
		for line := next; line < min(h.start, n); line++ {
			vis = append(vis, line)
		}
		next = max(next, h.end+1)
	}
	for line := next; line < n; line++ {
		vis = append(vis, line)
	}
	if vis == nil {
		vis = []int{}
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
	if o.numsN != len(o.Nums) || o.numsN == 0 {
		o.maxNum = 0
		for _, n := range o.Nums {
			o.maxNum = max(o.maxNum, n)
		}
		o.numsN = len(o.Nums)
	}
	return max(len(strconv.Itoa(o.maxNum))+3, o.MinGutter)
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
	renderDiffGutterWithSigns(surface, 0, y, gutterW, side, collapsedDiffGutterStyle(side.Kind, o.EmphasizeGaps, o.gapHovered(line)), e.diffSignCol(gutterW) >= 0, o.SignsColor)
}

// diffSignCol is where an editor-layout gutter draws +, − and ▶, or -1 when
// signs are off; the minimal gutter has no room for them.
func (e *EditorPaneWidget) diffSignCol(gutterW int) int {
	o := e.DiffOverlay
	if o == nil || !o.Signs || e.GutterStyle == "minimal" || gutterW < 3 {
		return -1
	}
	if e.GutterStyle == "extended" {
		return gutterW - 2
	}
	return gutterW - 1
}

func (e *EditorPaneWidget) renderDiffSign(surface Surface, y, gutterW int, kind diff.LineKind, bg term.Style) {
	col := e.diffSignCol(gutterW)
	if col < 0 {
		return
	}
	added, deleted := term.StyleGutterAdded, term.StyleGutterDeleted
	if !e.DiffOverlay.SignsColor {
		added, deleted = term.StyleLineNumber, term.StyleLineNumber
	}
	switch kind {
	case diff.Added:
		surface.SetCell(col, y, term.Cell{Ch: '+', Style: added, BgStyle: bg})
	case diff.Deleted:
		surface.SetCell(col, y, term.Cell{Ch: '−', Style: deleted, BgStyle: bg})
	case diff.Collapsed:
		surface.SetCell(col, y, term.Cell{Ch: '▶', Style: term.StyleLineNumber, BgStyle: bg})
	}
}

func (e *EditorPaneWidget) SetDiffOverlay(o *DiffOverlay) {
	if o == nil && e.DiffOverlay == nil {
		return
	}
	if o != nil && o == e.DiffOverlay {
		e.syncDiffOverlay()
	}
	e.DiffOverlay = o
	e.overlayGen++
	e.phantoms = nil
	if o == nil {
		return
	}
	if o.buf == nil {
		o.buf, o.ver = e.Buf, e.Buf.Version()
	}
	e.syncDiffOverlay()
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
		gutterStyle := collapsedDiffGutterStyle(diff.Collapsed, o.EmphasizeGaps, hovered)
		cell := term.Cell{Ch: ' ', Style: term.StyleLineNumber}
		if gutterStyle != term.StyleDefault {
			cell.Style = gutterStyle
		}
		for x := 0; x < gutterW; x++ {
			surface.SetCell(x, y, cell)
		}
		if col := e.diffSignCol(gutterW); col >= 0 {
			surface.SetCell(col, y, term.Cell{Ch: '▶', Style: cell.Style})
		}
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
		if e.LineNumbers && old.Num > 0 && row.startCol == 0 {
			padded = []rune(e.gutterNumber(strconv.Itoa(old.Num), gutterW))
		}
		for i := 0; i < gutterW; i++ {
			ch := ' '
			if i < len(padded) {
				ch = padded[i]
			}
			surface.SetCell(i, y, term.Cell{Ch: ch, Style: term.StyleLineNumber, BgStyle: term.StyleDiffDeleted})
		}
		if row.startCol == 0 {
			e.renderDiffSign(surface, y, gutterW, diff.Deleted, term.StyleDiffDeleted)
		}
	}
	var spans []highlight.Span
	if whole, ok := e.DiffOverlay.deletedSyntax(old); ok {
		spans = whole
	} else if e.Highlighter != nil && old.Text != "" {
		spans = e.Highlighter.HighlightLine(old.Text)
	}
	leftCol := e.Viewport.LeftCol
	if e.WordWrap {
		leftCol = bufColToVisualCol(old.Text, row.startCol, e.resolveTabSize())
	}
	cells := e.renderLineToScreen([]rune(old.Text), spans, false, nil, e.resolveTabSize(), leftCol, editorW)
	matches := e.DiffOverlay.DeletedMatches[row.bufLine]
	for x := 0; x < editorW; x++ {
		cell := term.Cell{Ch: cells[x].ch, Style: cells[x].style, BgStyle: term.StyleDiffDeleted}
		for i, m := range matches {
			if m.Line != row.phantom || cells[x].bufCol < m.Col || cells[x].bufCol >= m.Col+m.Len {
				continue
			}
			cell.Style, cell.BgStyle = term.StyleSearchMatch, 0
			if e.DiffOverlay.DeletedActive == [2]int{row.bufLine, i} {
				cell.Style = term.StyleSearchActive
			}
			break
		}
		surface.SetCell(gutterW+x, y, cell)
	}
}

func renderDiffGutterWithSigns(surface Surface, x, y, width int, line diff.SideLine, collapsedStyle term.Style, signs, colored bool) {
	number := ""
	if line.Num > 0 {
		number = fmt.Sprintf("%d", line.Num)
	}
	marker := ' '
	style := term.StyleLineNumber
	bgStyle := term.StyleDefault
	switch line.Kind {
	case diff.Added:
		marker = '+'
		style = term.StyleGutterAdded
		bgStyle = term.StyleDiffAdded
	case diff.Deleted:
		marker = '−'
		style = term.StyleGutterDeleted
		bgStyle = term.StyleDiffDeleted
	case diff.Collapsed:
		marker = '▶'
		if collapsedStyle != term.StyleDefault {
			style = collapsedStyle
		}
	}
	if !signs {
		marker = ' '
	}
	if !colored && (line.Kind == diff.Added || line.Kind == diff.Deleted) {
		style = term.StyleLineNumber
	}
	text := fmt.Sprintf("%*s %c", width-2, number, marker)
	for column, ch := range []rune(text) {
		if column >= width {
			break
		}
		surface.SetCell(x+column, y, term.Cell{Ch: ch, Style: style, BgStyle: bgStyle})
	}
}
