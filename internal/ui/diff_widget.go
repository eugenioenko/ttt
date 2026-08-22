package ui

import (
	"sort"
	"time"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/highlight"
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v3"
)

type diffMergedRef struct {
	isRight bool
	sideIdx int
}

type DiffViewWidget struct {
	BaseWidget
	FilePath        string
	Lines           []diff.DiffLine
	Highlighter     *highlight.Highlighter
	TopLine         int
	LeftCol         int
	maxLineW        int
	viewH           int
	contentW        int
	totalVisualRows int
	scrollbar       Scrollbar
	hscrollbar      HScrollbar
	rhscrollbar     HScrollbar

	// layout cache for mouse hit-testing
	layoutDividerX   int
	layoutLeftStart  int
	layoutLeftW      int
	layoutRightStart int
	layoutRightW     int
	layoutGutterW    int
	wrapMap          []diffWrapEntry
	wrapTopOffset    int

	// selection state
	selecting     bool
	hasSelection  bool
	selRight      bool
	selection     diffTextSelection
	lastClickTime time.Time
	lastClickPos  diffSelPos

	SearchMatchesLeft   []FindMatch
	SearchMatchesRight  []FindMatch
	searchMergedRefs    []diffMergedRef
	searchActiveRight   bool
	searchActiveSideIdx int

	// diff view modes and their projected rows
	extended        bool
	wrapMode        DiffWrapMode
	wrapExplicit    bool
	mode            DiffMode
	modeExplicit    bool
	contextMode     DiffContextMode
	contextExplicit bool
	contextLoaded   bool
	expandedGaps    map[int]bool
	gapByLine       map[int]int
	pendingGap      int
	hoveredGap      int
	hasHoveredGap   bool
	primaryPressed  bool
	highContrast    bool
	fileDiff        diff.FileDiff
	oldLines        []string
	newLines        []string
	unifiedLines    []diffUnifiedLine

	OnFetchExtended func(dv *DiffViewWidget)
	Loading         bool
}

func (d *DiffViewWidget) SetDiffHighContrast(enabled bool) { d.highContrast = enabled }

func (d *DiffViewWidget) DiffHighContrast() bool { return d.highContrast }

func NewDiffViewWidget(filePath string, fd diff.FileDiff, oldLines, newLines []string, extended bool) *DiffViewWidget {
	dv := &DiffViewWidget{
		FilePath:            filePath,
		Highlighter:         highlight.New(filePath),
		searchActiveSideIdx: -1,
		fileDiff:            fd,
		oldLines:            oldLines,
		newLines:            newLines,
		extended:            extended,
		contextMode:         DiffContextChangesOnly,
		contextExplicit:     extended,
		contextLoaded:       oldLines != nil || newLines != nil,
		expandedGaps:        make(map[int]bool),
		pendingGap:          -1,
		hoveredGap:          -1,
	}
	if extended {
		dv.contextMode = DiffContextFullFile
	}
	dv.rebuildLines()
	return dv
}

func (d *DiffViewWidget) SetOldLines(lines []string) {
	d.oldLines = lines
}

func (d *DiffViewWidget) SetNewLines(lines []string) {
	d.newLines = lines
}

func (d *DiffViewWidget) IsExtended() bool {
	return d.contextMode == DiffContextFullFile
}

func (d *DiffViewWidget) ContextMode() DiffContextMode { return d.contextMode }

func (d *DiffViewWidget) SetContextMode(mode DiffContextMode) {
	d.contextExplicit = true
	d.applyContextMode(mode)
}

func (d *DiffViewWidget) ApplyDefaultContextMode(mode DiffContextMode) {
	if d.contextExplicit {
		return
	}
	d.applyContextMode(mode)
}

func (d *DiffViewWidget) applyContextMode(mode DiffContextMode) {
	d.applyExtended(mode == DiffContextFullFile)
}

func (d *DiffViewWidget) IsWrapped() bool {
	return d.wrapMode == DiffWrapOn
}

func (d *DiffViewWidget) SetWrapped(wrapped bool) {
	mode := DiffWrapOff
	if wrapped {
		mode = DiffWrapOn
	}
	d.SetWrapMode(mode)
}

func (d *DiffViewWidget) WrapMode() DiffWrapMode { return d.wrapMode }

func (d *DiffViewWidget) SetWrapMode(mode DiffWrapMode) {
	d.wrapExplicit = true
	d.applyWrapMode(mode)
}

func (d *DiffViewWidget) ApplyDefaultWrapMode(mode DiffWrapMode) {
	if d.wrapExplicit {
		return
	}
	d.applyWrapMode(mode)
}

func (d *DiffViewWidget) applyWrapMode(mode DiffWrapMode) {
	if mode == d.wrapMode {
		return
	}
	d.wrapMode = mode
	d.TopLine = 0
	d.wrapTopOffset = 0
	d.LeftCol = 0
	d.ClearSelection()
}

func (d *DiffViewWidget) IsUnified() bool {
	return d.mode == DiffModeUnified
}

func (d *DiffViewWidget) SetUnified(unified bool) {
	mode := DiffModeSplit
	if unified {
		mode = DiffModeUnified
	}
	d.SetMode(mode)
}

func (d *DiffViewWidget) Mode() DiffMode { return d.mode }

func (d *DiffViewWidget) SetMode(mode DiffMode) {
	d.modeExplicit = true
	d.applyMode(mode)
}

func (d *DiffViewWidget) ApplyDefaultMode(mode DiffMode) {
	if d.modeExplicit {
		return
	}
	d.applyMode(mode)
}

func (d *DiffViewWidget) applyMode(mode DiffMode) {
	if mode == d.mode {
		return
	}
	d.mode = mode
	d.TopLine = 0
	d.wrapTopOffset = 0
	d.LeftCol = 0
	d.ClearSelection()
	if len(d.SearchMatchesLeft) > 0 || len(d.SearchMatchesRight) > 0 {
		d.SetSearchMatches(d.SearchMatchesLeft, d.SearchMatchesRight)
	}
}

func (d *DiffViewWidget) SetExtended(extended bool) {
	d.contextExplicit = true
	d.applyExtended(extended)
}

func (d *DiffViewWidget) applyExtended(extended bool) {
	if extended && !d.contextLoaded && d.OnFetchExtended != nil {
		d.Loading = true
		d.extended = true
		d.contextMode = DiffContextFullFile
		d.OnFetchExtended(d)
		return
	}
	d.extended = extended
	if extended {
		d.contextMode = DiffContextFullFile
	} else {
		d.contextMode = DiffContextChangesOnly
		d.pendingGap = -1
		clear(d.expandedGaps)
	}
	d.rebuildLines()
	d.TopLine = 0
	d.wrapTopOffset = 0
	d.LeftCol = 0
	d.ClearSearch()
	d.ClearSelection()
}

func (d *DiffViewWidget) FinishLoading() {
	d.Loading = false
	d.contextLoaded = true
	if d.pendingGap >= 0 {
		d.expandedGaps[d.pendingGap] = true
		d.pendingGap = -1
		d.extended = false
		d.contextMode = DiffContextChangesOnly
	}
	d.rebuildLines()
	d.TopLine = 0
	d.wrapTopOffset = 0
	d.LeftCol = 0
	d.ClearSearch()
	d.ClearSelection()
}

func (d *DiffViewWidget) rebuildLines() {
	if d.extended && d.contextLoaded {
		d.Lines = diff.FullDiffLines(d.oldLines, d.newLines)
		d.gapByLine = nil
	} else {
		d.Lines, d.gapByLine = compactDiffLinesWithContext(d.fileDiff, d.oldLines, d.newLines, d.expandedGaps)
	}
	d.unifiedLines = buildUnifiedDiffLines(d.Lines)
	maxW := 0
	for _, dl := range d.Lines {
		if lw := diffLineVisualWidth(dl.Left.Text); lw > maxW {
			maxW = lw
		}
		if rw := diffLineVisualWidth(dl.Right.Text); rw > maxW {
			maxW = rw
		}
	}
	d.maxLineW = maxW
}

func (d *DiffViewWidget) expandContextGap(gap int) {
	d.hasHoveredGap = false
	if gap < 0 || d.expandedGaps[gap] {
		return
	}
	if !d.contextLoaded && d.OnFetchExtended != nil {
		d.pendingGap = gap
		d.Loading = true
		d.OnFetchExtended(d)
		return
	}
	if !d.contextLoaded {
		return
	}
	d.expandedGaps[gap] = true
	d.rebuildLines()
	d.ClearSearch()
	d.ClearSelection()
}

func (d *DiffViewWidget) screenSourceLine(my int) int {
	r := d.GetRect()
	localY := my - r.Y
	if localY < 0 || localY >= d.viewH {
		return -1
	}
	line := d.TopLine + localY
	if d.IsWrapped() {
		if localY >= len(d.wrapMap) {
			return -1
		}
		line = d.wrapMap[localY].line
	}
	if d.IsUnified() {
		if line < 0 || line >= len(d.unifiedLines) {
			return -1
		}
		return d.unifiedLines[line].sourceLine
	}
	return line
}

func (d *DiffViewWidget) Focusable() bool { return true }

func (d *DiffViewWidget) LeftLines() []string {
	lines := make([]string, len(d.Lines))
	for i, dl := range d.Lines {
		lines[i] = dl.Left.Text
	}
	return lines
}

func (d *DiffViewWidget) RightLines() []string {
	lines := make([]string, len(d.Lines))
	for i, dl := range d.Lines {
		lines[i] = dl.Right.Text
	}
	return lines
}

func (d *DiffViewWidget) CombinedLines() []string {
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

func (d *DiffViewWidget) ApplySearchHighlight(query string, opts SearchOptions) {
	if query == "" {
		return
	}
	leftMatches, _ := FindInLines(d.LeftLines(), query, opts)
	rightMatches, _ := FindInLines(d.RightLines(), query, opts)
	d.SetSearchMatches(leftMatches, rightMatches)
}

func (d *DiffViewWidget) ScrollToLine(line int) {
	if d.IsUnified() {
		line = d.unifiedIndexForSearchLine(line)
	}
	if d.IsWrapped() && d.viewH > 0 {
		target := 0
		if d.IsUnified() {
			target = unifiedLineToVisualRow(d.unifiedLines, line, d.layoutLeftW)
		} else {
			target = diffLineToVisualRow(d.Lines, line, d.layoutLeftW, d.layoutRightW)
		}
		top := d.topVisualRow()
		if target < top || target >= top+d.viewH {
			d.setTopVisualRow(target - d.viewH/2)
		}
		return
	}
	if d.viewH <= 0 {
		d.TopLine = line
		return
	}
	if line < d.TopLine || line >= d.TopLine+d.viewH {
		d.TopLine = line - d.viewH/2
		if d.TopLine < 0 {
			d.TopLine = 0
		}
		lineCount := len(d.Lines)
		if d.IsUnified() {
			lineCount = len(d.unifiedLines)
		}
		max := lineCount - d.viewH
		if max < 0 {
			max = 0
		}
		if d.TopLine > max {
			d.TopLine = max
		}
	}
}

func (d *DiffViewWidget) ClearSearch() {
	d.SearchMatchesLeft = nil
	d.SearchMatchesRight = nil
	d.searchMergedRefs = nil
	d.searchActiveRight = false
	d.searchActiveSideIdx = -1
}

func (d *DiffViewWidget) SetSearchMatches(left, right []FindMatch) []FindMatch {
	d.SearchMatchesLeft = left
	d.SearchMatchesRight = right

	type entry struct {
		match   FindMatch
		isRight bool
		sideIdx int
	}
	var entries []entry
	for i, m := range left {
		if !d.IsUnified() || d.unifiedIndexForSource(m.Line, false) >= 0 {
			entries = append(entries, entry{m, false, i})
		}
	}
	for i, m := range right {
		if !d.IsUnified() || d.unifiedIndexForSource(m.Line, true) >= 0 {
			entries = append(entries, entry{m, true, i})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if d.IsUnified() {
			leftIndex := d.unifiedIndexForSource(entries[i].match.Line, entries[i].isRight)
			rightIndex := d.unifiedIndexForSource(entries[j].match.Line, entries[j].isRight)
			if leftIndex != rightIndex {
				return leftIndex < rightIndex
			}
			return entries[i].match.Col < entries[j].match.Col
		}
		if entries[i].match.Line != entries[j].match.Line {
			return entries[i].match.Line < entries[j].match.Line
		}
		if entries[i].isRight != entries[j].isRight {
			return !entries[i].isRight
		}
		return entries[i].match.Col < entries[j].match.Col
	})

	merged := make([]FindMatch, len(entries))
	d.searchMergedRefs = make([]diffMergedRef, len(entries))
	for i, e := range entries {
		merged[i] = e.match
		d.searchMergedRefs[i] = diffMergedRef{isRight: e.isRight, sideIdx: e.sideIdx}
	}
	return merged
}

func (d *DiffViewWidget) SetActiveMatch(mergedIdx int) {
	if mergedIdx >= 0 && mergedIdx < len(d.searchMergedRefs) {
		ref := d.searchMergedRefs[mergedIdx]
		d.searchActiveRight = ref.isRight
		d.searchActiveSideIdx = ref.sideIdx
	} else {
		d.searchActiveRight = false
		d.searchActiveSideIdx = -1
	}
}

func (d *DiffViewWidget) gutterWidth() int {
	maxLine := 0
	for _, dl := range d.Lines {
		if dl.Left.Num > maxLine {
			maxLine = dl.Left.Num
		}
		if dl.Right.Num > maxLine {
			maxLine = dl.Right.Num
		}
	}
	digits := 1
	for n := maxLine; n >= 10; n /= 10 {
		digits++
	}
	return digits + 3
}

func (d *DiffViewWidget) Render(surface Surface) {
	originalW, originalH := surface.Size()
	w, h := originalW, originalH
	r := d.GetRect()

	if d.Loading {
		msg := "Loading..."
		for i, ch := range msg {
			if i < w {
				surface.SetCell(i, 0, term.Cell{Ch: ch, Style: term.StyleDefault})
			}
		}
		return
	}

	gutterW := d.gutterWidth()

	showVScroll, showHScroll := false, false
	var dividerX, leftStart, leftW, rightStart, rightW int
	for range 3 {
		w, h = originalW, originalH
		if showVScroll {
			w--
		}
		leftStart = gutterW
		if d.IsUnified() {
			dividerX = -1
			leftW = w - gutterW
			rightStart, rightW = 0, 0
		} else {
			dividerX = (w - 1) / 2
			leftW = dividerX - gutterW
			rightStart = dividerX + 1 + gutterW
			rightW = w - rightStart
		}
		if leftW < 1 || (!d.IsUnified() && rightW < 1) {
			return
		}
		showHScroll = !d.IsWrapped() && d.maxLineW > leftW
		if showHScroll {
			h--
		}
		totalRows := len(d.Lines)
		if d.IsUnified() {
			totalRows = len(d.unifiedLines)
		}
		if d.IsWrapped() {
			if d.IsUnified() {
				totalRows = totalUnifiedVisualRows(d.unifiedLines, leftW)
			} else {
				totalRows = totalDiffVisualRows(d.Lines, leftW, rightW)
			}
		}
		newShowVScroll := totalRows > h
		if newShowVScroll == showVScroll {
			d.totalVisualRows = totalRows
			break
		}
		showVScroll = newShowVScroll
	}

	d.viewH = h
	d.contentW = leftW

	d.layoutDividerX = dividerX
	d.layoutLeftStart = leftStart
	d.layoutLeftW = leftW
	d.layoutRightStart = rightStart
	d.layoutRightW = rightW
	d.layoutGutterW = gutterW

	if d.IsWrapped() {
		d.setTopVisualRow(d.topVisualRow())
		if d.IsUnified() {
			d.wrapMap = buildUnifiedWrapMap(d.unifiedLines, d.TopLine, d.wrapTopOffset, h, leftW)
		} else {
			d.wrapMap = buildDiffWrapMap(d.Lines, d.TopLine, d.wrapTopOffset, h, leftW, rightW)
		}
	} else {
		d.wrapMap = nil
		d.wrapTopOffset = 0
		if d.IsUnified() {
			d.totalVisualRows = len(d.unifiedLines)
		} else {
			d.totalVisualRows = len(d.Lines)
		}
		d.setTopVisualRow(d.TopLine)
	}

	for y := 0; y < h; y++ {
		if d.IsUnified() {
			d.renderUnifiedRow(surface, y, w, gutterW, leftStart, leftW)
			continue
		}
		idx := d.TopLine + y
		leftSegment, rightSegment := 0, 0
		continuation := false
		if d.IsWrapped() {
			entry := d.wrapMap[y]
			idx = entry.line
			leftSegment = entry.leftStart
			rightSegment = entry.rightStart
			continuation = entry.continuation
		}
		surface.SetCell(dividerX, y, term.Cell{Ch: '│', Style: term.StyleBorder})

		if idx >= len(d.Lines) {
			for x := 0; x < dividerX; x++ {
				surface.SetCell(x, y, term.Cell{Ch: ' '})
			}
			for x := dividerX + 1; x < w; x++ {
				surface.SetCell(x, y, term.Cell{Ch: ' '})
			}
			continue
		}

		dl := d.Lines[idx]

		leftStyle := diffKindStyle(dl.Left.Kind)
		rightStyle := diffKindStyle(dl.Right.Kind)
		if gap, ok := d.gapByLine[idx]; ok && d.hasHoveredGap && gap == d.hoveredGap {
			leftStyle = term.StyleDiffCollapsed
			rightStyle = term.StyleDiffCollapsed
		}

		if continuation {
			renderDiffGutter(surface, 0, y, gutterW, diff.SideLine{})
			renderDiffGutter(surface, dividerX+1, y, gutterW, diff.SideLine{})
		} else {
			renderDiffGutter(surface, 0, y, gutterW, dl.Left)
			renderDiffGutter(surface, dividerX+1, y, gutterW, dl.Right)
		}

		var leftSpans, rightSpans []highlight.Span
		if d.Highlighter != nil {
			if dl.Left.Text != "" && dl.Left.Kind != diff.Collapsed {
				leftSpans = d.Highlighter.HighlightLine(dl.Left.Text)
			}
			if dl.Right.Text != "" && dl.Right.Kind != diff.Collapsed {
				rightSpans = d.Highlighter.HighlightLine(dl.Right.Text)
			}
		}
		leftActive := -1
		if !d.searchActiveRight {
			leftActive = d.searchActiveSideIdx
		}
		rightActive := -1
		if d.searchActiveRight {
			rightActive = d.searchActiveSideIdx
		}
		leftScroll, rightScroll := d.LeftCol, d.LeftCol
		if d.IsWrapped() {
			leftScroll, rightScroll = 0, 0
		}
		d.renderSide(surface, leftStart, y, leftW, dl.Left.Text, leftStyle, diffKindForeground(dl.Left.Kind, d.highContrast), leftSpans, idx, idx, d.SearchMatchesLeft, leftActive, !d.selRight, leftSegment, leftScroll)
		d.renderSide(surface, rightStart, y, rightW, dl.Right.Text, rightStyle, diffKindForeground(dl.Right.Kind, d.highContrast), rightSpans, idx, idx, d.SearchMatchesRight, rightActive, d.selRight, rightSegment, rightScroll)
	}

	if showVScroll {
		d.scrollbar.X = r.X + w
		d.scrollbar.Y = r.Y
		d.scrollbar.Height = h
		d.scrollbar.TotalItems = d.totalVisualRows
		d.scrollbar.TopItem = d.topVisualRow()
		d.scrollbar.Render(surface, w, 0)
	}

	if showHScroll {
		d.hscrollbar.X = r.X + leftStart
		d.hscrollbar.Y = r.Y + h
		d.hscrollbar.Width = leftW
		d.hscrollbar.TotalCols = d.maxLineW
		d.hscrollbar.LeftCol = d.LeftCol
		d.hscrollbar.Render(surface, leftStart, h)

		for x := 0; x < gutterW; x++ {
			surface.SetCell(x, h, term.Cell{Ch: ' '})
		}
		if !d.IsUnified() {
			surface.SetCell(dividerX, h, term.Cell{Ch: '│', Style: term.StyleBorder})
			for x := dividerX + 1; x < rightStart; x++ {
				surface.SetCell(x, h, term.Cell{Ch: ' '})
			}

			d.rhscrollbar.X = r.X + rightStart
			d.rhscrollbar.Y = r.Y + h
			d.rhscrollbar.Width = rightW
			d.rhscrollbar.TotalCols = d.maxLineW
			d.rhscrollbar.LeftCol = d.LeftCol
			d.rhscrollbar.Render(surface, rightStart, h)
		}
	}
}

func (d *DiffViewWidget) renderUnifiedRow(surface Surface, y, w, gutterW, contentStart, contentW int) {
	displayIndex := d.TopLine + y
	segmentStart := 0
	continuation := false
	if d.IsWrapped() {
		entry := d.wrapMap[y]
		displayIndex = entry.line
		segmentStart = entry.leftStart
		continuation = entry.continuation
	}
	if displayIndex >= len(d.unifiedLines) {
		for x := 0; x < w; x++ {
			surface.SetCell(x, y, term.Cell{Ch: ' '})
		}
		return
	}

	line := d.unifiedLines[displayIndex]
	baseStyle := diffKindStyle(line.side.Kind)
	if gap, ok := d.gapByLine[line.sourceLine]; ok && d.hasHoveredGap && gap == d.hoveredGap {
		baseStyle = term.StyleDiffCollapsed
	}
	if continuation {
		renderDiffGutter(surface, 0, y, gutterW, diff.SideLine{})
	} else {
		renderDiffGutter(surface, 0, y, gutterW, line.side)
	}

	var spans []highlight.Span
	if d.Highlighter != nil && line.side.Text != "" && line.side.Kind != diff.Collapsed {
		spans = d.Highlighter.HighlightLine(line.side.Text)
	}
	matches, active := d.SearchMatchesLeft, -1
	if line.right {
		matches = d.SearchMatchesRight
		if d.searchActiveRight {
			active = d.searchActiveSideIdx
		}
	} else if !d.searchActiveRight {
		active = d.searchActiveSideIdx
	}
	leftScroll := d.LeftCol
	if d.IsWrapped() {
		leftScroll = 0
	}
	d.renderSide(surface, contentStart, y, contentW, line.side.Text, baseStyle, diffKindForeground(line.side.Kind, d.highContrast), spans, line.sourceLine, displayIndex, matches, active, true, segmentStart, leftScroll)
}

func (d *DiffViewWidget) renderSide(surface Surface, x, y, w int, text string, baseStyle, foregroundStyle term.Style, spans []highlight.Span, matchLineIdx, selectionLineIdx int, matches []FindMatch, activeIdx int, selSide bool, segmentStart, leftVisualCol int) {
	renderDiffText(surface, x, y, w, text, baseStyle, foregroundStyle, spans, segmentStart, leftVisualCol, func(colIdx int, cell term.Cell) term.Cell {
		for mi, m := range matches {
			if m.Line == matchLineIdx && colIdx >= m.Col && colIdx < m.Col+m.Len {
				if mi == activeIdx {
					cell.Style = term.StyleSearchActive
				} else {
					cell.Style = term.StyleSearchMatch
				}
				cell.BgStyle = 0
				break
			}
		}
		if selSide && d.hasSelection && d.selection.Contains(selectionLineIdx, colIdx) {
			cell.BgStyle = term.StyleSelection
		}
		return cell
	})
}

func (d *DiffViewWidget) screenToSel(mx, my int) (pos diffSelPos, right bool, ok bool) {
	r := d.GetRect()
	localX := mx - r.X
	localY := my - r.Y
	if localY < 0 || localY >= d.viewH {
		return diffSelPos{}, false, false
	}
	line := d.TopLine + localY
	leftStart, rightStart := 0, 0
	if d.IsWrapped() {
		if localY >= len(d.wrapMap) {
			return diffSelPos{}, false, false
		}
		entry := d.wrapMap[localY]
		line = entry.line
		leftStart = entry.leftStart
		rightStart = entry.rightStart
	}
	if d.IsUnified() {
		if line < 0 || line >= len(d.unifiedLines) {
			return diffSelPos{}, false, false
		}
		if localX < d.layoutLeftStart || localX >= d.layoutLeftStart+d.layoutLeftW || leftStart < 0 {
			return diffSelPos{}, false, false
		}
		displayLine := d.unifiedLines[line]
		visualCol := localX - d.layoutLeftStart
		if !d.IsWrapped() {
			visualCol += d.LeftCol
		}
		col := diffSegmentVisualColToRune(displayLine.side.Text, leftStart, visualCol)
		return diffSelPos{Line: line, Col: col}, displayLine.right, true
	}
	if line < 0 || line >= len(d.Lines) {
		return diffSelPos{}, false, false
	}

	if localX >= d.layoutLeftStart && localX < d.layoutLeftStart+d.layoutLeftW {
		if leftStart < 0 {
			return diffSelPos{}, false, false
		}
		visualCol := localX - d.layoutLeftStart
		if !d.IsWrapped() {
			visualCol += d.LeftCol
		}
		col := diffSegmentVisualColToRune(d.Lines[line].Left.Text, leftStart, visualCol)
		return diffSelPos{Line: line, Col: col}, false, true
	}
	if localX >= d.layoutRightStart && localX < d.layoutRightStart+d.layoutRightW {
		if rightStart < 0 {
			return diffSelPos{}, false, false
		}
		visualCol := localX - d.layoutRightStart
		if !d.IsWrapped() {
			visualCol += d.LeftCol
		}
		col := diffSegmentVisualColToRune(d.Lines[line].Right.Text, rightStart, visualCol)
		return diffSelPos{Line: line, Col: col}, true, true
	}
	return diffSelPos{}, false, false
}

func (d *DiffViewWidget) CopySelection() string {
	if !d.hasSelection {
		return ""
	}
	text := d.selection.Text(d.selectionLineCount(), d.selectionTextAt)
	if text != "" {
		d.ClearSelection()
	}
	return text
}

func (d *DiffViewWidget) ClearSelection() {
	d.hasSelection = false
	d.selecting = false
}

func (d *DiffViewWidget) selectionLineCount() int {
	if d.IsUnified() {
		return len(d.unifiedLines)
	}
	return len(d.Lines)
}

func (d *DiffViewWidget) selectionTextAt(line int) (string, bool) {
	if line < 0 || line >= d.selectionLineCount() {
		return "", false
	}
	if d.IsUnified() {
		return d.unifiedLines[line].side.Text, true
	}
	if d.selRight {
		return d.Lines[line].Right.Text, true
	}
	return d.Lines[line].Left.Text, true
}

func (d *DiffViewWidget) clampLeftCol() {
	if d.IsWrapped() {
		d.LeftCol = 0
		return
	}
	max := d.maxLineW - d.contentW
	if max < 0 {
		max = 0
	}
	if d.LeftCol > max {
		d.LeftCol = max
	}
	if d.LeftCol < 0 {
		d.LeftCol = 0
	}
}

func (d *DiffViewWidget) HandleEvent(ev tcell.Event) EventResult {
	if newTop, consumed := d.scrollbar.HandleEvent(ev); consumed {
		d.TopLine = newTop
		if d.scrollbar.IsDragging() {
			return EventCaptured
		}
		return EventConsumed
	}
	if !d.IsWrapped() {
		if newLeft, consumed := d.hscrollbar.HandleEvent(ev); consumed {
			d.LeftCol = newLeft
			if d.hscrollbar.IsDragging() {
				return EventCaptured
			}
			return EventConsumed
		}
		if !d.IsUnified() {
			if newLeft, consumed := d.rhscrollbar.HandleEvent(ev); consumed {
				d.LeftCol = newLeft
				if d.rhscrollbar.IsDragging() {
					return EventCaptured
				}
				return EventConsumed
			}
		}
	}

	switch tev := ev.(type) {
	case *tcell.EventKey:
		d.hasHoveredGap = false
		switch tev.Key() {
		case tcell.KeyUp:
			d.scrollVertical(-1)
			return EventConsumed
		case tcell.KeyDown:
			d.scrollVertical(1)
			return EventConsumed
		case tcell.KeyLeft:
			if d.LeftCol > 0 {
				d.LeftCol--
			}
			return EventConsumed
		case tcell.KeyRight:
			d.LeftCol++
			d.clampLeftCol()
			return EventConsumed
		case tcell.KeyPgUp:
			d.scrollVertical(-d.viewH)
			return EventConsumed
		case tcell.KeyPgDn:
			d.scrollVertical(d.viewH)
			return EventConsumed
		case tcell.KeyHome:
			d.TopLine = 0
			d.wrapTopOffset = 0
			d.LeftCol = 0
			return EventConsumed
		case tcell.KeyEnd:
			d.setTopVisualRow(d.totalVisualRows)
			return EventConsumed
		}
	case *tcell.EventMouse:
		btn := tev.Buttons()
		mod := tev.Modifiers()
		if btn&tcell.WheelUp != 0 {
			d.hasHoveredGap = false
			if mod&tcell.ModShift != 0 {
				d.LeftCol -= 4
				if d.LeftCol < 0 {
					d.LeftCol = 0
				}
			} else {
				d.scrollVertical(-3)
			}
			return EventConsumed
		}
		if btn&tcell.WheelDown != 0 {
			d.hasHoveredGap = false
			if mod&tcell.ModShift != 0 {
				d.LeftCol += 4
				d.clampLeftCol()
			} else {
				d.scrollVertical(3)
			}
			return EventConsumed
		}
		if btn&tcell.WheelLeft != 0 {
			d.hasHoveredGap = false
			d.LeftCol -= 4
			if d.LeftCol < 0 {
				d.LeftCol = 0
			}
			return EventConsumed
		}
		if btn&tcell.WheelRight != 0 {
			d.hasHoveredGap = false
			d.LeftCol += 4
			d.clampLeftCol()
			return EventConsumed
		}
		mx, my := tev.Position()
		hoveredGap, overGap := d.gapByLine[d.screenSourceLine(my)]
		hoverChanged := overGap != d.hasHoveredGap || (overGap && hoveredGap != d.hoveredGap)
		d.hasHoveredGap = overGap
		if overGap {
			d.hoveredGap = hoveredGap
		}
		if btn == tcell.ButtonNone {
			d.primaryPressed = false
		}
		if btn&tcell.Button1 != 0 {
			freshPress := !d.primaryPressed
			d.primaryPressed = true
			if freshPress {
				if gap, ok := d.gapByLine[d.screenSourceLine(my)]; ok {
					d.expandContextGap(gap)
					return EventConsumed
				}
			}
			pos, right, ok := d.screenToSel(mx, my)
			if ok {
				if !d.selecting {
					now := time.Now()
					isDoubleClick := now.Sub(d.lastClickTime) < DoubleClickMs*time.Millisecond &&
						pos.Line == d.lastClickPos.Line && pos.Col == d.lastClickPos.Col
					d.lastClickTime = now
					d.lastClickPos = pos
					d.selRight = right
					if isDoubleClick {
						if text, selectable := d.selectionTextAt(pos.Line); selectable {
							if d.selection.SelectWord(pos.Line, pos.Col, text) {
								d.hasSelection = true
							}
						}
						return EventConsumed
					}
					d.selecting = true
					d.hasSelection = true
					d.selection.Anchor = pos
					d.selection.Current = pos
				} else {
					d.selection.Current = pos
				}
				return EventCaptured
			}
		}
		if d.selecting && btn == tcell.ButtonNone {
			d.selecting = false
			start, end := d.selection.Range()
			if start.Line == end.Line && start.Col == end.Col {
				d.hasSelection = false
			}
		}
		if hoverChanged {
			return EventConsumed
		}
	}
	return EventIgnored
}
