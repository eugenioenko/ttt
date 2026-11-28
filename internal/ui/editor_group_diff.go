package ui

import (
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/fold"
)

func (g *EditorGroupWidget) tabIndexByPath(path string) int {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path && g.tabs[i].Content == nil && g.tabs[i].Buf != nil {
			return i
		}
	}
	return -1
}

func (g *EditorGroupWidget) EnableInlineDiff(path string, base []string) bool {
	i := g.tabIndexByPath(path)
	if i < 0 {
		return false
	}
	t := &g.tabs[i]
	t.Preview = false
	d := NewEditableDiffWidget(t.FilePath, base, diff.FullDiffLines(base, t.Buf.Lines))
	d.SetSyntaxHighlight(g.SyntaxHighlight)
	g.ApplyDiffDefaults(d)
	if t.Diff != nil {
		t.Diff.Close()
	}
	t.Diff = d
	if i == g.active {
		g.syncTabs()
	}
	return true
}

func (g *EditorGroupWidget) DisableInlineDiff(path string) {
	i := g.tabIndexByPath(path)
	if i < 0 || g.tabs[i].Diff == nil {
		return
	}
	t := &g.tabs[i]
	t.Diff.Close()
	t.Diff = nil
	if t.Folds != nil {
		t.Folds.SetRanges(fold.ComputeIndentRanges(t.Buf.Lines))
	}
	if i == g.active {
		g.Editor.SetDiffOverlay(nil)
		g.syncTabs()
	}
}

func (g *EditorGroupWidget) SetInlineDiff(path string, base []string, lines []diff.DiffLine, ver uint64, snap []string) {
	i := g.tabIndexByPath(path)
	if i < 0 || g.tabs[i].Diff == nil {
		return
	}
	g.tabs[i].Diff.SetLiveDiff(base, lines, ver, snap)
}

func (g *EditorGroupWidget) IsInlineDiffPath(path string) bool {
	i := g.tabIndexByPath(path)
	return i >= 0 && g.tabs[i].Diff != nil
}

func (g *EditorGroupWidget) IsInlineDiffActive() bool {
	return g.activeInlineDiff() != nil
}

func (g *EditorGroupWidget) ActiveInlineDiff() *DiffEditorWidget {
	return g.activeInlineDiff()
}

func (g *EditorGroupWidget) activeInlineDiff() *DiffEditorWidget {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return nil
	}
	return t.Diff
}

// keyEditor is the pane editor commands act on: the one that has the keys,
// which in an editable split diff is the read-only base pane once clicked.
func (g *EditorGroupWidget) keyEditor() *EditorPaneWidget {
	if m := g.visualMerge(); m != nil {
		return m.pane(m.focus)
	}
	if d := g.activeInlineDiff(); d != nil {
		return d.keyPane()
	}
	if g.IsEditorActive() {
		return g.Editor
	}
	return nil
}

func (g *EditorGroupWidget) editableKeyEditor() *EditorPaneWidget {
	if e := g.keyEditor(); e != nil && !e.ReadOnly {
		return e
	}
	return nil
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
