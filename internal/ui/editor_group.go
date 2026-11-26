package ui

import (
	"context"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"
	"log/slog"
	"slices"

	"github.com/gdamore/tcell/v3"
)

type DiagnosticSeverity int

const (
	DiagError       DiagnosticSeverity = 1
	DiagWarning     DiagnosticSeverity = 2
	DiagInformation DiagnosticSeverity = 3
	DiagHint        DiagnosticSeverity = 4
)

type Diagnostic struct {
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	Severity  DiagnosticSeverity
	Style     term.Style
	Message   string
	Source    string
	Code      any
}

type editorTab struct {
	ID       string
	FilePath string
	// PluginKey identifies a plugin's tab. It is kept apart from FilePath so a
	// file or buffer at the same path is never taken for the plugin's tab.
	PluginKey   string
	IconPath    string
	Title       string
	Buf         *buffer.Buffer
	Cur         *cursor.Cursor
	Vp          *view.Viewport
	Undo        *undo.UndoStack
	Sel         *selection.Selection
	autoClosing *autoClosingState
	Multi       *multicursor.MultiCursor
	Highlighter *highlight.Highlighter
	Diagnostics []Diagnostic
	Folds       *fold.State
	TabSize     int
	UseTabs     bool
	Content     Widget
	Preview     bool
	Virtual     bool
	Placeholder bool
	LineChanges []diff.LineChangeKind
	ReadOnly    bool

	LanguageSelection string
	// Keep document lifecycle and language servers independent of the highlighting selection.
	DetectedLanguage string
	// Diff shows the tab's own buffer as an editable diff against its base.
	Diff *DiffEditorWidget
	// Annotations shows the decorations for the tab's file when it is not
	// showing a diff.
	Annotations    *DiffOverlay
	viewVersion    uint64
	viewText       string
	viewAnchorText string
}

// findSession keeps the editor query after its bar closes and tracks which buffer version the matches describe.
type findSession struct {
	query          string
	options        SearchOptions
	matchesBuffer  *buffer.Buffer
	matchesVersion uint64
	activeMatch    int // Match index in matchesBuffer; -1 when no match is selected.
}

type EditorGroupWidget struct {
	BaseWidget
	TabBar            *TabBarWidget
	Editor            *EditorPaneWidget
	Autocomplete      *AutocompleteWidget
	Hover             *HoverWidget
	SignatureHelp     *SignatureHelpWidget
	tabs              []editorTab
	nextTabIdentity   uint64
	active            int
	lastDocumentPath  string
	pinnedCount       int
	closedTabs        []closedTab
	TabSize           int
	InsertSpaces      bool
	LineNumbers       bool
	IndentGuides      bool
	FullLineSelection bool
	GutterStyle       string
	WordWrap          bool
	PreviewTabs       bool
	WordWrapColumn    int
	WordWrapBreak     string
	LanguageOverrides map[string]config.EditorLanguageOverride
	AutoSurround      config.AutoSurroundSettings
	WordWrapMaxBytes  int64
	// editorInDiff is set while the shared editor is bound to an editable
	// diff, which sets its word wrap from the diff's own wrap mode.
	editorInDiff            bool
	DiffMode                DiffMode
	DiffContext             DiffContextMode
	DiffWordWrap            bool
	MarkdownMaxWidth        int
	DiffHighContrast        bool
	DiffCollapsedEmphasis   bool
	DiffScrollbarGitMarks   bool
	LegacyScrollbarGlyphs   bool
	DiffSigns               bool
	DiffSignsColor          bool
	DiffGutterHighlight     bool
	DiffColorGutterNumbers  bool
	ChevronCollapsed        rune
	ChevronExpanded         rune
	ImageProtocol           string
	ImageCellW              int
	ImageCellH              int
	ImageRedraw             func()
	SyntaxHighlight         bool
	BracketPairColorization bool
	BracketColorStyles      []term.Style
	InsertFinalNewline      bool
	ShowTrailingNewline     bool
	TrimTrailingWhitespace  bool
	UndoDeleteCursorStart   bool
	HighlightActiveBorder   bool
	Borders                 *term.BorderSet
	OnFileOpen              func(path, lang string, lines []string)
	OnFileChange            func(path, lang, text string)
	OnFileClose             func(path, lang string)
	OnContentTabClose       func(id string)
	// FileHandler may claim a file that would open as the binary card; nil keeps the card.
	FileHandler   func(path string, head []byte) Widget
	OnFileHandled func(path string)
	// OnEmpty fires after the last tab closes and the untitled placeholder
	// takes its place.
	OnEmpty           func()
	OnActiveTabChange func()
	OnActivate        func()
	OpenFileElsewhere func(string) bool
	DropTarget        bool
	DropZone          EditorDropZone
	// RelatedGroups includes this group and all peers sharing its buffers.
	RelatedGroups   func() []*EditorGroupWidget
	lastActiveTabID string
	lastActiveBuf   *buffer.Buffer
	// EmptyStateID names a content tab that stands in for the placeholder:
	// like it, it has no close button while it is the only tab.
	EmptyStateID  string
	OnError       func(msg string)
	OnNotify      func(msg string)
	pendingNotify []string
	focused       bool
	find          findSession
	// diagSources holds diagnostics keyed by source ("lsp:<server>", "plugin:<name>")
	// then by file path. Merged per-path into each tab's Diagnostics.
	diagSources map[string]map[string][]Diagnostic
	// decoSources holds each source's decorations by file path.
	decoSources map[string]map[string]*decoSource
	// bookmarksByPath holds bookmarks keyed by file path so they survive a
	// preview tab being replaced (its editorTab struct is discarded entirely).
	bookmarksByPath   map[string]map[int]Bookmark
	bookmarkSyncPath  string
	OnBookmarkChanged func(path string, line int, action string, icon rune, style term.Style)
	OnGutterClick     func(path string, line int) bool
	LinkWorkDir       func() string
	ControlModifiers  *tcell.ModMask
	NativeLinks       bool
	OnOpenURL         func(string)
	OnOpenFileLink    func(string, int, int)
	OnLinkHover       func(*EditorPaneWidget)
	ResolveFileLinks  FileLinkResolver
	// OnDiagnosticsChanged fires whenever any source's diagnostics change, so
	// the Diagnostics panel can rebuild from DiagnosticsByPath().
	OnDiagnosticsChanged func()

	// FoldTask computes an opened file's folds after its first frame, and a
	// large file's folds after an edit once FoldDelay passes without another;
	// without them they are computed on the next frame.
	FoldTask  *asynctask.Task[[]fold.Range]
	FoldDelay *asynctask.Task[struct{}]
	foldBuf   *buffer.Buffer
	foldGen   int
}

// FoldRecomputeDelay lets a run of Enters in a large file recompute folds once.
const FoldRecomputeDelay = 150 * time.Millisecond

func NewEditorGroupWidget(borders *term.BorderSet, tabSize int, lineNumbers bool, gutterStyle string) *EditorGroupWidget {
	editor := NewEditorPaneWidget(
		&buffer.Buffer{Lines: []string{""}},
		&cursor.Cursor{},
		&view.Viewport{},
	)
	editor.TabSize = tabSize
	editor.LineNumbers = lineNumbers
	editor.GutterStyle = gutterStyle

	tabBar := NewTabBarWidget()
	tabBar.Borders = borders
	tabBar.MoreButton = NewMoreButtonWidget()

	g := &EditorGroupWidget{
		TabBar:      tabBar,
		Editor:      editor,
		TabSize:     tabSize,
		LineNumbers: lineNumbers,
		GutterStyle: gutterStyle,
		Borders:     borders,

		DiffSignsColor:      true,
		DiffGutterHighlight: true,
		MarkdownMaxWidth:    80,
		PreviewTabs:         true,
	}
	tabBar.OnTabClick = func(index int) {
		g.SwitchTab(index)
	}
	tabBar.OnTabReorder = func(from, to int) {
		g.MoveTab(from, to)
	}
	tabBar.NormalizeDropTarget = g.NormalizeTabMoveTarget
	tabBar.OnNextTab = func() { g.NextTab() }
	tabBar.OnPrevTab = func() { g.PrevTab() }
	tabBar.OnDoubleClick = func() { g.NewFile() }
	editor.OnBookmarkChange = func(line int, action string, b Bookmark) {
		if g.OnBookmarkChanged != nil {
			g.OnBookmarkChanged(g.ActiveFilePath(), line, action, b.Icon, b.Style)
		}
	}
	editor.OnGutterClick = func(line int) bool {
		return g.OnGutterClick != nil && g.OnGutterClick(g.ActiveFilePath(), line)
	}
	undoStack := g.newUndoStack()
	sel := &selection.Selection{}
	editor.Undo = undoStack
	editor.Selection = sel
	g.tabs = []editorTab{{
		FilePath:    "untitled",
		Buf:         editor.Buf,
		Cur:         editor.Cursor,
		Vp:          editor.Viewport,
		Undo:        undoStack,
		Sel:         sel,
		Virtual:     true,
		Placeholder: true,
	}}
	g.syncTabs()
	return g
}

func (g *EditorGroupWidget) Focusable() bool { return true }

func (g *EditorGroupWidget) SetFocused(focused bool) {
	g.focused = focused
	if focused && g.OnActivate != nil {
		g.OnActivate()
	}
	if t := g.activeTab(); t != nil && t.Content != nil {
		if setter, ok := t.Content.(interface{ SetFocused(bool) }); ok {
			setter.SetFocused(focused)
		}
	}
}

func (g *EditorGroupWidget) ActiveLanguageSelection() string {
	if t := g.activeTab(); t != nil {
		return t.LanguageSelection
	}
	return ""
}

func (g *EditorGroupWidget) ActiveLanguage() string {
	if t := g.activeTab(); t != nil && t.Content == nil {
		return t.DetectedLanguage
	}
	return ""
}

func (g *EditorGroupWidget) SetActiveLanguage(selection string) {
	if g.ActiveBuffer() == nil {
		return
	}
	t := g.activeTab()
	t.LanguageSelection = selection
	t.Highlighter.Close()
	t.Highlighter = g.tabHighlighter(t)
	if t.autoClosing != nil {
		t.autoClosing.syntax.Close()
		t.autoClosing.syntax = nil
	}
	g.syncTabs()
	g.SyncDocumentViews()
}

func (g *EditorGroupWidget) tabHighlighter(t *editorTab) *highlight.Highlighter {
	if !g.SyntaxHighlight {
		return nil
	}
	switch t.LanguageSelection {
	case "":
		return highlight.NewForLines(t.FilePath, t.Buf.Lines)
	case highlight.PlainText:
		return nil
	default:
		return highlight.NewForLanguage(t.LanguageSelection)
	}
}

func (g *EditorGroupWidget) rebuildHighlighter(t *editorTab) {
	t.Highlighter.Close()
	t.Highlighter = nil
	t.DetectedLanguage = ""
	if g.SyntaxHighlight {
		t.Highlighter = highlight.NewForLines(t.FilePath, t.Buf.Lines)
		if t.Highlighter != nil {
			t.DetectedLanguage = t.Highlighter.Language()
		}
	}
	if t.LanguageSelection != "" {
		t.Highlighter.Close()
		t.Highlighter = g.tabHighlighter(t)
	}
}

func (g *EditorGroupWidget) reportError(msg string) {
	if g.OnError != nil {
		g.OnError(msg)
	} else {
		slog.Error(msg)
	}
}

func (g *EditorGroupWidget) notify(msg string) {
	if g.OnNotify != nil {
		g.OnNotify(msg)
	} else {
		g.pendingNotify = append(g.pendingNotify, msg)
	}
}

func (g *EditorGroupWidget) FlushNotifications() {
	if g.OnNotify == nil {
		return
	}
	for _, msg := range g.pendingNotify {
		g.OnNotify(msg)
	}
	g.pendingNotify = nil
}

func (g *EditorGroupWidget) OpenDiff(path string, fd diff.FileDiff, oldLines, newLines []string, extended bool) {
	g.OpenDiffTab(path+" (diff)", "", path, fd, oldLines, newLines, extended)
}

func (g *EditorGroupWidget) OpenDiffTab(tabName, title, path string, fd diff.FileDiff, oldLines, newLines []string, extended bool) {
	widget := NewDiffEditorWidget(path, fd, oldLines, newLines, extended)
	widget.SetSyntaxHighlight(g.SyntaxHighlight)
	g.ApplyDiffDefaults(widget)
	for i, t := range g.tabs {
		if t.FilePath == tabName {
			if closer, ok := t.Content.(interface{ Close() }); ok {
				closer.Close()
			}
			t.Content = widget
			t.IconPath = path
			t.Title = title
			g.tabs[i] = t
			g.SwitchTab(i)
			return
		}
	}
	g.tabs = append(g.tabs, editorTab{
		FilePath: tabName,
		IconPath: path,
		Title:    title,
		Content:  widget,
	})
	g.SwitchTab(len(g.tabs) - 1)
}

func (g *EditorGroupWidget) ApplyDiffDefaults(surface DiffModeSurface) {
	if detail, ok := surface.(*CommitDetailWidget); ok {
		detail.SetMarkdownMaxWidth(g.MarkdownMaxWidth)
	}
	surface.ApplyDefaultMode(g.DiffMode)
	if contextSurface, ok := surface.(DiffContextSurface); ok {
		contextSurface.ApplyDefaultContextMode(g.DiffContext)
	}
	wrapMode := DiffWrapOff
	if g.DiffWordWrap {
		wrapMode = DiffWrapOn
	}
	surface.ApplyDefaultWrapMode(wrapMode)
	surface.SetDiffHighContrast(g.DiffHighContrast)
	surface.SetDiffCollapsedEmphasis(g.DiffCollapsedEmphasis)
	if guides, ok := surface.(indentGuideSurface); ok {
		guides.SetIndentGuides(g.IndentGuides)
	}
	if sel, ok := surface.(fullLineSelectionSurface); ok {
		sel.SetFullLineSelection(g.FullLineSelection)
	}
	if d, ok := surface.(diffSignsSurface); ok {
		d.SetDiffSigns(g.DiffSigns)
		d.SetDiffSignsColor(g.DiffSignsColor)
		d.SetDiffGutterHighlight(g.DiffGutterHighlight)
		d.SetDiffColorGutterNumbers(g.DiffColorGutterNumbers)
	}
	if c, ok := surface.(chevronSurface); ok {
		c.SetChevrons(g.ChevronCollapsed, g.ChevronExpanded)
	}
	if m, ok := surface.(scrollMarkSurface); ok {
		m.SetDiffScrollMarks(g.DiffScrollbarGitMarks, g.LegacyScrollbarGlyphs)
	}
}

func (g *EditorGroupWidget) SetIndentGuides(enabled bool) {
	g.IndentGuides = enabled
	if g.Editor != nil {
		g.Editor.IndentGuides = enabled
	}
	for _, surface := range g.diffSurfaces() {
		if guides, ok := surface.(indentGuideSurface); ok {
			guides.SetIndentGuides(enabled)
		}
	}
}

type indentGuideSurface interface {
	SetIndentGuides(bool)
}

func (g *EditorGroupWidget) SetFullLineSelection(enabled bool) {
	g.FullLineSelection = enabled
	if g.Editor != nil {
		g.Editor.FullLineSelection = enabled
	}
	for _, surface := range g.diffSurfaces() {
		if sel, ok := surface.(fullLineSelectionSurface); ok {
			sel.SetFullLineSelection(enabled)
		}
	}
}

type fullLineSelectionSurface interface {
	SetFullLineSelection(bool)
}

func (g *EditorGroupWidget) SetFileIconMode(mode string) {
	g.TabBar.Icons = mode
	for i := range g.tabs {
		g.applyFileIconMode(g.tabs[i].Content)
	}
}

func (g *EditorGroupWidget) applyFileIconMode(content Widget) {
	if setter, ok := content.(interface{ SetFileIconMode(string) }); ok {
		setter.SetFileIconMode(g.TabBar.Icons)
	}
}

func (g *EditorGroupWidget) SetDiffDefaults(mode DiffMode, contextMode DiffContextMode, wordWrap bool) {
	g.DiffMode = mode
	g.DiffContext = contextMode
	g.DiffWordWrap = wordWrap
	for _, surface := range g.diffSurfaces() {
		g.ApplyDiffDefaults(surface)
	}
}

func (g *EditorGroupWidget) SetMarkdownMaxWidth(width int) {
	g.MarkdownMaxWidth = width
	for _, surface := range g.diffSurfaces() {
		if detail, ok := surface.(*CommitDetailWidget); ok {
			detail.SetMarkdownMaxWidth(width)
		}
	}
}

func (g *EditorGroupWidget) SetDiffHighContrast(enabled bool) {
	g.DiffHighContrast = enabled
	for _, surface := range g.diffSurfaces() {
		surface.SetDiffHighContrast(enabled)
	}
}

func (g *EditorGroupWidget) SetDiffCollapsedEmphasis(enabled bool) {
	g.DiffCollapsedEmphasis = enabled
	for _, surface := range g.diffSurfaces() {
		surface.SetDiffCollapsedEmphasis(enabled)
	}
}

func (g *EditorGroupWidget) SetDiffSigns(enabled bool) {
	g.DiffSigns = enabled
	g.eachDiffSigns(func(d diffSignsSurface) { d.SetDiffSigns(enabled) })
}

func (g *EditorGroupWidget) SetDiffSignsColor(enabled bool) {
	g.DiffSignsColor = enabled
	g.eachDiffSigns(func(d diffSignsSurface) { d.SetDiffSignsColor(enabled) })
}

func (g *EditorGroupWidget) SetDiffGutterHighlight(enabled bool) {
	g.DiffGutterHighlight = enabled
	g.eachDiffSigns(func(d diffSignsSurface) { d.SetDiffGutterHighlight(enabled) })
}

func (g *EditorGroupWidget) SetDiffColorGutterNumbers(enabled bool) {
	g.DiffColorGutterNumbers = enabled
	g.eachDiffSigns(func(d diffSignsSurface) { d.SetDiffColorGutterNumbers(enabled) })
}

func (g *EditorGroupWidget) eachDiffSigns(apply func(diffSignsSurface)) {
	for _, surface := range g.diffSurfaces() {
		if d, ok := surface.(diffSignsSurface); ok {
			apply(d)
		}
	}
}

func (g *EditorGroupWidget) SetDiffScrollMarks(enabled, legacyGlyphs bool) {
	g.DiffScrollbarGitMarks, g.LegacyScrollbarGlyphs = enabled, legacyGlyphs
	for _, surface := range g.diffSurfaces() {
		if m, ok := surface.(scrollMarkSurface); ok {
			m.SetDiffScrollMarks(enabled, legacyGlyphs)
		}
	}
}

// diffSurfaces are the diff tabs' widgets and the editable diffs of file tabs.
func (g *EditorGroupWidget) diffSurfaces() []DiffModeSurface {
	var out []DiffModeSurface
	for _, tab := range g.tabs {
		if surface, ok := tab.Content.(DiffModeSurface); ok {
			out = append(out, surface)
		}
		if tab.Diff != nil {
			out = append(out, tab.Diff)
		}
	}
	return out
}

// SetChevrons sets the collapsed and expanded glyphs for the editor's fold
// gutter and every diff tab's gap rows.
func (g *EditorGroupWidget) SetChevrons(collapsed, expanded rune) {
	g.ChevronCollapsed, g.ChevronExpanded = collapsed, expanded
	if g.Editor != nil {
		g.Editor.FoldChevronCollapsed, g.Editor.FoldChevronExpanded = collapsed, expanded
	}
	for _, surface := range g.diffSurfaces() {
		if c, ok := surface.(chevronSurface); ok {
			c.SetChevrons(collapsed, expanded)
		}
	}
}

type chevronSurface interface {
	SetChevrons(collapsed, expanded rune)
}

type diffSignsSurface interface {
	SetDiffSigns(bool)
	SetDiffSignsColor(bool)
	SetDiffGutterHighlight(bool)
	SetDiffColorGutterNumbers(bool)
	setGutterStyle(string)
}

type scrollMarkSurface interface {
	SetDiffScrollMarks(enabled, legacyGlyphs bool)
}

// An empty protocol means auto-detection.
func (g *EditorGroupWidget) SetImageProtocol(protocol string) {
	g.ImageProtocol = protocol
	for _, tab := range g.tabs {
		if iv, ok := tab.Content.(*ImageViewWidget); ok {
			iv.Protocol = protocol
		}
	}
}

// w and h are pixels per cell.
func (g *EditorGroupWidget) SetImageCellSize(w, h int) {
	g.ImageCellW, g.ImageCellH = w, h
	for _, tab := range g.tabs {
		if iv, ok := tab.Content.(*ImageViewWidget); ok {
			iv.CellW, iv.CellH = w, h
		}
	}
}

func (g *EditorGroupWidget) SetImageRedraw(redraw func()) {
	g.ImageRedraw = redraw
	for _, tab := range g.tabs {
		if iv, ok := tab.Content.(*ImageViewWidget); ok {
			iv.Redraw = redraw
		}
	}
}

func (g *EditorGroupWidget) IsEditorActive() bool {
	t := g.activeTab()
	return t != nil && t.Content == nil
}

func (g *EditorGroupWidget) ActiveDiffWidget() *DiffEditorWidget {
	t := g.activeTab()
	if t == nil || t.Content == nil {
		return nil
	}
	dv, _ := t.Content.(*DiffEditorWidget)
	return dv
}

func (g *EditorGroupWidget) ActiveCommitDetailWidget() *CommitDetailWidget {
	t := g.activeTab()
	if t == nil || t.Content == nil {
		return nil
	}
	detail, _ := t.Content.(*CommitDetailWidget)
	return detail
}

func (g *EditorGroupWidget) ActiveCurrentChangesWidget() *CommitDetailWidget {
	detail := g.ActiveCommitDetailWidget()
	if detail != nil && detail.CurrentChanges {
		return detail
	}
	return nil
}

func (g *EditorGroupWidget) ActiveDiffModeSurface() DiffModeSurface {
	if d := g.activeInlineDiff(); d != nil {
		return d
	}
	t := g.activeTab()
	if t == nil || t.Content == nil {
		return nil
	}
	surface, _ := t.Content.(DiffModeSurface)
	return surface
}

func (g *EditorGroupWidget) ActiveDiffContextSurface() DiffContextSurface {
	if d := g.activeInlineDiff(); d != nil {
		return d
	}
	t := g.activeTab()
	if t == nil || t.Content == nil {
		return nil
	}
	surface, _ := t.Content.(DiffContextSurface)
	return surface
}

func (g *EditorGroupWidget) DiffWidgetByTab(tabName string) *DiffEditorWidget {
	for _, t := range g.tabs {
		if t.FilePath == tabName {
			dv, _ := t.Content.(*DiffEditorWidget)
			return dv
		}
	}
	return nil
}

func (g *EditorGroupWidget) CommitDetailWidgetByTab(tabName string) *CommitDetailWidget {
	for _, t := range g.tabs {
		if t.FilePath == tabName {
			detail, _ := t.Content.(*CommitDetailWidget)
			return detail
		}
	}
	return nil
}

func (g *EditorGroupWidget) CurrentChangesWidgetByTab(tabName string) *CommitDetailWidget {
	detail := g.CommitDetailWidgetByTab(tabName)
	if detail != nil && detail.CurrentChanges {
		return detail
	}
	return nil
}

func (g *EditorGroupWidget) DiffTabSources() []DiffSearchSource {
	var result []DiffSearchSource
	for _, t := range g.tabs {
		if dv, ok := t.Content.(*DiffEditorWidget); ok {
			iconPath := t.IconPath
			if iconPath == "" {
				iconPath = dv.FilePath
			}
			result = append(result, DiffSearchSource{TabName: t.FilePath, IconPath: iconPath, Lines: dv.CombinedLines()})
		}
	}
	return result
}

func editorCursorVisible(e *EditorPaneWidget) bool {
	x, y := e.CursorX, e.CursorY
	r := e.GetRect()
	if !r.Contains(x, y) || y >= r.Y+e.Viewport.Height {
		return false
	}
	if e.WordWrap {
		return true
	}
	gutter := e.GutterWidth()
	return x >= r.X+gutter && x < r.X+gutter+e.Viewport.Width
}

func (g *EditorGroupWidget) WantsRawKeys() bool {
	rk, ok := g.activeContent().(RawKeyConsumer)
	return ok && rk.WantsRawKeys()
}

func (g *EditorGroupWidget) ActiveTerminalTab() *TerminalTabWidget {
	tt, _ := g.activeContent().(*TerminalTabWidget)
	return tt
}

func (g *EditorGroupWidget) CursorPosition() (int, int, bool) {
	// Content tabs (settings, plugin panels) own their own cursor.
	if t := g.activeTab(); t != nil && t.Content != nil {
		if cp, ok := t.Content.(widgets.CursorPositioner); ok {
			return cp.CursorPosition()
		}
	}
	if g.IsEditorActive() {
		if d := g.activeInlineDiff(); d != nil && d.headFocused() {
			return d.left.CursorX, d.left.CursorY, editorCursorVisible(d.left)
		}
		if g.Editor.isMultiActive() {
			return 0, 0, false
		}
		return g.Editor.CursorX, g.Editor.CursorY, editorCursorVisible(g.Editor)
	}
	return 0, 0, false
}

func (g *EditorGroupWidget) SetTabSize(size int) {
	if t := g.activeTab(); t != nil {
		t.TabSize = size
	}
	g.Editor.TabSize = size
}

func (g *EditorGroupWidget) SetUseTabs(useTabs bool) {
	if t := g.activeTab(); t != nil {
		t.UseTabs = useTabs
	}
	g.Editor.UseTabs = useTabs
}

// RefreshHighlighters rebuilds every file tab's highlighter after the set of
// available grammars changes.
func (g *EditorGroupWidget) RefreshHighlighters() {
	if !g.SyntaxHighlight {
		return
	}
	for i := range g.tabs {
		t := &g.tabs[i]
		if t.Content != nil || t.Buf == nil || (t.FilePath == "" && t.LanguageSelection == "") {
			continue
		}
		prevLang := t.DetectedLanguage
		g.rebuildHighlighter(t)
		lang := t.DetectedLanguage
		if lang == prevLang || t.Virtual || t.FilePath == "" {
			continue
		}
		if g.OnFileClose != nil && prevLang != "" {
			g.OnFileClose(t.FilePath, prevLang)
		}
		if g.OnFileOpen != nil && lang != "" {
			g.OnFileOpen(t.FilePath, lang, t.Buf.Lines)
		}
	}
	g.syncTabs()
}

func (g *EditorGroupWidget) wrapsTab(t *editorTab) bool {
	enabled, _, _ := g.tabWordWrap(t)
	return enabled
}

func (g *EditorGroupWidget) tabWordWrap(t *editorTab) (enabled bool, column int, words bool) {
	language := ""
	if t != nil && t.Buf != nil {
		language = highlight.DetectLanguageID(t.FilePath, t.Buf.Lines)
	}
	s := config.EditorSettings{WordWrap: g.WordWrap, WordWrapColumn: g.WordWrapColumn, WordWrapBreak: g.WordWrapBreak, LanguageOverrides: g.LanguageOverrides}
	enabled, column = s.ResolveWordWrap(language)
	words = s.ResolveWordWrapBreak(language) == config.WordWrapBreakWord
	if !enabled {
		return false, column, words
	}
	return g.WordWrapMaxBytes <= 0 || t == nil || t.Buf == nil || t.Buf.DiskSize() <= g.WordWrapMaxBytes, column, words
}

func (g *EditorGroupWidget) ApplyFileWordWrap() {
	t := g.activeTab()
	if g.Editor == nil || t == nil || t.Content != nil || t.Diff != nil {
		return
	}
	enabled, column, words := g.tabWordWrap(t)
	g.Editor.WordWrap, g.Editor.WordWrapColumn, g.Editor.WrapWords = enabled, column, words
}

// ActiveFileWordWrap reports whether the active file tab should wrap.
func (g *EditorGroupWidget) ActiveFileWordWrap() bool {
	t := g.activeTab()
	if t != nil && t.Content == nil && t.Diff == nil {
		return g.wrapsTab(t)
	}
	return g.WordWrap
}

// ActiveBuffer returns the buffer backing the active tab, or nil if the active
// tab is not a text buffer (e.g. a diff view).
func (g *EditorGroupWidget) ActiveBuffer() *buffer.Buffer {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return nil
	}
	return t.Buf
}

func (g *EditorGroupWidget) IsDirty() bool {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return false
	}
	return t.Buf.Dirty
}

func (g *EditorGroupWidget) AnyDirty() bool {
	for _, t := range g.tabs {
		if t.Content != nil {
			continue
		}
		if t.Buf.Dirty {
			return true
		}
	}
	return false
}

func (g *EditorGroupWidget) SetSearch(query string, opts SearchOptions, matches []FindMatch) {
	g.find = findSession{}
	g.applySearch(query, opts, matches)
}

func (g *EditorGroupWidget) SetFindSearch(query string, matches []FindMatch, opts SearchOptions) {
	g.find = findSession{query: query, options: opts, activeMatch: -1}
	if !g.IsEditorActive() {
		return
	}
	g.find.matchesBuffer = g.Editor.Buf
	g.find.matchesVersion = g.Editor.Buf.Version()
	g.applySearch(query, opts, matches)
}

func (g *EditorGroupWidget) applySearch(query string, opts SearchOptions, matches []FindMatch) {
	if !g.IsEditorActive() {
		return
	}
	g.Editor.SearchQuery = query
	g.Editor.SearchOptions = opts
	g.Editor.SearchMatches = matches
	g.Editor.SearchActive = 0
	g.Editor.buildSearchIndex()
}

func (g *EditorGroupWidget) SetSearchActive(idx int) {
	if !g.IsEditorActive() {
		return
	}
	g.Editor.SearchActive = idx
	if g.find.query != "" && g.find.matchesBuffer == g.Editor.Buf {
		g.find.activeMatch = idx
	}
}

func (g *EditorGroupWidget) ClearSearch() {
	g.find = findSession{}
	g.clearSearchState()
}

func (g *EditorGroupWidget) ClearFindHighlights() {
	active := g.Editor.SearchActive
	g.clearSearchState()
	g.Editor.SearchActive = active
	g.find.activeMatch = active
}

// clearSearchState also runs while an image or binary tab is active: the
// shared editor keeps the last text tab's highlights and shows them again
// when that tab comes back.
func (g *EditorGroupWidget) clearSearchState() {
	g.Editor.SearchQuery = ""
	g.Editor.SearchOptions = SearchOptions{}
	g.Editor.SearchMatches = nil
	g.Editor.SearchActive = 0
	g.Editor.searchByLine = nil
}

func (g *EditorGroupWidget) refreshFindSearch() bool {
	if !g.IsEditorActive() || g.find.query == "" ||
		(g.find.matchesBuffer == g.Editor.Buf && g.Editor.SearchQuery == g.find.query &&
			g.find.matchesVersion == g.Editor.Buf.Version()) {
		return false
	}
	matches, err := FindInLines(g.Editor.Buf.Lines, g.find.query, g.find.options)
	if err != nil {
		return false
	}
	bufferChanged := g.find.matchesBuffer != g.Editor.Buf
	active := g.find.activeMatch
	if bufferChanged {
		active = -1
	}
	g.find.matchesBuffer = g.Editor.Buf
	g.find.matchesVersion = g.Editor.Buf.Version()
	g.applySearch(g.find.query, g.find.options, matches)
	if len(matches) == 0 {
		g.find.activeMatch = -1
	} else {
		if active < 0 || active >= len(matches) {
			active = 0
		}
		g.find.activeMatch = active
		g.Editor.SearchActive = active
	}
	return bufferChanged
}

// PositionAt maps screen coordinates to a 0-based buffer line/col and the word
// under that position in the active editor. ok is false when the editor is not
// active or the coordinates fall outside the editor content area.
func (g *EditorGroupWidget) PositionAt(mx, my int) (line, col int, word string, ok bool) {
	if !g.IsEditorActive() || g.Editor == nil || g.Editor.Buf == nil {
		return 0, 0, "", false
	}
	r := g.Editor.GetRect()
	if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
		return 0, 0, "", false
	}
	line, col = g.Editor.mouseToPos(r, mx, my)
	if line < 0 || line >= len(g.Editor.Buf.Lines) {
		return line, col, "", true
	}
	return line, col, wordAt(g.Editor.Buf.Lines[line], col), true
}

// SetDiagnostics replaces the LSP diagnostics for path. It is a thin wrapper
// over SetDiagnosticsSource so LSP and plugin diagnostics merge on the same tab.
func (g *EditorGroupWidget) SetDiagnostics(path string, diags []Diagnostic) {
	g.SetDiagnosticsSource("lsp", path, diags)
}

// SetDiagnosticsSource stores the diagnostics for a given source/path pair and
// recomputes the merged diagnostics shown for that path.
func (g *EditorGroupWidget) SetDiagnosticsSource(source, path string, diags []Diagnostic) {
	if g.diagSources == nil {
		g.diagSources = make(map[string]map[string][]Diagnostic)
	}
	byPath := g.diagSources[source]
	if byPath == nil {
		byPath = make(map[string][]Diagnostic)
		g.diagSources[source] = byPath
	}
	if len(diags) == 0 {
		delete(byPath, path)
	} else {
		byPath[path] = diags
	}
	g.recomputeDiagnostics(path)
	if g.OnDiagnosticsChanged != nil {
		g.OnDiagnosticsChanged()
	}
}

// DiagnosticsByPath returns every diagnostic across all sources, merged by
// file path — used to populate the Diagnostics panel.
func (g *EditorGroupWidget) DiagnosticsByPath() map[string][]Diagnostic {
	merged := make(map[string][]Diagnostic)
	for _, byPath := range g.diagSources {
		for path, diags := range byPath {
			merged[path] = append(merged[path], diags...)
		}
	}
	return merged
}

func (g *EditorGroupWidget) DiagnosticsForSource(source, path string) []Diagnostic {
	return append([]Diagnostic(nil), g.diagSources[source][path]...)
}

// ClearDiagnosticsSource removes all diagnostics published by source and
// recomputes every path it touched.
func (g *EditorGroupWidget) ClearDiagnosticsSource(source string) {
	byPath, ok := g.diagSources[source]
	if !ok {
		return
	}
	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	delete(g.diagSources, source)
	for _, path := range paths {
		g.recomputeDiagnostics(path)
	}
	if g.OnDiagnosticsChanged != nil {
		g.OnDiagnosticsChanged()
	}
}

// recomputeDiagnostics merges every source's diagnostics for path and applies
// the result to the matching tab (and the active editor if it is that tab).
func (g *EditorGroupWidget) recomputeDiagnostics(path string) {
	var merged []Diagnostic
	for _, byPath := range g.diagSources {
		if diags := byPath[path]; len(diags) > 0 {
			merged = append(merged, diags...)
		}
	}
	for i := range g.tabs {
		if g.tabs[i].FilePath == path {
			g.tabs[i].Diagnostics = merged
			if i == g.active {
				g.Editor.Diagnostics = merged
				g.Editor.buildDiagIndex()
			}
			return
		}
	}
}

// SetLineChanges updates the git gutter indicators for the tab with the given path.
func (g *EditorGroupWidget) SetLineChanges(path string, changes []diff.LineChangeKind) {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path {
			g.tabs[i].LineChanges = changes
			if i == g.active {
				g.Editor.LineChanges = changes
			}
			return
		}
	}
}

func (g *EditorGroupWidget) FindNext() {
	if !g.IsEditorActive() {
		return
	}
	refreshed := g.refreshFindSearch()
	if len(g.Editor.SearchMatches) == 0 {
		return
	}
	g.Editor.ResetCursorGoal()
	cur := g.Editor.SearchActive
	if refreshed {
		cur = -1
	}
	cur = (cur + 1) % len(g.Editor.SearchMatches)
	g.SetSearchActive(cur)
	m := g.Editor.SearchMatches[cur]
	g.Editor.ExpandFoldContaining(m.Line)
	g.Editor.Cursor.Line = m.Line
	g.Editor.Cursor.Col = m.Col
	g.Editor.scrollViewport()
}

func (g *EditorGroupWidget) ReplaceMatch(match FindMatch, replacement string) {
	if !g.IsEditorActive() {
		return
	}
	if match.Line < 0 || match.Line >= len(g.Editor.Buf.Lines) {
		return
	}
	if g.Editor.Undo != nil {
		g.Editor.Undo.BeginTransaction()
		defer g.Editor.Undo.EndTransaction()
	}
	runes := []rune(g.Editor.Buf.Lines[match.Line])
	endCol := min(match.Col+match.Len, len(runes))
	g.Editor.exec(&undo.DeleteSelectionCommand{
		StartLine: match.Line, StartCol: match.Col,
		EndLine: match.Line, EndCol: endCol,
	})
	if replacement != "" {
		g.Editor.exec(&undo.InsertStringCommand{
			Line: match.Line, Col: match.Col, Text: replacement,
		})
	}
	g.Editor.Cursor.Line = match.Line
	g.Editor.Cursor.Col = match.Col + len([]rune(replacement))
	g.Editor.scrollViewport()
}

func (g *EditorGroupWidget) ReplaceAll(query, replacement string, opts SearchOptions) {
	if !g.IsEditorActive() || query == "" {
		return
	}
	matches, _ := FindInLines(g.Editor.Buf.Lines, query, opts)
	if g.Editor.Undo != nil {
		g.Editor.Undo.BeginTransaction()
		defer g.Editor.Undo.EndTransaction()
	}
	for _, match := range slices.Backward(matches) {
		g.ReplaceMatch(match, replacement)
	}
}

func (g *EditorGroupWidget) FindPrev() {
	if !g.IsEditorActive() {
		return
	}
	refreshed := g.refreshFindSearch()
	if len(g.Editor.SearchMatches) == 0 {
		return
	}
	g.Editor.ResetCursorGoal()
	cur := g.Editor.SearchActive
	if refreshed {
		cur = 0
	}
	cur = (cur - 1 + len(g.Editor.SearchMatches)) % len(g.Editor.SearchMatches)
	g.SetSearchActive(cur)
	m := g.Editor.SearchMatches[cur]
	g.Editor.ExpandFoldContaining(m.Line)
	g.Editor.Cursor.Line = m.Line
	g.Editor.Cursor.Col = m.Col
	g.Editor.scrollViewport()
}

func (g *EditorGroupWidget) scheduleFolds() {
	t := g.activeTab()
	// Wrapped panes do not fold, so folds wait until wrap is turned off.
	if t == nil || t.Folds == nil || !t.Folds.Pending() || g.Editor.WordWrap {
		return
	}
	if g.FoldTask == nil || (t.Folds.Stale() && g.FoldDelay == nil) {
		t.Folds.Resolve()
		return
	}
	gen := t.Folds.Generation()
	if g.foldBuf == t.Buf && g.foldGen == gen {
		return
	}
	buf, folds := t.Buf, t.Folds
	g.foldBuf, g.foldGen = buf, gen
	compute := func() {
		lines := slices.Clone(buf.Lines)
		g.FoldTask.Schedule(func(context.Context) ([]fold.Range, error) {
			return fold.ComputeIndentRanges(lines), nil
		}, func(ranges []fold.Range, _ error) {
			g.foldBuf = nil
			// Only edits that add or remove lines start a new generation, so
			// ranges for the same generation and line count are as current as
			// an edit would leave them. Rejecting them on every keystroke would
			// keep folds from landing while the user types.
			if folds.Pending() && folds.Generation() == gen && len(buf.Lines) == len(lines) {
				folds.SetRanges(ranges)
			}
		})
	}
	if !folds.Stale() {
		compute()
		return
	}
	g.FoldDelay.Schedule(func(context.Context) (struct{}, error) {
		return struct{}{}, nil
	}, func(_ struct{}, err error) {
		if err == nil && folds.Pending() && folds.Generation() == gen {
			compute()
		}
	})
}

// ContentRect is the screen rect below the tab bar that tab content fills.
func (g *EditorGroupWidget) ContentRect() Rect {
	r := g.GetRect()
	height := g.TabBar.Height()
	return Rect{X: r.X, Y: r.Y + height, W: r.W, H: max(r.H-height, 0)}
}

func (g *EditorGroupWidget) Render(surface Surface) {
	g.scheduleFolds()
	g.syncTabs()
	w, h := surface.Size()
	r := g.GetRect()
	tabBarH := g.TabBar.Height()

	if w <= 0 || h <= tabBarH {
		g.TabBar.InvalidatePointerInteraction()
		return
	}

	g.TabBar.SetRect(Rect{X: r.X, Y: r.Y, W: r.W, H: tabBarH})
	tabSurface := surface.Sub(Rect{X: 0, Y: 0, W: w, H: tabBarH})
	split := len(g.relatedGroups()) > 1
	g.TabBar.ShowGroupFocus = split && g.HighlightActiveBorder
	g.TabBar.GroupActive = g.focused
	g.Editor.Passive = split && !g.focused
	g.TabBar.Render(tabSurface)
	if g.effectiveDropZone() == EditorDropCenter {
		borders := term.SingleBorderSet()
		if g.Borders != nil {
			borders = *g.Borders
		}
		for x := 0; x < w; x++ {
			tabSurface.SetCell(x, 0, term.Cell{Ch: borders.Horizontal, Style: term.StyleBorderActive})
		}
	}

	contentH := h - tabBarH
	contentRect := g.ContentRect()
	contentSurface := surface.Sub(Rect{X: 0, Y: tabBarH, W: w, H: contentH})

	t := g.activeTab()
	if t == nil {
		return
	}
	if d, ok := t.Content.(diffSignsSurface); ok {
		d.setGutterStyle(g.GutterStyle)
	}
	if d := t.diffWidget(); d != nil {
		for _, p := range d.viewPanes() {
			g.configurePaneLinks(p, d.FilePath)
		}
	}
	if detail, ok := t.Content.(*CommitDetailWidget); ok {
		detail.configurePaneLinks = g.configurePaneLinks
	}
	if t.Content != nil {
		t.Content.SetRect(contentRect)
		t.Content.Render(contentSurface)
	} else if t.Diff != nil {
		t.Diff.setGutterStyle(g.GutterStyle)
		t.Diff.SetRect(contentRect)
		t.Diff.Render(contentSurface)
	} else {
		path := t.FilePath
		if t.Virtual {
			path = ""
		}
		g.configurePaneLinks(g.Editor, path)
		g.Editor.SetRect(contentRect)
		g.Editor.Render(contentSurface)
	}

	if g.SignatureHelp != nil && g.SignatureHelp.Label != "" {
		g.SignatureHelp.AnchorX = g.Editor.CursorX - r.X
		g.SignatureHelp.AnchorY = g.Editor.CursorY - r.Y
		g.SignatureHelp.Borders = g.Borders
		g.SignatureHelp.Render(surface)
	}

	if g.Autocomplete != nil && len(g.Autocomplete.Items) > 0 {
		g.Autocomplete.AnchorX = g.Editor.CursorX - r.X
		g.Autocomplete.AnchorY = g.Editor.CursorY - r.Y
		g.Autocomplete.Borders = g.Borders
		g.Autocomplete.Render(surface)
	}

	if g.Hover != nil && g.Hover.HasContent() {
		g.Hover.OffsetX = r.X
		g.Hover.OffsetY = r.Y
		g.Hover.Borders = g.Borders
		g.Hover.Render(surface)
	}

	g.renderDropPreview(contentSurface)
}

func (g *EditorGroupWidget) CancelPointerCapture() bool {
	canceled := g.TabBar.CancelPointerCapture()
	t := g.activeTab()
	if t != nil {
		if t.Content != nil {
			canceled = widgets.CancelPointerCapture(t.Content) || canceled
		} else if t.Diff != nil {
			canceled = widgets.CancelPointerCapture(t.Diff) || canceled
		}
	}
	if g.Editor != nil {
		canceled = widgets.CancelPointerCapture(g.Editor) || canceled
	}
	return canceled
}

func (g *EditorGroupWidget) OwnsPointerCapture() bool {
	if g.TabBar.OwnsPointerCapture() {
		return true
	}
	return g.contentOwnsPointerCapture()
}

func (g *EditorGroupWidget) PointerGestureActive() bool {
	return g.TabBar.PointerGestureActive()
}

func (g *EditorGroupWidget) contentOwnsPointerCapture() bool {
	t := g.activeTab()
	switch {
	case t == nil:
		return false
	case t.Content != nil:
		owner, ok := t.Content.(widgets.PointerCaptureOwner)
		return ok && owner.OwnsPointerCapture()
	case t.Diff != nil:
		return t.Diff.OwnsPointerCapture()
	}
	return g.Editor != nil && g.Editor.OwnsPointerCapture()
}

func (g *EditorGroupWidget) InvalidatePointerInteraction() bool {
	return g.TabBar.InvalidatePointerInteraction()
}

func (g *EditorGroupWidget) SetPointerCaptureInvalidated(invalidated func()) {
	g.TabBar.SetPointerCaptureInvalidated(invalidated)
}

func (g *EditorGroupWidget) HandleEvent(ev tcell.Event) EventResult {
	if mouse, ok := ev.(*tcell.EventMouse); ok && mouse.Buttons()&(tcell.Button1|tcell.Button2|tcell.Button3) != 0 {
		x, y := mouse.Position()
		if g.GetRect().Contains(x, y) && g.OnActivate != nil {
			g.OnActivate()
		}
	}
	if _, ok := ev.(*tcell.EventMouse); ok && g.TabBar.PointerGestureActive() {
		return g.TabBar.HandleEvent(ev)
	}
	if g.Hover != nil {
		result := g.Hover.HandleEvent(ev)
		if result == EventDismissed {
			g.Hover = nil
		}
		if result == EventConsumed {
			return EventConsumed
		}
	}
	if g.SignatureHelp != nil {
		if kev, ok := ev.(*tcell.EventKey); ok && kev.Key() == tcell.KeyEscape && g.Autocomplete == nil {
			g.SignatureHelp = nil
			return EventConsumed
		}
	}
	if g.Autocomplete != nil {
		result := g.Autocomplete.HandleEvent(ev)
		if result == EventConsumed {
			return EventConsumed
		}
	}
	if _, isMouse := ev.(*tcell.EventMouse); !isMouse || !g.contentOwnsPointerCapture() {
		result := g.TabBar.HandleEvent(ev)
		slog.Debug("editorGroup", "tabBarResult", result)
		if result != EventIgnored {
			return result
		}
	}
	t := g.activeTab()
	if t == nil {
		return EventIgnored
	}
	if t.Content != nil {
		if result := t.Content.HandleEvent(ev); result != EventIgnored {
			return result
		}
		if _, ok := ev.(*tcell.EventMouse); ok {
			return EventConsumed
		}
		return EventIgnored
	}
	var result EventResult
	if t.Diff != nil {
		result = t.Diff.HandleEvent(ev)
	} else {
		result = g.Editor.HandleEvent(ev)
	}
	g.saveMultiState()
	return result
}
