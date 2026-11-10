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
	existing := -1
	for i := range target.tabs {
		if tab.Buf != nil && target.tabs[i].Buf == tab.Buf {
			existing = i
			break
		}
	}
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
	if existing >= 0 {
		g.notifyContentTabClose(tab)
	}
	g.tabs = slices.Delete(g.tabs, g.active, g.active+1)
	emptied := len(g.tabs) == 0
	if emptied {
		g.tabs = []editorTab{g.placeholderTab()}
	}
	g.active = min(g.active, len(g.tabs)-1)
	g.syncTabs()

	if existing >= 0 {
		target.SwitchTab(existing)
		if emptied {
			g.notifyEmpty()
		}
		return
	}
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
	if emptied {
		g.notifyEmpty()
	}
}

func (g *EditorGroupWidget) MergeTabsFrom(source *EditorGroupWidget) {
	if g == source {
		return
	}
	activeBuf, activeContent := g.activeTab().Buf, g.activeTab().Content
	for i := 0; i < source.TabCount(); {
		source.SwitchTab(i)
		if source.isUntouchedPlaceholder(source.activeTab()) && !source.IsActiveTabPinned() {
			i++
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
