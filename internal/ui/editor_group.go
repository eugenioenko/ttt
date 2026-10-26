package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
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
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	// Diff shows the tab's own buffer as an editable diff against its base.
	Diff *DiffEditorWidget
	// Annotations shows the decorations for the tab's file when it is not
	// showing a diff.
	Annotations *DiffOverlay
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
	pinnedCount       int
	TabSize           int
	InsertSpaces      bool
	LineNumbers       bool
	GutterStyle       string
	WordWrap          bool
	WordWrapColumn    int
	LanguageOverrides map[string]config.EditorLanguageOverride
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
	lastActiveTabID   string
	lastActiveBuf     *buffer.Buffer
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

func (g *EditorGroupWidget) newUndoStack() *undo.UndoStack {
	return &undo.UndoStack{DeleteCursorStart: g.UndoDeleteCursorStart}
}

func (g *EditorGroupWidget) ApplyUndoDeleteCursorStart(v bool) {
	for i := range g.tabs {
		if g.tabs[i].Undo != nil {
			g.tabs[i].Undo.DeleteCursorStart = v
		}
	}
}

func (g *EditorGroupWidget) Focusable() bool { return true }

func (g *EditorGroupWidget) SetFocused(focused bool) {
	g.focused = focused
	if t := g.activeTab(); t != nil && t.Content != nil {
		if setter, ok := t.Content.(interface{ SetFocused(bool) }); ok {
			setter.SetFocused(focused)
		}
	}
}

func (g *EditorGroupWidget) activeTab() *editorTab {
	if len(g.tabs) == 0 || g.active < 0 || g.active >= len(g.tabs) {
		return nil
	}
	return &g.tabs[g.active]
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

func (g *EditorGroupWidget) CommitActiveTab() {
	if t := g.activeTab(); t != nil {
		t.Preview = false
	}
}

func (g *EditorGroupWidget) TogglePinTab() {
	if len(g.tabs) == 0 {
		return
	}
	idx := g.active
	if idx < g.pinnedCount {
		tab := g.tabs[idx]
		g.tabs = append(g.tabs[:idx], g.tabs[idx+1:]...)
		g.pinnedCount--
		g.tabs = slices.Insert(g.tabs, g.pinnedCount, tab)
		g.active = g.pinnedCount
	} else {
		tab := g.tabs[idx]
		tab.Preview = false
		g.tabs = append(g.tabs[:idx], g.tabs[idx+1:]...)
		g.tabs = slices.Insert(g.tabs, g.pinnedCount, tab)
		g.active = g.pinnedCount
		g.pinnedCount++
	}
	g.syncTabs()
}

func (g *EditorGroupWidget) IsActiveTabPinned() bool {
	return g.active < g.pinnedCount
}

func (g *EditorGroupWidget) NormalizeTabMoveTarget(from, to int) int {
	if from < 0 || from >= len(g.tabs) || to < 0 || to >= len(g.tabs) {
		return -1
	}
	if from < g.pinnedCount {
		return min(to, g.pinnedCount-1)
	}
	return max(to, g.pinnedCount)
}

func (g *EditorGroupWidget) MoveTab(from, to int) bool {
	to = g.NormalizeTabMoveTarget(from, to)
	if to < 0 {
		return false
	}
	if from == to {
		return false
	}

	tab := g.tabs[from]
	tab.Preview = false
	g.tabs = append(g.tabs[:from], g.tabs[from+1:]...)
	g.tabs = slices.Insert(g.tabs, to, tab)
	switch {
	case g.active == from:
		g.active = to
	case from < g.active && to >= g.active:
		g.active--
	case from > g.active && to <= g.active:
		g.active++
	}
	g.syncTabs()
	return true
}

func (g *EditorGroupWidget) CanMoveActiveTab(direction int) bool {
	to := g.active + direction
	if direction == 0 || to < 0 || to >= len(g.tabs) {
		return false
	}
	if g.active < g.pinnedCount {
		return to < g.pinnedCount
	}
	return to >= g.pinnedCount
}

func (g *EditorGroupWidget) MoveActiveTab(direction int) bool {
	if !g.CanMoveActiveTab(direction) {
		return false
	}
	return g.MoveTab(g.active, g.active+direction)
}

func (g *EditorGroupWidget) OpenFile(path string) {
	if isSpecialFile(path) {
		g.reportError("Cannot open " + path + ": not a regular file")
		return
	}
	for i := range g.tabs {
		if g.tabs[i].FilePath == path {
			g.tabs[i].Preview = false
			if g.tabs[i].Content != nil {
				g.applyImagePrefs(g.tabs[i].Content)
				if refresher, ok := g.tabs[i].Content.(interface{ Refresh() }); ok {
					refresher.Refresh()
				}
				g.SwitchTab(i)
				return
			}
			if g.tabs[i].Buf != nil && !g.tabs[i].Buf.Dirty {
				g.tabs[i].Buf.LoadFile(path)
			}
			g.SwitchTab(i)
			return
		}
	}
	switch kind, head := sniffFileKind(path); kind {
	case fileKindImage:
		g.openContentFileTab(path, NewImageViewWidget(path))
		return
	case fileKindBinary:
		if g.FileHandler != nil {
			if content := g.FileHandler(path, head); content != nil {
				g.openContentFileTab(path, content)
				g.fileHandled(path)
				return
			}
		}
		g.openContentFileTab(path, NewBinaryFileWidget(path))
		return
	}
	newBuf := &buffer.Buffer{Lines: []string{""}, ShowTrailingNewline: g.ShowTrailingNewline}
	ec := g.applySaveSettings(newBuf, path)
	if err := newBuf.LoadFile(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			g.reportError(fmt.Sprintf("Failed to open %s: %v", path, err))
			return
		}
		g.notify(fmt.Sprintf("New file: %s", filepath.Base(path)))
	}
	tabSize := g.TabSize
	useTabs := !g.InsertSpaces
	detected := buffer.DetectIndent(newBuf.Lines)
	if ec.IndentStyle == "tab" {
		useTabs = true
	} else if ec.IndentStyle == "space" {
		useTabs = false
	} else if detected.UseTabs || detected.Size > 0 {
		useTabs = detected.UseTabs
	}
	if ec.IndentSize > 0 {
		tabSize = ec.IndentSize
	} else if detected.Size > 0 && !detected.UseTabs {
		tabSize = detected.Size
	}
	folds := fold.NewState()
	folds.SetPending(func() []fold.Range { return fold.ComputeIndentRanges(newBuf.Lines) })
	newTab := editorTab{
		FilePath: path,
		IconPath: path,
		Buf:      newBuf,
		Cur:      &cursor.Cursor{},
		Vp:       &view.Viewport{},
		Undo:     g.newUndoStack(),
		Sel:      &selection.Selection{},
		Folds:    folds,
		TabSize:  tabSize,
		UseTabs:  useTabs,
		Preview:  true,
	}
	if g.SyntaxHighlight {
		newTab.Highlighter = highlight.NewForLines(path, newBuf.Lines)
	}
	newTab.Annotations = g.mergedDecorations(path, newBuf, tabSize)
	if t := g.activeTab(); g.canReplaceTab(t) && (isFileViewerTab(t) || (t.Content == nil && t.Buf != nil && !t.Buf.Dirty)) {
		g.notifyContentTabClose(*t)
		g.tabs[g.active] = newTab
		g.syncTabs()
	} else {
		g.tabs = append(g.tabs, newTab)
		g.SwitchTab(len(g.tabs) - 1)
	}
	if g.OnFileOpen != nil && newTab.Highlighter != nil {
		g.OnFileOpen(path, newTab.Highlighter.Language(), newBuf.Lines)
	}
}

func isFileViewerTab(t *editorTab) bool {
	switch t.Content.(type) {
	case *ImageViewWidget, *BinaryFileWidget:
		return true
	}
	return false
}

func (g *EditorGroupWidget) applyImagePrefs(content Widget) {
	if iv, ok := content.(*ImageViewWidget); ok {
		iv.Protocol = g.ImageProtocol
		iv.CellW, iv.CellH = g.ImageCellW, g.ImageCellH
		iv.Redraw = g.ImageRedraw
	}
}

const specialFileMask = os.ModeNamedPipe | os.ModeSocket | os.ModeDevice | os.ModeCharDevice | os.ModeIrregular

// Reading a pipe or device blocks until a peer shows up, which freezes the UI thread.
func isSpecialFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode()&specialFileMask != 0
}

type fileKind int

const (
	fileKindText fileKind = iota
	fileKindImage
	fileKindBinary
)

// The binary check mirrors isBinaryContent in internal/app/current_changes.go, which this package cannot import without a cycle.
func sniffFileKind(path string) (fileKind, []byte) {
	f, err := os.Open(path)
	if err != nil {
		return fileKindText, nil
	}
	defer f.Close()
	var head [512]byte
	n, _ := f.Read(head[:])
	sample := head[:n]
	ct := http.DetectContentType(sample)
	switch ct {
	case "image/png", "image/jpeg", "image/gif":
		return fileKindImage, head[:n]
	}
	if len(sample) == 0 {
		return fileKindText, nil
	}
	// A rune cut by the 512-byte window is not invalid UTF-8.
	if n == len(head) {
		for i := len(sample) - 1; i >= len(sample)-utf8.UTFMax && i >= 0; i-- {
			if utf8.RuneStart(sample[i]) {
				if !utf8.FullRune(sample[i:]) {
					sample = sample[:i]
				}
				break
			}
		}
	}
	if bytes.IndexByte(sample, 0) >= 0 || !utf8.Valid(sample) {
		return fileKindBinary, head[:n]
	}
	if !strings.HasPrefix(ct, "text/") {
		return fileKindBinary, head[:n]
	}
	return fileKindText, nil
}

// RetryFileHandlers offers binary-card tabs to FileHandler again: files opened from the command line load before plugins do.
func (g *EditorGroupWidget) RetryFileHandlers() {
	if g.FileHandler == nil {
		return
	}
	for i := range g.tabs {
		if _, ok := g.tabs[i].Content.(*BinaryFileWidget); !ok {
			continue
		}
		path := g.tabs[i].FilePath
		kind, head := sniffFileKind(path)
		if kind != fileKindBinary {
			continue
		}
		if content := g.FileHandler(path, head); content != nil {
			g.replaceContent(i, content)
			g.fileHandled(path)
		}
	}
	g.syncTabs()
}

func (g *EditorGroupWidget) ShowBinaryCard(path string) {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path && g.tabs[i].Content != nil {
			g.replaceContent(i, NewBinaryFileWidget(path))
		}
	}
	g.syncTabs()
}

func (g *EditorGroupWidget) fileHandled(path string) {
	if g.OnFileHandled != nil {
		g.OnFileHandled(path)
	}
}

func (g *EditorGroupWidget) replaceContent(i int, content Widget) {
	old := g.tabs[i].Content
	if setter, ok := old.(interface{ SetFocused(bool) }); ok {
		setter.SetFocused(false)
	}
	if closer, ok := old.(interface{ Close() }); ok {
		closer.Close()
	}
	g.tabs[i].Content = content
	g.tabs[i].PluginKey = contentPluginKey(content)
	if i == g.active && g.focused {
		if setter, ok := content.(interface{ SetFocused(bool) }); ok {
			setter.SetFocused(true)
		}
	}
}

// Content tabs carry no buffer.
func (g *EditorGroupWidget) openContentFileTab(path string, content Widget) {
	g.applyImagePrefs(content)
	newTab := editorTab{
		FilePath:  path,
		PluginKey: contentPluginKey(content),
		IconPath:  path,
		Content:   content,
		Preview:   true,
	}
	if t := g.activeTab(); g.canReplaceTab(t) && (t.Content != nil || (t.Buf != nil && !t.Buf.Dirty)) {
		g.notifyContentTabClose(*t)
		g.tabs[g.active] = newTab
		g.syncTabs()
		return
	}
	g.tabs = append(g.tabs, newTab)
	g.SwitchTab(len(g.tabs) - 1)
}

func (g *EditorGroupWidget) canReplaceTab(t *editorTab) bool {
	if t == nil || g.IsActiveTabPinned() {
		return false
	}
	if t.Preview {
		return true
	}
	return len(g.tabs) == 1 && g.isUntouchedPlaceholder(t)
}

func (g *EditorGroupWidget) isUntouchedPlaceholder(t *editorTab) bool {
	return t != nil && t.Placeholder && t.Buf != nil && !t.Buf.Dirty &&
		len(t.Buf.Lines) <= 1 && (len(t.Buf.Lines) == 0 || t.Buf.Lines[0] == "")
}

func (g *EditorGroupWidget) NewFile() {
	name := g.nextUntitledName()
	g.tabs = append(g.tabs, editorTab{
		FilePath: name,
		Buf:      &buffer.Buffer{Lines: []string{""}},
		Cur:      &cursor.Cursor{},
		Vp:       &view.Viewport{},
		Undo:     g.newUndoStack(),
		Sel:      &selection.Selection{},
		Virtual:  true,
	})
	g.SwitchTab(len(g.tabs) - 1)
}

func (g *EditorGroupWidget) NewFileFromText(text string) {
	buf := &buffer.Buffer{ShowTrailingNewline: g.ShowTrailingNewline}
	buf.SetText(text)
	tab := editorTab{
		Buf:     buf,
		Cur:     &cursor.Cursor{},
		Vp:      &view.Viewport{},
		Undo:    g.newUndoStack(),
		Sel:     &selection.Selection{},
		Virtual: true,
	}
	if t := g.activeTab(); len(g.tabs) == 1 && g.isUntouchedPlaceholder(t) {
		tab.FilePath = t.FilePath
		g.tabs[g.active] = tab
		g.syncTabs()
		return
	}
	tab.FilePath = g.nextUntitledName()
	g.tabs = append(g.tabs, tab)
	g.SwitchTab(len(g.tabs) - 1)
}

func (g *EditorGroupWidget) nextUntitledName() string {
	taken := make(map[string]bool)
	for _, t := range g.tabs {
		if t.Virtual {
			taken[t.FilePath] = true
		}
	}
	if !taken["untitled"] {
		return "untitled"
	}
	for n := 2; ; n++ {
		name := fmt.Sprintf("untitled-%d", n)
		if !taken[name] {
			return name
		}
	}
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

func (g *EditorGroupWidget) OpenPluginTab(key, title string, content Widget) {
	g.applyFileIconMode(content)
	for i, t := range g.tabs {
		if key != "" && t.PluginKey == key {
			g.notifyContentTabClose(t)
			t.Content = content
			t.IconPath = ""
			t.Title = title
			g.tabs[i] = t
			g.SwitchTab(i)
			return
		}
	}
	g.tabs = append(g.tabs, editorTab{
		FilePath:  key,
		PluginKey: key,
		Title:     title,
		Content:   content,
	})
	g.SwitchTab(len(g.tabs) - 1)
}

// PluginTabKey returns the key of the plugin tab at index, or "" for any other tab.
func (g *EditorGroupWidget) PluginTabKey(index int) string {
	if index < 0 || index >= len(g.tabs) {
		return ""
	}
	return g.tabs[index].PluginKey
}

func contentPluginKey(content Widget) string {
	if k, ok := content.(interface{ PluginTabKey() string }); ok {
		return k.PluginTabKey()
	}
	return ""
}

func (g *EditorGroupWidget) placeholderTab() editorTab {
	return editorTab{
		FilePath:    "untitled",
		Buf:         &buffer.Buffer{Lines: []string{""}},
		Cur:         &cursor.Cursor{},
		Vp:          &view.Viewport{},
		Undo:        g.newUndoStack(),
		Sel:         &selection.Selection{},
		Virtual:     true,
		Placeholder: true,
	}
}

func (g *EditorGroupWidget) notifyEmpty() {
	if g.OnEmpty != nil {
		g.OnEmpty()
	}
}

// ClosePluginTab closes the plugin tab keyed key and reports whether there was one.
func (g *EditorGroupWidget) ClosePluginTab(key string) bool {
	if key == "" {
		return false
	}
	prev := g.activeContent()
	for i, t := range g.tabs {
		if t.PluginKey == key {
			g.notifyContentTabClose(t)
			if i < g.pinnedCount {
				g.pinnedCount--
			}
			if i < g.active {
				g.active--
			}
			g.tabs = append(g.tabs[:i], g.tabs[i+1:]...)
			emptied := len(g.tabs) == 0
			if emptied {
				g.tabs = []editorTab{g.placeholderTab()}
				g.active = 0
			} else if g.active >= len(g.tabs) {
				g.active = len(g.tabs) - 1
			}
			g.syncTabs()
			g.refocusAfterClose(prev)
			if emptied {
				g.notifyEmpty()
			}
			return true
		}
	}
	return false
}

func (g *EditorGroupWidget) notifyContentTabClose(tab editorTab) {
	tab.Highlighter.Close()
	if tab.Diff != nil {
		tab.Diff.Close()
	}
	if tab.Content == nil {
		return
	}
	if closer, ok := tab.Content.(interface{ Close() }); ok {
		closer.Close()
	}
	if g.OnContentTabClose != nil {
		g.OnContentTabClose(tab.FilePath)
	}
}

func (g *EditorGroupWidget) ReloadFile(path string) bool {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path && g.tabs[i].Buf != nil {
			if err := g.tabs[i].Buf.LoadFile(path); err != nil {
				g.reportError(fmt.Sprintf("Failed to reload %s: %v", path, err))
				return false
			}
			g.tabs[i].Buf.Dirty = false
			g.tabs[i].Undo = g.newUndoStack()
			if g.tabs[i].Folds != nil {
				g.tabs[i].Folds.SetRanges(fold.ComputeIndentRanges(g.tabs[i].Buf.Lines))
			}
			g.clampCursor(&g.tabs[i])
			if i == g.active {
				g.syncTabs()
			}
			return true
		}
	}
	return false
}

// clampCursor keeps a tab's cursor within the bounds of its buffer, which may
// have shrunk after an external reload.
func (g *EditorGroupWidget) clampCursor(t *editorTab) {
	if t.Cur == nil || t.Buf == nil {
		return
	}
	n := len(t.Buf.Lines)
	if n == 0 {
		t.Cur.Line, t.Cur.Col = 0, 0
		return
	}
	if t.Cur.Line >= n {
		t.Cur.Line = n - 1
	}
	if t.Cur.Line < 0 {
		t.Cur.Line = 0
	}
	lineLen := len([]rune(t.Buf.Lines[t.Cur.Line]))
	if t.Cur.Col > lineLen {
		t.Cur.Col = lineLen
	}
}

// OpenFilePaths returns the paths of all tabs backed by a real file on disk
// (excluding untitled buffers and non-text content like diff views). The order
// is unspecified.
func (g *EditorGroupWidget) OpenFilePaths() []string {
	var paths []string
	for i := range g.tabs {
		t := &g.tabs[i]
		if t.Content != nil || t.Buf == nil {
			continue
		}
		if t.FilePath == "" || t.Virtual {
			continue
		}
		paths = append(paths, t.FilePath)
	}
	return paths
}

type OpenDocument struct {
	Path     string
	Language string
	Text     string
}

func (g *EditorGroupWidget) OpenDocuments() []OpenDocument {
	var docs []OpenDocument
	for i := range g.tabs {
		t := &g.tabs[i]
		if t.Content != nil || t.Buf == nil || t.Highlighter == nil || t.Virtual || t.FilePath == "" {
			continue
		}
		docs = append(docs, OpenDocument{
			Path:     t.FilePath,
			Language: t.Highlighter.Language(),
			Text:     strings.Join(t.Buf.Lines, "\n"),
		})
	}
	return docs
}

// OpenFiles is OpenDocuments without joining each buffer's text.
func (g *EditorGroupWidget) OpenFiles() []OpenDocument {
	var docs []OpenDocument
	for i := range g.tabs {
		t := &g.tabs[i]
		if t.Content != nil || t.Buf == nil || t.Highlighter == nil || t.Virtual || t.FilePath == "" {
			continue
		}
		docs = append(docs, OpenDocument{Path: t.FilePath, Language: t.Highlighter.Language()})
	}
	return docs
}

// BufferForPath returns the buffer of the tab with the given path, or nil.
func (g *EditorGroupWidget) BufferForPath(path string) *buffer.Buffer {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path {
			return g.tabs[i].Buf
		}
	}
	return nil
}

// IsDirtyPath reports whether the tab with the given path has unsaved changes.
func (g *EditorGroupWidget) IsDirtyPath(path string) bool {
	b := g.BufferForPath(path)
	return b != nil && b.Dirty
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

// TextSelectionContent is tab content that selects the text it draws.
type TextSelectionContent interface {
	CopyTextSelection() bool
	SelectAllText()
}

func (g *EditorGroupWidget) ActiveTextSelection() TextSelectionContent {
	t := g.activeTab()
	if t == nil || t.Content == nil {
		return nil
	}
	sel, _ := t.Content.(TextSelectionContent)
	return sel
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

func (g *EditorGroupWidget) SwitchToTabByPath(path string) bool {
	for i, t := range g.tabs {
		if t.FilePath == path {
			g.SwitchTab(i)
			return true
		}
	}
	return false
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

func (g *EditorGroupWidget) CursorPosition() (int, int, bool) {
	// Content tabs (settings, plugin panels) own their own cursor.
	if t := g.activeTab(); t != nil && t.Content != nil {
		if cp, ok := t.Content.(widgets.CursorPositioner); ok {
			return cp.CursorPosition()
		}
	}
	if g.IsEditorActive() {
		if d := g.activeInlineDiff(); d != nil && d.headFocused() {
			return d.left.CursorX, d.left.CursorY, true
		}
		if g.Editor.isMultiActive() {
			return 0, 0, false
		}
		return g.Editor.CursorX, g.Editor.CursorY, true
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

func (g *EditorGroupWidget) SwitchTab(idx int) {
	if idx >= 0 && idx < len(g.tabs) {
		changed := idx != g.active
		if changed && g.TabBar.OwnsPointerCapture() {
			g.TabBar.CancelPointerCapture()
		}
		if t := g.activeTab(); t != nil && t.Content != nil {
			if setter, ok := t.Content.(interface{ SetFocused(bool) }); ok {
				setter.SetFocused(false)
			}
		}
		g.saveMultiState()
		g.active = idx
		g.syncTabs()
		g.focusActiveContent()
	}
}

func (g *EditorGroupWidget) focusActiveContent() {
	if !g.focused {
		return
	}
	if t := g.activeTab(); t != nil && t.Content != nil {
		if setter, ok := t.Content.(interface{ SetFocused(bool) }); ok {
			setter.SetFocused(true)
		}
	}
}

func (g *EditorGroupWidget) activeContent() Widget {
	if t := g.activeTab(); t != nil {
		return t.Content
	}
	return nil
}

func (g *EditorGroupWidget) refocusAfterClose(prev Widget) {
	if prev != nil && prev != g.activeContent() {
		if setter, ok := prev.(interface{ SetFocused(bool) }); ok {
			setter.SetFocused(false)
		}
	}
	g.focusActiveContent()
}

func (g *EditorGroupWidget) saveMultiState() {
	if t := g.activeTab(); t != nil && t.Content == nil {
		t.Multi = g.Editor.Multi
	}
}

func (g *EditorGroupWidget) NextTab() {
	if len(g.tabs) > 1 {
		g.SwitchTab((g.active + 1) % len(g.tabs))
	}
}

func (g *EditorGroupWidget) PrevTab() {
	if len(g.tabs) > 1 {
		g.SwitchTab((g.active - 1 + len(g.tabs)) % len(g.tabs))
	}
}

func (g *EditorGroupWidget) CloseTab() {
	if len(g.tabs) == 0 {
		return
	}
	closing := g.tabs[g.active]
	prev := closing.Content
	if g.OnFileClose != nil && closing.Highlighter != nil && !closing.Virtual {
		g.OnFileClose(closing.FilePath, closing.Highlighter.Language())
	}
	g.notifyContentTabClose(closing)
	if g.active < g.pinnedCount {
		g.pinnedCount--
	}
	g.tabs = append(g.tabs[:g.active], g.tabs[g.active+1:]...)
	emptied := len(g.tabs) == 0
	if emptied {
		g.tabs = []editorTab{g.placeholderTab()}
		g.active = 0
	} else if g.active >= len(g.tabs) {
		g.active = len(g.tabs) - 1
	}
	g.syncTabs()
	g.refocusAfterClose(prev)
	if emptied {
		g.notifyEmpty()
	}
}

func (g *EditorGroupWidget) CloseOtherTabs() {
	t := g.activeTab()
	if t == nil || len(g.tabs) <= 1 {
		return
	}
	activeFile := t.FilePath
	var kept []editorTab
	for i, tab := range g.tabs {
		if i == g.active || i < g.pinnedCount {
			kept = append(kept, tab)
			continue
		}
		g.notifyContentTabClose(tab)
	}
	g.tabs = kept
	for i, tab := range g.tabs {
		if tab.FilePath == activeFile {
			g.active = i
			break
		}
	}
	g.syncTabs()
}

func (g *EditorGroupWidget) CloseOtherSaved() {
	t := g.activeTab()
	if t == nil || len(g.tabs) <= 1 {
		return
	}
	activeFile := t.FilePath
	var kept []editorTab
	for i := range g.tabs {
		if i == g.active || i < g.pinnedCount {
			kept = append(kept, g.tabs[i])
			continue
		}
		if g.tabs[i].Buf != nil && g.tabs[i].Buf.Dirty {
			kept = append(kept, g.tabs[i])
			continue
		}
		g.notifyContentTabClose(g.tabs[i])
	}
	g.tabs = kept
	for i, tab := range g.tabs {
		if tab.FilePath == activeFile {
			g.active = i
			break
		}
	}
	g.syncTabs()
}

func (g *EditorGroupWidget) HasDirtyOtherTabs() bool {
	for i := range g.tabs {
		if i == g.active {
			continue
		}
		if g.tabs[i].Buf != nil && g.tabs[i].Buf.Dirty {
			return true
		}
	}
	return false
}

func (g *EditorGroupWidget) CloseAllTabs() {
	prev := g.activeContent()
	for i := g.pinnedCount; i < len(g.tabs); i++ {
		g.notifyContentTabClose(g.tabs[i])
	}
	kept := slices.Clone(g.tabs[:g.pinnedCount])
	emptied := len(kept) == 0
	if emptied {
		kept = []editorTab{g.placeholderTab()}
		g.pinnedCount = 0
	}
	g.tabs = kept
	g.active = 0
	g.syncTabs()
	g.refocusAfterClose(prev)
	if emptied {
		g.notifyEmpty()
	}
}

func (g *EditorGroupWidget) CloseAllSaved() {
	prev := g.activeContent()
	var kept []editorTab
	for i := range g.tabs {
		if i < g.pinnedCount || (g.tabs[i].Buf != nil && g.tabs[i].Buf.Dirty) {
			kept = append(kept, g.tabs[i])
			continue
		}
		g.notifyContentTabClose(g.tabs[i])
	}
	emptied := len(kept) == 0
	if emptied {
		kept = []editorTab{g.placeholderTab()}
		g.pinnedCount = 0
	}
	g.tabs = kept
	if g.active >= len(g.tabs) {
		g.active = len(g.tabs) - 1
	}
	g.syncTabs()
	g.refocusAfterClose(prev)
	if emptied {
		g.notifyEmpty()
	}
}

func (g *EditorGroupWidget) HasDirtyTabs() bool {
	for i := range g.tabs {
		if g.tabs[i].Buf != nil && g.tabs[i].Buf.Dirty {
			return true
		}
	}
	return false
}

// applySaveCleanups puts the save-time trim and final-newline handling on the
// undo stack, so Ctrl+Z reaches them (#312).
func (g *EditorGroupWidget) applySaveCleanups(t *editorTab) {
	if t.Buf == nil || g.Editor == nil {
		return
	}
	cleaned := t.Buf.CleanedLines()
	if slices.Equal(cleaned, t.Buf.Lines) {
		return
	}

	g.Editor.ExecCommand(&undo.ReplaceLinesCommand{
		Start:    0,
		OldLines: append([]string(nil), t.Buf.Lines...),
		NewLines: cleaned,
	})

	// Trimming can shorten the line the cursor sits on.
	g.Editor.clampCursor()
	if n := len([]rune(t.Buf.Lines[g.Editor.Cursor.Line])); g.Editor.Cursor.Col > n {
		g.Editor.Cursor.Col = n
	}
}

func (g *EditorGroupWidget) Save() bool {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return false
	}
	if t.Virtual || t.ReadOnly {
		return false
	}
	g.applySaveCleanups(t)
	if err := t.Buf.SaveFile(t.FilePath); err != nil {
		g.reportError(fmt.Sprintf("Failed to save %s: %v", t.FilePath, err))
		return false
	}
	if t.Undo != nil {
		t.Undo.MarkSaved()
	}
	g.syncTabs()
	return true
}

// applySaveSettings sets buf's save-time cleanups for path from settings and .editorconfig.
func (g *EditorGroupWidget) applySaveSettings(buf *buffer.Buffer, path string) config.EditorConfigProps {
	buf.InsertFinalNewline = g.InsertFinalNewline
	buf.TrimTrailingWhitespace = g.TrimTrailingWhitespace
	ec := config.LoadEditorConfig(path)
	if ec.InsertFinalNLSet {
		buf.InsertFinalNewline = ec.InsertFinalNewline
	}
	if ec.TrimTrailingWSSet {
		buf.TrimTrailingWhitespace = ec.TrimTrailingWS
	}
	return ec
}

func (g *EditorGroupWidget) SaveAs(path string) bool {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return false
	}
	g.applySaveSettings(t.Buf, path)
	g.applySaveCleanups(t)
	if err := t.Buf.SaveFile(path); err != nil {
		g.reportError(fmt.Sprintf("Failed to save %s: %v", path, err))
		return false
	}
	if t.Undo != nil {
		t.Undo.MarkSaved()
	}
	g.movePathState(t.FilePath, path)
	t.FilePath = path
	t.IconPath = path
	t.Virtual = false
	t.Placeholder = false
	if g.SyntaxHighlight {
		t.Highlighter = highlight.NewForLines(path, t.Buf.Lines)
	} else {
		t.Highlighter = nil
	}
	g.syncTabs()
	return true
}

// movePathState moves the bookmarks and decorations kept for oldPath, or for
// the files under it, to newPath.
func (g *EditorGroupWidget) movePathState(oldPath, newPath string) {
	prefix := oldPath + string(filepath.Separator)
	remap := func(p string) (string, bool) {
		switch {
		case p == oldPath:
			return newPath, true
		case oldPath != "" && strings.HasPrefix(p, prefix):
			return filepath.Join(newPath, strings.TrimPrefix(p, prefix)), true
		}
		return "", false
	}
	g.moveDecorations(remap)
	if p, ok := remap(g.bookmarkSyncPath); ok {
		g.bookmarkSyncPath = p
	}
	moved := map[string]map[int]Bookmark{}
	for p, b := range g.bookmarksByPath {
		if np, ok := remap(p); ok {
			delete(g.bookmarksByPath, p)
			moved[np] = b
		}
	}
	for p, b := range moved {
		g.bookmarksByPath[p] = b
	}
}

// RenamePath repoints open tabs after a path is renamed on disk. oldPath may be
// a file, matched exactly, or a folder, in which case every tab beneath it moves
// with it. Without this a renamed file's tab keeps pointing at the path it no
// longer occupies, so the next save writes the old name back out as a duplicate.
//
// Buffer, cursor, selection and undo history are deliberately preserved: the
// document did not change, only its name. Path-derived state (the syntax
// highlighter, and the language server's notion of which document this is) is
// rebuilt, since a rename can change the extension.
func (g *EditorGroupWidget) RenamePath(oldPath, newPath string) bool {
	if oldPath == "" || newPath == "" || oldPath == newPath {
		return false
	}
	g.movePathState(oldPath, newPath)
	prefix := oldPath + string(filepath.Separator)
	remapPath := func(path string) (string, bool) {
		switch {
		case path == oldPath:
			return newPath, true
		case strings.HasPrefix(path, prefix):
			return filepath.Join(newPath, strings.TrimPrefix(path, prefix)), true
		default:
			return "", false
		}
	}
	renamed := false
	for i := range g.tabs {
		t := &g.tabs[i]
		if t.Virtual {
			continue
		}
		if t.Content != nil {
			oldIconPath := t.IconPath
			updatedIconPath, ok := remapPath(oldIconPath)
			if !ok {
				continue
			}
			setter, ok := t.Content.(interface{ SetFilePath(string) })
			if !ok {
				continue
			}
			setter.SetFilePath(updatedIconPath)
			t.IconPath = updatedIconPath
			if updated, ok := remapPath(t.FilePath); ok {
				t.FilePath = updated
			} else if strings.HasPrefix(t.FilePath, oldIconPath+" (") {
				t.FilePath = updatedIconPath + strings.TrimPrefix(t.FilePath, oldIconPath)
			}
			renamed = true
			continue
		}
		if t.Buf == nil {
			continue
		}
		updated, ok := remapPath(t.FilePath)
		if !ok {
			continue
		}
		if g.OnFileClose != nil && t.Highlighter != nil {
			g.OnFileClose(t.FilePath, t.Highlighter.Language())
		}
		t.FilePath = updated
		if t.Diff != nil {
			t.Diff.SetFilePath(updated)
		}
		if t.IconPath != "" {
			t.IconPath = updated
		}
		if g.SyntaxHighlight {
			t.Highlighter = highlight.NewForLines(updated, t.Buf.Lines)
		} else {
			t.Highlighter = nil
		}
		if g.OnFileOpen != nil && t.Highlighter != nil {
			g.OnFileOpen(updated, t.Highlighter.Language(), t.Buf.Lines)
		}
		renamed = true
	}
	if renamed {
		g.syncTabs()
	}
	return renamed
}

// RefreshHighlighters rebuilds every file tab's highlighter after the set of
// available grammars changes.
func (g *EditorGroupWidget) RefreshHighlighters() {
	if !g.SyntaxHighlight {
		return
	}
	for i := range g.tabs {
		t := &g.tabs[i]
		if t.Content != nil || t.Buf == nil || t.FilePath == "" {
			continue
		}
		prevLang := ""
		if t.Highlighter != nil {
			prevLang = t.Highlighter.Language()
		}
		t.Highlighter = highlight.NewForLines(t.FilePath, t.Buf.Lines)
		lang := ""
		if t.Highlighter != nil {
			lang = t.Highlighter.Language()
		}
		if lang == prevLang || t.Virtual {
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
	enabled, _ := g.tabWordWrap(t)
	return enabled
}

func (g *EditorGroupWidget) tabWordWrap(t *editorTab) (bool, int) {
	language := ""
	if t != nil && t.Buf != nil {
		language = highlight.DetectLanguageID(t.FilePath, t.Buf.Lines)
	}
	s := config.EditorSettings{WordWrap: g.WordWrap, WordWrapColumn: g.WordWrapColumn, LanguageOverrides: g.LanguageOverrides}
	enabled, column := s.ResolveWordWrap(language)
	if !enabled {
		return false, column
	}
	return g.WordWrapMaxBytes <= 0 || t == nil || t.Buf == nil || t.Buf.DiskSize() <= g.WordWrapMaxBytes, column
}

func (g *EditorGroupWidget) ApplyFileWordWrap() {
	t := g.activeTab()
	if g.Editor == nil || t == nil || t.Content != nil || t.Diff != nil {
		return
	}
	enabled, column := g.tabWordWrap(t)
	if g.Editor.WordWrap != enabled || g.Editor.WordWrapColumn != column {
		g.Editor.wrapGoal = wrapGoal{}
	}
	g.Editor.WordWrap, g.Editor.WordWrapColumn = enabled, column
}

// ActiveFileWordWrap reports whether the active file tab should wrap.
func (g *EditorGroupWidget) ActiveFileWordWrap() bool {
	t := g.activeTab()
	if t != nil && t.Content == nil && t.Diff == nil {
		return g.wrapsTab(t)
	}
	return g.WordWrap
}

func (g *EditorGroupWidget) ActiveFilePath() string {
	if t := g.activeTab(); t != nil {
		return t.FilePath
	}
	return ""
}

func (g *EditorGroupWidget) IsActiveVirtual() bool {
	if t := g.activeTab(); t != nil {
		return t.Virtual
	}
	return true
}

func (g *EditorGroupWidget) IsActiveReadOnly() bool {
	if t := g.activeTab(); t != nil {
		return t.ReadOnly
	}
	return false
}

func (g *EditorGroupWidget) OpenFileReadOnly(path, title string) {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path && g.tabs[i].ReadOnly {
			g.SwitchTab(i)
			return
		}
	}
	newBuf := &buffer.Buffer{Lines: []string{""}}
	if err := newBuf.LoadFile(path); err != nil {
		g.reportError(fmt.Sprintf("Failed to open %s: %v", path, err))
		return
	}
	tabSize := g.TabSize
	detected := buffer.DetectIndent(newBuf.Lines)
	if detected.Size > 0 && !detected.UseTabs {
		tabSize = detected.Size
	}
	folds := fold.NewState()
	folds.SetRanges(fold.ComputeIndentRanges(newBuf.Lines))
	tabTitle := title
	if tabTitle == "" {
		tabTitle = filepath.Base(path)
	}
	newTab := editorTab{
		FilePath: path,
		IconPath: path,
		Title:    tabTitle,
		Buf:      newBuf,
		Cur:      &cursor.Cursor{},
		Vp:       &view.Viewport{},
		Undo:     g.newUndoStack(),
		Sel:      &selection.Selection{},
		Folds:    folds,
		TabSize:  tabSize,
		UseTabs:  detected.UseTabs,
		ReadOnly: true,
	}
	if g.SyntaxHighlight {
		newTab.Highlighter = highlight.NewForLines(path, newBuf.Lines)
	}
	g.tabs = append(g.tabs, newTab)
	g.SwitchTab(len(g.tabs) - 1)
}

func (g *EditorGroupWidget) OpenBufferReadOnly(title, filePath string, lines []string) {
	for i := range g.tabs {
		if g.tabs[i].Title == title && g.tabs[i].ReadOnly {
			g.tabs[i].Buf.SetLines(lines)
			g.SwitchTab(i)
			return
		}
	}
	newBuf := &buffer.Buffer{Lines: lines}
	if len(lines) == 0 {
		newBuf.SetLines([]string{""})
	}
	tabSize := g.TabSize
	detected := buffer.DetectIndent(newBuf.Lines)
	if detected.Size > 0 && !detected.UseTabs {
		tabSize = detected.Size
	}
	folds := fold.NewState()
	folds.SetRanges(fold.ComputeIndentRanges(newBuf.Lines))
	newTab := editorTab{
		FilePath: filePath,
		IconPath: filePath,
		Title:    title,
		Buf:      newBuf,
		Cur:      &cursor.Cursor{},
		Vp:       &view.Viewport{},
		Undo:     g.newUndoStack(),
		Sel:      &selection.Selection{},
		Folds:    folds,
		TabSize:  tabSize,
		UseTabs:  detected.UseTabs,
		ReadOnly: true,
	}
	if g.SyntaxHighlight && filePath != "" {
		newTab.Highlighter = highlight.NewForLines(filePath, newBuf.Lines)
	}
	g.tabs = append(g.tabs, newTab)
	g.SwitchTab(len(g.tabs) - 1)
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

func (g *EditorGroupWidget) ActiveCursor() (line, col int) {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return 0, 0
	}
	return t.Cur.Line, t.Cur.Col
}

func (g *EditorGroupWidget) TabCount() int {
	return len(g.tabs)
}

func (g *EditorGroupWidget) ActiveTabIndex() int {
	return g.active
}

func (g *EditorGroupWidget) TabContent(index int) Widget {
	if index < 0 || index >= len(g.tabs) {
		return nil
	}
	return g.tabs[index].Content
}

func (g *EditorGroupWidget) IsTabUnnamed(index int) bool {
	if index < 0 || index >= len(g.tabs) {
		return false
	}
	t := &g.tabs[index]
	return t.Content == nil && t.Buf != nil && (t.Virtual || t.FilePath == "")
}

func (g *EditorGroupWidget) TabInfo(index int) (path string, modified bool) {
	if index < 0 || index >= len(g.tabs) {
		return "", false
	}
	t := &g.tabs[index]
	dirty := t.Buf != nil && t.Buf.Dirty
	return t.FilePath, dirty
}

func (g *EditorGroupWidget) ActiveSelection() (active bool, sl, sc, el, ec int) {
	t := g.activeTab()
	if t == nil || t.Content != nil || t.Sel == nil || !t.Sel.Active {
		return false, 0, 0, 0, 0
	}
	start, end := t.Sel.Range(t.Cur.Line, t.Cur.Col)
	return true, start.Line, start.Col, end.Line, end.Col
}

func (g *EditorGroupWidget) ActiveFileName() string {
	t := g.activeTab()
	if t == nil {
		return "untitled"
	}
	return filepath.Base(t.FilePath)
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

func (g *EditorGroupWidget) undoRedoPostProcess() {
	if g.Editor.Folds != nil {
		g.Editor.refreshFolds()
		g.Editor.ExpandFoldContaining(g.Editor.Cursor.Line)
	}
	g.Editor.bufferDirty = true
}

func (g *EditorGroupWidget) Undo() {
	t := g.activeTab()
	if t == nil || g.editableKeyEditor() == nil {
		return
	}
	// Undo has no concept of e.Multi; collapsing first avoids stranding
	// secondary cursors at stale offsets that corrupt on the next
	// multicursor keystroke (BUG-008).
	if g.IsEditorActive() && g.Editor.isMultiActive() {
		g.Editor.collapseMulti()
	}
	if t.Undo != nil {
		if pos := t.Undo.Undo(t.Buf); pos != nil {
			g.Editor.Cursor.Line = pos.Line
			g.Editor.Cursor.Col = pos.Col
		}
		if t.Undo.AtSavePoint() && !t.Buf.LineEndingChanged() {
			t.Buf.Dirty = false
		}
		g.undoRedoPostProcess()
	}
}

func (g *EditorGroupWidget) Redo() {
	t := g.activeTab()
	if t == nil || g.editableKeyEditor() == nil {
		return
	}
	if g.IsEditorActive() && g.Editor.isMultiActive() {
		g.Editor.collapseMulti()
	}
	if t.Undo != nil {
		if pos := t.Undo.Redo(t.Buf); pos != nil {
			g.Editor.Cursor.Line = pos.Line
			g.Editor.Cursor.Col = pos.Col
		}
		if t.Undo.AtSavePoint() && !t.Buf.LineEndingChanged() {
			t.Buf.Dirty = false
		}
		g.undoRedoPostProcess()
	}
}

func (g *EditorGroupWidget) SelectAll() {
	if sel := g.ActiveTextSelection(); sel != nil {
		sel.SelectAllText()
		return
	}
	e := g.keyEditor()
	if e == nil || e.Selection == nil {
		return
	}
	e.collapseMulti()
	g.saveMultiState()
	e.Selection.Start(0, 0)
	lastLine := len(e.Buf.Lines) - 1
	e.Cursor.Line = lastLine
	e.Cursor.Col = len([]rune(e.Buf.Lines[lastLine]))
}

func (g *EditorGroupWidget) SetSearch(query string, matches []FindMatch) {
	g.find = findSession{}
	g.applySearch(query, matches)
}

func (g *EditorGroupWidget) SetFindSearch(query string, matches []FindMatch, opts SearchOptions) {
	g.find = findSession{query: query, options: opts, activeMatch: -1}
	if !g.IsEditorActive() {
		return
	}
	g.find.matchesBuffer = g.Editor.Buf
	g.find.matchesVersion = g.Editor.Buf.Version()
	g.applySearch(query, matches)
}

func (g *EditorGroupWidget) applySearch(query string, matches []FindMatch) {
	if !g.IsEditorActive() {
		return
	}
	g.Editor.SearchQuery = query
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

// PlaceCursor moves the cursor to a 1-based line and column, clamping both to
// the buffer and expanding any fold hiding the line. It reports whether an
// editor was active. Unlike GoToLineCol it never touches the viewport, so it is
// safe before the first render: a scroll computed against a zero-height
// viewport leaves the tab mis-framed for as long as it stays open.
func (g *EditorGroupWidget) PlaceCursor(line, col int) bool {
	if !g.IsEditorActive() {
		return false
	}
	if line < 1 {
		line = 1
	}
	if line > len(g.Editor.Buf.Lines) {
		line = len(g.Editor.Buf.Lines)
	}
	bufLine := line - 1
	g.Editor.ExpandFoldContaining(bufLine)
	g.Editor.Cursor.Line = bufLine
	g.Editor.Cursor.Col = 0
	if col > 1 {
		lineLen := len([]rune(g.Editor.Buf.Lines[bufLine]))
		c := col - 1
		if c > lineLen {
			c = lineLen
		}
		g.Editor.Cursor.Col = c
	}
	return true
}

func (g *EditorGroupWidget) GoToLine(line int) {
	if !g.PlaceCursor(line, 0) {
		return
	}
	bufLine := g.Editor.Cursor.Line
	h := g.Editor.Viewport.Height
	if h <= 0 {
		r := g.GetRect()
		h = r.H - 3
		if h > 0 {
			g.Editor.Viewport.Height = h
		}
	}
	if h > 0 {
		margin := h / 3
		top := bufLine - margin
		if top < 0 {
			top = 0
		}
		g.Editor.Viewport.TopLine = top
	}
	g.Editor.scrollViewport()
}

// GoToLineCol moves the cursor to a 1-based line and column. The column is
// clamped to the line's rune length; col 0 or 1 leaves the cursor at the
// start of the line, matching GoToLine.
func (g *EditorGroupWidget) GoToLineCol(line, col int) {
	g.GoToLine(line)
	if !g.IsEditorActive() || col <= 1 {
		return
	}
	lineLen := len([]rune(g.Editor.Buf.Lines[g.Editor.Cursor.Line]))
	c := col - 1
	if c > lineLen {
		c = lineLen
	}
	g.Editor.Cursor.Col = c
	g.Editor.scrollViewport()
}

func (g *EditorGroupWidget) ScrollToCursor() {
	if g.IsEditorActive() {
		g.Editor.scrollViewport()
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
	g.applySearch(g.find.query, matches)
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
	runes := []rune(g.Editor.Buf.Lines[match.Line])
	endCol := match.Col + match.Len
	if endCol > len(runes) {
		endCol = len(runes)
	}
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
	for i := len(matches) - 1; i >= 0; i-- {
		g.ReplaceMatch(matches[i], replacement)
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

func (g *EditorGroupWidget) MoveLineUp() {
	if e := g.editableKeyEditor(); e != nil {
		e.MoveLineUp()
	}
}

func (g *EditorGroupWidget) MoveLineDown() {
	if e := g.editableKeyEditor(); e != nil {
		e.MoveLineDown()
	}
}

func (g *EditorGroupWidget) DuplicateLine() {
	if e := g.editableKeyEditor(); e != nil {
		e.DuplicateLine()
	}
}

func (g *EditorGroupWidget) DeleteLine() {
	if e := g.editableKeyEditor(); e != nil {
		e.DeleteLine()
	}
}

func (g *EditorGroupWidget) JoinLines() {
	if e := g.editableKeyEditor(); e != nil {
		e.JoinLines()
	}
}

func (g *EditorGroupWidget) InsertLineBelow() {
	if e := g.editableKeyEditor(); e != nil {
		e.InsertLineBelow()
	}
}

func (g *EditorGroupWidget) InsertLineAbove() {
	if e := g.editableKeyEditor(); e != nil {
		e.InsertLineAbove()
	}
}

func (g *EditorGroupWidget) ToggleLineComment() {
	if e := g.editableKeyEditor(); e != nil {
		e.ToggleLineComment()
	}
}

func (g *EditorGroupWidget) SortLinesAsc() {
	if e := g.editableKeyEditor(); e != nil {
		e.SortLinesAsc()
	}
}

func (g *EditorGroupWidget) SortLinesDesc() {
	if e := g.editableKeyEditor(); e != nil {
		e.SortLinesDesc()
	}
}

func (g *EditorGroupWidget) ReverseLines() {
	if e := g.editableKeyEditor(); e != nil {
		e.ReverseLines()
	}
}

func (g *EditorGroupWidget) UniqueLines() {
	if e := g.editableKeyEditor(); e != nil {
		e.UniqueLines()
	}
}

func (g *EditorGroupWidget) TrimTrailingWhitespaceLines() {
	if e := g.editableKeyEditor(); e != nil {
		e.TrimTrailingWhitespaceLines()
	}
}

func (g *EditorGroupWidget) MoveToFileEdge(end, shift bool) {
	if e := g.keyEditor(); e != nil {
		e.MoveToFileEdge(end, shift)
	} else if d := g.ActiveDiffWidget(); d != nil {
		d.KeyPane().MoveToFileEdge(end, shift)
	}
}

func (g *EditorGroupWidget) MoveWordLeft(shift bool) {
	if e := g.keyEditor(); e != nil {
		e.MoveWordLeft(shift)
	}
}

func (g *EditorGroupWidget) MoveWordRight(shift bool) {
	if e := g.keyEditor(); e != nil {
		e.MoveWordRight(shift)
	}
}

func (g *EditorGroupWidget) DeleteWordLeft() {
	if e := g.editableKeyEditor(); e != nil {
		e.DeleteWordLeft()
	}
}

func (g *EditorGroupWidget) DeleteWordRight() {
	if e := g.editableKeyEditor(); e != nil {
		e.DeleteWordRight()
	}
}

func (g *EditorGroupWidget) SelectNextOccurrence() {
	if e := g.keyEditor(); e != nil {
		e.SelectNextOccurrence()
		g.saveMultiState()
	}
}

func (g *EditorGroupWidget) SelectAllOccurrences() {
	if e := g.keyEditor(); e != nil {
		e.SelectAllOccurrences()
		g.saveMultiState()
	}
}

func (g *EditorGroupWidget) UndoLastCursor() {
	if e := g.keyEditor(); e != nil {
		e.UndoLastCursor()
		g.saveMultiState()
	}
}

func (g *EditorGroupWidget) SplitSelectionToLines() {
	if e := g.keyEditor(); e != nil {
		e.SplitSelectionToLines()
		g.saveMultiState()
	}
}

func (g *EditorGroupWidget) UpperCase() {
	if e := g.editableKeyEditor(); e != nil {
		e.UpperCase()
	}
}

func (g *EditorGroupWidget) LowerCase() {
	if e := g.editableKeyEditor(); e != nil {
		e.LowerCase()
	}
}

func (g *EditorGroupWidget) TitleCase() {
	if e := g.editableKeyEditor(); e != nil {
		e.TitleCase()
	}
}

func (g *EditorGroupWidget) GoToMatchingBracket() {
	if e := g.keyEditor(); e != nil {
		e.GoToMatchingBracket()
	}
}

func (g *EditorGroupWidget) IsMultiCursorActive() bool {
	return g.IsEditorActive() && g.Editor.isMultiActive()
}

func (g *EditorGroupWidget) MultiCursorCount() int {
	if g.IsEditorActive() && g.Editor.Multi != nil {
		return len(g.Editor.Multi.Cursors)
	}
	return 1
}

func (g *EditorGroupWidget) CollapseMultiCursor() {
	if g.IsEditorActive() {
		g.Editor.collapseMulti()
	}
}

func (g *EditorGroupWidget) AddCursor(line, col int) {
	if !g.IsEditorActive() {
		return
	}
	g.Editor.ensureMulti()
	g.Editor.syncToMulti()
	g.Editor.Multi.Add(line, col)
	g.Editor.syncFromMulti()
	g.saveMultiState()
}

func (g *EditorGroupWidget) GetCursors() []multicursor.CursorState {
	if !g.IsEditorActive() || g.Editor.Multi == nil {
		return nil
	}
	return g.Editor.Multi.Cursors
}

func (g *EditorGroupWidget) Copy() {
	t := g.activeTab()
	if t == nil {
		return
	}
	if dv, ok := t.Content.(*DiffEditorWidget); ok {
		if text := dv.CopySelection(); text != "" {
			clipboard.Set(text)
		}
		return
	}
	if detail, ok := t.Content.(*CommitDetailWidget); ok {
		if text := detail.CopySelection(); text != "" {
			clipboard.Set(text)
		}
		return
	}
	if t.Content != nil {
		// Non-editor tab (settings UI, plugin panel, ...): no buffer to copy from.
		return
	}
	if t.Diff != nil {
		if text, ok := t.Diff.HeadSelection(); ok {
			if text != "" {
				clipboard.Set(text)
			}
			return
		}
	}
	e := g.keyEditor()

	// Copy from every cursor when multicursor mode is active.
	if e.isMultiActive() {
		pieces := e.multiCopyPieces()
		if len(pieces) > 0 {
			clipboard.Set(strings.Join(pieces, "\n"))
		}
		return
	}

	if e.Selection == nil || !e.Selection.Active {
		// No selection: copy the whole current line.
		if e.Cursor.Line >= 0 && e.Cursor.Line < len(e.Buf.Lines) {
			clipboard.Set(e.Buf.Lines[e.Cursor.Line] + "\n")
		}
		return
	}

	text := e.Selection.Text(e.Buf.Lines, e.Cursor.Line, e.Cursor.Col)
	clipboard.Set(text)
}

func (g *EditorGroupWidget) Cut() {
	t := g.activeTab()
	e := g.editableKeyEditor()
	if t == nil || e == nil {
		return
	}
	// Cut from every cursor when multicursor mode is active.
	if e.isMultiActive() {
		pieces := e.multiCopyPieces()
		if len(pieces) > 0 {
			clipboard.Set(strings.Join(pieces, "\n"))
		}
		e.multiCut()
		return
	}
	if t.Sel == nil || !t.Sel.Active {
		// No selection: cut the whole current line.
		if t.Cur.Line >= 0 && t.Cur.Line < len(t.Buf.Lines) {
			clipboard.Set(t.Buf.Lines[t.Cur.Line] + "\n")
			g.Editor.DeleteLine()
		}
		return
	}

	text := t.Sel.Text(t.Buf.Lines, t.Cur.Line, t.Cur.Col)
	clipboard.Set(text)
	g.Editor.deleteSelection()
}

func (g *EditorGroupWidget) Paste() {
	if !g.IsEditorActive() {
		return
	}
	text := clipboard.Get()
	if text == "" {
		return
	}
	g.pasteIntoEditor(text)
}

func (g *EditorGroupWidget) PasteText(text string) {
	if !g.IsEditorActive() {
		return
	}
	if text == "" {
		return
	}
	g.pasteIntoEditor(text)
}

func (g *EditorGroupWidget) pasteIntoEditor(text string) {
	if d := g.activeInlineDiff(); d != nil {
		if d.headFocused() {
			return
		}
		d.revealCursorGap()
	}
	g.Editor.pasteText(text)
}

func (g *EditorGroupWidget) syncTabs() {
	g.ensureTabIdentities()
	t := g.activeTab()
	if t == nil {
		g.TabBar.SetTabs(nil)
		return
	}
	if t.Content == nil {
		if g.Editor.Buf != t.Buf {
			g.Editor.maxWidthSeen = 0
			if g.Editor.BracketPairColorization && len(t.Buf.Lines) > maxBracketColorLines {
				g.notify("Bracket pair colorization disabled for large file")
			}
			if g.bookmarkSyncPath != "" {
				if g.bookmarksByPath == nil {
					g.bookmarksByPath = make(map[string]map[int]Bookmark)
				}
				g.bookmarksByPath[g.bookmarkSyncPath] = g.Editor.Bookmarks
			}
			g.Editor.Bookmarks = g.bookmarksByPath[t.FilePath]
			g.bookmarkSyncPath = t.FilePath
		}
		g.Editor.Buf = t.Buf
		g.Editor.Cursor = t.Cur
		g.Editor.Viewport = t.Vp
		g.Editor.Undo = t.Undo
		g.Editor.Selection = t.Sel
		g.Editor.Multi = t.Multi
		g.Editor.Highlighter = t.Highlighter
		g.Editor.Diagnostics = t.Diagnostics
		g.Editor.Folds = t.Folds
		g.Editor.LineChanges = t.LineChanges
		if t.Diff != nil {
			t.Highlighter.SetSyncLines(0)
			t.Diff.bind(g.Editor)
			g.editorInDiff = true
		} else {
			t.Highlighter.SetSyncLines(highlight.DefaultSyncLines)
			g.Editor.SetDiffOverlay(t.Annotations)
			if g.editorInDiff {
				g.Editor.wrapCols = 0
			}
			g.editorInDiff = false
			g.ApplyFileWordWrap()
		}
		g.Editor.buildDiagIndex()
		g.Editor.InvalidateBracketColors()
		if t.TabSize > 0 {
			g.Editor.TabSize = t.TabSize
		}
		g.Editor.UseTabs = t.UseTabs
		g.Editor.ReadOnly = t.ReadOnly
	}
	var uiTabs []Tab
	for i, ts := range g.tabs {
		dirty := false
		if ts.Buf != nil {
			dirty = ts.Buf.Dirty
		}
		isEmptyState := g.EmptyStateID != "" && ts.FilePath == g.EmptyStateID
		closable := !(len(g.tabs) == 1 && (g.isUntouchedPlaceholder(&ts) || isEmptyState))
		name := ts.FilePath
		if ts.Title != "" {
			name = ts.Title
		}
		if ts.Diff != nil {
			name += " (diff)"
		}
		uiTabs = append(uiTabs, Tab{
			ID:       ts.ID,
			Name:     name,
			FilePath: ts.IconPath,
			Active:   i == g.active,
			Dirty:    dirty,
			Closable: closable,
			Pinned:   i < g.pinnedCount,
			ReadOnly: ts.ReadOnly || (ts.Buf != nil && ts.Buf.ReadOnlyOnDisk),
		})
	}
	g.TabBar.SetTabs(uiTabs)
	if g.lastActiveTabID != t.ID || g.lastActiveBuf != t.Buf {
		g.lastActiveTabID = t.ID
		g.lastActiveBuf = t.Buf
		if g.OnActiveTabChange != nil {
			g.OnActiveTabChange()
		}
	}
}

func (g *EditorGroupWidget) ensureTabIdentities() {
	seen := make(map[string]bool, len(g.tabs))
	for i := range g.tabs {
		id := g.tabs[i].ID
		if id != "" && !seen[id] {
			seen[id] = true
			continue
		}
		for {
			g.nextTabIdentity++
			id = fmt.Sprintf("editor-tab-%d", g.nextTabIdentity)
			if !seen[id] {
				break
			}
		}
		g.tabs[i].ID = id
		seen[id] = true
	}
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

func (g *EditorGroupWidget) Render(surface Surface) {
	g.scheduleFolds()
	g.syncTabs()
	w, h := surface.Size()
	r := g.GetRect()

	const tabBarH = 3
	if w <= 0 || h <= tabBarH {
		g.TabBar.InvalidatePointerInteraction()
		return
	}

	g.TabBar.SetRect(Rect{X: r.X, Y: r.Y, W: r.W, H: tabBarH})
	tabSurface := surface.Sub(Rect{X: 0, Y: 0, W: w, H: tabBarH})
	g.TabBar.Render(tabSurface)

	contentH := h - tabBarH
	contentRect := Rect{X: r.X, Y: r.Y + tabBarH, W: r.W, H: contentH}
	contentSurface := surface.Sub(Rect{X: 0, Y: tabBarH, W: w, H: contentH})

	t := g.activeTab()
	if t == nil {
		return
	}
	if d, ok := t.Content.(diffSignsSurface); ok {
		d.setGutterStyle(g.GutterStyle)
	}
	if t.Content != nil {
		t.Content.SetRect(contentRect)
		t.Content.Render(contentSurface)
	} else if t.Diff != nil {
		t.Diff.setGutterStyle(g.GutterStyle)
		t.Diff.SetRect(contentRect)
		t.Diff.Render(contentSurface)
	} else {
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
}

func (g *EditorGroupWidget) CancelPointerCapture() bool {
	return g.TabBar.CancelPointerCapture()
}

func (g *EditorGroupWidget) OwnsPointerCapture() bool {
	if g.TabBar.OwnsPointerCapture() {
		return true
	}
	if t := g.activeTab(); t != nil && t.Content != nil {
		owner, ok := t.Content.(widgets.PointerCaptureOwner)
		return ok && owner.OwnsPointerCapture()
	}
	if t := g.activeTab(); t != nil && t.Diff != nil {
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
	if _, isMouse := ev.(*tcell.EventMouse); !isMouse || !g.Editor.mouseDown {
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
