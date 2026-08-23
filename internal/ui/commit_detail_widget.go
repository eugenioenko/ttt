package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

// CommitDetailFile is one file section in a commit detail view. Files remain in
// the order Git reports them; a failed or hunk-less diff still gets a heading
// and an explanatory row so a touched path never silently disappears.
type CommitDetailFile struct {
	Status      string
	Path        string
	OldPath     string
	Stage       CommitDetailStage
	Boundary    CommitDetailBoundary
	IndexStages []byte
	// ConflictCode marks a two-way resolution view between the preferred
	// available conflict stage and the worktree, not a full three-way merge view.
	ConflictCode string
	ContentKind  CommitDetailContentKind
	Diff         diff.FileDiff
	Error        string

	FullFileState CommitDetailFullFileState
	FullFileErr   string

	highlighter  *highlight.Highlighter
	lines        []diff.DiffLine
	unified      []diffUnifiedLine
	oldLines     []string
	newLines     []string
	expandedGaps map[int]bool
	gapByLine    map[int]int
	pendingGap   int
}

type CommitDetailBoundary uint8

const (
	CommitDetailBoundaryNone CommitDetailBoundary = iota
	CommitDetailBoundaryHeadToIndex
	CommitDetailBoundaryIndexToWorktree
	CommitDetailBoundaryConflictToWorktree
)

type CommitDetailStage uint8

const (
	CommitDetailStageNone CommitDetailStage = iota
	CommitDetailStageStaged
	CommitDetailStageUnstaged
	CommitDetailStageMixed
	CommitDetailStageConflict
)

type CommitDetailContentKind uint8

const (
	CommitDetailContentText CommitDetailContentKind = iota
	CommitDetailContentBinary
	CommitDetailContentEmpty
)

type CommitDetailFullFileState uint8

const (
	CommitDetailFullFileIdle CommitDetailFullFileState = iota
	CommitDetailFullFileLoading
	CommitDetailFullFileLoaded
	CommitDetailFullFileFailed
)

type commitDetailRowKind uint8

const (
	commitDetailMessageHeaderRow commitDetailRowKind = iota
	commitDetailHeaderDividerRow
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
	danger    bool
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

type commitDetailSelectionPoint struct {
	key       string
	lineIndex int
	col       int
}

type commitDetailPreservedSelection struct {
	anchor  commitDetailSelectionPoint
	current commitDetailSelectionPoint
	right   bool
}

// CommitDetailWidget renders an entire commit as one virtualized scrollable
// document. Unlike stacking several DiffViewWidgets, it owns one vertical
// viewport and only draws visible rows, so a large commit does not allocate a
// full-screen cell grid for every changed line on every redraw.
type CommitDetailWidget struct {
	BaseWidget
	Dir            string
	Ref            string
	Short          string
	Incarnation    uint64
	Loading        bool
	Error          string
	Header         string
	LoadingText    string
	CurrentChanges bool
	RefreshError   string
	hasDetail      bool

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
	emphasizeGaps   bool
	collapsedFiles  []bool

	rows             []commitDetailRow
	visualRows       []commitDetailVisualRow
	visualRowsW      int
	totalVisualRows  int
	maxLineW         int
	gutterW          int
	viewH            int
	contentW         int
	scrollbar        widgets.VerticalScrollbar
	hscrollbar       widgets.HorizontalScrollbar
	rhscroll         widgets.HorizontalScrollbar
	scrollbarCapture scrollbarCaptureState

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
	hasHoveredGap     bool
	hoveredFile       int
	hoveredGap        int

	OnFetchContext func(fileIndex int, file CommitDetailFile)
	OnClose        func()
}

func NewCommitDetailWidget(dir, ref, short string, syntaxHighlight bool) *CommitDetailWidget {
	return &CommitDetailWidget{
		Dir:             dir,
		Ref:             ref,
		Short:           short,
		Loading:         true,
		Header:          "Commit message",
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
		LoadingText:     "Loading current changes…",
		CurrentChanges:  true,
		SyntaxHighlight: syntaxHighlight,
	}
}

func CommitDetailFileWithContent(file CommitDetailFile, oldLines, newLines []string) CommitDetailFile {
	file.oldLines = append([]string(nil), oldLines...)
	file.newLines = append([]string(nil), newLines...)
	file.FullFileState = CommitDetailFullFileLoaded
	return file
}

func (d *CommitDetailWidget) Focusable() bool { return true }

func (d *CommitDetailWidget) Close() {
	if d.OnClose == nil {
		return
	}
	onClose := d.OnClose
	d.OnClose = nil
	onClose()
}

func (d *CommitDetailWidget) SetDiffHighContrast(enabled bool) { d.highContrast = enabled }

func (d *CommitDetailWidget) DiffHighContrast() bool { return d.highContrast }

func (d *CommitDetailWidget) SetDiffCollapsedEmphasis(enabled bool) { d.emphasizeGaps = enabled }

func (d *CommitDetailWidget) DiffCollapsedEmphasis() bool { return d.emphasizeGaps }

func (d *CommitDetailWidget) ContextMode() DiffContextMode { return d.contextMode }

func (d *CommitDetailWidget) SetContextMode(mode DiffContextMode) {
	d.contextExplicit = true
	if d.contextMode == mode {
		if mode == DiffContextFullFile {
			for i := range d.Files {
				d.requestFileContext(i, -1)
			}
		}
		return
	}
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

func (d *CommitDetailWidget) requestFileContext(fileIndex, gap int) {
	if fileIndex < 0 || fileIndex >= len(d.Files) {
		return
	}
	file := &d.Files[fileIndex]
	if file.FullFileState == CommitDetailFullFileLoaded {
		if gap >= 0 {
			file.expandedGaps[gap] = true
			d.rebuildRows()
			d.ClearSelection()
		}
		return
	}
	if file.FullFileState == CommitDetailFullFileLoading || d.OnFetchContext == nil {
		return
	}
	if gap >= 0 {
		file.pendingGap = gap
	}
	file.FullFileState = CommitDetailFullFileLoading
	file.FullFileErr = ""
	d.rebuildRows()
	d.OnFetchContext(fileIndex, *file)
}

func (d *CommitDetailWidget) ApplyFileContext(fileIndex int, key string, oldLines, newLines []string, contextErr ...string) bool {
	errText := ""
	if len(contextErr) > 0 {
		errText = contextErr[0]
	} else if oldLines == nil && newLines == nil {
		errText = "Could not load full file"
	}
	return d.ApplyFileContextContent(fileIndex, key, oldLines, newLines, CommitDetailContentText, errText)
}

func (d *CommitDetailWidget) ApplyFileContextContent(fileIndex int, key string, oldLines, newLines []string, contentKind CommitDetailContentKind, errText string) bool {
	if fileIndex < 0 || fileIndex >= len(d.Files) || commitDetailFileKey(d.Files[fileIndex]) != key {
		return false
	}
	file := &d.Files[fileIndex]
	if errText == "" {
		file.oldLines = oldLines
		file.newLines = newLines
		file.ContentKind = contentKind
		file.FullFileState = CommitDetailFullFileLoaded
		file.FullFileErr = ""
	} else {
		file.FullFileState = CommitDetailFullFileFailed
		file.FullFileErr = errText
	}
	if file.FullFileState == CommitDetailFullFileLoaded && file.pendingGap >= 0 {
		file.expandedGaps[file.pendingGap] = true
	}
	file.pendingGap = -1
	d.rebuildRows()
	d.ClearSelection()
	return true
}

func CommitDetailContextKey(file CommitDetailFile) string { return commitDetailFileKey(file) }

func commitDetailFileKey(file CommitDetailFile) string {
	return file.OldPath + "\x00" + file.Path + "\x00" + string([]byte{byte(file.Boundary)}) + "\x00" + file.ConflictCode + "\x00" + string(file.IndexStages)
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
	if d.contextMode == DiffContextFullFile {
		for i := range d.Files {
			d.requestFileContext(i, -1)
		}
	}
}

func (d *CommitDetailWidget) SetDetail(message string, files []CommitDetailFile, errText string) {
	d.Loading = false
	d.Error = errText
	d.Message = message
	d.Files = files
	d.collapsedFiles = make([]bool, len(files))
	for i := range d.Files {
		d.Files[i].expandedGaps = make(map[int]bool)
		d.Files[i].pendingGap = -1
		if d.SyntaxHighlight && d.Files[i].Path != "" {
			d.Files[i].highlighter = highlight.New(d.Files[i].Path)
		}
	}
	d.TopLine = 0
	d.LeftCol = 0
	d.ClearSelection()
	d.rebuildRows()
	if d.contextMode == DiffContextFullFile {
		for i := range d.Files {
			d.requestFileContext(i, -1)
		}
	}
}

func (d *CommitDetailWidget) SetCurrentChanges(message string, files []CommitDetailFile, errText string) {
	if !d.CurrentChanges {
		return
	}
	d.Loading = false
	if errText != "" {
		if d.hasDetail {
			d.Error = ""
			d.RefreshError = errText
			d.rebuildRows()
			return
		}
		d.Error = errText
		d.RefreshError = ""
		return
	}

	type preservedFileState struct {
		collapsed    bool
		expandedGaps map[int]bool
	}
	preservedSelection, hasPreservedSelection := d.captureCurrentChangesSelection()
	preserved := make(map[string]preservedFileState, len(d.Files))
	for i, file := range d.Files {
		state := preservedFileState{expandedGaps: make(map[int]bool)}
		if i < len(d.collapsedFiles) {
			state.collapsed = d.collapsedFiles[i]
		}
		for gap, expanded := range file.expandedGaps {
			state.expandedGaps[gap] = expanded
		}
		preserved[commitDetailFileKey(file)] = state
	}
	topLine := d.TopLine
	d.Error = ""
	d.RefreshError = ""
	d.Message = message
	d.Files = files
	d.collapsedFiles = make([]bool, len(files))
	for i := range d.Files {
		d.Files[i].pendingGap = -1
		state, ok := preserved[commitDetailFileKey(d.Files[i])]
		if ok {
			d.collapsedFiles[i] = state.collapsed
			d.Files[i].expandedGaps = state.expandedGaps
		} else {
			d.Files[i].expandedGaps = make(map[int]bool)
		}
		if d.SyntaxHighlight && d.Files[i].Path != "" && d.Files[i].ContentKind == CommitDetailContentText {
			d.Files[i].highlighter = highlight.New(d.Files[i].Path)
		}
	}
	d.hasDetail = true
	d.TopLine = topLine
	d.rebuildRows()
	if !hasPreservedSelection || !d.restoreCurrentChangesSelection(preservedSelection) {
		d.ClearSelection()
	}
	d.clampScroll()
}

func (d *CommitDetailWidget) captureCurrentChangesSelection() (commitDetailPreservedSelection, bool) {
	if !d.hasSelection {
		return commitDetailPreservedSelection{}, false
	}
	capture := func(pos diffSelPos) (commitDetailSelectionPoint, bool) {
		if pos.Line < 0 || pos.Line >= len(d.rows) {
			return commitDetailSelectionPoint{}, false
		}
		row := d.rows[pos.Line]
		if row.kind != commitDetailDiffRow || row.fileIndex < 0 || row.fileIndex >= len(d.Files) {
			return commitDetailSelectionPoint{}, false
		}
		return commitDetailSelectionPoint{key: commitDetailFileKey(d.Files[row.fileIndex]), lineIndex: row.lineIndex, col: pos.Col}, true
	}
	anchor, anchorOK := capture(d.selection.Anchor)
	current, currentOK := capture(d.selection.Current)
	return commitDetailPreservedSelection{anchor: anchor, current: current, right: d.selRight}, anchorOK && currentOK
}

func (d *CommitDetailWidget) restoreCurrentChangesSelection(selection commitDetailPreservedSelection) bool {
	restore := func(point commitDetailSelectionPoint) (diffSelPos, bool) {
		for rowIndex, row := range d.rows {
			if row.kind == commitDetailDiffRow && row.lineIndex == point.lineIndex && row.fileIndex >= 0 && row.fileIndex < len(d.Files) && commitDetailFileKey(d.Files[row.fileIndex]) == point.key {
				return diffSelPos{Line: rowIndex, Col: point.col}, true
			}
		}
		return diffSelPos{}, false
	}
	anchor, anchorOK := restore(selection.anchor)
	current, currentOK := restore(selection.current)
	if !anchorOK || !currentOK {
		return false
	}
	d.selection.Anchor = anchor
	d.selection.Current = current
	d.selRight = selection.right
	d.hasSelection = true
	d.selecting = false
	return true
}

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
	d.hasHoveredGap = false
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
		d.rows = append(d.rows, commitDetailRow{kind: commitDetailSpacerRow})
	}
	message := strings.TrimRight(d.Message, "\r\n")
	if message == "" {
		if d.CurrentChanges {
			message = "Working tree clean"
		} else {
			message = "(No commit message)"
		}
	}
	for i, line := range strings.Split(message, "\n") {
		d.rows = append(d.rows, commitDetailRow{kind: commitDetailMessageRow, text: line, bold: i == 0})
		d.recordWidth(line)
	}
	if d.RefreshError != "" {
		d.rows = append(d.rows, commitDetailRow{kind: commitDetailNoticeRow, text: d.RefreshError, danger: true})
		d.recordWidth(d.RefreshError)
	}
	d.rows = append(d.rows, commitDetailRow{kind: commitDetailHeaderDividerRow})
	d.rows = append(d.rows, commitDetailRow{kind: commitDetailSpacerRow})

	maxLine := 0
	for fileIndex := range d.Files {
		file := &d.Files[fileIndex]
		if d.contextMode == DiffContextFullFile && file.FullFileState == CommitDetailFullFileLoaded {
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
		if d.contextMode == DiffContextFullFile {
			var notice string
			var danger bool
			switch file.FullFileState {
			case CommitDetailFullFileIdle:
				notice = "Full file not loaded"
			case CommitDetailFullFileLoading:
				notice = "Loading full file…"
			case CommitDetailFullFileFailed:
				notice = file.FullFileErr
				if notice == "" {
					notice = "Could not load full file"
				}
				danger = true
			}
			if notice != "" {
				d.rows = append(d.rows, commitDetailRow{kind: commitDetailNoticeRow, text: notice, danger: danger, fileIndex: fileIndex})
				d.recordWidth(notice)
			}
		}

		switch {
		case file.Error != "":
			d.rows = append(d.rows, commitDetailRow{kind: commitDetailNoticeRow, text: file.Error, fileIndex: fileIndex})
			d.recordWidth(file.Error)
		case file.ContentKind == CommitDetailContentBinary:
			const binary = "Binary file changed"
			d.rows = append(d.rows, commitDetailRow{kind: commitDetailNoticeRow, text: binary, fileIndex: fileIndex})
			d.recordWidth(binary)
		case file.ContentKind == CommitDetailContentEmpty:
			empty := "Empty file changed"
			switch file.Status {
			case "A", "?":
				empty = "Empty file added"
			case "D":
				empty = "Empty file deleted"
			}
			d.rows = append(d.rows, commitDetailRow{kind: commitDetailNoticeRow, text: empty, fileIndex: fileIndex})
			d.recordWidth(empty)
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
	if len(d.Files) == 0 && !d.CurrentChanges {
		const noFiles = "No files"
		d.rows = append(d.rows, commitDetailRow{kind: commitDetailNoticeRow, text: noFiles})
		d.recordWidth(noFiles)
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
	path := displayCommitDetailPath(file.Path)
	if file.OldPath != "" && file.OldPath != file.Path {
		path = fmt.Sprintf("%s → %s", displayCommitDetailPath(file.OldPath), path)
	}
	if file.Stage == CommitDetailStageNone {
		return path
	}
	if file.Stage == CommitDetailStageConflict {
		return fmt.Sprintf("%s  %s - conflict (%s)", StatusBadge(file.Status), path, file.ConflictCode)
	}
	stage := "unstaged"
	switch file.Stage {
	case CommitDetailStageStaged:
		stage = "staged"
	case CommitDetailStageMixed:
		stage = "mixed"
	}
	return fmt.Sprintf("%s  %s · %s", StatusBadge(file.Status), path, stage)
}

func displayCommitDetailPath(path string) string {
	for _, r := range path {
		if !unicode.IsGraphic(r) {
			return strconv.QuoteToGraphic(path)
		}
	}
	return path
}

func (d *CommitDetailWidget) Render(surface Surface) {
	w, h := surface.Size()
	r := d.GetRect()
	if w <= 0 || h <= 0 {
		d.InvalidatePointerInteraction()
		return
	}
	surface.Fill(term.Cell{Ch: ' ', Style: term.StyleDefault})

	if d.Loading {
		d.InvalidatePointerInteraction()
		loadingText := d.LoadingText
		if loadingText == "" {
			loadingText = fmt.Sprintf("Loading commit %s…", d.Short)
		}
		surface.DrawText(0, 0, loadingText, w, term.StyleMuted)
		return
	}
	if d.Error != "" {
		d.InvalidatePointerInteraction()
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

	vGeometry := widgets.ScrollbarGeometry{}
	if showV {
		vGeometry = widgets.NewScrollbarGeometry(
			Rect{X: viewW, Y: 0, W: 1, H: viewH},
			Rect{X: r.X + viewW, Y: r.Y, W: 1, H: viewH},
		)
	}
	_, vInvalidated := d.scrollbar.Render(surface, vGeometry, widgets.NewScrollRange(viewH, d.totalVisualRows, d.TopLine))
	hInvalidated := false
	if showH && viewH < h {
		hInvalidated = d.renderHorizontalScrollbars(surface, r, viewW, viewH)
	} else {
		_, leftInvalidated := d.hscrollbar.Render(surface, widgets.ScrollbarGeometry{}, widgets.NewScrollRange(leftW, leftW, 0))
		_, rightInvalidated := d.rhscroll.Render(surface, widgets.ScrollbarGeometry{}, widgets.NewScrollRange(rightW, rightW, 0))
		hInvalidated = leftInvalidated || rightInvalidated
	}
	d.scrollbarCapture.notify(vInvalidated || hInvalidated)
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
		case commitDetailMessageHeaderRow, commitDetailHeaderDividerRow, commitDetailSpacerRow:
			rightStarts = nil
		case commitDetailHeadingRow:
			leftStarts = diffWrapStarts(row.text, viewW-3)
			rightStarts = nil
		case commitDetailMetadataRow, commitDetailMessageRow:
			leftStarts = diffWrapStarts(row.text, viewW-2)
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
	case commitDetailHeaderDividerRow:
		d.renderHeaderDivider(surface, y, viewW)
	case commitDetailMetadataRow:
		d.drawTextRow(surface, 1, y, viewW-2, row.text, term.StyleMuted, term.StyleCommitHeader, false, visual.leftStart, rowIndex)
	case commitDetailSpacerRow:
		return
	case commitDetailMessageRow:
		d.drawTextRow(surface, 1, y, viewW-2, row.text, term.StyleCommitHeader, term.StyleCommitHeader, row.bold, visual.leftStart, rowIndex)
	case commitDetailHeadingRow:
		d.renderHeading(surface, rowIndex, row, visual, y, viewW)
	case commitDetailNoticeRow:
		style := term.StyleMuted
		if row.danger || row.fileIndex >= 0 && row.fileIndex < len(d.Files) && d.Files[row.fileIndex].Error != "" {
			style = term.StyleDanger
		}
		d.drawTextRow(surface, 0, y, viewW, row.text, style, term.StyleDefault, false, visual.leftStart, rowIndex)
	case commitDetailDiffRow:
		d.renderDiffRow(surface, rowIndex, row, visual, y, viewW)
	}
}

func (d *CommitDetailWidget) renderMessageHeader(surface Surface, y, viewW int) {
	for column := 0; column < viewW; column++ {
		surface.SetCell(column, y, term.Cell{Ch: ' ', Style: term.StyleCommitHeader})
	}
	header := d.Header
	if header == "" {
		header = "Commit message"
	}
	d.drawStaticText(surface, 1, y, viewW-2, header, term.StyleCommitHeader, term.StyleCommitHeader, true)
	if len(d.Files) == 0 {
		return
	}
	label := "Collapse all"
	if d.allFilesCollapsed() {
		label = "Expand all"
	}
	controlW := textwidth.String(label)
	controlX := viewW - controlW - 1
	if controlX <= 1+textwidth.String(header) {
		return
	}
	d.drawStaticText(surface, controlX, y, controlW, label, term.StyleCommitHeader, term.StyleCommitHeader, true)
	r := d.GetRect()
	d.topControl = Rect{X: r.X + controlX, Y: r.Y + y, W: controlW, H: 1}
}

func (d *CommitDetailWidget) renderHeaderDivider(surface Surface, y, viewW int) {
	for column := 0; column < viewW; column++ {
		surface.SetCell(column, y, term.Cell{Ch: '─', Style: term.StyleBorder})
	}
}

func (d *CommitDetailWidget) renderHeading(surface Surface, rowIndex int, row commitDetailRow, visual commitDetailVisualRow, y, viewW int) {
	headingStyle := term.StyleDefault
	if row.fileIndex >= 0 && row.fileIndex < len(d.Files) && d.Files[row.fileIndex].Stage != CommitDetailStageNone {
		headingStyle = StatusStyle(d.Files[row.fileIndex].Status)
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
	gap, isGap := file.gapByLine[row.lineIndex]
	gapHovered := isGap && d.hasHoveredGap && d.hoveredFile == row.fileIndex && d.hoveredGap == gap
	leftStyle := collapsedDiffRowStyle(line.Left.Kind, d.emphasizeGaps, gapHovered)
	rightStyle := collapsedDiffRowStyle(line.Right.Kind, d.emphasizeGaps, gapHovered)
	if visual.continuation {
		renderDiffGutter(surface, 0, y, d.gutterW, diff.SideLine{})
		renderDiffGutter(surface, dividerX+1, y, d.gutterW, diff.SideLine{})
	} else {
		renderDiffGutterWithCollapsedStyle(surface, 0, y, d.gutterW, line.Left, collapsedDiffGutterStyle(line.Left.Kind, d.emphasizeGaps, gapHovered))
		renderDiffGutterWithCollapsedStyle(surface, dividerX+1, y, d.gutterW, line.Right, collapsedDiffGutterStyle(line.Right.Kind, d.emphasizeGaps, gapHovered))
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
	renderDiffText(surface, leftStart, y, leftW, line.Left.Text, leftStyle, diffKindForeground(line.Left.Kind, d.highContrast), leftSpans, visual.leftStart, leftScroll, d.selectionDecorator(rowIndex, false))
	renderDiffText(surface, rightStart, y, rightW, line.Right.Text, rightStyle, diffKindForeground(line.Right.Kind, d.highContrast), rightSpans, visual.rightStart, leftScroll, d.selectionDecorator(rowIndex, true))
}

func (d *CommitDetailWidget) renderUnifiedDiffRow(surface Surface, rowIndex int, file *CommitDetailFile, lineIndex int, visual commitDetailVisualRow, y, viewW int) {
	if lineIndex >= len(file.unified) {
		return
	}
	line := file.unified[lineIndex].side
	fileIndex := d.rows[rowIndex].fileIndex
	contentStart, contentW, _, _ := d.sideGeometry(viewW)
	if contentW <= 0 {
		return
	}
	gap, isGap := file.gapByLine[file.unified[lineIndex].sourceLine]
	style := collapsedDiffRowStyle(line.Kind, d.emphasizeGaps, isGap && d.hasHoveredGap && d.hoveredFile == fileIndex && d.hoveredGap == gap)
	if visual.continuation {
		renderDiffGutter(surface, 0, y, d.gutterW, diff.SideLine{})
	} else {
		renderDiffGutterWithCollapsedStyle(surface, 0, y, d.gutterW, line, collapsedDiffGutterStyle(line.Kind, d.emphasizeGaps, isGap && d.hasHoveredGap && d.hoveredFile == fileIndex && d.hoveredGap == gap))
	}
	var spans []highlight.Span
	if file.highlighter != nil && line.Text != "" && line.Kind != diff.Collapsed {
		spans = file.highlighter.HighlightLine(line.Text)
	}
	leftScroll := d.LeftCol
	if d.IsWrapped() {
		leftScroll = 0
	}
	renderDiffText(surface, contentStart, y, contentW, line.Text, style, diffKindForeground(line.Kind, d.highContrast), spans, visual.leftStart, leftScroll, d.selectionDecorator(rowIndex, false))
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
			line := file.unified[row.lineIndex].side
			return line.Text, line.Kind != diff.Collapsed
		}
		if row.lineIndex >= len(file.lines) {
			return "", false
		}
		if right {
			line := file.lines[row.lineIndex].Right
			return line.Text, line.Kind != diff.Collapsed
		}
		line := file.lines[row.lineIndex].Left
		return line.Text, line.Kind != diff.Collapsed
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
	case commitDetailMessageRow:
		textX, textW = 1, d.layoutViewW-2
	case commitDetailNoticeRow:
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
	headingStyle := term.StyleDefault
	if d.Files[row.fileIndex].Stage != CommitDetailStageNone {
		headingStyle = StatusStyle(d.Files[row.fileIndex].Status)
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

func (d *CommitDetailWidget) renderHorizontalScrollbars(surface Surface, r Rect, viewW, y int) bool {
	leftStart, leftW, rightStart, rightW := d.sideGeometry(viewW)
	_, leftInvalidated := d.hscrollbar.Render(
		surface,
		widgets.NewScrollbarGeometry(
			Rect{X: leftStart, Y: y, W: leftW, H: 1},
			Rect{X: r.X + leftStart, Y: r.Y + y, W: leftW, H: 1},
		),
		widgets.NewScrollRange(leftW, d.maxLineW, d.LeftCol),
	)
	for column := 0; column < d.gutterW; column++ {
		surface.SetCell(column, y, term.Cell{Ch: ' '})
	}
	if d.mode == DiffModeUnified {
		_, rightInvalidated := d.rhscroll.Render(surface, widgets.ScrollbarGeometry{}, widgets.NewScrollRange(rightW, rightW, 0))
		return leftInvalidated || rightInvalidated
	}

	dividerX := (viewW - 1) / 2
	_, rightInvalidated := d.rhscroll.Render(
		surface,
		widgets.NewScrollbarGeometry(
			Rect{X: rightStart, Y: y, W: rightW, H: 1},
			Rect{X: r.X + rightStart, Y: r.Y + y, W: rightW, H: 1},
		),
		widgets.NewScrollRange(rightW, d.maxLineW, d.LeftCol),
	)

	surface.SetCell(dividerX, y, term.Cell{Ch: '│', Style: term.StyleBorder})
	for column := dividerX + 1; column < rightStart; column++ {
		surface.SetCell(column, y, term.Cell{Ch: ' '})
	}
	return leftInvalidated || rightInvalidated
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

func (d *CommitDetailWidget) CancelPointerCapture() bool {
	canceled := d.selecting || d.primaryPressed || d.disclosurePressed
	d.selecting = false
	d.primaryPressed = false
	d.disclosurePressed = false
	canceled = d.scrollbar.CancelPointerCapture() || canceled
	canceled = d.hscrollbar.CancelPointerCapture() || canceled
	canceled = d.rhscroll.CancelPointerCapture() || canceled
	d.scrollbarCapture.notify(canceled)
	return canceled
}

func (d *CommitDetailWidget) InvalidatePointerInteraction() bool {
	return d.CancelPointerCapture()
}

func (d *CommitDetailWidget) OwnsPointerCapture() bool {
	return d.selecting || d.primaryPressed || d.disclosurePressed || d.scrollbarCapture.owns(&d.scrollbar, &d.hscrollbar, &d.rhscroll)
}

func (d *CommitDetailWidget) SetPointerCaptureInvalidated(invalidated func()) {
	d.scrollbarCapture.invalidated = invalidated
}

func (d *CommitDetailWidget) HandleEvent(ev tcell.Event) EventResult {
	if newTop, result := d.scrollbar.HandleEvent(ev); result != EventIgnored {
		d.TopLine = newTop
		return result
	}
	if !d.IsWrapped() {
		if newLeft, result := d.hscrollbar.HandleEvent(ev); result != EventIgnored {
			d.LeftCol = newLeft
			return result
		}
		if newLeft, result := d.rhscroll.HandleEvent(ev); result != EventIgnored {
			d.LeftCol = newLeft
			return result
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
			hoveredFile, hoveredGap, overGap := d.contextGapAtScreenY(my)
			hoverChanged := overGap != d.hasHoveredGap || (overGap && (hoveredFile != d.hoveredFile || hoveredGap != d.hoveredGap))
			d.hasHoveredGap = overGap
			if overGap {
				d.hoveredFile = hoveredFile
				d.hoveredGap = hoveredGap
			}
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
				if freshPrimaryPress && !d.selecting && pointInCommitDetailRect(mx, my, d.topControl) {
					if d.allFilesCollapsed() {
						d.ExpandAllFiles()
					} else {
						d.CollapseAllFiles()
					}
					d.disclosurePressed = true
					return EventConsumed
				}
				if freshPrimaryPress && !d.selecting && pointInCommitDetailRect(mx, my, d.stickyControl.rect) {
					d.toggleFile(d.stickyControl.fileIndex)
					d.disclosurePressed = true
					return EventConsumed
				}
				if freshPrimaryPress && !d.selecting {
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
			if hoverChanged {
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
