package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/view"
)

func (g *EditorGroupWidget) activeTab() *editorTab {
	if len(g.tabs) == 0 || g.active < 0 || g.active >= len(g.tabs) {
		return nil
	}
	return &g.tabs[g.active]
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

func (g *EditorGroupWidget) SwitchToTabByPath(path string) bool {
	for i, t := range g.tabs {
		if t.FilePath == path {
			g.SwitchTab(i)
			return true
		}
	}
	return false
}

func (g *EditorGroupWidget) FocusPluginTab(key string) bool {
	if key == "" {
		return false
	}
	for i, t := range g.tabs {
		if t.PluginKey == key {
			g.SwitchTab(i)
			return true
		}
	}
	return false
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
	g.rememberClosed(closing)
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

const maxClosedTabs = 20

type closedTab struct {
	path      string
	line, col int
}

// rememberClosed records a closed file tab for ReopenClosedTab. Tabs without a
// file on disk to reopen (plugin, virtual, diff and placeholder tabs) are skipped.
func (g *EditorGroupWidget) rememberClosed(tab editorTab) {
	if tab.FilePath == "" || tab.PluginKey != "" || tab.Virtual || tab.Placeholder || tab.Diff != nil {
		return
	}
	c := closedTab{path: tab.FilePath}
	if tab.Cur != nil {
		c.line, c.col = tab.Cur.Line, tab.Cur.Col
	}
	g.closedTabs = slices.DeleteFunc(g.closedTabs, func(o closedTab) bool { return o.path == c.path })
	g.closedTabs = append(g.closedTabs, c)
	if len(g.closedTabs) > maxClosedTabs {
		g.closedTabs = g.closedTabs[1:]
	}
}

func (g *EditorGroupWidget) HasClosedTabs() bool { return len(g.closedTabs) > 0 }

// ReopenClosedTab reopens the most recently closed file tab at its cursor
// position. Files deleted since are skipped, since OpenFile would start a new
// empty file in their place. It reports whether a tab was reopened.
func (g *EditorGroupWidget) ReopenClosedTab() bool {
	for len(g.closedTabs) > 0 {
		c := g.closedTabs[len(g.closedTabs)-1]
		g.closedTabs = g.closedTabs[:len(g.closedTabs)-1]
		if info, err := os.Stat(c.path); err != nil || info.IsDir() {
			continue
		}
		if g.OpenFile(c.path) != nil {
			continue
		}
		if t := g.activeTab(); t != nil && t.Preview {
			t.Preview = false
			g.syncTabs()
		}
		g.GoToLineCol(c.line+1, c.col+1)
		return true
	}
	return false
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
		g.rememberClosed(tab)
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
		g.rememberClosed(g.tabs[i])
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
		if g.tabs[i].Buf != nil && g.tabs[i].Buf.Dirty && !g.otherDocumentView(g.tabs[i]) {
			return true
		}
	}
	return false
}

func (g *EditorGroupWidget) CloseAllTabs() {
	prev := g.activeContent()
	for i := g.pinnedCount; i < len(g.tabs); i++ {
		g.rememberClosed(g.tabs[i])
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
		g.rememberClosed(g.tabs[i])
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
		if g.tabs[i].Buf != nil && g.tabs[i].Buf.Dirty && !g.otherDocumentView(g.tabs[i]) {
			return true
		}
	}
	return false
}

func (g *EditorGroupWidget) canReplaceTab(t *editorTab) bool {
	if t == nil || g.IsActiveTabPinned() {
		return false
	}
	if t.Preview && g.PreviewTabs {
		return true
	}
	return len(g.tabs) == 1 && g.isUntouchedPlaceholder(t)
}

func (g *EditorGroupWidget) isUntouchedPlaceholder(t *editorTab) bool {
	return t != nil && t.Placeholder && t.Buf != nil && !t.Buf.Dirty && t.Buf.Version() == 0 &&
		len(t.Buf.Lines) <= 1 && (len(t.Buf.Lines) == 0 || t.Buf.Lines[0] == "")
}

func (g *EditorGroupWidget) OpenPluginTab(key, title string, content Widget) {
	if adapter, ok := content.(*WidgetAdapter); ok {
		adapter.SetControlModifiers(g.ControlModifiers)
	}
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
		FilePath:    g.nextUntitledName(nil),
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
	if !g.otherDocumentView(tab) && g.OnFileClose != nil && tab.DetectedLanguage != "" && !tab.Virtual {
		g.OnFileClose(tab.FilePath, tab.DetectedLanguage)
	}
	g.releaseTabResources(tab)
}

func (g *EditorGroupWidget) releaseTabResources(tab editorTab) {
	tab.Highlighter.Close()
	if tab.autoClosing != nil {
		tab.autoClosing.syntax.Close()
	}
	if tab.Diff != nil {
		tab.Diff.Close()
	}
	if tab.Content == nil {
		return
	}
	if closer, ok := tab.Content.(interface{ Close() }); ok {
		closer.Close()
	}
	if g.OnContentTabClose != nil && !g.otherContentView(tab) {
		g.OnContentTabClose(tab.FilePath)
	}
}

func (g *EditorGroupWidget) syncTabs() {
	g.ensureTabIdentities()
	t := g.activeTab()
	if t == nil {
		g.TabBar.SetTabs(nil)
		return
	}
	if path := documentPath(t); path != "" {
		g.lastDocumentPath = path
	}
	if t.Content != nil || g.Editor.Buf != t.Buf {
		g.Editor.goalContext = cursorGoalContext{}
	}
	if t.Content == nil {
		if g.Editor.Buf != t.Buf {
			g.Editor.maxWidthSeen = 0
			if g.Editor.BracketPairColorization && len(t.Buf.Lines) > maxBracketColorLines {
				g.notify(i18n.Text("editorGroup.bracketPairColorizationDisabledForLargeFile"))
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
		if t.autoClosing == nil {
			t.autoClosing = &autoClosingState{}
		}
		g.Editor.autoClosing = t.autoClosing
		g.Editor.autoClosingPath = t.FilePath
		g.Editor.plainText = t.LanguageSelection == highlight.PlainText
		g.ApplyFileAutoSurround()
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
		t.rememberView()
	}
	var uiTabs []Tab
	for i, ts := range g.tabs {
		dirty := false
		if ts.Buf != nil {
			dirty = ts.Buf.Dirty
		}
		isEmptyState := g.EmptyStateID != "" && ts.FilePath == g.EmptyStateID
		closable := len(g.relatedGroups()) > 1 || !(len(g.tabs) == 1 && (g.isUntouchedPlaceholder(&ts) || isEmptyState))
		name := ts.FilePath
		if ts.Title != "" {
			name = ts.Title
		}
		viewSuffix := ""
		if ts.Diff != nil {
			viewSuffix = " (diff)"
			name += viewSuffix
		} else if ts.Title == "" && ts.diffWidget() != nil {
			switch filepath.Base(ts.FilePath) {
			case filepath.Base(ts.IconPath) + " (diff)":
				viewSuffix = " (diff)"
			case filepath.Base(ts.IconPath) + " (staged)":
				viewSuffix = " (staged)"
			}
		}
		icon := ""
		if provider, ok := ts.Content.(interface{ TabIcon(string) string }); ok {
			icon = provider.TabIcon(g.TabBar.Icons)
		}
		uiTabs = append(uiTabs, Tab{
			Icon:       icon,
			ID:         ts.ID,
			Name:       name,
			ViewSuffix: viewSuffix,
			FilePath:   ts.IconPath,
			Active:     i == g.active,
			Dirty:      dirty,
			Closable:   closable,
			Pinned:     i < g.pinnedCount,
			ReadOnly:   ts.ReadOnly || (ts.Buf != nil && ts.Buf.ReadOnlyOnDisk),
		})
	}
	g.TabBar.SetTabs(uiTabs)
	if g.lastActiveTabID != t.ID || g.lastActiveBuf != t.Buf {
		g.Editor.clearLinkHover()
		if d := t.diffWidget(); d != nil {
			for _, p := range d.viewPanes() {
				p.clearLinkHover()
			}
		}
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
