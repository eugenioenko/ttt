package ui

import (
	"github.com/eugenioenko/ttt/internal/icons"
	"sort"
	"strconv"
	"unsafe"

	"github.com/eugenioenko/ttt/internal/core/buffer"
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

	// Nums switches the gutter to the diff layout (line number plus change
	// mark), numbering buffer lines from the diff's source files instead of
	// by buffer position; 0 leaves a line unnumbered.
	Nums []int

	// Gaps maps a collapsed gap line to its gap index. Labels draws a gap
	// label in place of a buffer line's text.
	Gaps   map[int]int
	Labels map[int]string

	HoveredGap      int
	focusedGap      *int
	HighContrast    bool
	EmphasizeGaps   bool
	ScrollMarks     bool
	Signs           bool
	SignsColor      bool
	GutterHighlight bool
	ColorNumbers    bool
	GapChevron      rune
	// MinGutter widens the diff gutter so several stacked diffs line up.
	MinGutter int

	// DeletedMatches holds find matches on deleted rows, keyed like Deleted;
	// a match's Line is the row's index in its block. DeletedActive is the
	// anchor and index into DeletedMatches of the active match, or -1s.
	DeletedMatches map[int][]FindMatch
	DeletedActive  [2]int

	// Hidden holds buffer lines folded away behind a gap line, so a real
	// buffer can show changes only without touching the user's folds.
	Hidden  []diffHiddenRange
	visible []int
	visN    int

	// Syntax highlights lines taken from the diff's source files within those
	// whole files, so a line inside a multi-line comment or string keeps its
	// state. NewSide marks a Nums line as taken from the new file; deleted
	// rows always come from the old one.
	Syntax  *diffSyntax
	NewSide []bool

	// buf and ver are the buffer and version the overlay's line numbers refer
	// to. The pane replays later edits of that buffer onto the overlay before
	// using it, so it moves with the text until its owner rebuilds it.
	buf   *buffer.Buffer
	ver   uint64
	stale bool

	maxNum, numsN int

	// LineBg and Spans decorate an annotation overlay (see
	// NewAnnotationOverlay): LineBg tints buffer lines and Spans highlight
	// parts of lines on top of it. NoteTexts are notes drawn under the line
	// they are keyed by; they are wrapped into Notes, rows keyed like Deleted,
	// at the pane's width. parts are the overlays merged into this one; they
	// move with its edits so they can be merged again later.
	LineBg     map[int]term.Style
	Spans      map[int][]ColSpan
	NoteTexts  map[int][]NoteText
	Notes      map[int][]NoteRow
	noteWidth  int
	notesMoved bool
	parts      []*DiffOverlay
	annotate   bool
}

// NoteText is a note the pane wraps to its own width. Blank lines separate
// paragraphs. Indent is in terminal columns and is dropped when it would
// leave the note less than half the width.
type NoteText struct {
	Text   string
	Indent int
	Style  term.Style
	// MaxWidth caps the text's width in columns; 0 uses the pane's width.
	MaxWidth int
}

// NoteRow is one row of text drawn between buffer lines. Indent is in
// terminal columns.
type NoteRow struct {
	Text   string
	Indent int
	Style  term.Style
}

// ColSpan highlights the runes [Start, End) of one buffer line.
type ColSpan struct {
	Start, End int
	Style      term.Style
}

// NewAnnotationOverlay returns an overlay that only decorates lines. Unlike a
// diff overlay it keeps the editor's own gutter, git change bars and
// scrollbar marks.
func NewAnnotationOverlay() *DiffOverlay {
	return &DiffOverlay{LineBg: map[int]term.Style{}, Spans: map[int][]ColSpan{}, NoteTexts: map[int][]NoteText{}, annotate: true}
}

// spanStyle returns the style of the last span covering col, or 0.
func spanStyle(spans []ColSpan, col int) term.Style {
	for i := len(spans) - 1; i >= 0; i-- {
		if sp := spans[i]; col >= sp.Start && col < sp.End {
			return sp.Style
		}
	}
	return 0
}

// isDiff reports whether o shows a diff, as opposed to annotations only.
func (o *DiffOverlay) isDiff() bool { return o != nil && !o.annotate }

type diffHiddenRange struct{ start, end int }

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

// diffSyntax holds the whole-file highlighters of a diff's two sides.
type diffSyntax struct {
	old, new diffSideSyntax
}

// diffSideSyntax creates its highlighter on first use; path is empty when
// syntax highlighting is off.
type diffSideSyntax struct {
	path  string
	hl    *highlight.Highlighter
	lines []string
	peer  *diffSideSyntax
	// anchor is the editable side's own highlighter, whose single-line cache
	// the HEAD side shares while both draw the same unchanged lines.
	anchor *highlight.Highlighter
}

func (s *diffSideSyntax) reset(path string) {
	s.hl.Close()
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
		var peer *highlight.Highlighter
		if s.peer != nil {
			peer = s.peer.hl
		}
		s.hl.ShareTokens(peer)
		s.hl.SetSyncLines(0)
		if peer == nil {
			peer = s.anchor
		}
		s.hl.ShareSingleLines(peer)
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

func (o *DiffOverlay) kind(line int) diff.LineKind {
	if o == nil || line < 0 || line >= len(o.Kinds) {
		return diff.Context
	}
	return o.Kinds[line]
}

func (o *DiffOverlay) diffGutter() bool { return o != nil && o.Nums != nil }

func (o *DiffOverlay) num(line int) int {
	if o == nil || line < 0 || line >= len(o.Nums) {
		return 0
	}
	return o.Nums[line]
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

func (o *DiffOverlay) gapHovered(line int) bool {
	if o == nil {
		return false
	}
	gap, ok := o.Gaps[line]
	return ok && gap == o.HoveredGap
}

func (o *DiffOverlay) gapFocused(line int) bool {
	if o == nil || o.focusedGap == nil {
		return false
	}
	gap, ok := o.Gaps[line]
	return ok && gap == *o.focusedGap
}

func (o *DiffOverlay) gapStyle(line int) term.Style {
	if o.gapFocused(line) {
		return term.StyleSidebarSelected
	}
	return collapsedDiffRowStyle(diff.Collapsed, o.EmphasizeGaps, o.gapHovered(line))
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

// gutterWidth is the diff layout's width: the widest source line number, a
// space and the change mark, with the editor gutter's leading space.
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

// maxDeletedNum is the widest old line number on a deleted row, so an editor
// layout gutter fits numbers from a source file longer than the buffer.
func (o *DiffOverlay) maxDeletedNum() int {
	if o == nil {
		return 0
	}
	n := 0
	for _, block := range o.Deleted {
		for _, l := range block {
			n = max(n, l.Num)
		}
	}
	return n
}

// diffLineFg overrides syntax colors: collapsed separators are muted labels
// and high contrast paints changed text with its semantic color. full reports
// that the style also owns the row background.
func (e *EditorPaneWidget) diffLineFg(line int) (style term.Style, override, full bool) {
	o := e.DiffOverlay
	kind := o.kind(line)
	if kind == diff.Collapsed {
		row := o.gapStyle(line)
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
	collapsed := collapsedDiffGutterStyle(side.Kind, o.EmphasizeGaps, o.gapHovered(line))
	if side.Kind == diff.Collapsed && o.gapFocused(line) {
		collapsed = term.StyleSidebarSelected
	}
	look := o.gutterLook(side.Kind, term.StyleLineNumber, collapsed)
	renderDiffGutterWithSigns(surface, 0, y, gutterW, side.Num, look, e.diffMarkCol(gutterW, side.Kind) >= 0)
}

// diffGutterLook is how a diff row's gutter is drawn. Every diff gutter, the
// compact diff layout and the editor layout alike, takes it from gutterLook
// so the two cannot drift apart.
type diffGutterLook struct {
	num, bg, sign term.Style
	mark          rune
}

// gutterLook styles a row of kind; base is the gutter's style for an
// unchanged row and collapsed the gap row style, if any.
func (o *DiffOverlay) gutterLook(kind diff.LineKind, base, collapsed term.Style) diffGutterLook {
	look := diffGutterLook{num: base, sign: base}
	switch kind {
	case diff.Added:
		look.mark, look.num, look.bg, look.sign = '+', term.StyleGutterAdded, term.StyleDiffAdded, term.StyleGutterAdded
	case diff.Deleted:
		look.mark, look.num, look.bg, look.sign = '−', term.StyleGutterDeleted, term.StyleDiffDeleted, term.StyleGutterDeleted
	case diff.Collapsed:
		look.mark = o.gapChevron()
		if collapsed != term.StyleDefault {
			look.num, look.sign = collapsed, collapsed
		}
		return look
	default:
		return look
	}
	if o != nil && !o.SignsColor {
		look.sign = term.StyleLineNumber
	}
	if o != nil && !o.ColorNumbers {
		look.num = base
	}
	if o != nil && !o.GutterHighlight {
		look.bg = 0
	}
	return look
}

func (o *DiffOverlay) gapChevron() rune {
	if o == nil || o.GapChevron == 0 {
		return icons.Rune(icons.ChevronRight)
	}
	return o.GapChevron
}

// diffMarkCol is where a gutter draws a row's mark, or -1 when it has none:
// + and − follow the signs setting, while a gap row's chevron always shows
// because it is what expands the gap. The minimal gutter has no room for
// either.
func (e *EditorPaneWidget) diffMarkCol(gutterW int, kind diff.LineKind) int {
	o := e.DiffOverlay
	if o == nil || (!o.Signs && kind != diff.Collapsed) || e.GutterStyle == "minimal" || gutterW < 3 {
		return -1
	}
	if e.GutterStyle == "extended" {
		return gutterW - 2
	}
	return gutterW - 1
}

// renderEditorDiffGutter draws an editor-layout gutter row of kind: the
// number, the gutter tint and the mark, all from gutterLook.
func (e *EditorPaneWidget) renderEditorDiffGutter(surface Surface, y, gutterW, num int, kind diff.LineKind, base term.Style) diffGutterLook {
	look := e.DiffOverlay.gutterLook(kind, base, term.StyleDefault)
	e.drawGutterNumber(surface, y, gutterW, num, term.Cell{Style: look.num, BgStyle: look.bg})
	if col := e.diffMarkCol(gutterW, kind); col >= 0 && look.mark != 0 {
		surface.SetCell(col, y, term.Cell{Ch: look.mark, Style: look.sign, BgStyle: look.bg})
	}
	return look
}

func (e *EditorPaneWidget) SetDiffOverlay(o *DiffOverlay) {
	if o == nil && e.DiffOverlay == nil {
		return
	}
	if o != nil && o == e.DiffOverlay {
		e.syncDiffOverlay()
	}
	e.DiffOverlay = o
	e.diffNumDigits = diffGutterDigits(o)
	if o == nil {
		e.setPhantoms(nil)
		return
	}
	if o.buf == nil {
		o.buf, o.ver = e.Buf, e.Buf.Version()
	}
	e.syncDiffOverlay()
	e.setPhantoms(o.phantoms())
}

// phantoms counts the rows o adds before each buffer line.
func (o *DiffOverlay) phantoms() map[int]int {
	phantoms := make(map[int]int, len(o.Deleted)+len(o.Fillers)+len(o.Notes))
	for anchor, block := range o.Deleted {
		phantoms[anchor] += len(block)
	}
	for anchor, n := range o.Fillers {
		phantoms[anchor] += n
	}
	for anchor, notes := range o.Notes {
		phantoms[anchor] += len(notes)
	}
	for anchor, n := range phantoms {
		if n == 0 {
			delete(phantoms, anchor)
		}
	}
	if len(phantoms) == 0 {
		return nil
	}
	return phantoms
}

// noteLine returns the note row shown at row, using l's placement of notes
// around folded lines.
func (o *DiffOverlay) noteLine(row editorRow, l *rowLayout) (NoteRow, bool) {
	if o == nil || len(o.Notes) == 0 {
		return NoteRow{}, false
	}
	notes := o.Notes
	if l != nil && l.notes != nil {
		notes = l.notes
	}
	k := row.phantom - len(o.Deleted[row.bufLine]) - o.Fillers[row.bufLine]
	rows := notes[row.bufLine]
	if k < 0 || k >= len(rows) {
		return NoteRow{}, false
	}
	return rows[k], true
}

func (e *EditorPaneWidget) diffLineBg(line int) term.Style {
	switch e.DiffOverlay.kind(line) {
	case diff.Added:
		return term.StyleDiffAdded
	case diff.Deleted:
		return term.StyleDiffDeleted
	case diff.Collapsed:
		return e.DiffOverlay.gapStyle(line)
	}
	if e.DiffOverlay != nil {
		return e.DiffOverlay.LineBg[line]
	}
	return 0
}

func (e *EditorPaneWidget) renderGapRow(surface Surface, y, gutterW, editorW, line int, label string) {
	o := e.DiffOverlay
	hovered := o.gapHovered(line)
	rowStyle := o.gapStyle(line)
	if gutterW > 0 {
		gutterStyle := collapsedDiffGutterStyle(diff.Collapsed, o.EmphasizeGaps, hovered)
		if o.gapFocused(line) {
			gutterStyle = rowStyle
		}
		cell := term.Cell{Ch: ' ', Style: term.StyleLineNumber}
		if gutterStyle != term.StyleDefault {
			cell.Style = gutterStyle
		}
		for x := 0; x < gutterW; x++ {
			surface.SetCell(x, y, cell)
		}
		if col := e.diffMarkCol(gutterW, diff.Collapsed); col >= 0 {
			surface.SetCell(col, y, term.Cell{Ch: o.gapChevron(), Style: cell.Style})
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

func (e *EditorPaneWidget) renderPhantomRow(surface Surface, y, gutterW, editorW int, row editorRow, l *rowLayout) {
	if note, ok := e.DiffOverlay.noteLine(row, l); ok {
		e.renderNoteRow(surface, y, gutterW, editorW, note)
		return
	}
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
		if row.startCol == 0 {
			e.renderEditorDiffGutter(surface, y, gutterW, num, diff.Deleted, term.StyleLineNumber)
		} else {
			look := e.DiffOverlay.gutterLook(diff.Deleted, term.StyleLineNumber, term.StyleDefault)
			e.drawGutterNumber(surface, y, gutterW, 0, term.Cell{Style: look.num, BgStyle: look.bg})
		}
	}
	var spans []highlight.Span
	if whole, ok := e.DiffOverlay.deletedSyntax(old); ok {
		spans = whole
	} else if e.Highlighter != nil && old.Text != "" {
		spans = e.Highlighter.HighlightLine(old.Text)
	}
	tabW := e.resolveTabSize()
	startCol, leftCol := 0, e.Viewport.LeftCol
	if e.WordWrap {
		startCol, leftCol = row.startCol, 0
	}
	cells := e.renderLineSegmentToScreen([]rune(old.Text), spans, false, nil, startCol, tabW, leftCol, editorW)
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

// renderDiffGutterWithSigns draws the compact diff gutter: the number
// right-aligned, a space, then the mark.
func renderDiffGutterWithSigns(surface Surface, x, y, width, num int, look diffGutterLook, signs bool) {
	var buf [20]byte
	number := buf[:0]
	if num > 0 {
		number = strconv.AppendInt(number, int64(num), 10)
	}
	marker, markerStyle := look.mark, look.sign
	if !signs || marker == 0 {
		marker, markerStyle = ' ', look.num
	}
	column := 0
	style := look.num
	put := func(ch rune) {
		if column < width {
			surface.SetCell(x+column, y, term.Cell{Ch: ch, Style: style, BgStyle: look.bg})
		}
		column++
	}
	for range max(width-2-len(number), 0) {
		put(' ')
	}
	for _, d := range number {
		put(rune(d))
	}
	put(' ')
	style = markerStyle
	put(marker)
}

func diffGutterDigits(o *DiffOverlay) int {
	if n := o.maxDeletedNum(); n > 0 {
		return len(strconv.Itoa(n))
	}
	return 0
}

func (e *EditorPaneWidget) renderNoteRow(surface Surface, y, gutterW, editorW int, note NoteRow) {
	for x := 0; x < gutterW; x++ {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: term.StyleLineNumber})
	}
	indent := min(note.Indent, editorW)
	for x := range indent {
		surface.SetCell(gutterW+x, y, term.Cell{Ch: ' '})
	}
	x := surface.DrawText(gutterW+indent, y, note.Text, gutterW+editorW, note.Style)
	for ; x < gutterW+editorW; x++ {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: note.Style})
	}
}
