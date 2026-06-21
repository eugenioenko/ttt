package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/highlight"
	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v2"
)

type diffMergedRef struct {
	isRight bool
	sideIdx int
}

type diffSelPos struct {
	Line int
	Col  int
}

type DiffViewWidget struct {
	BaseWidget
	FilePath    string
	Lines       []diff.DiffLine
	Highlighter *highlight.Highlighter
	TopLine     int
	LeftCol     int
	maxLineW    int
	viewH       int
	contentW    int
	scrollbar   Scrollbar
	hscrollbar  HScrollbar
	rhscrollbar HScrollbar

	// layout cache for mouse hit-testing
	layoutDividerX   int
	layoutLeftStart  int
	layoutLeftW      int
	layoutRightStart int
	layoutRightW     int
	layoutGutterW    int

	// selection state
	selecting     bool
	hasSelection  bool
	selRight      bool
	selAnchor     diffSelPos
	selCurrent    diffSelPos
	lastClickTime time.Time
	lastClickPos  diffSelPos

	SearchMatchesLeft   []FindMatch
	SearchMatchesRight  []FindMatch
	searchMergedRefs    []diffMergedRef
	searchActiveRight   bool
	searchActiveSideIdx int

	// extended diff mode
	extended bool
	fileDiff diff.FileDiff
	oldLines []string
	newLines []string

	OnFetchExtended func(dv *DiffViewWidget)
	Loading         bool

	// PR inline comments
	Comments    []github.PRComment
	displayRows []diffDisplayRow // flattened: diff lines interleaved with comment rows
}

type diffRowKind int

const (
	diffRowLine    diffRowKind = iota // a normal diff line
	diffRowComment                    // a comment block row
)

type diffDisplayRow struct {
	kind        diffRowKind
	lineIdx     int // index into d.Lines for diffRowLine
	comment     *github.PRComment
	commentLine int    // which line of the wrapped comment text this row represents
	commentText string // pre-computed text for this row
}

func NewDiffViewWidget(filePath string, fd diff.FileDiff, oldLines, newLines []string, extended bool) *DiffViewWidget {
	dv := &DiffViewWidget{
		FilePath:            filePath,
		Highlighter:         highlight.New(filePath),
		searchActiveSideIdx: -1,
		fileDiff:            fd,
		oldLines:            oldLines,
		newLines:            newLines,
		extended:            extended,
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
	return d.extended
}

func (d *DiffViewWidget) SetExtended(extended bool) {
	if extended && len(d.oldLines) == 0 && d.OnFetchExtended != nil {
		d.Loading = true
		d.extended = true
		d.OnFetchExtended(d)
		return
	}
	d.extended = extended
	d.rebuildLines()
	d.TopLine = 0
	d.LeftCol = 0
	d.ClearSearch()
	d.ClearSelection()
}

func (d *DiffViewWidget) FinishLoading() {
	d.Loading = false
	d.rebuildLines()
	d.TopLine = 0
	d.LeftCol = 0
	d.ClearSearch()
	d.ClearSelection()
}

func (d *DiffViewWidget) rebuildLines() {
	if d.extended && len(d.oldLines) > 0 {
		d.Lines = diff.FullDiffLines(d.oldLines, d.newLines)
	} else {
		d.Lines = d.fileDiff.AllLines()
	}
	maxW := 0
	for _, dl := range d.Lines {
		if lw := len([]rune(dl.Left.Text)); lw > maxW {
			maxW = lw
		}
		if rw := len([]rune(dl.Right.Text)); rw > maxW {
			maxW = rw
		}
	}
	d.maxLineW = maxW
	d.buildDisplayRows()
}

// SetComments sets the inline comments for this diff view and rebuilds display rows.
func (d *DiffViewWidget) SetComments(comments []github.PRComment) {
	d.Comments = comments
	d.buildDisplayRows()
}

// buildDisplayRows creates a flat list of display rows interleaving diff lines with comment blocks.
// Comments are placed after the diff line matching their line number on the right side.
func (d *DiffViewWidget) buildDisplayRows() {
	d.displayRows = nil
	if len(d.Comments) == 0 {
		// No comments - just add all diff lines directly
		for i := range d.Lines {
			d.displayRows = append(d.displayRows, diffDisplayRow{kind: diffRowLine, lineIdx: i})
		}
		return
	}

	// Build a map: right-side line number -> list of comments
	commentsByLine := make(map[int][]github.PRComment)
	for _, c := range d.Comments {
		if c.Line > 0 {
			commentsByLine[c.Line] = append(commentsByLine[c.Line], c)
		}
	}

	for i, dl := range d.Lines {
		d.displayRows = append(d.displayRows, diffDisplayRow{kind: diffRowLine, lineIdx: i})

		// Check if there are comments for this line (by right-side line number)
		rightNum := dl.Right.Num
		if comments, ok := commentsByLine[rightNum]; ok && rightNum > 0 {
			for ci := range comments {
				comment := &comments[ci]
				// Header row: @user - date
				header := fmt.Sprintf("  @%s  %s", comment.User, github.FormatCommentTime(comment.CreatedAt))
				d.displayRows = append(d.displayRows, diffDisplayRow{
					kind:        diffRowComment,
					lineIdx:     i,
					comment:     comment,
					commentLine: 0,
					commentText: header,
				})
				// Body rows: wrap comment body
				bodyLines := strings.Split(comment.Body, "\n")
				for li, bl := range bodyLines {
					d.displayRows = append(d.displayRows, diffDisplayRow{
						kind:        diffRowComment,
						lineIdx:     i,
						comment:     comment,
						commentLine: li + 1,
						commentText: "  " + bl,
					})
				}
			}
		}
	}
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
	// Map line index to display row index
	rowIdx := line
	if len(d.displayRows) > 0 {
		for i, row := range d.displayRows {
			if row.kind == diffRowLine && row.lineIdx >= line {
				rowIdx = i
				break
			}
		}
	}

	if d.viewH <= 0 {
		d.TopLine = rowIdx
		return
	}
	if rowIdx < d.TopLine || rowIdx >= d.TopLine+d.viewH {
		d.TopLine = rowIdx - d.viewH/2
		if d.TopLine < 0 {
			d.TopLine = 0
		}
		max := len(d.displayRows) - d.viewH
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
		entries = append(entries, entry{m, false, i})
	}
	for i, m := range right {
		entries = append(entries, entry{m, true, i})
	}
	sort.Slice(entries, func(i, j int) bool {
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

func (d *DiffViewWidget) Render(surface *RenderSurface) {
	w, h := surface.Size()
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

	totalRows := len(d.displayRows)
	showVScroll := totalRows > h
	contentW := (w-1)/2 - gutterW
	showHScroll := d.maxLineW > contentW

	if showHScroll {
		h--
	}
	if showVScroll {
		w--
	}

	d.viewH = h

	dividerX := (w - 1) / 2
	leftStart := gutterW
	leftW := dividerX - gutterW
	rightStart := dividerX + 1 + gutterW
	rightW := w - rightStart
	d.contentW = leftW

	d.layoutDividerX = dividerX
	d.layoutLeftStart = leftStart
	d.layoutLeftW = leftW
	d.layoutRightStart = rightStart
	d.layoutRightW = rightW
	d.layoutGutterW = gutterW

	if leftW < 1 || rightW < 1 {
		return
	}

	for y := 0; y < h; y++ {
		rowIdx := d.TopLine + y
		surface.SetCell(dividerX, y, term.Cell{Ch: '│', Style: term.StyleBorder})

		if rowIdx >= totalRows {
			for x := 0; x < dividerX; x++ {
				surface.SetCell(x, y, term.Cell{Ch: ' '})
			}
			for x := dividerX + 1; x < w; x++ {
				surface.SetCell(x, y, term.Cell{Ch: ' '})
			}
			continue
		}

		row := d.displayRows[rowIdx]

		if row.kind == diffRowComment {
			d.renderCommentRow(surface, y, w, dividerX, gutterW, row)
			continue
		}

		idx := row.lineIdx
		if idx >= len(d.Lines) {
			continue
		}

		dl := d.Lines[idx]

		leftStyle := kindToStyle(dl.Left.Kind)
		rightStyle := kindToStyle(dl.Right.Kind)

		d.renderGutter(surface, 0, y, gutterW, dl.Left, leftStyle)
		d.renderGutter(surface, dividerX+1, y, gutterW, dl.Right, rightStyle)

		var leftSpans, rightSpans []highlight.Span
		if d.Highlighter != nil {
			if dl.Left.Text != "" {
				leftSpans = d.Highlighter.HighlightLine(dl.Left.Text)
			}
			if dl.Right.Text != "" {
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
		d.renderSide(surface, leftStart, y, leftW, dl.Left.Text, leftStyle, leftSpans, idx, d.SearchMatchesLeft, leftActive, !d.selRight)
		d.renderSide(surface, rightStart, y, rightW, dl.Right.Text, rightStyle, rightSpans, idx, d.SearchMatchesRight, rightActive, d.selRight)
	}

	if showVScroll {
		d.scrollbar.X = r.X + w
		d.scrollbar.Y = r.Y
		d.scrollbar.Height = h
		d.scrollbar.TotalItems = totalRows
		d.scrollbar.TopItem = d.TopLine
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

func (d *DiffViewWidget) renderGutter(surface *RenderSurface, x, y, w int, sl diff.SideLine, style term.Style) {
	num := ""
	if sl.Num > 0 {
		num = fmt.Sprintf("%d", sl.Num)
	}
	padded := " " + fmt.Sprintf("%*s", w-3, num) + "  "
	for i, ch := range []rune(padded) {
		if i >= w {
			break
		}
		surface.SetCell(x+i, y, term.Cell{Ch: ch, Style: term.StyleLineNumber})
	}
}

func (d *DiffViewWidget) renderCommentRow(surface *RenderSurface, y, totalW, dividerX, gutterW int, row diffDisplayRow) {
	// Fill background with comment style across the full width
	bgStyle := term.StyleCommentBg
	for x := 0; x < totalW; x++ {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: bgStyle})
	}

	// Render comment text spanning the full width (no gutter/divider for comment rows)
	text := row.commentText
	textStyle := bgStyle
	if row.commentLine == 0 {
		// Header row: use author style
		textStyle = term.StyleCommentAuthor
	}

	runes := []rune(text)
	for i, ch := range runes {
		if i >= totalW {
			break
		}
		style := textStyle
		if row.commentLine == 0 && i >= 2 {
			// After the initial "  @user" part, switch to muted style for the date
			userEnd := 2 // "  "
			user := "@" + row.comment.User
			userEnd += len([]rune(user))
			if i >= userEnd {
				style = term.StyleMuted
				// Override bg to match comment bg
				surface.SetCell(i, y, term.Cell{Ch: ch, Style: style, BgStyle: bgStyle})
				continue
			}
		}
		surface.SetCell(i, y, term.Cell{Ch: ch, Style: style, BgStyle: bgStyle})
	}
}

func (d *DiffViewWidget) renderSide(surface *RenderSurface, x, y, w int, text string, baseStyle term.Style, spans []highlight.Span, lineIdx int, matches []FindMatch, activeIdx int, selSide bool) {
	runes := []rune(text)
	for i := 0; i < w; i++ {
		colIdx := d.LeftCol + i
		ch := ' '
		if colIdx < len(runes) {
			ch = runes[colIdx]
		}
		style := term.StyleDefault
		for _, sp := range spans {
			if colIdx >= sp.Start && colIdx < sp.End {
				style = sp.Style
				break
			}
		}
		for mi, m := range matches {
			if m.Line == lineIdx && colIdx >= m.Col && colIdx < m.Col+m.Len {
				if mi == activeIdx {
					style = term.StyleSearchActive
				} else {
					style = term.StyleSearchMatch
				}
				break
			}
		}
		cell := term.Cell{Ch: ch, Style: style}
		if selSide && d.isCellSelected(lineIdx, colIdx) {
			cell.BgStyle = term.StyleSelection
		} else if style != term.StyleSearchMatch && style != term.StyleSearchActive && baseStyle != term.StyleDefault {
			cell.BgStyle = baseStyle
		}
		surface.SetCell(x+i, y, cell)
	}
}

func kindToStyle(k diff.LineKind) term.Style {
	switch k {
	case diff.Added:
		return term.StyleDiffAdded
	case diff.Deleted:
		return term.StyleDiffDeleted
	default:
		return term.StyleDefault
	}
}

func (d *DiffViewWidget) selectionRange() (start, end diffSelPos) {
	a, b := d.selAnchor, d.selCurrent
	if a.Line < b.Line || (a.Line == b.Line && a.Col <= b.Col) {
		return a, b
	}
	return b, a
}

func (d *DiffViewWidget) isCellSelected(line, col int) bool {
	if !d.hasSelection {
		return false
	}
	start, end := d.selectionRange()
	if line < start.Line || line > end.Line {
		return false
	}
	if start.Line == end.Line {
		return col >= start.Col && col < end.Col
	}
	if line == start.Line {
		return col >= start.Col
	}
	if line == end.Line {
		return col < end.Col
	}
	return true
}

func (d *DiffViewWidget) screenToSel(mx, my int) (pos diffSelPos, right bool, ok bool) {
	r := d.GetRect()
	localX := mx - r.X
	localY := my - r.Y
	rowIdx := d.TopLine + localY

	// Check if this row is a comment row (not selectable)
	if rowIdx >= 0 && rowIdx < len(d.displayRows) && d.displayRows[rowIdx].kind == diffRowComment {
		return diffSelPos{}, false, false
	}

	// Map display row index back to diff line index
	line := rowIdx
	if rowIdx >= 0 && rowIdx < len(d.displayRows) {
		line = d.displayRows[rowIdx].lineIdx
	}

	if localX >= d.layoutLeftStart && localX < d.layoutLeftStart+d.layoutLeftW {
		col := d.LeftCol + (localX - d.layoutLeftStart)
		return diffSelPos{Line: line, Col: col}, false, true
	}
	if localX >= d.layoutRightStart && localX < d.layoutRightStart+d.layoutRightW {
		col := d.LeftCol + (localX - d.layoutRightStart)
		return diffSelPos{Line: line, Col: col}, true, true
	}
	return diffSelPos{}, false, false
}

func (d *DiffViewWidget) CopySelection() string {
	text := d.selectedText()
	if text != "" {
		d.ClearSelection()
	}
	return text
}

func (d *DiffViewWidget) ClearSelection() {
	d.hasSelection = false
	d.selecting = false
}

func (d *DiffViewWidget) selectedText() string {
	if !d.hasSelection {
		return ""
	}
	start, end := d.selectionRange()
	var lines []string
	for i := start.Line; i <= end.Line && i < len(d.Lines); i++ {
		var text string
		if d.selRight {
			text = d.Lines[i].Right.Text
		} else {
			text = d.Lines[i].Left.Text
		}
		runes := []rune(text)
		startCol := 0
		endCol := len(runes)
		if i == start.Line {
			startCol = start.Col
		}
		if i == end.Line {
			endCol = end.Col
		}
		if startCol > len(runes) {
			startCol = len(runes)
		}
		if endCol > len(runes) {
			endCol = len(runes)
		}
		if startCol < endCol {
			lines = append(lines, string(runes[startCol:endCol]))
		} else {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

func (d *DiffViewWidget) selectWord(line, col int) {
	if line < 0 || line >= len(d.Lines) {
		return
	}
	var text string
	if d.selRight {
		text = d.Lines[line].Right.Text
	} else {
		text = d.Lines[line].Left.Text
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return
	}
	if col >= len(runes) {
		col = len(runes) - 1
	}
	isWord := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
	}
	start, end := col, col
	if isWord(runes[col]) {
		for start > 0 && isWord(runes[start-1]) {
			start--
		}
		for end < len(runes)-1 && isWord(runes[end+1]) {
			end++
		}
	}
	end++
	d.hasSelection = true
	d.selAnchor = diffSelPos{Line: line, Col: start}
	d.selCurrent = diffSelPos{Line: line, Col: end}
}

func (d *DiffViewWidget) clampLeftCol() {
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
	if newLeft, consumed := d.hscrollbar.HandleEvent(ev); consumed {
		d.LeftCol = newLeft
		if d.hscrollbar.IsDragging() {
			return EventCaptured
		}
		return EventConsumed
	}
	if newLeft, consumed := d.rhscrollbar.HandleEvent(ev); consumed {
		d.LeftCol = newLeft
		if d.rhscrollbar.IsDragging() {
			return EventCaptured
		}
		return EventConsumed
	}

	switch tev := ev.(type) {
	case *tcell.EventKey:
		switch tev.Key() {
		case tcell.KeyUp:
			if d.TopLine > 0 {
				d.TopLine--
			}
			return EventConsumed
		case tcell.KeyDown:
			max := len(d.displayRows) - d.viewH
			if max < 0 {
				max = 0
			}
			if d.TopLine < max {
				d.TopLine++
			}
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
			d.TopLine -= d.viewH
			if d.TopLine < 0 {
				d.TopLine = 0
			}
			return EventConsumed
		case tcell.KeyPgDn:
			max := len(d.displayRows) - d.viewH
			if max < 0 {
				max = 0
			}
			d.TopLine += d.viewH
			if d.TopLine > max {
				d.TopLine = max
			}
			return EventConsumed
		case tcell.KeyHome:
			d.TopLine = 0
			d.LeftCol = 0
			return EventConsumed
		case tcell.KeyEnd:
			max := len(d.displayRows) - d.viewH
			if max < 0 {
				max = 0
			}
			d.TopLine = max
			return EventConsumed
		}
	case *tcell.EventMouse:
		btn := tev.Buttons()
		mod := tev.Modifiers()
		if btn&tcell.WheelUp != 0 {
			if mod&tcell.ModShift != 0 {
				d.LeftCol -= 4
				if d.LeftCol < 0 {
					d.LeftCol = 0
				}
			} else {
				d.TopLine -= 3
				if d.TopLine < 0 {
					d.TopLine = 0
				}
			}
			return EventConsumed
		}
		if btn&tcell.WheelDown != 0 {
			if mod&tcell.ModShift != 0 {
				d.LeftCol += 4
				d.clampLeftCol()
			} else {
				max := len(d.displayRows) - d.viewH
				if max < 0 {
					max = 0
				}
				d.TopLine += 3
				if d.TopLine > max {
					d.TopLine = max
				}
			}
			return EventConsumed
		}
		if btn&tcell.WheelLeft != 0 {
			d.LeftCol -= 4
			if d.LeftCol < 0 {
				d.LeftCol = 0
			}
			return EventConsumed
		}
		if btn&tcell.WheelRight != 0 {
			d.LeftCol += 4
			d.clampLeftCol()
			return EventConsumed
		}
		mx, my := tev.Position()
		if btn&tcell.Button1 != 0 {
			pos, right, ok := d.screenToSel(mx, my)
			if ok {
				if !d.selecting {
					now := time.Now()
					isDoubleClick := now.Sub(d.lastClickTime) < 400*time.Millisecond &&
						pos.Line == d.lastClickPos.Line && pos.Col == d.lastClickPos.Col
					d.lastClickTime = now
					d.lastClickPos = pos
					d.selRight = right
					if isDoubleClick {
						d.selectWord(pos.Line, pos.Col)
						return EventConsumed
					}
					d.selecting = true
					d.hasSelection = true
					d.selAnchor = pos
					d.selCurrent = pos
				} else {
					d.selCurrent = pos
				}
				return EventCaptured
			}
		}
		if d.selecting && btn == tcell.ButtonNone {
			d.selecting = false
			start, end := d.selectionRange()
			if start.Line == end.Line && start.Col == end.Col {
				d.hasSelection = false
			}
		}
	}
	return EventIgnored
}
