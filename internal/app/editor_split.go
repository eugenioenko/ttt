package app

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func (a *App) editorGroups() []*ui.EditorGroupWidget {
	if a.EditorSplit == nil {
		return []*ui.EditorGroupWidget{a.EditorGroup}
	}
	return a.EditorSplit.Groups()
}

func (a *App) wireEditorGroupActivation(g *ui.EditorGroupWidget) {
	g.OnActivate = func() {
		a.activateEditorGroup(g)
		if !a.Root.HasOverlay() {
			a.Root.SetFocus(g)
		}
	}
	g.OnEmpty = func() { a.editorGroupEmpty(g) }
	a.wireEditorTabDrag(g)
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

func (a *App) SplitEditor()     { a.splitEditor(ui.SplitRight) }
func (a *App) SplitEditorDown() { a.splitEditor(ui.SplitBottom) }

func (a *App) splitEditor(position ui.SplitPosition) {
	if a.Root.HasOverlay() || len(a.editorGroups()) >= ui.MaxEditorGroups {
		return
	}
	a.FlushEditorOnChange()
	left := a.EditorGroup
	right := a.newEditorGroupLike(left)
	a.EditorSplit.SplitGroup(left, right, position)
	a.activateEditorGroup(right)
	a.FocusEditor()
}

func (a *App) CloseEditorSplit() { a.removeEditorGroup(a.EditorGroup, true) }

func (a *App) editorGroupEmpty(group *ui.EditorGroupWidget) {
	if len(a.editorGroups()) > 1 {
		a.removeEditorGroup(group, false)
		return
	}
	if a.welcomeWhenEmpty && len(a.Workspace.Paths()) == 0 {
		a.ShowEmptyState()
	}
}

func (a *App) removeEditorGroup(closing *ui.EditorGroupWidget, merge bool) {
	if a.Root.HasOverlay() || a.EditorSplit == nil || len(a.editorGroups()) < 2 {
		return
	}
	a.FlushEditorOnChange()
	a.Root.CancelPointerCapture()
	groups := a.editorGroups()
	var keep *ui.EditorGroupWidget
	for i, group := range groups {
		if group == closing {
			keep = groups[(i+len(groups)-1)%len(groups)]
			break
		}
	}
	a.activateEditorGroup(keep)
	closing.OnEmpty = nil
	if merge {
		keep.DiscardDuplicateViews(closing)
		keep.MergeTabsFrom(closing)
	}
	a.EditorSplit.RemoveGroup(closing)
	closing.OnEmpty, closing.OnActivate, closing.OpenFileElsewhere = nil, nil, nil
	closing.CloseAllTabs()
	if closing.FoldTask != nil {
		a.releaseTask(closing.FoldTask)
	}
	if closing.FoldDelay != nil {
		a.releaseTask(closing.FoldDelay)
	}
	a.FocusEditor()
}

func (a *App) focusEditorAfterClick() {
	if !a.Root.HasOverlay() && !a.Root.FocusClaimed() {
		a.FocusEditor()
	}
}

func (a *App) EqualizeEditorGroups() { a.EditorSplit.Equalize() }
func (a *App) FocusEditorLeft()      { a.focusEditorDirection(-1, 0) }
func (a *App) FocusEditorRight()     { a.focusEditorDirection(1, 0) }
func (a *App) FocusEditorUp()        { a.focusEditorDirection(0, -1) }
func (a *App) FocusEditorDown()      { a.focusEditorDirection(0, 1) }
func (a *App) focusEditorDirection(dx, dy int) {
	if a.Root.HasOverlay() {
		return
	}
	if next := a.EditorSplit.Neighbor(a.EditorGroup, dx, dy); next != nil {
		a.Root.SetFocus(next)
	}
}

func (a *App) newEditorGroupLike(left *ui.EditorGroupWidget) *ui.EditorGroupWidget {
	right := newEditorGroup(a.Settings, a.Borders, left.BracketColorStyles)
	right.ControlModifiers = left.ControlModifiers
	right.SetChevrons(left.ChevronCollapsed, left.ChevronExpanded)
	right.SetImageCellSize(a.imageCellW, a.imageCellH)
	right.SetImageRedraw(left.ImageRedraw)
	right.TabBar.OnTabClose = left.TabBar.OnTabClose
	right.TabBar.OnTabUnpin = left.TabBar.OnTabUnpin
	right.TabBar.OnTabRightClick = left.TabBar.OnTabRightClick
	right.TabBar.MoreButton.OnClick = left.TabBar.MoreButton.OnClick
	if left.TabBar.PostDragAutoScrollTick != nil {
		a.wireEditorAutoScroll(right)
	}
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
	right.OnActiveTabChange = a.editorTabChanged
	a.wireEditorGroupActivation(right)
	right.NameUntitledPlaceholder()
	return right
}

func (a *App) isEditorPathDirty(path string) bool {
	for _, group := range a.editorGroups() {
		if group.IsDirtyPath(path) {
			return true
		}
	}
	return false
}

func (a *App) openEditorFiles() []ui.OpenDocument {
	var files []ui.OpenDocument
	seen := map[string]bool{}
	for _, group := range a.editorGroups() {
		for _, file := range group.OpenFiles() {
			if !seen[file.Path] {
				seen[file.Path] = true
				files = append(files, file)
			}
		}
	}
	return files
}

func (a *App) openEditorDocuments() []ui.OpenDocument {
	docs := a.openEditorFiles()
	for i := range docs {
		docs[i].Text = strings.Join(a.lspLines(docs[i].Path), "\n")
	}
	return docs
}

func (a *App) wireEditorAutoScroll(group *ui.EditorGroupWidget) {
	group.TabBar.PostDragAutoScrollTick = func(generation uint64) {
		a.Screen.PostEvent(tcell.NewEventInterrupt(&ui.TabDragAutoScrollTick{Target: group.TabBar, Generation: generation}))
	}
	group.Editor.PostDragAutoScrollTick = func(generation uint64) {
		a.Screen.PostEvent(tcell.NewEventInterrupt(&ui.EditorDragAutoScrollTick{Target: group.Editor, Generation: generation}))
	}
}
