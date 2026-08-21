package ui

import (
	"errors"
	"fmt"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/highlight"
	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
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
	FilePath    string
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
	LineChanges []diff.LineChangeKind
	ReadOnly    bool
}

type EditorGroupWidget struct {
	BaseWidget
	TabBar                  *TabBarWidget
	Editor                  *EditorPaneWidget
	Autocomplete            *AutocompleteWidget
	Hover                   *HoverWidget
	SignatureHelp           *SignatureHelpWidget
	tabs                    []editorTab
	active                  int
	pinnedCount             int
	TabSize                 int
	InsertSpaces            bool
	LineNumbers             bool
	GutterStyle             string
	WordWrap                bool
	DiffMode                DiffMode
	DiffWordWrap            bool
	DiffHighContrast        bool
	SyntaxHighlight         bool
	BracketPairColorization bool
	BracketColorStyles      []term.Style
	InsertFinalNewline      bool
	ShowTrailingNewline     bool
	TrimTrailingWhitespace  bool
	UndoDeleteCursorStart   bool
	Borders                 *term.BorderSet
	OnFileOpen              func(path, lang, text string)
	OnFileChange            func(path, lang, text string)
	OnFileClose             func(path, lang string)
	OnContentTabClose       func(id string)
	// OnActiveContentChange fires after a tab switch or an in-place replacement
	// changes what the editor group is showing.
	OnActiveContentChange func()
	OnError               func(msg string)
	OnNotify              func(msg string)
	pendingNotify         []string
	focused               bool
	// diagSources holds diagnostics keyed by source ("lsp", "plugin:<name>")
	// then by file path. Merged per-path into each tab's Diagnostics.
	diagSources map[string]map[string][]Diagnostic
	// OnDiagnosticsChanged fires whenever any source's diagnostics change, so
	// the Diagnostics panel can rebuild from DiagnosticsByPath().
	OnDiagnosticsChanged func()
}

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
	}
	tabBar.OnTabClick = func(index int) {
		g.SwitchTab(index)
	}
	tabBar.OnTabReorder = func(from, to int) {
		g.MoveTab(from, to)
	}
	tabBar.OnNextTab = func() { g.NextTab() }
	tabBar.OnPrevTab = func() { g.PrevTab() }
	tabBar.OnDoubleClick = func() { g.NewFile() }
	undoStack := g.newUndoStack()
	sel := &selection.Selection{}
	editor.Undo = undoStack
	editor.Selection = sel
	g.tabs = []editorTab{{
		FilePath: "untitled",
		Buf:      editor.Buf,
		Cur:      editor.Cursor,
		Vp:       editor.Viewport,
		Undo:     undoStack,
		Sel:      sel,
		Virtual:  true,
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

func (g *EditorGroupWidget) activeContentChanged() {
	if g.OnActiveContentChange != nil {
		g.OnActiveContentChange()
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

// MoveTab reorders a tab without changing its pinned state or the active
// document. Dragging a preview is an explicit placement action, so the preview
// becomes a regular tab instead of remaining eligible for replacement.
func (g *EditorGroupWidget) MoveTab(from, to int) bool {
	if from < 0 || from >= len(g.tabs) || to < 0 || to >= len(g.tabs) {
		return false
	}
	if from < g.pinnedCount {
		to = min(to, g.pinnedCount-1)
	} else {
		to = max(to, g.pinnedCount)
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
	for i := range g.tabs {
		if g.tabs[i].FilePath == path {
			wasActive := i == g.active
			g.tabs[i].Preview = false
			if g.tabs[i].Buf != nil && !g.tabs[i].Buf.Dirty {
				g.tabs[i].Buf.LoadFile(path)
			}
			g.SwitchTab(i)
			if wasActive {
				g.activeContentChanged()
			}
			return
		}
	}
	newBuf := &buffer.Buffer{Lines: []string{""}, InsertFinalNewline: g.InsertFinalNewline, ShowTrailingNewline: g.ShowTrailingNewline, TrimTrailingWhitespace: g.TrimTrailingWhitespace}
	ec := config.LoadEditorConfig(path)
	if ec.InsertFinalNLSet {
		newBuf.InsertFinalNewline = ec.InsertFinalNewline
	}
	if ec.TrimTrailingWSSet {
		newBuf.TrimTrailingWhitespace = ec.TrimTrailingWS
	}
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
	folds.SetRanges(fold.ComputeIndentRanges(newBuf.Lines))
	newTab := editorTab{
		FilePath: path,
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
		newTab.Highlighter = highlight.New(path)
	}
	if t := g.activeTab(); t != nil && t.Preview && t.Content == nil && t.Buf != nil && !t.Buf.Dirty {
		g.tabs[g.active] = newTab
		g.syncTabs()
		g.activeContentChanged()
	} else {
		g.tabs = append(g.tabs, newTab)
		g.SwitchTab(len(g.tabs) - 1)
	}
	if g.OnFileOpen != nil && newTab.Highlighter != nil {
		g.OnFileOpen(path, newTab.Highlighter.Language(), strings.Join(newBuf.Lines, "\n"))
	}
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

// OpenDiffTab opens a diff under an explicit tab key and label. Callers that can
// produce several diffs of the same file — one per commit, say — need their own
// key, or the second one would replace the first under an identical label.
// path is still the real file path so the highlighter picks the right language.
func (g *EditorGroupWidget) OpenDiffTab(tabName, title, path string, fd diff.FileDiff, oldLines, newLines []string, extended bool) {
	for i, t := range g.tabs {
		if t.FilePath == tabName {
			wasActive := i == g.active
			dw := NewDiffViewWidget(path, fd, oldLines, newLines, extended)
			g.ApplyDiffDefaults(dw)
			if !g.SyntaxHighlight {
				dw.Highlighter = nil
			}
			t.Content = dw
			t.Title = title
			g.tabs[i] = t
			g.SwitchTab(i)
			if wasActive {
				g.activeContentChanged()
			}
			return
		}
	}
	widget := NewDiffViewWidget(path, fd, oldLines, newLines, extended)
	g.ApplyDiffDefaults(widget)
	if !g.SyntaxHighlight {
		widget.Highlighter = nil
	}
	g.tabs = append(g.tabs, editorTab{
		FilePath: tabName,
		Title:    title,
		Content:  widget,
	})
	g.SwitchTab(len(g.tabs) - 1)
}

// ApplyDiffDefaults initializes or refreshes a reading surface. Each property
// follows the current Options default until the reader overrides that property
// in View.
func (g *EditorGroupWidget) ApplyDiffDefaults(surface DiffModeSurface) {
	surface.ApplyDefaultMode(g.DiffMode)
	wrapMode := DiffWrapOff
	if g.DiffWordWrap {
		wrapMode = DiffWrapOn
	}
	surface.ApplyDefaultWrapMode(wrapMode)
	surface.SetDiffHighContrast(g.DiffHighContrast)
}

// SetDiffDefaults updates the defaults used by future diff surfaces and
// refreshes every open surface that still inherits either property.
func (g *EditorGroupWidget) SetDiffDefaults(mode DiffMode, wordWrap bool) {
	g.DiffMode = mode
	g.DiffWordWrap = wordWrap
	for _, tab := range g.tabs {
		if surface, ok := tab.Content.(DiffModeSurface); ok {
			g.ApplyDiffDefaults(surface)
		}
	}
}

func (g *EditorGroupWidget) SetDiffHighContrast(enabled bool) {
	g.DiffHighContrast = enabled
	for _, tab := range g.tabs {
		if surface, ok := tab.Content.(DiffModeSurface); ok {
			surface.SetDiffHighContrast(enabled)
		}
	}
}

func (g *EditorGroupWidget) OpenPluginTab(id, title string, content Widget) {
	for i, t := range g.tabs {
		if t.FilePath == id {
			wasActive := i == g.active
			t.Content = content
			t.Title = title
			g.tabs[i] = t
			g.SwitchTab(i)
			if wasActive {
				g.activeContentChanged()
			}
			return
		}
	}
	g.tabs = append(g.tabs, editorTab{
		FilePath: id,
		Title:    title,
		Content:  content,
	})
	g.SwitchTab(len(g.tabs) - 1)
}

func (g *EditorGroupWidget) ClosePluginTab(id string) {
	for i, t := range g.tabs {
		if t.FilePath == id {
			wasActive := i == g.active
			if t.Content != nil && g.OnContentTabClose != nil {
				g.OnContentTabClose(t.FilePath)
			}
			if i < g.pinnedCount {
				g.pinnedCount--
			}
			g.tabs = append(g.tabs[:i], g.tabs[i+1:]...)
			if len(g.tabs) == 0 {
				g.tabs = []editorTab{{
					FilePath: "untitled",
					Buf:      &buffer.Buffer{Lines: []string{""}},
					Cur:      &cursor.Cursor{},
					Vp:       &view.Viewport{},
					Undo:     g.newUndoStack(),
					Sel:      &selection.Selection{},
					Virtual:  true,
				}}
				g.active = 0
			} else if g.active >= len(g.tabs) {
				g.active = len(g.tabs) - 1
			}
			g.syncTabs()
			if wasActive {
				g.activeContentChanged()
			}
			return
		}
	}
}

func (g *EditorGroupWidget) ReloadFile(path string) {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path && g.tabs[i].Buf != nil {
			g.tabs[i].Buf.LoadFile(path)
			g.tabs[i].Buf.Dirty = false
			g.tabs[i].Undo = g.newUndoStack()
			if g.tabs[i].Folds != nil {
				g.tabs[i].Folds.SetRanges(fold.ComputeIndentRanges(g.tabs[i].Buf.Lines))
			}
			g.clampCursor(&g.tabs[i])
			if i == g.active {
				g.syncTabs()
			}
			return
		}
	}
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

func (g *EditorGroupWidget) ActiveDiffWidget() *DiffViewWidget {
	t := g.activeTab()
	if t == nil || t.Content == nil {
		return nil
	}
	if dv, ok := t.Content.(*DiffViewWidget); ok {
		return dv
	}
	return nil
}

func (g *EditorGroupWidget) ActiveCommitDetailWidget() *CommitDetailWidget {
	t := g.activeTab()
	if t == nil || t.Content == nil {
		return nil
	}
	if detail, ok := t.Content.(*CommitDetailWidget); ok {
		return detail
	}
	return nil
}

func (g *EditorGroupWidget) ActiveDiffModeSurface() DiffModeSurface {
	if diffView := g.ActiveDiffWidget(); diffView != nil {
		return diffView
	}
	if detail := g.ActiveCommitDetailWidget(); detail != nil {
		return detail
	}
	return nil
}

func (g *EditorGroupWidget) DiffWidgetByTab(tabName string) *DiffViewWidget {
	for _, t := range g.tabs {
		if t.FilePath == tabName {
			if dv, ok := t.Content.(*DiffViewWidget); ok {
				return dv
			}
			return nil
		}
	}
	return nil
}

// CommitDetailWidgetByTab returns a still-open commit detail tab. Async detail
// results use this as their identity check: if the reader closed the loading
// tab before Git finished, the result is dropped instead of reopening it.
func (g *EditorGroupWidget) CommitDetailWidgetByTab(tabName string) *CommitDetailWidget {
	for _, t := range g.tabs {
		if t.FilePath == tabName {
			if detail, ok := t.Content.(*CommitDetailWidget); ok {
				return detail
			}
			return nil
		}
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
		if dv, ok := t.Content.(*DiffViewWidget); ok {
			result = append(result, DiffSearchSource{TabName: t.FilePath, Lines: dv.CombinedLines()})
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
		if t := g.activeTab(); t != nil && t.Content != nil {
			if setter, ok := t.Content.(interface{ SetFocused(bool) }); ok {
				setter.SetFocused(false)
			}
		}
		g.saveMultiState()
		g.active = idx
		g.syncTabs()
		if g.focused {
			if t := g.activeTab(); t != nil && t.Content != nil {
				if setter, ok := t.Content.(interface{ SetFocused(bool) }); ok {
					setter.SetFocused(true)
				}
			}
		}
		if changed {
			g.activeContentChanged()
		}
	}
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
	if g.OnFileClose != nil && closing.Highlighter != nil && !closing.Virtual {
		g.OnFileClose(closing.FilePath, closing.Highlighter.Language())
	}
	if closing.Content != nil && g.OnContentTabClose != nil {
		g.OnContentTabClose(closing.FilePath)
	}
	if g.active < g.pinnedCount {
		g.pinnedCount--
	}
	g.tabs = append(g.tabs[:g.active], g.tabs[g.active+1:]...)
	if len(g.tabs) == 0 {
		g.tabs = []editorTab{{
			FilePath: "untitled",
			Buf:      &buffer.Buffer{Lines: []string{""}},
			Cur:      &cursor.Cursor{},
			Vp:       &view.Viewport{},
			Undo:     g.newUndoStack(),
			Sel:      &selection.Selection{},
			Virtual:  true,
		}}
		g.active = 0
	} else if g.active >= len(g.tabs) {
		g.active = len(g.tabs) - 1
	}
	g.syncTabs()
	g.activeContentChanged()
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
		if g.OnContentTabClose != nil && tab.Content != nil {
			g.OnContentTabClose(tab.FilePath)
		}
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
		if g.tabs[i].Content != nil && g.OnContentTabClose != nil {
			g.OnContentTabClose(g.tabs[i].FilePath)
		}
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
	activeChanged := g.pinnedCount == 0 || g.active != 0
	kept := slices.Clone(g.tabs[:g.pinnedCount])
	if len(kept) == 0 {
		kept = []editorTab{{
			FilePath: "untitled",
			Buf:      &buffer.Buffer{Lines: []string{""}},
			Cur:      &cursor.Cursor{},
			Vp:       &view.Viewport{},
			Undo:     g.newUndoStack(),
			Sel:      &selection.Selection{},
			Virtual:  true,
		}}
		g.pinnedCount = 0
	}
	g.tabs = kept
	g.active = 0
	g.syncTabs()
	if activeChanged {
		g.activeContentChanged()
	}
}

func (g *EditorGroupWidget) CloseAllSaved() {
	activeFile := g.ActiveFilePath()
	var kept []editorTab
	for i := range g.tabs {
		if i < g.pinnedCount || (g.tabs[i].Buf != nil && g.tabs[i].Buf.Dirty) {
			kept = append(kept, g.tabs[i])
		}
	}
	if len(kept) == 0 {
		g.CloseAllTabs()
		return
	}
	g.tabs = kept
	if g.active >= len(g.tabs) {
		g.active = len(g.tabs) - 1
	}
	g.syncTabs()
	if g.ActiveFilePath() != activeFile {
		g.activeContentChanged()
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
	return true
}

func (g *EditorGroupWidget) SaveAs(path string) {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return
	}
	g.applySaveCleanups(t)
	if err := t.Buf.SaveFile(path); err != nil {
		g.reportError(fmt.Sprintf("Failed to save %s: %v", path, err))
		return
	}
	if t.Undo != nil {
		t.Undo.MarkSaved()
	}
	t.FilePath = path
	t.Virtual = false
	if g.SyntaxHighlight {
		t.Highlighter = highlight.New(path)
	} else {
		t.Highlighter = nil
	}
	g.syncTabs()
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
	prefix := oldPath + string(filepath.Separator)
	renamed := false
	for i := range g.tabs {
		t := &g.tabs[i]
		if t.Virtual || t.Content != nil || t.Buf == nil {
			continue
		}
		var updated string
		switch {
		case t.FilePath == oldPath:
			updated = newPath
		case strings.HasPrefix(t.FilePath, prefix):
			updated = filepath.Join(newPath, strings.TrimPrefix(t.FilePath, prefix))
		default:
			continue
		}
		if g.OnFileClose != nil && t.Highlighter != nil {
			g.OnFileClose(t.FilePath, t.Highlighter.Language())
		}
		t.FilePath = updated
		if g.SyntaxHighlight {
			t.Highlighter = highlight.New(updated)
		} else {
			t.Highlighter = nil
		}
		if g.OnFileOpen != nil && t.Highlighter != nil {
			g.OnFileOpen(updated, t.Highlighter.Language(), strings.Join(t.Buf.Lines, "\n"))
		}
		renamed = true
	}
	if renamed {
		g.syncTabs()
	}
	return renamed
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
		tabTitle = filepath.Base(path) + " (readonly)"
	}
	newTab := editorTab{
		FilePath: path,
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
		newTab.Highlighter = highlight.New(path)
	}
	g.tabs = append(g.tabs, newTab)
	g.SwitchTab(len(g.tabs) - 1)
}

func (g *EditorGroupWidget) OpenBufferReadOnly(title, filePath string, lines []string) {
	for i := range g.tabs {
		if g.tabs[i].Title == title && g.tabs[i].ReadOnly {
			wasActive := i == g.active
			g.tabs[i].Buf.Lines = lines
			g.SwitchTab(i)
			if wasActive {
				g.activeContentChanged()
			}
			return
		}
	}
	newBuf := &buffer.Buffer{Lines: lines}
	if len(lines) == 0 {
		newBuf.Lines = []string{""}
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
		newTab.Highlighter = highlight.New(filePath)
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
		g.Editor.Folds.SetRanges(fold.ComputeIndentRanges(g.Editor.Buf.Lines))
		g.Editor.ExpandFoldContaining(g.Editor.Cursor.Line)
	}
	g.Editor.bufferDirty = true
}

func (g *EditorGroupWidget) Undo() {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return
	}
	if t.Undo != nil {
		if pos := t.Undo.Undo(t.Buf); pos != nil {
			g.Editor.Cursor.Line = pos.Line
			g.Editor.Cursor.Col = pos.Col
		}
		if t.Undo.AtSavePoint() {
			t.Buf.Dirty = false
		}
		g.undoRedoPostProcess()
	}
}

func (g *EditorGroupWidget) Redo() {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return
	}
	if t.Undo != nil {
		if pos := t.Undo.Redo(t.Buf); pos != nil {
			g.Editor.Cursor.Line = pos.Line
			g.Editor.Cursor.Col = pos.Col
		}
		if t.Undo.AtSavePoint() {
			t.Buf.Dirty = false
		}
		g.undoRedoPostProcess()
	}
}

func (g *EditorGroupWidget) SelectAll() {
	t := g.activeTab()
	if t == nil || t.Content != nil || t.Sel == nil {
		return
	}
	t.Sel.Start(0, 0)
	lastLine := len(t.Buf.Lines) - 1
	t.Cur.Line = lastLine
	t.Cur.Col = len([]rune(t.Buf.Lines[lastLine]))
}

func (g *EditorGroupWidget) SetSearch(query string, matches []FindMatch) {
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
	if !g.IsEditorActive() {
		return
	}
	g.Editor.SearchQuery = ""
	g.Editor.SearchMatches = nil
	g.Editor.SearchActive = 0
	g.Editor.searchByLine = nil
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
	if !g.IsEditorActive() || len(g.Editor.SearchMatches) == 0 {
		return
	}
	cur := g.Editor.SearchActive
	cur = (cur + 1) % len(g.Editor.SearchMatches)
	g.Editor.SearchActive = cur
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
	if !g.IsEditorActive() || len(g.Editor.SearchMatches) == 0 {
		return
	}
	cur := g.Editor.SearchActive
	cur = (cur - 1 + len(g.Editor.SearchMatches)) % len(g.Editor.SearchMatches)
	g.Editor.SearchActive = cur
	m := g.Editor.SearchMatches[cur]
	g.Editor.ExpandFoldContaining(m.Line)
	g.Editor.Cursor.Line = m.Line
	g.Editor.Cursor.Col = m.Col
	g.Editor.scrollViewport()
}

func (g *EditorGroupWidget) MoveLineUp() {
	if g.IsEditorActive() {
		g.Editor.MoveLineUp()
	}
}

func (g *EditorGroupWidget) MoveLineDown() {
	if g.IsEditorActive() {
		g.Editor.MoveLineDown()
	}
}

func (g *EditorGroupWidget) DuplicateLine() {
	if g.IsEditorActive() {
		g.Editor.DuplicateLine()
	}
}

func (g *EditorGroupWidget) DeleteLine() {
	if g.IsEditorActive() {
		g.Editor.DeleteLine()
	}
}

func (g *EditorGroupWidget) JoinLines() {
	if g.IsEditorActive() {
		g.Editor.JoinLines()
	}
}

func (g *EditorGroupWidget) InsertLineBelow() {
	if g.IsEditorActive() {
		g.Editor.InsertLineBelow()
	}
}

func (g *EditorGroupWidget) InsertLineAbove() {
	if g.IsEditorActive() {
		g.Editor.InsertLineAbove()
	}
}

func (g *EditorGroupWidget) ToggleLineComment() {
	if g.IsEditorActive() {
		g.Editor.ToggleLineComment()
	}
}

func (g *EditorGroupWidget) SortLinesAsc() {
	if g.IsEditorActive() {
		g.Editor.SortLinesAsc()
	}
}

func (g *EditorGroupWidget) SortLinesDesc() {
	if g.IsEditorActive() {
		g.Editor.SortLinesDesc()
	}
}

func (g *EditorGroupWidget) ReverseLines() {
	if g.IsEditorActive() {
		g.Editor.ReverseLines()
	}
}

func (g *EditorGroupWidget) UniqueLines() {
	if g.IsEditorActive() {
		g.Editor.UniqueLines()
	}
}

func (g *EditorGroupWidget) MoveWordLeft(shift bool) {
	if g.IsEditorActive() {
		g.Editor.MoveWordLeft(shift)
	}
}

func (g *EditorGroupWidget) MoveWordRight(shift bool) {
	if g.IsEditorActive() {
		g.Editor.MoveWordRight(shift)
	}
}

func (g *EditorGroupWidget) DeleteWordLeft() {
	if g.IsEditorActive() {
		g.Editor.DeleteWordLeft()
	}
}

func (g *EditorGroupWidget) DeleteWordRight() {
	if g.IsEditorActive() {
		g.Editor.DeleteWordRight()
	}
}

func (g *EditorGroupWidget) SelectNextOccurrence() {
	if g.IsEditorActive() {
		g.Editor.SelectNextOccurrence()
		g.saveMultiState()
	}
}

func (g *EditorGroupWidget) SelectAllOccurrences() {
	if g.IsEditorActive() {
		g.Editor.SelectAllOccurrences()
		g.saveMultiState()
	}
}

func (g *EditorGroupWidget) UndoLastCursor() {
	if g.IsEditorActive() {
		g.Editor.UndoLastCursor()
		g.saveMultiState()
	}
}

func (g *EditorGroupWidget) SplitSelectionToLines() {
	if g.IsEditorActive() {
		g.Editor.SplitSelectionToLines()
		g.saveMultiState()
	}
}

func (g *EditorGroupWidget) UpperCase() {
	if g.IsEditorActive() {
		g.Editor.UpperCase()
	}
}

func (g *EditorGroupWidget) LowerCase() {
	if g.IsEditorActive() {
		g.Editor.LowerCase()
	}
}

func (g *EditorGroupWidget) TitleCase() {
	if g.IsEditorActive() {
		g.Editor.TitleCase()
	}
}

func (g *EditorGroupWidget) GoToMatchingBracket() {
	if g.IsEditorActive() {
		g.Editor.GoToMatchingBracket()
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
	if dv, ok := t.Content.(*DiffViewWidget); ok {
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
	if t.Sel == nil || !t.Sel.Active {
		// No selection: copy the whole current line, including a trailing
		// newline so a paste inserts it as a full line.
		if t.Cur.Line >= 0 && t.Cur.Line < len(t.Buf.Lines) {
			clipboard.Set(t.Buf.Lines[t.Cur.Line] + "\n")
		}
		return
	}
	text := t.Sel.Text(t.Buf.Lines, t.Cur.Line, t.Cur.Col)
	clipboard.Set(text)
}

func (g *EditorGroupWidget) Cut() {
	t := g.activeTab()
	if t == nil || t.Content != nil {
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
	g.Editor.pasteText(text)
}

func (g *EditorGroupWidget) PasteText(text string) {
	if !g.IsEditorActive() {
		return
	}
	if text == "" {
		return
	}
	g.Editor.pasteText(text)
}

func (g *EditorGroupWidget) syncTabs() {
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
		isEmptyUntitledTab := ts.Virtual && ts.Buf != nil && !ts.Buf.Dirty &&
			len(ts.Buf.Lines) <= 1 && (len(ts.Buf.Lines) == 0 || ts.Buf.Lines[0] == "")
		closable := !(len(g.tabs) == 1 && isEmptyUntitledTab)
		name := ts.FilePath
		if ts.Title != "" {
			name = ts.Title
		}
		uiTabs = append(uiTabs, Tab{
			Name:     name,
			Active:   i == g.active,
			Dirty:    dirty,
			Closable: closable,
			Pinned:   i < g.pinnedCount,
		})
	}
	g.TabBar.SetTabs(uiTabs)
	g.TabBar.Controls = nil
	if surface := g.ActiveDiffModeSurface(); surface != nil {
		g.TabBar.Controls = []TabBarControl{
			{
				Label:  "Split",
				Active: surface.Mode() == DiffModeSplit,
				OnClick: func() {
					surface.SetMode(DiffModeSplit)
				},
			},
			{
				Label:  "Unified",
				Active: surface.Mode() == DiffModeUnified,
				OnClick: func() {
					surface.SetMode(DiffModeUnified)
				},
			},
		}
	}
}

func (g *EditorGroupWidget) Render(surface Surface) {
	g.syncTabs()
	w, h := surface.Size()
	r := g.GetRect()

	const tabBarH = 3
	if h <= tabBarH {
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
	if t.Content != nil {
		t.Content.SetRect(contentRect)
		t.Content.Render(contentSurface)
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
	result := g.TabBar.HandleEvent(ev)
	slog.Debug("editorGroup", "tabBarResult", result)
	if result != EventIgnored {
		return result
	}
	t := g.activeTab()
	if t == nil {
		return EventIgnored
	}
	if t.Content != nil {
		result = t.Content.HandleEvent(ev)
		if result != EventIgnored {
			return result
		}
		if _, ok := ev.(*tcell.EventMouse); ok {
			return EventConsumed
		}
		return EventIgnored
	}
	result = g.Editor.HandleEvent(ev)
	g.saveMultiState()
	return result
}
