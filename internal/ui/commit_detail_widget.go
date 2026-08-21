package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

// CommitDetailFile is one file section in a commit detail view. Files remain in
// the order Git reports them; a failed or hunk-less diff still gets a heading
// and an explanatory row so a touched path never silently disappears.
type CommitDetailFile struct {
	Path         string
	OldPath      string
	Heading      string
	HeadingStyle term.Style
	Diff         diff.FileDiff
	Error        string

	highlighter    *highlight.Highlighter
	lines          []diff.DiffLine
	unified        []diffUnifiedLine
	oldLines       []string
	newLines       []string
	contextLoaded  bool
	contextLoading bool
	expandedGaps   map[int]bool
	gapByLine      map[int]int
	pendingGap     int
}

type commitDetailRowKind uint8

const (
	commitDetailMessageHeaderRow commitDetailRowKind = iota
	commitDetailMetadataRow
	commitDetailMessageRow
	commitDetailSpacerRow
	commitDetailHeadingRow
	commitDetailNoticeRow
	commitDetailDiffRow
)

type commitDetailRow struct {
	kind      commitDetailRowKind
	text      string
	bold      bool
	fileIndex int
	lineIndex int
}

type commitDetailVisualRow struct {
	row          int
	leftStart    int
	rightStart   int
	continuation bool
}

type commitDetailControl struct {
	rect      Rect
	fileIndex int
}

// CommitDetailWidget renders an entire commit as one virtualized scrollable
// document. Unlike stacking several DiffViewWidgets, it owns one vertical
// viewport and only draws visible rows, so a large commit does not allocate a
// full-screen cell grid for every changed line on every redraw.
type CommitDetailWidget struct {
	BaseWidget
	Dir             string
	Ref             string
	Short           string
	Loading         bool
	Error           string
	Header          string
	EmptyText       string
	LoadingText     string
	CurrentChanges  bool
	HideEmptyNotice bool
	LoadGen         uint64

	Message  string
	Metadata string
	Files    []CommitDetailFile

	SyntaxHighlight bool
	TopLine         int
	LeftCol         int
	wrapMode        DiffWrapMode
	wrapExplicit    bool
	mode            DiffMode
	modeExplicit    bool
	contextMode     DiffContextMode
	contextExplicit bool
	highContrast    bool
	collapsedFiles  []bool

	rows            []commitDetailRow
	visualRows      []commitDetailVisualRow
	visualRowsW     int
	totalVisualRows int
	maxLineW        int
	gutterW         int
	viewH           int
	contentW        int
	scrollbar       Scrollbar
	hscrollbar      HScrollbar
	rhscroll        HScrollbar

	// Render owns these layout values. Mouse hit-testing and sticky-heading
	// controls reuse them rather than trying to reconstruct the viewport.
	layoutViewW      int
	layoutLeftStart  int
	layoutLeftW      int
	layoutRightStart int
	layoutRightW     int
	fileControls     []commitDetailControl
	topControl       Rect
	stickyRect       Rect
	stickyControl    commitDetailControl

	// Selection positions point into logical rows and original, unwrapped text.
	selecting         bool
	hasSelection      bool
	selRight          bool
	selection         diffTextSelection
	lastClickTime     time.Time
	lastClickPos      diffSelPos
	primaryPressed    bool
	disclosurePressed bool

	OnFetchContext func(fileIndex int, file CommitDetailFile)
}

func NewCommitDetailWidget(dir, ref, short string, syntaxHighlight bool) *CommitDetailWidget {
	return &CommitDetailWidget{
		Dir:             dir,
		Ref:             ref,
		Short:           short,
		Loading:         true,
		Header:          "Commit message",
		EmptyText:       "No files",
		LoadingText:     fmt.Sprintf("Loading commit %s…", short),
		SyntaxHighlight: syntaxHighlight,
	}
}

func NewCurrentChangesWidget(dir string, syntaxHighlight bool) *CommitDetailWidget {
	return &CommitDetailWidget{
		Dir:             dir,
		Ref:             "working-tree",
		Loading:         true,
		Header:          "Current changes",
		EmptyText:       "No changes",
		LoadingText:     "Loading current changes…",
		CurrentChanges:  true,
		HideEmptyNotice: true,
		SyntaxHighlight: syntaxHighlight,
	}
}

func (d *CommitDetailWidget) Focusable() bool { return true }

func (d *CommitDetailWidget) SetDiffHighContrast(enabled bool) { d.highContrast = enabled }

func (d *CommitDetailWidget) DiffHighContrast() bool { return d.highContrast }

func (d *CommitDetailWidget) ContextMode() DiffContextMode { return d.contextMode }

func (d *CommitDetailWidget) SetContextMode(mode DiffContextMode) {
	d.contextExplicit = true
	d.applyContextMode(mode)
}

func (d *CommitDetailWidget) ApplyDefaultContextMode(mode DiffContextMode) {
	if d.contextExplicit {
		return
	}
	d.applyContextMode(mode)
}

func (d *CommitDetailWidget) applyContextMode(mode DiffContextMode) {
	if d.contextMode == mode {
		return
	}
	d.contextMode = mode
	if mode == DiffContextChangesOnly {
		for i := range d.Files {
			clear(d.Files[i].expandedGaps)
			d.Files[i].pendingGap = -1
		}
	}
	d.TopLine = 0
	d.LeftCol = 0
	d.ClearSelection()
	d.rebuildRows()
	if mode == DiffContextFullFile {
		for i := range d.Files {
			d.requestFileContext(i, -1)
		}
	}
}

func (d *CommitDetailWidget) IsWrapped() bool { return d.wrapMode == DiffWrapOn }

func (d *CommitDetailWidget) SetWrapped(wrapped bool) {
	mode := DiffWrapOff
	if wrapped {
		mode = DiffWrapOn
	}
	d.SetWrapMode(mode)
}

func (d *CommitDetailWidget) WrapMode() DiffWrapMode { return d.wrapMode }

func (d *CommitDetailWidget) SetWrapMode(mode DiffWrapMode) {
	d.wrapExplicit = true
	d.applyWrapMode(mode)
}

func (d *CommitDetailWidget) ApplyDefaultWrapMode(mode DiffWrapMode) {
	if d.wrapExplicit {
		return
	}
	d.applyWrapMode(mode)
}

func (d *CommitDetailWidget) applyWrapMode(mode DiffWrapMode) {
	if mode == d.wrapMode {
		return
	}
	d.wrapMode = mode
	d.TopLine = 0
	d.LeftCol = 0
	d.visualRowsW = -1
	d.ClearSelection()
}

func (d *CommitDetailWidget) Mode() DiffMode { return d.mode }

func (d *CommitDetailWidget) SetMode(mode DiffMode) {
	d.modeExplicit = true
	d.applyMode(mode)
}

func (d *CommitDetailWidget) ApplyDefaultMode(mode DiffMode) {
	if d.modeExplicit {
		return
	}
	d.applyMode(mode)
}

func (d *CommitDetailWidget) applyMode(mode DiffMode) {
	if mode == d.mode {
		return
	}
	d.mode = mode
	d.TopLine = 0
	d.LeftCol = 0
	d.ClearSelection()
	d.rebuildRows()
}

func (d *CommitDetailWidget) SetDetail(message string, files []CommitDetailFile, errText string) {
	collapsed := make(map[string]bool)
	topLine := 0
	if d.CurrentChanges {
		topLine = d.TopLine
		for i, file := range d.Files {
			if i < len(d.collapsedFiles) && d.collapsedFiles[i] {
				collapsed[commitDetailFileKey(file)] = true
			}
		}
	}
	d.Loading = false
	d.Error = errText
	d.Message = message
	d.Files = files
	d.collapsedFiles = make([]bool, len(files))
	for i := range d.Files {
		d.Files[i].expandedGaps = make(map[int]bool)
		d.Files[i].pendingGap = -1
		d.collapsedFiles[i] = collapsed[commitDetailFileKey(d.Files[i])]
		if d.SyntaxHighlight && d.Files[i].Path != "" {
			d.Files[i].highlighter = highlight.New(d.Files[i].Path)
		}
	}
	d.TopLine = topLine
	d.LeftCol = 0
	d.ClearSelection()
	d.rebuildRows()
	if d.contextMode == DiffContextFullFile {
		for i := range d.Files {
			d.requestFileContext(i, -1)
		}
	}
}

func (d *CommitDetailWidget) requestFileContext(fileIndex, gap int) {
	if fileIndex < 0 || fileIndex >= len(d.Files) {
		return
	}
	file := &d.Files[fileIndex]
	if file.contextLoaded {
		if gap >= 0 {
			file.expandedGaps[gap] = true
			d.rebuildRows()
			d.ClearSelection()
		}
		return
	}
	if gap >= 0 {
		file.pendingGap = gap
	}
	if file.contextLoading || d.OnFetchContext == nil {
		return
	}
	file.contextLoading = true
	d.OnFetchContext(fileIndex, *file)
}

func (d *CommitDetailWidget) ApplyFileContext(fileIndex int, key string, oldLines, newLines []string) bool {
	if fileIndex < 0 || fileIndex >= len(d.Files) || commitDetailFileKey(d.Files[fileIndex]) != key {
		return false
	}
	file := &d.Files[fileIndex]
	file.oldLines = oldLines
	file.newLines = newLines
	file.contextLoaded = true
	file.contextLoading = false
	if file.pendingGap >= 0 {
		file.expandedGaps[file.pendingGap] = true
		file.pendingGap = -1
	}
	d.rebuildRows()
	d.ClearSelection()
	return true
}

func CommitDetailContextKey(file CommitDetailFile) string { return commitDetailFileKey(file) }

func (d *CommitDetailWidget) allFilesCollapsed() bool {
	if len(d.Files) == 0 {
		return false
	}
	for fileIndex := range d.Files {
		if fileIndex >= len(d.collapsedFiles) || !d.collapsedFiles[fileIndex] {
			return false
		}
	}
	return true
}

func (d *CommitDetailWidget) CollapseAllFiles() {
	d.setAllFilesCollapsed(true)
}

func (d *CommitDetailWidget) ExpandAllFiles() {
	d.setAllFilesCollapsed(false)
}

func (d *CommitDetailWidget) setAllFilesCollapsed(collapsed bool) {
	if len(d.collapsedFiles) != len(d.Files) {
		d.collapsedFiles = make([]bool, len(d.Files))
	}
	for fileIndex := range d.collapsedFiles {
		d.collapsedFiles[fileIndex] = collapsed
	}
	d.afterCollapseChange()
}

func (d *CommitDetailWidget) toggleFile(fileIndex int) {
	if fileIndex < 0 || fileIndex >= len(d.Files) {
		return
	}
	if len(d.collapsedFiles) != len(d.Files) {
		d.collapsedFiles = make([]bool, len(d.Files))
	}
	d.collapsedFiles[fileIndex] = !d.collapsedFiles[fileIndex]
	d.afterCollapseChange()
}

func (d *CommitDetailWidget) afterCollapseChange() {
	d.ClearSelection()
	d.rebuildRows()
	d.clampScroll()
}

func (d *CommitDetailWidget) rebuildRows() {
	d.rows = nil
	d.visualRows = nil
	d.visualRowsW = -1
	d.maxLineW = 0
	d.gutterW = 4
	if d.Error != "" {
		return
	}

	header := d.Header
	if header == "" {
		header = "Commit message"
	}
	d.rows = append(d.rows, commitDetailRow{kind: commitDetailMessageHeaderRow, text: header, bold: true})
	if d.Metadata != "" {
		d.rows = append(d.rows, commitDetailRow{kind: commitDetailMetadataRow, text: d.Metadata})
		d.recordWidth(d.Metadata)
	}
	message := strings.TrimRight(d.Message, "\r\n")
	if message == "" {
		message = "(No commit message)"
		if d.CurrentChanges {
			message = "Working tree clean"
		}
	}
	for i, line := range strings.Split(message, "\n") {
		d.rows = append(d.rows, commitDetailRow{kind: commitDetailMessageRow, text: line, bold: i == 0})
		d.recordWidth(line)
	}
	d.rows = append(d.rows, commitDetailRow{kind: commitDetailSpacerRow})

	maxLine := 0
	for fileIndex := range d.Files {
		file := &d.Files[fileIndex]
		if d.contextMode == DiffContextFullFile && file.contextLoaded {
			file.lines = diff.FullDiffLines(file.oldLines, file.newLines)
			file.gapByLine = nil
		} else {
			file.lines, file.gapByLine = compactDiffLinesWithContext(file.Diff, file.oldLines, file.newLines, file.expandedGaps)
		}
		file.unified = buildUnifiedDiffLines(file.lines)
		if fileIndex > 0 {
			d.rows = append(d.rows, commitDetailRow{kind: commitDetailSpacerRow})
		}
		heading := commitDetailFileHeading(*file)
		d.rows = append(d.rows, commitDetailRow{kind: commitDetailHeadingRow, text: heading, bold: true, fileIndex: fileIndex})
		d.recordWidth(heading)
		if fileIndex < len(d.collapsedFiles) && d.collapsedFiles[fileIndex] {
			continue
		}

		switch {
		case file.Error != "":
			d.rows = append(d.rows, commitDetailRow{kind: commitDetailNoticeRow, text: file.Error, fileIndex: fileIndex})
			d.recordWidth(file.Error)
		default:
			if len(file.lines) == 0 {
				const noChanges = "No line changes"
				d.rows = append(d.rows, commitDetailRow{kind: commitDetailNoticeRow, text: noChanges, fileIndex: fileIndex})
				d.recordWidth(noChanges)
				continue
			}
			lineCount := len(file.lines)
			if d.mode == DiffModeUnified {
				lineCount = len(file.unified)
			}
			for lineIndex := 0; lineIndex < lineCount; lineIndex++ {
				d.rows = append(d.rows, commitDetailRow{kind: commitDetailDiffRow, fileIndex: fileIndex, lineIndex: lineIndex})
				if d.mode == DiffModeUnified {
					line := file.unified[lineIndex].side
					d.recordWidth(line.Text)
					if line.Num > maxLine {
						maxLine = line.Num
					}
				} else {
					line := file.lines[lineIndex]
					d.recordWidth(line.Left.Text)
					d.recordWidth(line.Right.Text)
					if line.Left.Num > maxLine {
						maxLine = line.Left.Num
					}
					if line.Right.Num > maxLine {
						maxLine = line.Right.Num
					}
				}
			}
		}
	}
	if len(d.Files) == 0 && !d.HideEmptyNotice {
		emptyText := d.EmptyText
		if emptyText == "" {
			emptyText = "No files"
		}
		d.rows = append(d.rows, commitDetailRow{kind: commitDetailNoticeRow, text: emptyText})
		d.recordWidth(emptyText)
	}
	if maxLine > 0 {
		d.gutterW = textwidth.String(strconv.Itoa(maxLine)) + 3
	}
	d.totalVisualRows = len(d.rows)
}

func (d *CommitDetailWidget) recordWidth(text string) {
	if width := diffLineVisualWidth(text); width > d.maxLineW {
		d.maxLineW = width
	}
}

func commitDetailFileHeading(file CommitDetailFile) string {
	if file.Heading != "" {
		return file.Heading
	}
	if file.OldPath != "" && file.OldPath != file.Path {
		return fmt.Sprintf("%s → %s", file.OldPath, file.Path)
	}
	return file.Path
}

func commitDetailFileKey(file CommitDetailFile) string {
	return file.OldPath + "\x00" + file.Path
}

func (d *CommitDetailWidget) Render(surface Surface) {
	w, h := surface.Size()
	r := d.GetRect()
	if w <= 0 || h <= 0 {
		return
	}
	surface.Fill(term.Cell{Ch: ' ', Style: term.StyleDefault})

	if d.Loading {
		loadingText := d.LoadingText
		if loadingText == "" {
			loadingText = fmt.Sprintf("Loading commit %s…", d.Short)
		}
		surface.DrawText(0, 0, loadingText, w, term.StyleMuted)
		return
	}
	if d.Error != "" {
		surface.DrawText(0, 0, d.Error, w, term.StyleDanger)
		return
	}

	viewW, viewH, showV, showH := d.layout(w, h)
	d.viewH = viewH
	leftStart, leftW, rightStart, rightW := d.sideGeometry(viewW)
	d.layoutViewW = viewW
	d.layoutLeftStart = leftStart
	d.layoutLeftW = leftW
	d.layoutRightStart = rightStart
	d.layoutRightW = rightW
	d.fileControls = nil
	d.topControl = Rect{}
	d.stickyRect = Rect{}
	d.stickyControl = commitDetailControl{fileIndex: -1}
	d.contentW = leftW
	if d.mode == DiffModeSplit {
		d.contentW = min(leftW, rightW)
	}
	d.clampScroll()

	for screenY := 0; screenY < viewH; screenY++ {
		rowIndex := d.TopLine + screenY
		visual := commitDetailVisualRow{row: rowIndex, leftStart: 0, rightStart: 0}
		if d.IsWrapped() {
			if rowIndex >= len(d.visualRows) {
				break
			}
			visual = d.visualRows[rowIndex]
			rowIndex = visual.row
		} else if rowIndex >= len(d.rows) {
			break
		}
		d.renderRow(surface, rowIndex, d.rows[rowIndex], visual, screenY, viewW)
	}
	d.renderStickyHeading(surface, viewW)

	if showV {
		d.scrollbar.X = r.X + viewW
		d.scrollbar.Y = r.Y
		d.scrollbar.Height = viewH
		d.scrollbar.TotalItems = d.totalVisualRows
		d.scrollbar.TopItem = d.TopLine
		d.scrollbar.Render(surface, viewW, 0)
	} else {
		d.scrollbar.TotalItems = 0
	}
	if showH && viewH < h {
		d.renderHorizontalScrollbars(surface, r, viewW, viewH)
	} else {
		d.hscrollbar.TotalCols = 0
		d.rhscroll.TotalCols = 0
	}
}

func (d *CommitDetailWidget) layout(w, h int) (viewW, viewH int, showV, showH bool) {
	viewH = h
	if d.IsWrapped() {
		showV = d.totalVisualRows > viewH
	}
	for range 3 {
		viewW = w
		if showV {
			viewW--
		}
		if d.IsWrapped() {
			if d.visualRowsW != viewW {
				d.visualRows = d.buildVisualRows(viewW)
				d.visualRowsW = viewW
			}
			d.totalVisualRows = len(d.visualRows)
			showH = false
		} else {
			d.visualRows = nil
			d.totalVisualRows = len(d.rows)
			_, leftW, _, rightW := d.sideGeometry(viewW)
			sideW := leftW
			if d.mode == DiffModeSplit {
				sideW = min(leftW, rightW)
			}
			showH = sideW > 0 && d.maxLineW > sideW
		}
		newViewH := h
		if showH {
			newViewH--
		}
		newShowV := d.totalVisualRows > newViewH
		if newViewH == viewH && newShowV == showV {
			break
		}
		viewH = newViewH
		showV = newShowV
	}
	if viewW < 0 {
		viewW = 0
	}
	if viewH < 0 {
		viewH = 0
	}
	return
}

func (d *CommitDetailWidget) buildVisualRows(viewW int) []commitDetailVisualRow {
	if viewW <= 0 {
		return nil
	}
	_, leftW, _, rightW := d.sideGeometry(viewW)
	visualRows := make([]commitDetailVisualRow, 0, len(d.rows))
	for rowIndex, row := range d.rows {
		leftStarts := []int{0}
		rightStarts := []int{0}
		switch row.kind {
		case commitDetailMessageHeaderRow, commitDetailSpacerRow:
			rightStarts = nil
		case commitDetailHeadingRow:
			leftStarts = diffWrapStarts(row.text, viewW-3)
			rightStarts = nil
		case commitDetailDiffRow:
			if row.fileIndex >= 0 && row.fileIndex < len(d.Files) && row.lineIndex >= 0 && leftW > 0 {
				file := &d.Files[row.fileIndex]
				if d.mode == DiffModeUnified && row.lineIndex < len(file.unified) {
					leftStarts = diffWrapStarts(file.unified[row.lineIndex].side.Text, leftW)
					rightStarts = nil
				} else if d.mode == DiffModeSplit && row.lineIndex < len(file.lines) && rightW > 0 {
					line := file.lines[row.lineIndex]
					leftStarts = diffWrapStarts(line.Left.Text, leftW)
					rightStarts = diffWrapStarts(line.Right.Text, rightW)
				}
			}
		default:
			leftStarts = diffWrapStarts(row.text, viewW)
			rightStarts = nil
		}

		rowCount := len(leftStarts)
		if len(rightStarts) > rowCount {
			rowCount = len(rightStarts)
		}
		for segment := 0; segment < rowCount; segment++ {
			leftStart, rightStart := -1, -1
			if segment < len(leftStarts) {
				leftStart = leftStarts[segment]
			}
			if segment < len(rightStarts) {
				rightStart = rightStarts[segment]
			}
			visualRows = append(visualRows, commitDetailVisualRow{
				row:          rowIndex,
				leftStart:    leftStart,
				rightStart:   rightStart,
				continuation: segment > 0,
			})
		}
	}
	return visualRows
}

// sideGeometry returns the content widths used for every diff row. The parent
// owns these layout values; row renderers and scrollbar placement reuse them.
func (d *CommitDetailWidget) sideGeometry(viewW int) (leftStart, leftW, rightStart, rightW int) {
	if d.mode == DiffModeUnified {
		return d.gutterW, viewW - d.gutterW, 0, 0
	}
	dividerX := (viewW - 1) / 2
	leftStart = d.gutterW
	leftW = dividerX - d.gutterW
	rightStart = dividerX + 1 + d.gutterW
	rightW = viewW - rightStart
	return
}

func (d *CommitDetailWidget) renderRow(surface Surface, rowIndex int, row commitDetailRow, visual commitDetailVisualRow, y, viewW int) {
	switch row.kind {
	case commitDetailMessageHeaderRow:
		d.renderMessageHeader(surface, y, viewW)
	case commitDetailMetadataRow:
		d.drawTextRow(surface, 0, y, viewW, row.text, term.StyleMuted, term.StyleCommitMessage, false, visual.leftStart, rowIndex)
	case commitDetailSpacerRow:
		return
	case commitDetailMessageRow:
		d.drawTextRow(surface, 0, y, viewW, row.text, term.StyleCommitMessage, term.StyleCommitMessage, row.bold, visual.leftStart, rowIndex)
	case commitDetailHeadingRow:
		d.renderHeading(surface, rowIndex, row, visual, y, viewW)
	case commitDetailNoticeRow:
		style := term.StyleMuted
		if row.fileIndex >= 0 && row.fileIndex < len(d.Files) && d.Files[row.fileIndex].Error != "" {
			style = term.StyleDanger
		}
		d.drawTextRow(surface, 0, y, viewW, row.text, style, term.StyleDefault, false, visual.leftStart, rowIndex)
	case commitDetailDiffRow:
		d.renderDiffRow(surface, rowIndex, row, visual, y, viewW)
	}
}

func (d *CommitDetailWidget) renderMessageHeader(surface Surface, y, viewW int) {
	for column := 0; column < viewW; column++ {
		surface.SetCell(column, y, term.Cell{Ch: ' ', Style: term.StyleCommitMessage})
	}
	header := d.Header
	if header == "" {
		header = "Commit message"
	}
	d.drawStaticText(surface, 0, y, viewW, header, term.StyleCommitMessage, term.StyleCommitMessage, true)
	if len(d.Files) == 0 {
		return
	}
	label := "Collapse all"
	if d.allFilesCollapsed() {
		label = "Expand all"
	}
	controlW := textwidth.String(label)
	controlX := viewW - controlW
	if controlX <= textwidth.String(header) {
		return
	}
	d.drawStaticText(surface, controlX, y, controlW, label, term.StyleCommitMessage, term.StyleCommitMessage, true)
	r := d.GetRect()
	d.topControl = Rect{X: r.X + controlX, Y: r.Y + y, W: controlW, H: 1}
}

func (d *CommitDetailWidget) renderHeading(surface Surface, rowIndex int, row commitDetailRow, visual commitDetailVisualRow, y, viewW int) {
	headingStyle := term.StyleDefault
	if row.fileIndex >= 0 && row.fileIndex < len(d.Files) && d.Files[row.fileIndex].HeadingStyle != 0 {
		headingStyle = d.Files[row.fileIndex].HeadingStyle
	}
	collapsed := row.fileIndex >= 0 && row.fileIndex < len(d.collapsedFiles) && d.collapsedFiles[row.fileIndex]
	if !visual.continuation {
		chevron := '▼'
		if collapsed {
			chevron = '▶'
		}
		surface.SetCell(1, y, term.Cell{Ch: chevron, Style: headingStyle, Bold: true})
	}
	r := d.GetRect()
	d.fileControls = append(d.fileControls, commitDetailControl{
		rect:      Rect{X: r.X, Y: r.Y + y, W: viewW, H: 1},
		fileIndex: row.fileIndex,
	})
	d.drawTextRow(surface, 3, y, viewW-3, row.text, headingStyle, term.StyleDefault, row.bold, visual.leftStart, rowIndex)
}

func (d *CommitDetailWidget) renderDiffRow(surface Surface, rowIndex int, row commitDetailRow, visual commitDetailVisualRow, y, viewW int) {
	if row.fileIndex < 0 || row.fileIndex >= len(d.Files) {
		return
	}
	file := &d.Files[row.fileIndex]
	if row.lineIndex < 0 {
		return
	}
	if d.mode == DiffModeUnified {
		d.renderUnifiedDiffRow(surface, rowIndex, file, row.lineIndex, visual, y, viewW)
		return
	}
	if row.lineIndex >= len(file.lines) {
		return
	}
	line := file.lines[row.lineIndex]
	leftStart, leftW, rightStart, rightW := d.sideGeometry(viewW)
	if leftW <= 0 || rightW <= 0 {
		return
	}
	dividerX := (viewW - 1) / 2
	surface.SetCell(dividerX, y, term.Cell{Ch: '│', Style: term.StyleBorder})
	leftStyle := diffKindStyle(line.Left.Kind)
	rightStyle := diffKindStyle(line.Right.Kind)
	if visual.continuation {
		renderDiffGutter(surface, 0, y, d.gutterW, diff.SideLine{}, leftStyle)
		renderDiffGutter(surface, dividerX+1, y, d.gutterW, diff.SideLine{}, rightStyle)
	} else {
		renderDiffGutter(surface, 0, y, d.gutterW, line.Left, leftStyle)
		renderDiffGutter(surface, dividerX+1, y, d.gutterW, line.Right, rightStyle)
	}

	var leftSpans, rightSpans []highlight.Span
	if file.highlighter != nil {
		if line.Left.Text != "" && line.Left.Kind != diff.Collapsed {
			leftSpans = file.highlighter.HighlightLine(line.Left.Text)
		}
		if line.Right.Text != "" && line.Right.Kind != diff.Collapsed {
			rightSpans = file.highlighter.HighlightLine(line.Right.Text)
		}
	}
	leftScroll := d.LeftCol
	if d.IsWrapped() {
		leftScroll = 0
	}
	leftForeground := diffKindForeground(line.Left.Kind, d.highContrast)
	rightForeground := diffKindForeground(line.Right.Kind, d.highContrast)
	renderDiffText(surface, leftStart, y, leftW, line.Left.Text, leftStyle, leftForeground, leftSpans, visual.leftStart, leftScroll, d.selectionDecorator(rowIndex, false))
	renderDiffText(surface, rightStart, y, rightW, line.Right.Text, rightStyle, rightForeground, rightSpans, visual.rightStart, leftScroll, d.selectionDecorator(rowIndex, true))
}

func (d *CommitDetailWidget) renderUnifiedDiffRow(surface Surface, rowIndex int, file *CommitDetailFile, lineIndex int, visual commitDetailVisualRow, y, viewW int) {
	if lineIndex >= len(file.unified) {
		return
	}
	line := file.unified[lineIndex].side
	contentStart, contentW, _, _ := d.sideGeometry(viewW)
	if contentW <= 0 {
		return
	}
	style := diffKindStyle(line.Kind)
	if visual.continuation {
		renderDiffGutter(surface, 0, y, d.gutterW, diff.SideLine{}, style)
	} else {
		renderDiffGutter(surface, 0, y, d.gutterW, line, style)
	}
	var spans []highlight.Span
	if file.highlighter != nil && line.Text != "" && line.Kind != diff.Collapsed {
		spans = file.highlighter.HighlightLine(line.Text)
	}
	leftScroll := d.LeftCol
	if d.IsWrapped() {
		leftScroll = 0
	}
	foreground := diffKindForeground(line.Kind, d.highContrast)
	renderDiffText(surface, contentStart, y, contentW, line.Text, style, foreground, spans, visual.leftStart, leftScroll, d.selectionDecorator(rowIndex, false))
}

func (d *CommitDetailWidget) drawText(surface Surface, x, y, width int, text string, style, bg term.Style, bold bool, segmentStart int) {
	d.drawTextRow(surface, x, y, width, text, style, bg, bold, segmentStart, -1)
}

func (d *CommitDetailWidget) drawStaticText(surface Surface, x, y, width int, text string, style, bg term.Style, bold bool) {
	blank := term.Cell{Ch: ' ', Style: style, BgStyle: bg}
	drawTextSegment(surface, x, y, width, text, 0, 0, blank, func(_ int, ch rune) term.Cell {
		return term.Cell{Ch: ch, Style: style, BgStyle: bg, Bold: bold}
	})
}

func (d *CommitDetailWidget) drawTextRow(surface Surface, x, y, width int, text string, style, bg term.Style, bold bool, segmentStart, rowIndex int) {
	leftScroll := d.LeftCol
	if d.IsWrapped() {
		leftScroll = 0
	}
	blank := term.Cell{Ch: ' ', Style: term.StyleDefault, BgStyle: bg}
	drawTextSegment(surface, x, y, width, text, segmentStart, leftScroll, blank, func(runeIndex int, ch rune) term.Cell {
		cell := term.Cell{Ch: ch, Style: style, BgStyle: bg, Bold: bold}
		if rowIndex >= 0 && d.hasSelection && d.selection.Contains(rowIndex, runeIndex) {
			cell.BgStyle = term.StyleSelection
		}
		return cell
	})
}

func (d *CommitDetailWidget) selectionDecorator(rowIndex int, right bool) diffCellDecorator {
	if !d.hasSelection || (d.mode == DiffModeSplit && right != d.selRight) {
		return nil
	}
	return func(runeIndex int, cell term.Cell) term.Cell {
		if d.selection.Contains(rowIndex, runeIndex) {
			cell.BgStyle = term.StyleSelection
		}
		return cell
	}
}

func (d *CommitDetailWidget) rowText(rowIndex int, right bool) (string, bool) {
	if rowIndex < 0 || rowIndex >= len(d.rows) {
		return "", false
	}
	row := d.rows[rowIndex]
	switch row.kind {
	case commitDetailMessageRow, commitDetailHeadingRow, commitDetailNoticeRow:
		return row.text, true
	case commitDetailDiffRow:
		if row.fileIndex < 0 || row.fileIndex >= len(d.Files) || row.lineIndex < 0 {
			return "", false
		}
		file := &d.Files[row.fileIndex]
		if d.mode == DiffModeUnified {
			if row.lineIndex >= len(file.unified) {
				return "", false
			}
			return file.unified[row.lineIndex].side.Text, true
		}
		if row.lineIndex >= len(file.lines) {
			return "", false
		}
		if right {
			return file.lines[row.lineIndex].Right.Text, true
		}
		return file.lines[row.lineIndex].Left.Text, true
	default:
		return "", false
	}
}

func (d *CommitDetailWidget) screenToSelection(mx, my int) (pos diffSelPos, right bool, ok bool) {
	if pointInCommitDetailRect(mx, my, d.stickyRect) {
		return diffSelPos{}, false, false
	}
	r := d.GetRect()
	localX, localY := mx-r.X, my-r.Y
	if localX < 0 || localX >= d.layoutViewW || localY < 0 || localY >= d.viewH {
		return diffSelPos{}, false, false
	}
	visualIndex := d.TopLine + localY
	rowIndex := visualIndex
	visual := commitDetailVisualRow{row: rowIndex, leftStart: 0, rightStart: 0}
	if d.IsWrapped() {
		if visualIndex < 0 || visualIndex >= len(d.visualRows) {
			return diffSelPos{}, false, false
		}
		visual = d.visualRows[visualIndex]
		rowIndex = visual.row
	} else if rowIndex < 0 || rowIndex >= len(d.rows) {
		return diffSelPos{}, false, false
	}
	row := d.rows[rowIndex]
	textX, textW, segmentStart := 0, d.layoutViewW, visual.leftStart
	switch row.kind {
	case commitDetailMessageRow, commitDetailNoticeRow:
	case commitDetailHeadingRow:
		textX, textW = 3, d.layoutViewW-3
	case commitDetailDiffRow:
		if d.mode == DiffModeUnified {
			textX, textW = d.layoutLeftStart, d.layoutLeftW
		} else if localX >= d.layoutLeftStart && localX < d.layoutLeftStart+d.layoutLeftW {
			textX, textW = d.layoutLeftStart, d.layoutLeftW
			right = false
		} else if localX >= d.layoutRightStart && localX < d.layoutRightStart+d.layoutRightW {
			textX, textW = d.layoutRightStart, d.layoutRightW
			segmentStart = visual.rightStart
			right = true
		} else {
			return diffSelPos{}, false, false
		}
	default:
		return diffSelPos{}, false, false
	}
	if textW <= 0 || localX < textX || localX >= textX+textW || segmentStart < 0 {
		return diffSelPos{}, false, false
	}
	text, selectable := d.rowText(rowIndex, right)
	if !selectable {
		return diffSelPos{}, false, false
	}
	visualCol := localX - textX
	if !d.IsWrapped() {
		visualCol += d.LeftCol
	}
	return diffSelPos{Line: rowIndex, Col: diffSegmentVisualColToRune(text, segmentStart, visualCol)}, right, true
}

func (d *CommitDetailWidget) selectionText() string {
	if !d.hasSelection {
		return ""
	}
	return d.selection.Text(len(d.rows), func(rowIndex int) (string, bool) {
		return d.rowText(rowIndex, d.selRight)
	})
}

func (d *CommitDetailWidget) CopySelection() string {
	text := d.selectionText()
	if text != "" {
		d.ClearSelection()
	}
	return text
}

func (d *CommitDetailWidget) ClearSelection() {
	d.hasSelection = false
	d.selecting = false
}

func (d *CommitDetailWidget) selectWordAt(rowIndex, col int) bool {
	text, ok := d.rowText(rowIndex, d.selRight)
	if !ok {
		return false
	}
	return d.selection.SelectWord(rowIndex, col, text)
}

func (d *CommitDetailWidget) renderStickyHeading(surface Surface, viewW int) {
	if viewW <= 0 || d.TopLine <= 0 {
		return
	}
	rowIndex := d.TopLine
	if d.IsWrapped() {
		if d.TopLine >= len(d.visualRows) {
			return
		}
		rowIndex = d.visualRows[d.TopLine].row
	}
	if rowIndex < 0 || rowIndex >= len(d.rows) {
		return
	}
	row := d.rows[rowIndex]
	if row.kind != commitDetailDiffRow && row.kind != commitDetailNoticeRow {
		return
	}
	if row.fileIndex < 0 || row.fileIndex >= len(d.Files) {
		return
	}
	for column := 0; column < viewW; column++ {
		surface.SetCell(column, 0, term.Cell{Ch: ' ', Style: term.StyleDefault})
	}
	headingStyle := d.Files[row.fileIndex].HeadingStyle
	if headingStyle == 0 {
		headingStyle = term.StyleDefault
	}
	surface.SetCell(1, 0, term.Cell{Ch: '▼', Style: headingStyle, Bold: true})
	path := truncateCommitDetailPath(commitDetailFileHeading(d.Files[row.fileIndex]), viewW-3)
	drawTextSegment(surface, 3, 0, viewW-3, path, 0, 0, term.Cell{Ch: ' '}, func(_ int, ch rune) term.Cell {
		return term.Cell{Ch: ch, Style: headingStyle, Bold: true}
	})
	r := d.GetRect()
	d.stickyRect = Rect{X: r.X, Y: r.Y, W: viewW, H: 1}
	d.stickyControl = commitDetailControl{
		rect:      d.stickyRect,
		fileIndex: row.fileIndex,
	}
}

func truncateCommitDetailPath(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if textwidth.String(text) <= width {
		return text
	}
	runes := []rune(text)
	tailWidth := 0
	start := len(runes)
	for start > 0 {
		runeWidth := textwidth.Rune(runes[start-1])
		if tailWidth+runeWidth > width-1 {
			break
		}
		tailWidth += runeWidth
		start--
	}
	return "…" + string(runes[start:])
}

func (d *CommitDetailWidget) renderHorizontalScrollbars(surface Surface, r Rect, viewW, y int) {
	leftStart, leftW, rightStart, rightW := d.sideGeometry(viewW)
	d.hscrollbar.X = r.X + leftStart
	d.hscrollbar.Y = r.Y + y
	d.hscrollbar.Width = leftW
	d.hscrollbar.TotalCols = d.maxLineW
	d.hscrollbar.LeftCol = d.LeftCol
	d.hscrollbar.Render(surface, leftStart, y)
	for column := 0; column < d.gutterW; column++ {
		surface.SetCell(column, y, term.Cell{Ch: ' '})
	}
	if d.mode == DiffModeUnified {
		d.rhscroll.TotalCols = 0
		return
	}

	dividerX := (viewW - 1) / 2
	d.rhscroll.X = r.X + rightStart
	d.rhscroll.Y = r.Y + y
	d.rhscroll.Width = rightW
	d.rhscroll.TotalCols = d.maxLineW
	d.rhscroll.LeftCol = d.LeftCol
	d.rhscroll.Render(surface, rightStart, y)

	surface.SetCell(dividerX, y, term.Cell{Ch: '│', Style: term.StyleBorder})
	for column := dividerX + 1; column < rightStart; column++ {
		surface.SetCell(column, y, term.Cell{Ch: ' '})
	}
}

func (d *CommitDetailWidget) clampScroll() {
	maxTop := d.totalVisualRows - d.viewH
	if maxTop < 0 {
		maxTop = 0
	}
	if d.TopLine < 0 {
		d.TopLine = 0
	}
	if d.TopLine > maxTop {
		d.TopLine = maxTop
	}
	maxLeft := d.maxLineW - d.contentW
	if d.IsWrapped() {
		maxLeft = 0
	}
	if maxLeft < 0 {
		maxLeft = 0
	}
	if d.LeftCol < 0 {
		d.LeftCol = 0
	}
	if d.LeftCol > maxLeft {
		d.LeftCol = maxLeft
	}
}

func (d *CommitDetailWidget) HandleEvent(ev tcell.Event) EventResult {
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
		if newLeft, consumed := d.rhscroll.HandleEvent(ev); consumed {
			d.LeftCol = newLeft
			if d.rhscroll.IsDragging() {
				return EventCaptured
			}
			return EventConsumed
		}
	}

	switch event := ev.(type) {
	case *tcell.EventKey:
		switch event.Key() {
		case tcell.KeyUp:
			d.TopLine--
		case tcell.KeyDown:
			d.TopLine++
		case tcell.KeyLeft:
			d.LeftCol--
		case tcell.KeyRight:
			d.LeftCol++
		case tcell.KeyPgUp:
			d.TopLine -= d.viewH
		case tcell.KeyPgDn:
			d.TopLine += d.viewH
		case tcell.KeyHome:
			d.TopLine = 0
			d.LeftCol = 0
		case tcell.KeyEnd:
			d.TopLine = d.totalVisualRows - d.viewH
		default:
			return EventIgnored
		}
		d.clampScroll()
		return EventConsumed
	case *tcell.EventMouse:
		buttons := event.Buttons()
		modifiers := event.Modifiers()
		switch {
		case buttons&tcell.WheelUp != 0 && modifiers&tcell.ModShift != 0:
			d.LeftCol -= 4
		case buttons&tcell.WheelDown != 0 && modifiers&tcell.ModShift != 0:
			d.LeftCol += 4
		case buttons&tcell.WheelUp != 0:
			d.TopLine -= 3
		case buttons&tcell.WheelDown != 0:
			d.TopLine += 3
		case buttons&tcell.WheelLeft != 0:
			d.LeftCol -= 4
		case buttons&tcell.WheelRight != 0:
			d.LeftCol += 4
		default:
			mx, my := event.Position()
			primaryPressed := buttons&tcell.Button1 != 0
			freshPrimaryPress := primaryPressed && !d.primaryPressed
			if buttons == tcell.ButtonNone {
				d.primaryPressed = false
				if d.disclosurePressed {
					d.disclosurePressed = false
					return EventConsumed
				}
			}
			if primaryPressed {
				d.primaryPressed = true
				if d.disclosurePressed {
					return EventConsumed
				}
				// A selection drag may cross a heading. Only a fresh press owns the
				// row-wide disclosure target; otherwise the file would collapse while
				// the reader was selecting adjacent diff text.
				if freshPrimaryPress && !d.selecting {
					if pointInCommitDetailRect(mx, my, d.topControl) {
						if d.allFilesCollapsed() {
							d.ExpandAllFiles()
						} else {
							d.CollapseAllFiles()
						}
						d.disclosurePressed = true
						return EventConsumed
					}
					if pointInCommitDetailRect(mx, my, d.stickyControl.rect) {
						d.toggleFile(d.stickyControl.fileIndex)
						d.disclosurePressed = true
						return EventConsumed
					}
					for _, control := range d.fileControls {
						if pointInCommitDetailRect(mx, my, control.rect) {
							d.toggleFile(control.fileIndex)
							d.disclosurePressed = true
							return EventConsumed
						}
					}
					if fileIndex, gap, ok := d.contextGapAtScreenY(my); ok {
						d.requestFileContext(fileIndex, gap)
						d.disclosurePressed = true
						return EventConsumed
					}
				}
				pos, right, ok := d.screenToSelection(mx, my)
				if ok {
					if !d.selecting {
						now := time.Now()
						isDoubleClick := now.Sub(d.lastClickTime) < DoubleClickMs*time.Millisecond &&
							pos.Line == d.lastClickPos.Line && pos.Col == d.lastClickPos.Col
						d.lastClickTime = now
						d.lastClickPos = pos
						d.selRight = right
						if isDoubleClick {
							if d.selectWordAt(pos.Line, pos.Col) {
								d.hasSelection = true
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
			if d.selecting && buttons == tcell.ButtonNone {
				d.selecting = false
				start, end := d.selection.Range()
				if start.Line == end.Line && start.Col == end.Col {
					d.hasSelection = false
				}
				return EventConsumed
			}
			return EventIgnored
		}
		d.clampScroll()
		return EventConsumed
	}
	return EventIgnored
}

func (d *CommitDetailWidget) contextGapAtScreenY(screenY int) (fileIndex, gap int, ok bool) {
	r := d.GetRect()
	localY := screenY - r.Y
	if localY < 0 || localY >= d.viewH {
		return 0, 0, false
	}
	visualRow := d.TopLine + localY
	rowIndex := visualRow
	if d.IsWrapped() {
		if visualRow >= len(d.visualRows) {
			return 0, 0, false
		}
		rowIndex = d.visualRows[visualRow].row
	}
	if rowIndex < 0 || rowIndex >= len(d.rows) {
		return 0, 0, false
	}
	row := d.rows[rowIndex]
	if row.kind != commitDetailDiffRow || row.fileIndex < 0 || row.fileIndex >= len(d.Files) {
		return 0, 0, false
	}
	lineIndex := row.lineIndex
	file := &d.Files[row.fileIndex]
	if d.mode == DiffModeUnified {
		if lineIndex < 0 || lineIndex >= len(file.unified) {
			return 0, 0, false
		}
		lineIndex = file.unified[lineIndex].sourceLine
	}
	gap, ok = file.gapByLine[lineIndex]
	return row.fileIndex, gap, ok
}

func pointInCommitDetailRect(x, y int, rect Rect) bool {
	return rect.W > 0 && rect.H > 0 && x >= rect.X && x < rect.X+rect.W && y >= rect.Y && y < rect.Y+rect.H
}
