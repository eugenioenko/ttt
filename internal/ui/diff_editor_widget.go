package ui

import (
	"sort"
	"unsafe"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"

	"github.com/gdamore/tcell/v3"
)

// DiffEditorWidget shows a diff with editor panes. A read-only diff uses
// synthetic buffers: unified mode holds old and new lines as real buffer
// lines, split mode holds each side in its own pane, aligned with filler rows.
// An editable diff shows the file's own buffer, with removed lines as phantom
// rows in unified mode and a read-only base pane beside it in split mode.
type DiffEditorWidget struct {
	BaseWidget
	FilePath string
	Lines    []diff.DiffLine

	fileDiff  diff.FileDiff
	oldLines  []string
	newLines  []string
	gapByLine map[int]int

	extended        bool
	contextMode     DiffContextMode
	contextExplicit bool
	contextLoaded   bool
	expandedGaps    map[int]bool
	pendingGap      int
	mode            DiffMode
	modeExplicit    bool
	wrapMode        DiffWrapMode
	wrapExplicit    bool
	highContrast    bool
	signs           bool
	signsColor      bool
	emphasizeGaps   bool
	hoveredGap      int
	minGutter       int

	OnFetchExtended func(dv *DiffEditorWidget)
	// OnRecompute runs after an editable diff takes a recomputed diff, which
	// clears its search; an open find bar re-runs its query from it.
	OnRecompute      func()
	Loading          bool
	extendedFetching bool
	loadingAnchor    diffEditorAnchor

	syntax      bool
	focused     bool
	unified     *EditorPaneWidget
	left        *EditorPaneWidget
	right       *EditorPaneWidget
	unifiedRows []diffUnifiedLine
	leftRows    []int
	rightRows   []int
	leftBase    *DiffOverlay
	rightBase   *DiffOverlay
	focusLeft   bool
	captured    *EditorPaneWidget
	pressed     bool

	SearchMatchesLeft  []FindMatch
	SearchMatchesRight []FindMatch
	searchRefs         []diffSearchRef
	searchActiveRight  bool

	pairs         []splitPair
	alignKey      splitAlignKey
	alignContribs []alignContrib
	alignVer      uint64
	wrapKey       [4]int

	docSyntax diffSyntax
	redraw    func()

	deferPanes bool
	panesGen   int
	unifiedGen int
	splitGen   int

	editable    bool
	live        *EditorPaneWidget
	full        []diff.DiffLine
	revealed    [][2]int
	liveRows    []int
	liveGapLen  map[int]int
	liveSpans   [][2]int
	liveN       int
	liveCursor  int
	liveUnified *DiffOverlay
	liveHits    []liveSearchHit
	liveRefs    []liveSearchRef
	liveActive  int
	liveTouched [][2]int

	// liveBuf is the file's buffer and liveVer the version the live rows and
	// overlays refer to. liveSnap holds that version's text, for edits that
	// cannot be replayed from the buffer's change log. fullChanges are the
	// line-count changes made since full was computed.
	liveBuf     *buffer.Buffer
	liveVer     uint64
	liveSnap    []string
	fullChanges []buffer.Change
	liveExpand  []int
}

type diffSearchRef struct {
	right   bool
	pane    *EditorPaneWidget
	paneIdx int
}

func NewDiffEditorWidget(filePath string, fd diff.FileDiff, oldLines, newLines []string, extended bool) *DiffEditorWidget {
	d := &DiffEditorWidget{
		FilePath:      filePath,
		fileDiff:      fd,
		oldLines:      oldLines,
		newLines:      newLines,
		extended:      extended,
		contextMode:   DiffContextChangesOnly,
		contextLoaded: oldLines != nil || newLines != nil,
		expandedGaps:  make(map[int]bool),
		pendingGap:    -1,
		hoveredGap:    -1,
		syntax:        true,
		signs:         true,
		signsColor:    true,
		unified:       newDiffPane(),
		left:          newDiffPane(),
		right:         newDiffPane(),
		deferPanes:    true,
	}
	if extended {
		d.contextMode = DiffContextFullFile
		d.contextExplicit = true
	}
	d.attachHighlighters()
	d.rebuild()
	return d
}

func newDiffPane() *EditorPaneWidget {
	p := NewEditorPaneWidget(&buffer.Buffer{Lines: []string{""}}, &cursor.Cursor{}, &view.Viewport{})
	p.Undo = &undo.UndoStack{}
	p.Selection = &selection.Selection{}
	p.ReadOnly = true
	p.NoWrapMargin = true
	p.LineNumbers = true
	p.TabSize = diffTabWidth
	return p
}

// panes are the panes the widget owns; an editable diff borrows its live
// pane from the editor group and must leave that pane's state alone.
func (d *DiffEditorWidget) panes() []*EditorPaneWidget {
	if d.editable {
		return []*EditorPaneWidget{d.left}
	}
	return []*EditorPaneWidget{d.unified, d.left, d.right}
}

func (d *DiffEditorWidget) viewPanes() []*EditorPaneWidget {
	if d.editable {
		if d.live == nil {
			return []*EditorPaneWidget{d.left}
		}
		return []*EditorPaneWidget{d.left, d.live}
	}
	return d.panes()
}

// attachHighlighters drops the panes' highlighters; Render creates them for
// the panes it draws, since loading a grammar is costly and a commit view
// holds a diff widget per file.
func (d *DiffEditorWidget) attachHighlighters() {
	for _, p := range d.panes() {
		p.Highlighter = nil
	}
	path := ""
	if d.syntax {
		path = d.FilePath
	}
	d.docSyntax.old.reset(path)
	d.docSyntax.new.reset(path)
	d.docSyntax.old.peer, d.docSyntax.new.peer = &d.docSyntax.new, &d.docSyntax.old
	d.syncDocSyntax()
}

func (d *DiffEditorWidget) ensureHighlighters(panes ...*EditorPaneWidget) {
	if !d.syntax {
		return
	}
	var peer *highlight.Highlighter
	for _, p := range d.panes() {
		if p != nil && p != d.live && p.Highlighter != nil {
			peer = p.Highlighter
			break
		}
	}
	for _, p := range panes {
		if p != nil && p != d.live && p.Highlighter == nil {
			p.Highlighter = highlight.New(d.FilePath)
			p.Highlighter.ShareTokens(peer)
			p.Highlighter.SetProgressive(d.redraw)
			p.Highlighter.ShareSingleLines(peer)
			if peer == nil {
				peer = p.Highlighter
			}
		}
	}
}

// syncDocSyntax points the whole-file highlighters at the current sides; a
// side whose content is not loaded falls back to the pane's own highlighting.
func (d *DiffEditorWidget) syncDocSyntax() {
	oldLines, newLines := d.oldLines, d.newLines
	if !d.editable && !d.contextLoaded {
		oldLines, newLines = nil, nil
	}
	d.docSyntax.old.set(oldLines)
	d.docSyntax.new.set(newLines)
}

func (d *DiffEditorWidget) SetSyntaxHighlight(enabled bool) {
	if d.syntax == enabled {
		return
	}
	d.syntax = enabled
	d.attachHighlighters()
}

func (d *DiffEditorWidget) Focusable() bool { return true }

func (d *DiffEditorWidget) SetFocused(focused bool) { d.focused = focused }

func (d *DiffEditorWidget) IsUnified() bool { return d.mode == DiffModeUnified }

func (d *DiffEditorWidget) IsWrapped() bool { return d.wrapMode == DiffWrapOn }

func (d *DiffEditorWidget) IsExtended() bool { return d.contextMode == DiffContextFullFile }

func (d *DiffEditorWidget) Mode() DiffMode { return d.mode }

func (d *DiffEditorWidget) WrapMode() DiffWrapMode { return d.wrapMode }

func (d *DiffEditorWidget) ContextMode() DiffContextMode { return d.contextMode }

func (d *DiffEditorWidget) DiffHighContrast() bool { return d.highContrast }

func (d *DiffEditorWidget) DiffCollapsedEmphasis() bool { return d.emphasizeGaps }

func (d *DiffEditorWidget) SetOldLines(lines []string) { d.oldLines = lines }

func (d *DiffEditorWidget) SetNewLines(lines []string) { d.newLines = lines }

func (d *DiffEditorWidget) SetUnified(unified bool) {
	mode := DiffModeSplit
	if unified {
		mode = DiffModeUnified
	}
	d.SetMode(mode)
}

func (d *DiffEditorWidget) SetMode(mode DiffMode) {
	d.modeExplicit = true
	d.applyMode(mode)
}

func (d *DiffEditorWidget) ApplyDefaultMode(mode DiffMode) {
	if !d.modeExplicit {
		d.applyMode(mode)
	}
}

func (d *DiffEditorWidget) applyMode(mode DiffMode) {
	if mode == d.mode {
		return
	}
	if d.editable {
		d.mode = mode
		d.focusLeft = false
		d.trackLiveEdits()
		d.buildLiveOverlays()
		return
	}
	row, _ := d.topDiffRow()
	d.mode = mode
	d.ensurePanes()
	d.ClearSelection()
	if len(d.SearchMatchesLeft) > 0 || len(d.SearchMatchesRight) > 0 {
		d.SetSearchMatches(d.SearchMatchesLeft, d.SearchMatchesRight)
	}
	d.scrollToDiffRow(row, diffAnySide, true)
}

func (d *DiffEditorWidget) SetWrapped(wrapped bool) {
	mode := DiffWrapOff
	if wrapped {
		mode = DiffWrapOn
	}
	d.SetWrapMode(mode)
}

func (d *DiffEditorWidget) SetWrapMode(mode DiffWrapMode) {
	d.wrapExplicit = true
	d.applyWrapMode(mode)
}

func (d *DiffEditorWidget) ApplyDefaultWrapMode(mode DiffWrapMode) {
	if !d.wrapExplicit {
		d.applyWrapMode(mode)
	}
}

func (d *DiffEditorWidget) applyWrapMode(mode DiffWrapMode) {
	if mode == d.wrapMode {
		return
	}
	if d.editable {
		d.wrapMode = mode
		for _, p := range d.viewPanes() {
			p.WordWrap = mode == DiffWrapOn
			p.Viewport.LeftCol = 0
		}
		return
	}
	row, right := d.topDiffRow()
	side := diffAnySide
	if d.IsUnified() {
		side = sideOf(right)
	}
	d.wrapMode = mode
	for _, p := range d.panes() {
		p.WordWrap = mode == DiffWrapOn
		p.Viewport.LeftCol = 0
	}
	d.ClearSelection()
	d.scrollToDiffRow(row, side, true)
}

func (d *DiffEditorWidget) SetDiffHighContrast(enabled bool) {
	d.highContrast = enabled
	d.applyOverlayOptions()
}

// Close stops background highlighting for a diff that is no longer shown.
func (d *DiffEditorWidget) Close() { d.SetRedrawRequest(nil) }

// SetRedrawRequest lets lines deep in a large diff draw before the lines
// above them are tokenized; notify must be safe to call off the main thread.
func (d *DiffEditorWidget) SetRedrawRequest(notify func()) {
	d.redraw = notify
	d.docSyntax.old.setNotify(notify)
	d.docSyntax.new.setNotify(notify)
	for _, p := range d.panes() {
		if p != nil && p != d.live {
			p.Highlighter.SetProgressive(notify)
		}
	}
}

func (d *DiffEditorWidget) SetDiffSigns(enabled bool) {
	d.signs = enabled
	d.applyOverlayOptions()
}

func (d *DiffEditorWidget) SetDiffSignsColor(enabled bool) {
	d.signsColor = enabled
	d.applyOverlayOptions()
}

func (d *DiffEditorWidget) setGutterStyle(style string) {
	for _, p := range d.panes() {
		p.GutterStyle = style
	}
}

func (d *DiffEditorWidget) SetDiffCollapsedEmphasis(enabled bool) {
	d.emphasizeGaps = enabled
	d.applyOverlayOptions()
}

func (d *DiffEditorWidget) applyOverlayOptions() {
	overlays := []*DiffOverlay{d.leftBase, d.rightBase, d.liveUnified}
	for _, p := range d.viewPanes() {
		overlays = append(overlays, p.DiffOverlay)
	}
	for _, o := range overlays {
		if o != nil {
			o.HighContrast = d.highContrast
			o.Signs = d.signs
			o.SignsColor = d.signsColor
			o.EmphasizeGaps = d.emphasizeGaps
			o.HoveredGap = d.hoveredGap
			o.MinGutter = d.minGutter
		}
	}
}

func (d *DiffEditorWidget) SetContextMode(mode DiffContextMode) {
	d.contextExplicit = true
	d.applyExtended(mode == DiffContextFullFile)
}

func (d *DiffEditorWidget) ApplyDefaultContextMode(mode DiffContextMode) {
	if !d.contextExplicit {
		d.applyExtended(mode == DiffContextFullFile)
	}
}

func (d *DiffEditorWidget) SetExtended(extended bool) {
	d.contextExplicit = true
	d.applyExtended(extended)
}

func (d *DiffEditorWidget) SetExtendedFetcher(fetch func(dv *DiffEditorWidget)) {
	d.OnFetchExtended = fetch
	if d.contextMode == DiffContextFullFile && !d.contextLoaded {
		d.applyExtended(true)
	}
}

func (d *DiffEditorWidget) applyExtended(extended bool) {
	if d.editable {
		d.extended = extended
		d.contextMode = DiffContextChangesOnly
		if extended {
			d.contextMode = DiffContextFullFile
		}
		d.revealed = nil
		d.rebuild()
		return
	}
	row, right := d.topDiffRow()
	anchor := d.anchorFor(row, right)
	d.extended = extended
	if extended {
		d.contextMode = DiffContextFullFile
		if !d.contextLoaded && d.OnFetchExtended != nil {
			d.startExtendedFetch(-1)
			return
		}
	} else {
		d.contextMode = DiffContextChangesOnly
		d.pendingGap = -1
		clear(d.expandedGaps)
	}
	d.rebuild()
	d.restoreAnchor(anchor)
}

func (d *DiffEditorWidget) FinishLoading() {
	d.Loading = false
	d.extendedFetching = false
	d.contextLoaded = true
	if d.pendingGap >= 0 && d.contextMode != DiffContextFullFile {
		d.expandedGaps[d.pendingGap] = true
		d.pendingGap = -1
		d.extended = false
		d.contextMode = DiffContextChangesOnly
	} else if d.contextMode == DiffContextFullFile {
		d.pendingGap = -1
		d.extended = true
	}
	anchor := d.loadingAnchor
	d.rebuild()
	d.restoreAnchor(anchor)
}

func (d *DiffEditorWidget) FailLoading() {
	d.Loading = false
	d.extendedFetching = false
	d.extended = false
	d.contextMode = DiffContextChangesOnly
	d.pendingGap = -1
	anchor := d.loadingAnchor
	d.rebuild()
	d.restoreAnchor(anchor)
}

func (d *DiffEditorWidget) startExtendedFetch(gap int) bool {
	if d.extendedFetching || d.contextLoaded || d.OnFetchExtended == nil {
		return false
	}
	row, right := d.topDiffRow()
	d.loadingAnchor = d.anchorFor(row, right)
	d.extendedFetching = true
	d.Loading = true
	d.pendingGap = gap
	d.hoveredGap = -1
	d.captured = nil
	d.pressed = false
	d.OnFetchExtended(d)
	return true
}

func (d *DiffEditorWidget) expandContextGap(gap int) {
	d.hoveredGap = -1
	if d.editable {
		if gap >= 0 && gap < len(d.liveSpans) {
			d.revealed = append(d.revealed, d.liveSpans[gap])
			d.rebuild()
		}
		return
	}
	if gap < 0 || d.expandedGaps[gap] || d.extendedFetching {
		return
	}
	if !d.contextLoaded && d.OnFetchExtended != nil {
		d.startExtendedFetch(gap)
		return
	}
	if !d.contextLoaded {
		return
	}
	row, right := d.topDiffRow()
	anchor := d.anchorFor(row, right)
	d.expandedGaps[gap] = true
	d.rebuild()
	d.restoreAnchor(anchor)
}

func (d *DiffEditorWidget) rebuild() {
	if d.editable {
		d.rebuildLive()
		return
	}
	if d.extended && d.contextLoaded {
		d.Lines = diff.FullDiffLines(d.oldLines, d.newLines)
		d.gapByLine = nil
	} else {
		d.Lines, d.gapByLine = compactDiffLinesWithContext(d.fileDiff, d.oldLines, d.newLines, d.expandedGaps)
	}
	d.hoveredGap = -1
	d.captured = nil

	d.syncDocSyntax()
	d.unifiedRows = buildUnifiedDiffLines(d.Lines)
	d.pairs = d.pairs[:0]
	li, ri := 0, 0
	for _, dl := range d.Lines {
		pair := splitPair{l: -1, r: -1}
		if dl.Left.Kind != diff.Blank {
			pair.l, li = li, li+1
		}
		if dl.Right.Kind != diff.Blank {
			pair.r, ri = ri, ri+1
		}
		d.pairs = append(d.pairs, pair)
	}
	d.panesGen++
	if !d.deferPanes {
		d.buildUnifiedPane()
		d.buildSplitPanes()
	}
	d.applyOverlayOptions()
	d.ClearSearch()
}

// ensurePanes builds the current mode's panes for a widget that defers them.
// A commit view holds a diff widget per file and only draws the visible ones,
// so it builds panes on demand rather than for every file and both modes.
func (d *DiffEditorWidget) ensurePanes() {
	if !d.deferPanes || d.editable {
		return
	}
	if d.IsUnified() {
		if d.unifiedGen != d.panesGen {
			d.buildUnifiedPane()
			d.applyOverlayOptions()
		}
		return
	}
	if d.splitGen != d.panesGen {
		d.buildSplitPanes()
		d.applyOverlayOptions()
	}
}

func (d *DiffEditorWidget) buildUnifiedPane() {
	d.unifiedGen = d.panesGen
	uLines := make([]string, 0, len(d.unifiedRows))
	uo := &DiffOverlay{Nums: []int{}, Gaps: map[int]int{}, Syntax: &d.docSyntax}
	for i, u := range d.unifiedRows {
		uLines = append(uLines, u.side.Text)
		uo.Kinds = append(uo.Kinds, u.side.Kind)
		uo.Nums = append(uo.Nums, u.side.Num)
		uo.NewSide = append(uo.NewSide, u.right || d.Lines[u.sourceLine].Left.Kind == diff.Blank)
		if gap, ok := d.gapByLine[u.sourceLine]; ok {
			uo.Gaps[i] = gap
		}
	}
	d.resetPane(d.unified, uLines, uo)
}

func (d *DiffEditorWidget) buildSplitPanes() {
	d.splitGen = d.panesGen
	d.leftBase = &DiffOverlay{Nums: []int{}, Gaps: map[int]int{}, Fillers: map[int]int{}, Syntax: &d.docSyntax}
	d.rightBase = &DiffOverlay{Nums: []int{}, Gaps: map[int]int{}, Fillers: map[int]int{}, Syntax: &d.docSyntax}
	n := len(d.Lines)
	lLines, rLines := make([]string, 0, n), make([]string, 0, n)
	d.leftRows, d.rightRows = make([]int, 0, n), make([]int, 0, n)
	d.leftBase.Kinds, d.rightBase.Kinds = make([]diff.LineKind, 0, n), make([]diff.LineKind, 0, n)
	d.leftBase.Nums, d.rightBase.Nums = make([]int, 0, n), make([]int, 0, n)
	for i, dl := range d.Lines {
		lLines, d.leftRows = appendDiffSide(d.leftBase, lLines, d.leftRows, dl.Left, i, d.gapByLine)
		rLines, d.rightRows = appendDiffSide(d.rightBase, rLines, d.rightRows, dl.Right, i, d.gapByLine)
	}
	d.rightBase.NewSide = make([]bool, len(rLines))
	for i := range d.rightBase.NewSide {
		d.rightBase.NewSide[i] = true
	}
	d.resetPane(d.left, lLines, d.leftBase)
	d.resetPane(d.right, rLines, d.rightBase)
}

func appendDiffSide(o *DiffOverlay, lines []string, rows []int, side diff.SideLine, row int, gaps map[int]int) ([]string, []int) {
	if side.Kind == diff.Blank {
		o.Fillers[len(lines)]++
		return lines, rows
	}
	if gap, ok := gaps[row]; ok {
		o.Gaps[len(lines)] = gap
	}
	o.Kinds = append(o.Kinds, side.Kind)
	o.Nums = append(o.Nums, side.Num)
	return append(lines, side.Text), append(rows, row)
}

func (d *DiffEditorWidget) resetPane(p *EditorPaneWidget, lines []string, o *DiffOverlay) {
	if len(lines) == 0 {
		lines = []string{""}
	}
	p.Buf.SetLines(lines)
	p.Cursor.Line = p.Buf.ClampLine(p.Cursor.Line)
	if n := len([]rune(lines[p.Cursor.Line])); p.Cursor.Col > n {
		p.Cursor.Col = n
	}
	p.Selection.Clear()
	p.mouseDown = false
	if p.Highlighter != nil {
		p.Highlighter.ClearCache()
	}
	p.InvalidateBracketColors()
	p.WordWrap = d.IsWrapped()
	p.SetDiffOverlay(o)
}

func (d *DiffEditorWidget) lead() *EditorPaneWidget {
	d.ensurePanes()
	if d.IsUnified() {
		return d.unified
	}
	if d.focusLeft {
		return d.left
	}
	return d.right
}

func (d *DiffEditorWidget) follower() *EditorPaneWidget {
	if d.IsUnified() {
		return nil
	}
	if d.focusLeft {
		return d.right
	}
	return d.left
}

func (d *DiffEditorWidget) paneRows(p *EditorPaneWidget) []int {
	if p == d.left {
		return d.leftRows
	}
	return d.rightRows
}

// bufLineForDiffRow maps a diff row onto a split pane's buffer: a row that is
// blank on that side resolves to the next real line.
func (d *DiffEditorWidget) bufLineForDiffRow(p *EditorPaneWidget, row int) int {
	if p == d.unified {
		for i, u := range d.unifiedRows {
			if u.sourceLine >= row {
				return i
			}
		}
		return max(len(d.unifiedRows)-1, 0)
	}
	rows := d.paneRows(p)
	i := sort.SearchInts(rows, row)
	if i >= len(rows) {
		return max(len(rows)-1, 0)
	}
	return i
}

func (d *DiffEditorWidget) diffRowForBufLine(p *EditorPaneWidget, line int) (row int, right bool) {
	if p == d.unified {
		if line < 0 || line >= len(d.unifiedRows) {
			return len(d.Lines), false
		}
		return d.unifiedRows[line].sourceLine, d.unifiedRows[line].right
	}
	rows := d.paneRows(p)
	if line < 0 || line >= len(rows) {
		return len(d.Lines), p == d.right
	}
	return rows[line], p == d.right
}

func (d *DiffEditorWidget) topDiffRow() (row int, right bool) {
	p := d.lead()
	l := p.layout()
	line, _ := l.rowToTop(p.topRow(l))
	return d.diffRowForBufLine(p, line)
}

type diffEditorAnchor struct {
	row     int
	right   bool
	lineNum int
	atTop   bool
}

func (d *DiffEditorWidget) anchorFor(row int, right bool) diffEditorAnchor {
	p := d.lead()
	a := diffEditorAnchor{row: row, right: right, atTop: p.topRow(p.layout()) == 0}
	for distance := 0; distance < len(d.Lines); distance++ {
		for _, index := range []int{row - distance, row + distance} {
			if index < 0 || index >= len(d.Lines) {
				continue
			}
			line := d.Lines[index]
			if line.Left.Num == 0 && line.Right.Num == 0 {
				continue
			}
			if (a.right && line.Right.Num > 0) || line.Left.Num == 0 {
				a.lineNum, a.right = line.Right.Num, true
			} else {
				a.lineNum, a.right = line.Left.Num, false
			}
			return a
		}
	}
	return a
}

func (d *DiffEditorWidget) restoreAnchor(a diffEditorAnchor) {
	if a.atTop {
		p := d.lead()
		p.setTopRow(p.layout(), 0)
		return
	}
	row, best := -1, -1
	for index, line := range d.Lines {
		num := line.Left.Num
		if a.right {
			num = line.Right.Num
		}
		if num == 0 {
			continue
		}
		distance := diffLineNumberDistance(a.lineNum, num)
		if best >= 0 && distance >= best {
			continue
		}
		row, best = index, distance
	}
	if row < 0 {
		row = min(max(a.row, 0), max(len(d.Lines)-1, 0))
	}
	d.scrollToDiffRow(row, sideOf(a.right), true)
}

type diffSide int

const (
	diffAnySide diffSide = iota
	diffLeftSide
	diffRightSide
)

func sideOf(right bool) diffSide {
	if right {
		return diffRightSide
	}
	return diffLeftSide
}

// scrollToDiffRow moves the lead pane's cursor to a diff row and scrolls it
// into view, as the top row when top is set. In unified mode side picks
// between the removed and added projection of a changed row.
func (d *DiffEditorWidget) scrollToDiffRow(row int, side diffSide, top bool) {
	if d.editable {
		return
	}
	p := d.lead()
	line := d.bufLineForDiffRow(p, row)
	if p == d.unified && side != diffAnySide {
		if i := d.unifiedIndex(row, side == diffRightSide); i >= 0 {
			line = i
		}
	}
	p.Cursor.Line = p.Buf.ClampLine(line)
	p.Cursor.Col = 0
	l := p.layout()
	start := l.startRow(p.Cursor.Line) + l.phantomRows(p.Cursor.Line)
	h := p.Viewport.Height
	if top {
		if h > 0 {
			start = min(start, max(l.total()-h, 0))
		}
		p.setTopRow(l, start)
		return
	}
	cur := p.topRow(l)
	if h <= 0 {
		p.setTopRow(l, start)
		return
	}
	if start < cur || start >= cur+h {
		p.setTopRow(l, start-h/2)
	}
}

func (d *DiffEditorWidget) Render(surface Surface) {
	w, h := surface.Size()
	r := d.GetRect()
	d.ensurePanes()
	if d.Loading {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				surface.SetCell(x, y, term.Cell{Ch: ' '})
			}
		}
		for i, ch := range "Loading..." {
			if i < w {
				surface.SetCell(i, 0, term.Cell{Ch: ch, Style: term.StyleDefault})
			}
		}
		return
	}
	wrap := d.IsWrapped()
	for _, p := range d.viewPanes() {
		p.WordWrap = wrap
	}
	if d.editable {
		if d.live == nil {
			return
		}
		d.syncLive()
	}
	if d.IsUnified() {
		if !d.editable {
			d.unified.Passive = !d.focused
		}
		d.ensureHighlighters(d.unified)
		d.unified.SetRect(r)
		d.unified.Render(surface)
		return
	}
	d.ensureHighlighters(d.left, d.right)
	divider := (w - 1) / 2
	if divider < 1 {
		d.right.SetRect(r)
		d.right.Render(surface)
		return
	}
	d.left.SetRect(Rect{X: r.X, Y: r.Y, W: divider, H: h})
	d.right.SetRect(Rect{X: r.X + divider + 1, Y: r.Y, W: w - divider - 1, H: h})
	lead, follow := d.lead(), d.follower()
	if lead != d.live {
		lead.Passive = !d.focused && !d.editable
	}
	if follow != d.live {
		follow.Passive = true
	}
	surfaceFor := func(p *EditorPaneWidget) Surface {
		pr := p.GetRect()
		return surface.Sub(Rect{X: pr.X - r.X, Y: 0, W: pr.W, H: h})
	}
	// Resetting the common wrap width every frame would render each pane at
	// its own width first, leaving a second row layout per pane to keep current.
	if wk := [4]int{d.left.GetRect().W, d.right.GetRect().W, d.left.GutterWidth(), d.right.GutterWidth()}; wk != d.wrapKey {
		d.wrapKey = wk
		d.left.wrapCols, d.right.wrapCols = 0, 0
	}
	for pass := 0; pass < 3; pass++ {
		widths := [2]int{d.left.Viewport.Width, d.right.Viewport.Width}
		d.alignSplit()
		lead.Render(surfaceFor(lead))
		syncFollower(lead, follow)
		follow.Render(surfaceFor(follow))
		if !wrap {
			break
		}
		lw, rw := d.left.Viewport.Width, d.right.Viewport.Width
		if lw != rw {
			c := min(lw, rw)
			d.left.wrapCols, d.right.wrapCols = c, c
			d.left.Viewport.Width, d.right.Viewport.Width = c, c
			continue
		}
		if widths == [2]int{lw, rw} {
			break
		}
	}
	for y := 0; y < h; y++ {
		surface.SetCell(divider, y, term.Cell{Ch: '│', Style: term.StyleBorder})
	}
}

func syncFollower(lead, follow *EditorPaneWidget) {
	follow.Viewport.Width = max(follow.Viewport.Width, 1)
	follow.setTopRow(follow.layout(), lead.topRow(lead.layout()))
	follow.Viewport.LeftCol = lead.Viewport.LeftCol
}

type splitAlignKey struct {
	pairs        *splitPair
	nPairs       int
	left, right  *DiffOverlay
	lPane, rPane paneAlignKey
}

type paneAlignKey struct {
	buf     *buffer.Buffer
	lines   *string
	n       int
	version uint64
	wrap    bool
	width   int
	tabW    int
}

func paneAlignKeyOf(p *EditorPaneWidget) paneAlignKey {
	k := paneAlignKey{buf: p.Buf, lines: unsafe.SliceData(p.Buf.Lines), n: len(p.Buf.Lines), version: p.Buf.Version(), wrap: p.WordWrap, tabW: p.resolveTabSize()}
	if p.WordWrap {
		k.width = max(p.Viewport.Width, 1)
	}
	return k
}

// alignSplit recomputes filler rows only when something they depend on
// changed: setting an overlay invalidates the panes' cached row layouts.
func (d *DiffEditorWidget) alignSplit() {
	key := splitAlignKey{
		pairs: unsafe.SliceData(d.pairs), nPairs: len(d.pairs),
		left: d.leftBase, right: d.rightBase,
		lPane: paneAlignKeyOf(d.left), rPane: paneAlignKeyOf(d.right),
	}
	if key == d.alignKey && d.left.DiffOverlay == d.leftBase && d.right.DiffOverlay == d.rightBase {
		return
	}
	d.alignKey = key
	d.leftBase.Fillers, d.rightBase.Fillers, d.alignContribs = alignSplitFillers(d.pairs, d.left, d.right)
	d.alignVer = d.right.Buf.Version()
	d.left.SetDiffOverlay(d.leftBase)
	d.right.SetDiffOverlay(d.rightBase)
}

type alignContrib struct{ lKey, lAmt, rKey, rAmt int }

func pairContrib(p splitPair, lr, rr, nl0, nl1, nr0, nr1 int) alignContrib {
	m := max(lr, rr)
	var c alignContrib
	if p.l < 0 {
		c.lKey, c.lAmt = nl0, m
	} else if m > lr {
		c.lKey, c.lAmt = nl1, m-lr
	}
	if p.r < 0 {
		c.rKey, c.rAmt = nr0, m
	} else if m > rr {
		c.rKey, c.rAmt = nr1, m-rr
	}
	return c
}

func alignSegs(p *EditorPaneWidget, l *rowLayout, line int) int {
	if line < 0 || !p.WordWrap || line >= len(p.Buf.Lines) {
		return 1
	}
	if _, ok := p.DiffOverlay.label(line); ok {
		return 1
	}
	return l.textRows(line)
}

// alignSplitFillers gives every diff row the same number of screen rows on
// both sides: a side that is blank, or wraps into fewer segments, is padded
// with filler rows before the next line it shows.
func alignSplitFillers(pairs []splitPair, left, right *EditorPaneWidget) (lf, rf map[int]int, contribs []alignContrib) {
	lf, rf = make(map[int]int), make(map[int]int)
	ll, rl := left.layout(), right.layout()
	nextL := make([]int, len(pairs)+1)
	nextR := make([]int, len(pairs)+1)
	nextL[len(pairs)], nextR[len(pairs)] = len(left.Buf.Lines), len(right.Buf.Lines)
	for i := len(pairs) - 1; i >= 0; i-- {
		nextL[i], nextR[i] = nextL[i+1], nextR[i+1]
		if pairs[i].l >= 0 {
			nextL[i] = pairs[i].l
		}
		if pairs[i].r >= 0 {
			nextR[i] = pairs[i].r
		}
	}
	contribs = make([]alignContrib, len(pairs))
	for i, p := range pairs {
		c := pairContrib(p, alignSegs(left, ll, p.l), alignSegs(right, rl, p.r), nextL[i], nextL[i+1], nextR[i], nextR[i+1])
		contribs[i] = c
		if c.lAmt > 0 {
			lf[c.lKey] += c.lAmt
		}
		if c.rAmt > 0 {
			rf[c.rKey] += c.rAmt
		}
	}
	return lf, rf, contribs
}

func (d *DiffEditorWidget) paneAt(x, y int) *EditorPaneWidget {
	if d.IsUnified() {
		return d.unified
	}
	for _, p := range []*EditorPaneWidget{d.left, d.right} {
		pr := p.GetRect()
		if x >= pr.X && x < pr.X+pr.W && y >= pr.Y && y < pr.Y+pr.H {
			return p
		}
	}
	return nil
}

func (d *DiffEditorWidget) gapAt(p *EditorPaneWidget, y int) (int, bool) {
	localY := y - p.GetRect().Y
	if localY < 0 || localY >= p.Viewport.Height {
		return 0, false
	}
	row := p.rowAt(localY)
	if row.isPhantom() || p.DiffOverlay == nil {
		return 0, false
	}
	gap, ok := p.DiffOverlay.Gaps[row.bufLine]
	return gap, ok
}

func (d *DiffEditorWidget) OwnsPointerCapture() bool {
	if d.captured != nil {
		return true
	}
	for _, p := range d.viewPanes() {
		if p.OwnsPointerCapture() {
			return true
		}
	}
	return false
}

func (d *DiffEditorWidget) CursorPosition() (int, int, bool) {
	if d.Loading || !d.focused {
		return 0, 0, false
	}
	p := d.lead()
	return p.CursorX, p.CursorY, true
}

func (d *DiffEditorWidget) HandleEvent(ev tcell.Event) EventResult {
	d.ensurePanes()
	if d.extendedFetching || d.Loading {
		return EventIgnored
	}
	if d.editable {
		if d.live == nil {
			return EventIgnored
		}
		d.syncLive()
		if kev, ok := ev.(*tcell.EventKey); ok {
			return d.handleLiveKey(kev)
		}
	}
	switch tev := ev.(type) {
	case *tcell.EventKey:
		p := d.lead()
		if tev.Key() == tcell.KeyEnter && tev.Modifiers() == 0 && p.DiffOverlay != nil {
			if gap, ok := p.DiffOverlay.Gaps[p.Cursor.Line]; ok {
				d.expandContextGap(gap)
				return EventConsumed
			}
		}
		return p.HandleEvent(ev)
	case *tcell.EventMouse:
		return d.handleMouse(tev)
	}
	return EventIgnored
}

func (d *DiffEditorWidget) handleMouse(ev *tcell.EventMouse) EventResult {
	btn := ev.Buttons()
	x, y := ev.Position()
	if btn&(tcell.WheelUp|tcell.WheelDown|tcell.WheelLeft|tcell.WheelRight) != 0 {
		d.hoveredGap = -1
		d.applyOverlayOptions()
		return d.lead().HandleEvent(ev)
	}
	target := d.captured
	if target == nil {
		target = d.paneAt(x, y)
	}
	if target == nil {
		return EventIgnored
	}
	if btn == tcell.ButtonNone {
		d.pressed = false
		if d.captured == nil {
			gap, ok := d.gapAt(target, y)
			hovered := -1
			if ok {
				hovered = gap
			}
			if hovered != d.hoveredGap {
				d.hoveredGap = hovered
				d.applyOverlayOptions()
				target.HandleEvent(ev)
				return EventConsumed
			}
		}
	}
	if btn&tcell.Button1 != 0 && !d.pressed {
		d.pressed = true
		if gap, ok := d.gapAt(target, y); ok && d.captured == nil {
			d.expandContextGap(gap)
			return EventConsumed
		}
		if !d.IsUnified() && d.captured == nil {
			d.focusLeft = target == d.left
			if d.editable && !d.focusLeft {
				d.left.Selection.Clear()
			}
		}
	}
	result := target.HandleEvent(ev)
	if btn == tcell.ButtonNone {
		d.captured = nil
	} else if result == EventCaptured {
		d.captured = target
	}
	return result
}

func (d *DiffEditorWidget) ClearSelection() {
	for _, p := range d.panes() {
		p.Selection.Clear()
	}
}

func (d *DiffEditorWidget) CopySelection() string {
	d.ensurePanes()
	p := d.lead()
	if !p.Selection.Active {
		return ""
	}
	start, end := p.Selection.Range(p.Cursor.Line, p.Cursor.Col)
	sel := diffTextSelection{
		Anchor:  diffSelPos{Line: start.Line, Col: start.Col},
		Current: diffSelPos{Line: end.Line, Col: end.Col},
	}
	text := sel.Text(len(p.Buf.Lines), func(line int) (string, bool) {
		return p.Buf.Lines[line], p.DiffOverlay.kind(line) != diff.Collapsed
	})
	if text != "" {
		p.Selection.Clear()
	}
	return text
}

func (d *DiffEditorWidget) LeftLines() []string {
	rows := d.searchRows()
	lines := make([]string, len(rows))
	for i, dl := range rows {
		lines[i] = dl.Left.Text
	}
	return lines
}

func (d *DiffEditorWidget) RightLines() []string {
	rows := d.searchRows()
	lines := make([]string, len(rows))
	for i, dl := range rows {
		lines[i] = dl.Right.Text
	}
	return lines
}

func (d *DiffEditorWidget) CombinedLines() []string {
	lines := make([]string, len(d.Lines))
	for i, dl := range d.Lines {
		if dl.Left.Text == dl.Right.Text {
			lines[i] = dl.Left.Text
		} else {
			lines[i] = dl.Left.Text + " " + dl.Right.Text
		}
	}
	return lines
}

func (d *DiffEditorWidget) ApplySearchHighlight(query string, opts SearchOptions) {
	if query == "" {
		return
	}
	leftMatches, _ := FindInLines(d.LeftLines(), query, opts)
	rightMatches, _ := FindInLines(d.RightLines(), query, opts)
	d.SetSearchMatches(leftMatches, rightMatches)
}

func (d *DiffEditorWidget) ClearSearch() {
	if d.editable {
		d.clearLiveSearch()
	}
	d.SearchMatchesLeft = nil
	d.SearchMatchesRight = nil
	d.searchRefs = nil
	d.searchActiveRight = false
	for _, p := range d.panes() {
		p.SearchMatches = nil
		p.SearchActive = -1
		p.searchByLine = nil
	}
}

func (d *DiffEditorWidget) unifiedIndex(row int, right bool) int {
	for i, u := range d.unifiedRows {
		if u.sourceLine == row && u.right == right {
			return i
		}
	}
	return -1
}

// SetSearchMatches takes matches indexed by diff row (see LeftLines and
// RightLines) and returns them merged in display order.
func (d *DiffEditorWidget) SetSearchMatches(left, right []FindMatch) []FindMatch {
	if d.editable {
		return d.setLiveSearch(left, right)
	}
	d.ensurePanes()
	d.SearchMatchesLeft = left
	d.SearchMatchesRight = right
	type entry struct {
		match FindMatch
		right bool
		pane  *EditorPaneWidget
		line  int
	}
	var entries []entry
	add := func(m FindMatch, isRight bool) {
		if d.IsUnified() {
			if line := d.unifiedIndex(m.Line, isRight); line >= 0 {
				entries = append(entries, entry{m, isRight, d.unified, line})
			}
			return
		}
		p := d.left
		if isRight {
			p = d.right
		}
		rows := d.paneRows(p)
		if i := sort.SearchInts(rows, m.Line); i < len(rows) && rows[i] == m.Line {
			entries = append(entries, entry{m, isRight, p, i})
		}
	}
	for _, m := range left {
		add(m, false)
	}
	for _, m := range right {
		add(m, true)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if d.IsUnified() {
			if a.line != b.line {
				return a.line < b.line
			}
			return a.match.Col < b.match.Col
		}
		if a.match.Line != b.match.Line {
			return a.match.Line < b.match.Line
		}
		if a.right != b.right {
			return !a.right
		}
		return a.match.Col < b.match.Col
	})
	for _, p := range d.panes() {
		p.SearchMatches = nil
		p.SearchActive = -1
	}
	merged := make([]FindMatch, len(entries))
	d.searchRefs = make([]diffSearchRef, len(entries))
	for i, e := range entries {
		merged[i] = e.match
		d.searchRefs[i] = diffSearchRef{right: e.right, pane: e.pane, paneIdx: len(e.pane.SearchMatches)}
		e.pane.SearchMatches = append(e.pane.SearchMatches, FindMatch{Line: e.line, Col: e.match.Col, Len: e.match.Len})
	}
	for _, p := range d.panes() {
		p.buildSearchIndex()
	}
	return merged
}

func (d *DiffEditorWidget) SetActiveMatch(mergedIdx int) {
	d.ensurePanes()
	if d.editable {
		d.setLiveActive(mergedIdx)
		return
	}
	for _, p := range d.panes() {
		p.SearchActive = -1
	}
	d.searchActiveRight = false
	if mergedIdx < 0 || mergedIdx >= len(d.searchRefs) {
		return
	}
	ref := d.searchRefs[mergedIdx]
	ref.pane.SearchActive = ref.paneIdx
	d.searchActiveRight = ref.right
	if !d.IsUnified() {
		d.focusLeft = ref.pane == d.left
	}
}

// ScrollToLine reveals a diff row (an index into LeftLines/RightLines).
func (d *DiffEditorWidget) ScrollToLine(line int) {
	if d.editable {
		d.scrollToLiveHit(line)
		return
	}
	d.scrollToDiffRow(line, sideOf(d.searchActiveRight), false)
}

// DiffCombinedLines is CombinedLines for a changes-only diff that has no tab.
func DiffCombinedLines(fd diff.FileDiff) []string {
	d := &DiffEditorWidget{}
	d.Lines, _ = compactDiffLinesWithContext(fd, nil, nil, nil)
	return d.CombinedLines()
}

func diffLineNumberDistance(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
