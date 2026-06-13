package ui

import (
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
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
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
	Message   string
	Source    string
}

type editorTab struct {
	FilePath    string
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
	Content     Widget
	Pinned      bool
	Bookmarks   map[int]bool
}

type EditorGroupWidget struct {
	BaseWidget
	TabBar                 *TabBarWidget
	Editor                 *EditorPaneWidget
	Autocomplete           *AutocompleteWidget
	Hover                  *HoverWidget
	SignatureHelp          *SignatureHelpWidget
	tabs                   []editorTab
	active                 int
	TabSize                int
	LineNumbers            bool
	GutterStyle            string
	InsertFinalNewline     bool
	TrimTrailingWhitespace bool
	Borders                *term.BorderSet
	OnFileOpen             func(path, lang, text string)
	OnFileChange           func(path, lang, text string)
	OnFileClose            func(path, lang string)
	OnError                func(msg string)
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
	undoStack := &undo.UndoStack{}
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
	}}
	g.syncTabs()
	return g
}

func (g *EditorGroupWidget) Focusable() bool { return true }

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

func (g *EditorGroupWidget) PinActiveTab() {
	if t := g.activeTab(); t != nil {
		t.Pinned = true
	}
}

func (g *EditorGroupWidget) OpenFile(path string) {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path {
			g.tabs[i].Pinned = true
			if g.tabs[i].Buf != nil && !g.tabs[i].Buf.Dirty {
				g.tabs[i].Buf.LoadFile(path)
			}
			g.SwitchTab(i)
			return
		}
	}
	newBuf := &buffer.Buffer{Lines: []string{""}, InsertFinalNewline: g.InsertFinalNewline, TrimTrailingWhitespace: g.TrimTrailingWhitespace}
	ec := config.LoadEditorConfig(path)
	if ec.InsertFinalNLSet {
		newBuf.InsertFinalNewline = ec.InsertFinalNewline
	}
	if ec.TrimTrailingWSSet {
		newBuf.TrimTrailingWhitespace = ec.TrimTrailingWS
	}
	if err := newBuf.LoadFile(path); err != nil {
		g.reportError(fmt.Sprintf("Failed to open %s: %v", path, err))
		return
	}
	tabSize := g.TabSize
	if ec.IndentSize > 0 {
		tabSize = ec.IndentSize
	} else if detected := buffer.DetectIndent(newBuf.Lines); detected.Size > 0 {
		tabSize = detected.Size
	}
	folds := fold.NewState()
	folds.SetRanges(fold.ComputeIndentRanges(newBuf.Lines))
	newTab := editorTab{
		FilePath:    path,
		Buf:         newBuf,
		Cur:         &cursor.Cursor{},
		Vp:          &view.Viewport{},
		Undo:        &undo.UndoStack{},
		Sel:         &selection.Selection{},
		Highlighter: highlight.New(path),
		Folds:       folds,
		TabSize:     tabSize,
	}
	if t := g.activeTab(); t != nil && !t.Pinned && t.Content == nil && t.Buf != nil && !t.Buf.Dirty {
		g.tabs[g.active] = newTab
		g.syncTabs()
	} else {
		g.tabs = append(g.tabs, newTab)
		g.SwitchTab(len(g.tabs) - 1)
	}
	if g.OnFileOpen != nil && newTab.Highlighter != nil {
		g.OnFileOpen(path, newTab.Highlighter.Language(), strings.Join(newBuf.Lines, "\n"))
	}
}

func (g *EditorGroupWidget) OpenBuffer(path string, buf *buffer.Buffer) {
	for i, t := range g.tabs {
		if t.FilePath == path {
			g.SwitchTab(i)
			return
		}
	}
	folds := fold.NewState()
	folds.SetRanges(fold.ComputeIndentRanges(buf.Lines))
	g.tabs = append(g.tabs, editorTab{
		FilePath: path,
		Buf:      buf,
		Cur:      &cursor.Cursor{},
		Vp:       &view.Viewport{},
		Undo:     &undo.UndoStack{},
		Sel:      &selection.Selection{},
		Folds:    folds,
	})
	g.SwitchTab(len(g.tabs) - 1)
}

func (g *EditorGroupWidget) OpenDiff(path string, fd diff.FileDiff, oldLines, newLines []string, extended bool) {
	tabName := path + " (diff)"
	for i, t := range g.tabs {
		if t.FilePath == tabName {
			t.Content = NewDiffViewWidget(path, fd, oldLines, newLines, extended)
			g.tabs[i] = t
			g.SwitchTab(i)
			return
		}
	}
	widget := NewDiffViewWidget(path, fd, oldLines, newLines, extended)
	g.tabs = append(g.tabs, editorTab{
		FilePath: tabName,
		Content:  widget,
	})
	g.SwitchTab(len(g.tabs) - 1)
}

func (g *EditorGroupWidget) ReloadFile(path string) {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path && g.tabs[i].Buf != nil {
			g.tabs[i].Buf.LoadFile(path)
			g.tabs[i].Buf.Dirty = false
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
		if t.FilePath == "" || t.FilePath == "untitled" {
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

func (g *EditorGroupWidget) SwitchTab(idx int) {
	if idx >= 0 && idx < len(g.tabs) {
		g.saveMultiState()
		g.active = idx
		g.syncTabs()
	}
}

func (g *EditorGroupWidget) saveMultiState() {
	if t := g.activeTab(); t != nil && t.Content == nil {
		t.Multi = g.Editor.Multi
		t.Bookmarks = g.Editor.Bookmarks
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
	if g.OnFileClose != nil && closing.Highlighter != nil && closing.FilePath != "untitled" {
		g.OnFileClose(closing.FilePath, closing.Highlighter.Language())
	}
	g.tabs = append(g.tabs[:g.active], g.tabs[g.active+1:]...)
	if len(g.tabs) == 0 {
		g.tabs = []editorTab{{
			FilePath: "untitled",
			Buf:      &buffer.Buffer{Lines: []string{""}},
			Cur:      &cursor.Cursor{},
			Vp:       &view.Viewport{},
			Undo:     &undo.UndoStack{},
			Sel:      &selection.Selection{},
		}}
		g.active = 0
	} else if g.active >= len(g.tabs) {
		g.active = len(g.tabs) - 1
	}
	g.syncTabs()
}

func (g *EditorGroupWidget) CloseOtherTabs() {
	t := g.activeTab()
	if t == nil || len(g.tabs) <= 1 {
		return
	}
	g.tabs = []editorTab{*t}
	g.active = 0
	g.syncTabs()
}

func (g *EditorGroupWidget) CloseAllTabs() {
	g.tabs = []editorTab{{
		FilePath: "untitled",
		Buf:      &buffer.Buffer{Lines: []string{""}},
		Cur:      &cursor.Cursor{},
		Vp:       &view.Viewport{},
		Undo:     &undo.UndoStack{},
		Sel:      &selection.Selection{},
	}}
	g.active = 0
	g.syncTabs()
}

func (g *EditorGroupWidget) Save() bool {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return false
	}
	if t.FilePath == "untitled" {
		return false
	}
	if err := t.Buf.SaveFile(t.FilePath); err != nil {
		g.reportError(fmt.Sprintf("Failed to save %s: %v", t.FilePath, err))
		return false
	}
	return true
}

func (g *EditorGroupWidget) SaveAs(path string) {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return
	}
	if err := t.Buf.SaveFile(path); err != nil {
		g.reportError(fmt.Sprintf("Failed to save %s: %v", path, err))
		return
	}
	t.FilePath = path
	t.Highlighter = highlight.New(path)
	g.syncTabs()
}

func (g *EditorGroupWidget) ActiveFilePath() string {
	if t := g.activeTab(); t != nil {
		return t.FilePath
	}
	return ""
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
	g.Editor.InvalidateMaxLineWidth()
	if g.Editor.Folds != nil {
		g.Editor.Folds.SetRanges(fold.ComputeIndentRanges(g.Editor.Buf.Lines))
		g.Editor.ExpandFoldContaining(g.Editor.Cursor.Line)
	}
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

func (g *EditorGroupWidget) GoToLine(line int) {
	if !g.IsEditorActive() {
		return
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

func (g *EditorGroupWidget) SetDiagnostics(path string, diags []Diagnostic) {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path {
			g.tabs[i].Diagnostics = diags
			if i == g.active {
				g.Editor.Diagnostics = diags
				g.Editor.buildDiagIndex()
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

func (g *EditorGroupWidget) ReplaceAll(query, replacement string) {
	if !g.IsEditorActive() || query == "" {
		return
	}
	matches, _ := FindInLines(g.Editor.Buf.Lines, query, SearchOptions{})
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

func (g *EditorGroupWidget) ToggleBookmark() {
	if g.IsEditorActive() {
		g.Editor.ToggleBookmark()
		g.saveMultiState()
	}
}

func (g *EditorGroupWidget) NextBookmark() {
	if g.IsEditorActive() {
		g.Editor.NextBookmark()
	}
}

func (g *EditorGroupWidget) PrevBookmark() {
	if g.IsEditorActive() {
		g.Editor.PrevBookmark()
	}
}

func (g *EditorGroupWidget) ClearBookmarks() {
	if g.IsEditorActive() {
		g.Editor.ClearBookmarks()
		g.saveMultiState()
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
	if t.Sel == nil || !t.Sel.Active {
		return
	}
	text := t.Sel.Text(t.Buf.Lines, t.Cur.Line, t.Cur.Col)
	clipboard.Set(text)
}

func (g *EditorGroupWidget) Cut() {
	t := g.activeTab()
	if t == nil || t.Content != nil || t.Sel == nil || !t.Sel.Active {
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

func (g *EditorGroupWidget) syncTabs() {
	t := g.activeTab()
	if t == nil {
		g.TabBar.SetTabs(nil)
		return
	}
	if t.Content == nil {
		g.Editor.Buf = t.Buf
		g.Editor.Cursor = t.Cur
		g.Editor.Viewport = t.Vp
		g.Editor.Undo = t.Undo
		g.Editor.Selection = t.Sel
		g.Editor.Multi = t.Multi
		g.Editor.Highlighter = t.Highlighter
		g.Editor.Diagnostics = t.Diagnostics
		g.Editor.Folds = t.Folds
		g.Editor.Bookmarks = t.Bookmarks
		g.Editor.buildDiagIndex()
		g.Editor.InvalidateMaxLineWidth()
		if t.TabSize > 0 {
			g.Editor.TabSize = t.TabSize
		}
	}
	var uiTabs []Tab
	for i, ts := range g.tabs {
		dirty := false
		if ts.Buf != nil {
			dirty = ts.Buf.Dirty
		}
		closable := true
		if len(g.tabs) == 1 && ts.FilePath == "untitled" && ts.Buf != nil && !ts.Buf.Dirty && len(ts.Buf.Lines) <= 1 && (len(ts.Buf.Lines) == 0 || ts.Buf.Lines[0] == "") {
			closable = false
		}
		uiTabs = append(uiTabs, Tab{
			Name:     ts.FilePath,
			Active:   i == g.active,
			Dirty:    dirty,
			Closable: closable,
		})
	}
	g.TabBar.SetTabs(uiTabs)
}

func (g *EditorGroupWidget) Render(surface *RenderSurface) {
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

	if g.Hover != nil && len(g.Hover.Lines) > 0 {
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
	if result == EventConsumed {
		return EventConsumed
	}
	t := g.activeTab()
	if t == nil {
		return EventIgnored
	}
	if t.Content != nil {
		return t.Content.HandleEvent(ev)
	}
	result = g.Editor.HandleEvent(ev)
	g.saveMultiState()
	return result
}
