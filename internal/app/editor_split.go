package app

import "github.com/eugenioenko/ttt/internal/ui"

func (a *App) editorGroups() []*ui.EditorGroupWidget {
	if a.EditorSplit == nil || !a.EditorSplit.ShowBottom {
		return []*ui.EditorGroupWidget{a.EditorGroup}
	}
	return []*ui.EditorGroupWidget{a.EditorSplit.Top.(*ui.EditorGroupWidget), a.EditorSplit.Bottom.(*ui.EditorGroupWidget)}
}

func (a *App) wireEditorGroupActivation(g *ui.EditorGroupWidget) {
	g.OnActivate = func() {
		a.activateEditorGroup(g)
		if !a.Root.HasOverlay() {
			a.Root.SetFocus(g)
		}
	}
	g.RelatedGroups = a.editorGroups
	g.OpenFileElsewhere = func(path string) bool {
		if g.BufferForPath(path) != nil {
			return false
		}
		for _, other := range a.editorGroups() {
			if other != g && g.OpenSharedFileFrom(other, path) {
				return true
			}
		}
		return false
	}
}

func (a *App) activateEditorGroup(g *ui.EditorGroupWidget) {
	if a.EditorGroup == g {
		return
	}
	a.FlushEditorOnChange()
	a.CloseAutocomplete()
	a.DismissHover()
	a.DismissSignatureHelp()
	a.Root.ClearEditorLinkHover()
	if a.EditorGroup.Editor.Undo != nil {
		a.EditorGroup.Editor.Undo.BreakGroup()
	}
	a.EditorGroup = g
	if g.OnActiveTabChange != nil {
		g.OnActiveTabChange()
	}
}

func (a *App) SplitEditor() {
	if a.Root.HasOverlay() || a.EditorSplit.ShowBottom {
		return
	}
	a.FlushEditorOnChange()
	left := a.EditorGroup
	right := newEditorGroup(a.Settings, a.Borders, left.BracketColorStyles)
	right.SetChevrons(left.ChevronCollapsed, left.ChevronExpanded)
	right.SetImageCellSize(a.imageCellW, a.imageCellH)
	right.SetImageRedraw(left.ImageRedraw)
	right.TabBar.OnTabClose = left.TabBar.OnTabClose
	right.TabBar.OnTabUnpin = left.TabBar.OnTabUnpin
	right.TabBar.OnTabRightClick = left.TabBar.OnTabRightClick
	right.TabBar.MoreButton.OnClick = left.TabBar.MoreButton.OnClick
	right.TabBar.PostDragAutoScrollTick = left.TabBar.PostDragAutoScrollTick
	right.Editor.PostDragAutoScrollTick = left.Editor.PostDragAutoScrollTick
	right.Editor.OnChange = left.Editor.OnChange
	right.OnError = left.OnError
	right.OnNotify = left.OnNotify
	right.OnFileOpen = left.OnFileOpen
	right.OnFileClose = left.OnFileClose
	right.OnContentTabClose = left.OnContentTabClose
	right.OnDiagnosticsChanged = left.OnDiagnosticsChanged
	right.FileHandler = left.FileHandler
	right.OnFileHandled = left.OnFileHandled
	right.LinkWorkDir = left.LinkWorkDir
	right.OnOpenURL = left.OnOpenURL
	right.OnOpenFileLink = left.OnOpenFileLink
	right.OnLinkHover = left.OnLinkHover
	right.ResolveFileLinks = left.ResolveFileLinks
	right.OnBookmarkChanged = left.OnBookmarkChanged
	right.OnGutterClick = left.OnGutterClick
	right.OnEmpty = left.OnEmpty
	right.OnActiveTabChange = a.editorTabChanged
	a.wireEditorGroupActivation(right)
	if !left.CloneActiveTabTo(right) {
		left.MoveActiveTabTo(right)
	}
	a.EditorSplit.SetPanels(left, right)
	a.EditorSplit.ShowBottom = true
	a.EditorSplit.RightW = max((a.EditorSplit.GetRect().W-1)/2, 1)
	a.activateEditorGroup(right)
	a.FocusEditor()
}

func (a *App) CloseEditorSplit() {
	if a.Root.HasOverlay() || a.EditorSplit == nil || !a.EditorSplit.ShowBottom {
		return
	}
	a.FlushEditorOnChange()
	a.Root.CancelPointerCapture()
	keep := a.EditorGroup
	for _, group := range a.editorGroups() {
		if group != keep {
			keep.DiscardDuplicateViews(group)
			keep.MergeTabsFrom(group)
			group.OnActivate = nil
			group.OpenFileElsewhere = nil
			group.OnEmpty = nil
			group.CloseAllTabs()
			if group.FoldTask != nil {
				a.releaseTask(group.FoldTask)
			}
			if group.FoldDelay != nil {
				a.releaseTask(group.FoldDelay)
			}
		}
	}
	a.EditorSplit.SetPanels(keep, nil)
	a.EditorSplit.ShowBottom = false
	a.FocusEditor()
}

func (a *App) focusEditorAfterClick() {
	if !a.Root.HasOverlay() && !a.Root.FocusClaimed() {
		a.FocusEditor()
	}
}
