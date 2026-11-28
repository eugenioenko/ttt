package ui

import (
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/highlight"
)

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
		h := highlight.NewForLanguage(t.LanguageSelection)
		if h != nil {
			h.SetFilename(t.FilePath)
		}
		return h
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
