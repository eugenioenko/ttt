package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
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

	view         *DiffEditorWidget
	oldLines     []string
	newLines     []string
	expandedGaps map[int]bool
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
}

// commitDetailVisualRow is one screen row. A file's diff is a single logical
// row drawn by that file's diff widget; offset is the row within it.
type commitDetailVisualRow struct {
	row          int
	leftStart    int
	offset       int
	continuation bool
}

type commitDetailControl struct {
	rect      Rect
	fileIndex int
}

type commitDetailPreservedSelection struct {
	key  string
	mark diffSelectionMark
}

// CommitDetailWidget renders an entire commit as one virtualized scrollable
// document. It owns one vertical viewport; each file's diff is a
// DiffEditorWidget asked to draw only the rows of it that are on screen.
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
	signs           bool
	signsColor      bool
	gutterStyle     string
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

	// Selection positions point into logical rows and original, unwrapped
	// text. They cover message, heading, and notice rows; a selection inside a
	// file's diff belongs to that file's diff widget.
	selecting         bool
	hasSelection      bool
	selection         diffTextSelection
	lastClickTime     time.Time
	lastClickPos      diffSelPos
	primaryPressed    bool
	disclosurePressed bool
	capturedFile      int

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
		signs:           true,
		signsColor:      true,
		capturedFile:    -1,
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
		signs:           true,
		signsColor:      true,
		capturedFile:    -1,
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

func (d *CommitDetailWidget) SetDiffHighContrast(enabled bool) {
	d.highContrast = enabled
	d.applyViewOptions()
}

func (d *CommitDetailWidget) DiffHighContrast() bool { return d.highContrast }

func (d *CommitDetailWidget) SetDiffCollapsedEmphasis(enabled bool) {
	d.emphasizeGaps = enabled
	d.applyViewOptions()
}

func (d *CommitDetailWidget) SetDiffSigns(enabled bool) {
	d.signs = enabled
	d.applyViewOptions()
}

func (d *CommitDetailWidget) SetDiffSignsColor(enabled bool) {
	d.signsColor = enabled
	d.applyViewOptions()
}

func (d *CommitDetailWidget) setGutterStyle(style string) {
	if d.gutterStyle == style {
		return
	}
	d.gutterStyle = style
	for i := range d.Files {
		if v := d.Files[i].view; v != nil {
			v.setGutterStyle(style)
		}
	}
	d.visualRowsW = -1
}

func (d *CommitDetailWidget) applyViewOptions() {
	for i := range d.Files {
		v := d.Files[i].view
		if v == nil {
			continue
		}
		v.highContrast = d.highContrast
		v.emphasizeGaps = d.emphasizeGaps
		v.signs = d.signs
		v.signsColor = d.signsColor
		v.minGutter = d.gutterW
		v.applyOverlayOptions()
	}
}
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
	d.ClearSelection()
	d.rebuildRows()
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
		view         *DiffEditorWidget
	}
	preservedSelection, hasPreservedSelection := d.captureCurrentChangesSelection()
	preserved := make(map[string]preservedFileState, len(d.Files))
	for i, file := range d.Files {
		state := preservedFileState{expandedGaps: make(map[int]bool), view: file.view}
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
			d.Files[i].view = state.view
		} else {
			d.Files[i].expandedGaps = make(map[int]bool)
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
	for _, file := range d.Files {
		if file.view == nil {
			continue
		}
		if mark, ok := file.view.captureSelection(); ok {
			return commitDetailPreservedSelection{key: commitDetailFileKey(file), mark: mark}, true
		}
	}
	return commitDetailPreservedSelection{}, false
}

func (d *CommitDetailWidget) restoreCurrentChangesSelection(selection commitDetailPreservedSelection) bool {
	for _, file := range d.Files {
		if file.view != nil && commitDetailFileKey(file) == selection.key {
			return file.view.restoreSelection(selection.mark)
		}
	}
	return false
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
			v := d.fileView(file)
			if len(v.Lines) == 0 {
				const noChanges = "No line changes"
				d.rows = append(d.rows, commitDetailRow{kind: commitDetailNoticeRow, text: noChanges, fileIndex: fileIndex})
				d.recordWidth(noChanges)
				continue
			}
			d.rows = append(d.rows, commitDetailRow{kind: commitDetailDiffRow, fileIndex: fileIndex})
			d.maxLineW = max(d.maxLineW, v.maxTextWidth())
			maxLine = max(maxLine, v.maxLineNumber())
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
	d.applyViewOptions()
}

// fileView projects a file's diff into its diff widget, creating the widget
// on first use. The widget keeps its own panes across rebuilds so a refresh
// can restore a selection inside it.
func (d *CommitDetailWidget) fileView(file *CommitDetailFile) *DiffEditorWidget {
	v := file.view
	if v == nil {
		v = NewDiffEditorWidget(file.Path, diff.FileDiff{}, nil, nil, false)
		v.setEmbedded()
		file.view = v
	}
	v.SetSyntaxHighlight(d.SyntaxHighlight && file.Path != "")
	v.mode = d.mode
	v.wrapMode = d.wrapMode
	v.focusLeft = false
	if d.gutterStyle != "" {
		v.setGutterStyle(d.gutterStyle)
	}
	loaded := file.FullFileState == CommitDetailFullFileLoaded
	v.setSource(file.Diff, file.oldLines, file.newLines, loaded, d.contextMode == DiffContextFullFile, file.expandedGaps)
	return v
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

	leftCol := d.LeftCol
	if d.IsWrapped() {
		leftCol = 0
	}
	for screenY := 0; screenY < viewH; {
		visualIndex := d.TopLine + screenY
		if visualIndex >= len(d.visualRows) {
			break
		}
		visual := d.visualRows[visualIndex]
		row := d.rows[visual.row]
		if row.kind != commitDetailDiffRow {
			d.renderRow(surface, visual.row, row, visual, screenY, viewW)
			screenY++
			continue
		}
		n := 1
		for screenY+n < viewH && visualIndex+n < len(d.visualRows) && d.visualRows[visualIndex+n].row == visual.row {
			n++
		}
		if v := d.rowView(visual.row); v != nil {
			rect := Rect{X: r.X, Y: r.Y + screenY, W: viewW, H: n}
			v.renderEmbedded(surface.Sub(Rect{X: 0, Y: screenY, W: viewW, H: n}), rect, visual.offset, leftCol)
		}
		screenY += n
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

func (d *CommitDetailWidget) rowView(rowIndex int) *DiffEditorWidget {
	if rowIndex < 0 || rowIndex >= len(d.rows) {
		return nil
	}
	row := d.rows[rowIndex]
	if row.kind != commitDetailDiffRow || row.fileIndex < 0 || row.fileIndex >= len(d.Files) {
		return nil
	}
	return d.Files[row.fileIndex].view
}

func (d *CommitDetailWidget) layout(w, h int) (viewW, viewH int, showV, showH bool) {
	viewH = h
	for range 3 {
		viewW = max(w, 0)
		if showV {
			viewW--
		}
		if d.visualRowsW != viewW {
			d.visualRows = d.buildVisualRows(viewW)
			d.visualRowsW = viewW
		}
		d.totalVisualRows = len(d.visualRows)
		showH = false
		if !d.IsWrapped() {
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
	wrap := d.IsWrapped()
	visualRows := make([]commitDetailVisualRow, 0, len(d.rows))
	for rowIndex, row := range d.rows {
		if row.kind == commitDetailDiffRow {
			v := d.rowView(rowIndex)
			if v == nil {
				continue
			}
			for offset := range v.embedRows(viewW) {
				visualRows = append(visualRows, commitDetailVisualRow{row: rowIndex, offset: offset})
			}
			continue
		}
		starts := []int{0}
		if wrap {
			switch row.kind {
			case commitDetailMessageHeaderRow, commitDetailHeaderDividerRow, commitDetailSpacerRow:
			case commitDetailHeadingRow:
				starts = diffWrapStarts(row.text, viewW-3)
			case commitDetailMetadataRow, commitDetailMessageRow:
				starts = diffWrapStarts(row.text, viewW-2)
			default:
				starts = diffWrapStarts(row.text, viewW)
			}
		}
		for segment, start := range starts {
			visualRows = append(visualRows, commitDetailVisualRow{row: rowIndex, leftStart: start, continuation: segment > 0})
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

func (d *CommitDetailWidget) rowText(rowIndex int) (string, bool) {
	if rowIndex < 0 || rowIndex >= len(d.rows) {
		return "", false
	}
	row := d.rows[rowIndex]
	switch row.kind {
	case commitDetailMessageRow, commitDetailHeadingRow, commitDetailNoticeRow:
		return row.text, true
	default:
		return "", false
	}
}

func (d *CommitDetailWidget) visualAt(my int) (commitDetailVisualRow, bool) {
	localY := my - d.GetRect().Y
	if localY < 0 || localY >= d.viewH {
		return commitDetailVisualRow{}, false
	}
	visualIndex := d.TopLine + localY
	if visualIndex < 0 || visualIndex >= len(d.visualRows) {
		return commitDetailVisualRow{}, false
	}
	return d.visualRows[visualIndex], true
}

// fileAt reports the file whose diff is drawn under the pointer.
func (d *CommitDetailWidget) fileAt(mx, my int) (int, bool) {
	if pointInCommitDetailRect(mx, my, d.stickyRect) {
		return 0, false
	}
	localX := mx - d.GetRect().X
	if localX < 0 || localX >= d.layoutViewW {
		return 0, false
	}
	visual, ok := d.visualAt(my)
	if !ok || d.rows[visual.row].kind != commitDetailDiffRow || d.rowView(visual.row) == nil {
		return 0, false
	}
	return d.rows[visual.row].fileIndex, true
}

func (d *CommitDetailWidget) screenToSelection(mx, my int) (pos diffSelPos, ok bool) {
	if pointInCommitDetailRect(mx, my, d.stickyRect) {
		return diffSelPos{}, false
	}
	localX := mx - d.GetRect().X
	if localX < 0 || localX >= d.layoutViewW {
		return diffSelPos{}, false
	}
	visual, ok := d.visualAt(my)
	if !ok {
		return diffSelPos{}, false
	}
	rowIndex := visual.row
	textX, textW := 0, d.layoutViewW
	switch d.rows[rowIndex].kind {
	case commitDetailMessageRow:
		textX, textW = 1, d.layoutViewW-2
	case commitDetailNoticeRow:
	case commitDetailHeadingRow:
		textX, textW = 3, d.layoutViewW-3
	default:
		return diffSelPos{}, false
	}
	if textW <= 0 || localX < textX || localX >= textX+textW {
		return diffSelPos{}, false
	}
	text, selectable := d.rowText(rowIndex)
	if !selectable {
		return diffSelPos{}, false
	}
	visualCol := localX - textX
	if !d.IsWrapped() {
		visualCol += d.LeftCol
	}
	return diffSelPos{Line: rowIndex, Col: diffSegmentVisualColToRune(text, visual.leftStart, visualCol)}, true
}

func (d *CommitDetailWidget) selectionText() string {
	if !d.hasSelection {
		return ""
	}
	return d.selection.Text(len(d.rows), d.rowText)
}

func (d *CommitDetailWidget) CopySelection() string {
	if text := d.selectionText(); text != "" {
		d.ClearSelection()
		return text
	}
	for i := range d.Files {
		if v := d.Files[i].view; v != nil && v.hasSelection() {
			return v.CopySelection()
		}
	}
	return ""
}

func (d *CommitDetailWidget) ClearSelection() {
	d.hasSelection = false
	d.selecting = false
	d.clearFileSelections(-1)
}

func (d *CommitDetailWidget) clearFileSelections(except int) {
	for i := range d.Files {
		if v := d.Files[i].view; v != nil && i != except {
			v.ClearSelection()
		}
	}
}

func (d *CommitDetailWidget) selectWordAt(rowIndex, col int) bool {
	text, ok := d.rowText(rowIndex)
	if !ok {
		return false
	}
	return d.selection.SelectWord(rowIndex, col, text)
}

func (d *CommitDetailWidget) renderStickyHeading(surface Surface, viewW int) {
	if viewW <= 0 || d.TopLine <= 0 {
		return
	}
	if d.TopLine >= len(d.visualRows) {
		return
	}
	row := d.rows[d.visualRows[d.TopLine].row]
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

func (d *CommitDetailWidget) OwnsPointerCapture() bool {
	return d.selecting || d.capturedFile >= 0 || d.scrollbar.IsDragging() || d.hscrollbar.IsDragging() || d.rhscroll.IsDragging()
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
			return d.handlePointer(event)
		}
		d.clampScroll()
		return EventConsumed
	}
	return EventIgnored
}

func (d *CommitDetailWidget) handlePointer(event *tcell.EventMouse) EventResult {
	buttons := event.Buttons()
	mx, my := event.Position()
	if d.capturedFile >= 0 {
		v := d.Files[d.capturedFile].view
		result := EventConsumed
		if v != nil {
			result = v.HandleEvent(event)
		}
		if buttons == tcell.ButtonNone {
			d.capturedFile = -1
			d.primaryPressed = false
			return EventConsumed
		}
		return result
	}
	fileIndex, overFile := d.fileAt(mx, my)
	hoverChanged := d.updateHoveredGap(fileIndex, overFile, mx, my)
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
			if overFile {
				if gap, ok := d.Files[fileIndex].view.gapAtPoint(mx, my); ok {
					d.requestFileContext(fileIndex, gap)
					d.disclosurePressed = true
					return EventConsumed
				}
			}
		}
		if freshPrimaryPress && !d.selecting && overFile {
			d.hasSelection = false
			d.clearFileSelections(fileIndex)
			result := d.Files[fileIndex].view.HandleEvent(event)
			if result == EventCaptured {
				d.capturedFile = fileIndex
				return EventCaptured
			}
			return EventConsumed
		}
		pos, ok := d.screenToSelection(mx, my)
		if ok {
			if !d.selecting {
				now := time.Now()
				isDoubleClick := now.Sub(d.lastClickTime) < DoubleClickMs*time.Millisecond &&
					pos.Line == d.lastClickPos.Line && pos.Col == d.lastClickPos.Col
				d.lastClickTime = now
				d.lastClickPos = pos
				d.clearFileSelections(-1)
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

// updateHoveredGap keeps at most one gap row highlighted across all files.
func (d *CommitDetailWidget) updateHoveredGap(fileIndex int, overFile bool, mx, my int) bool {
	changed := false
	for i := range d.Files {
		v := d.Files[i].view
		if v == nil {
			continue
		}
		gap := -1
		if overFile && i == fileIndex {
			if g, ok := v.gapAtPoint(mx, my); ok {
				gap = g
			}
		}
		if v.setHoveredGap(gap) {
			changed = true
		}
	}
	return changed
}

func pointInCommitDetailRect(x, y int, rect Rect) bool {
	return rect.W > 0 && rect.H > 0 && x >= rect.X && x < rect.X+rect.W && y >= rect.Y && y < rect.Y+rect.H
}

func diffWrapStarts(text string, width int) []int {
	return wrapLineSegments([]rune(text), width, diffTabWidth)
}

// drawTextSegment draws one horizontally clipped or wrapped segment. Rune
// indexes remain indexes into the original text so syntax, search, and
// selection spans do not need their own wrapping logic.
func drawTextSegment(surface Surface, x, y, width int, text string, segmentStart, leftVisualCol int, blank term.Cell, cellAt func(runeIndex int, ch rune) term.Cell) {
	for column := 0; column < width; column++ {
		surface.SetCell(x+column, y, blank)
	}
	if segmentStart < 0 {
		return
	}

	runes := []rune(text)
	visualColumn := 0
	for runeIndex := segmentStart; runeIndex < len(runes); runeIndex++ {
		ch := runes[runeIndex]
		cell := cellAt(runeIndex, ch)
		if ch == '\t' {
			nextStop := ((visualColumn / diffTabWidth) + 1) * diffTabWidth
			for tabColumn := visualColumn; tabColumn < nextStop; tabColumn++ {
				drawColumn := tabColumn - leftVisualCol
				if drawColumn >= 0 && drawColumn < width {
					cell.Ch = ' '
					surface.SetCell(x+drawColumn, y, cell)
				}
			}
			visualColumn = nextStop
		} else {
			runeWidth := textwidth.Rune(ch)
			drawColumn := visualColumn - leftVisualCol
			if drawColumn >= 0 && drawColumn < width {
				if runeWidth > 1 && drawColumn == width-1 {
					cell.Ch = ' '
				}
				surface.SetCell(x+drawColumn, y, cell)
			}
			visualColumn += runeWidth
		}
		if visualColumn-leftVisualCol >= width {
			break
		}
	}
}

func diffSegmentVisualColToRune(text string, startCol, visualCol int) int {
	if startCol < 0 {
		return 0
	}
	runes := []rune(text)
	if startCol >= len(runes) {
		return len(runes)
	}
	return startCol + visualColToBufCol(string(runes[startCol:]), visualCol, diffTabWidth)
}
