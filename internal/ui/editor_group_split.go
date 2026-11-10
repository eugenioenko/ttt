package ui

import "slices"

func (g *EditorGroupWidget) MoveActiveTabTo(target *EditorGroupWidget) {
	if g == target || g.activeTab() == nil {
		return
	}
	g.InvalidatePointerInteraction()
	target.InvalidatePointerInteraction()
	g.saveMultiState()
	target.saveMultiState()
	tab := g.tabs[g.active]
	tab.Preview = false
	pinned := g.active < g.pinnedCount
	if pinned {
		g.pinnedCount--
	}
	if tab.Content == nil {
		if target.bookmarksByPath == nil {
			target.bookmarksByPath = make(map[string]map[int]Bookmark)
		}
		target.bookmarksByPath[tab.FilePath] = g.Editor.Bookmarks
	}
	g.tabs = slices.Delete(g.tabs, g.active, g.active+1)
	if len(g.tabs) == 0 {
		g.tabs = []editorTab{g.placeholderTab()}
	}
	g.active = min(g.active, len(g.tabs)-1)
	g.syncTabs()

	if len(target.tabs) == 1 && target.isUntouchedPlaceholder(target.activeTab()) {
		target.tabs = nil
		target.pinnedCount = 0
	}
	index := len(target.tabs)
	if pinned {
		index = target.pinnedCount
		target.pinnedCount++
	}
	target.tabs = slices.Insert(target.tabs, index, tab)
	target.active = index
	target.syncTabs()
}

func (g *EditorGroupWidget) MergeTabsFrom(source *EditorGroupWidget) {
	if g == source {
		return
	}
	activeBuf, activeContent := g.activeTab().Buf, g.activeTab().Content
	count := source.TabCount()
	for range count {
		source.SwitchTab(0)
		if source.isUntouchedPlaceholder(source.activeTab()) {
			continue
		}
		source.MoveActiveTabTo(g)
	}
	for i := range g.tabs {
		if g.tabs[i].Buf == activeBuf && g.tabs[i].Content == activeContent {
			g.SwitchTab(i)
			break
		}
	}
}
