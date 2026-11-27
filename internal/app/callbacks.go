package app

import (
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

func registerWidgetCallbacks(app *App) {
	reg := app.Reg

	for i := range menuBarMenus {
		idx := i
		reg.Register(command.Command{
			ID:    menuBarLabels[idx],
			Title: i18n.Format("status.menu", i18n.Args{"name": app.MenuBar.Items[idx].Name}),
			Handler: func() {
				openMenuBarDropdown(app, idx)
			},
		})
	}

	app.MenuBar.OnSelect = func(index int) {
		openMenuBarDropdown(app, index)
	}

	app.Root.OnRightClick = func(mx, my int) {
		handleRightClick(app, mx, my)
	}

	// A click that opened a dialog (a welcome or empty-Explorer action runs on
	// the press) or claimed focus elsewhere must leave the focus there, or
	// typing goes nowhere.
	app.SplitPanel.OnLeftClick = func() {
		if app.Root.HasModalOverlay() || app.Root.FocusClaimed() {
			return
		}
		reg.Execute("sidebar.focus")
	}
	app.SplitPanel.OnRightClick = func() {}

	app.Sidebar.Tabs.Config.Actions = []widgets.TabAction{
		{Icon: string(icons.Rune(icons.More)), OnClick: app.ShowSidebarMoreMenu},
	}
	app.Sidebar.OnPanelReorder = app.persistSidebarPanelOrder

	app.Sidebar.Tabs.Config.OnOverflow = func(sx, sy int) {
		ids, titles := app.Sidebar.HiddenTabs()
		var items []ui.ContextMenuItem
		for i, id := range ids {
			panelID := id
			items = append(items, ui.ContextMenuItem{Label: titles[i], Command: "sidebar.overflow." + panelID})
			reg.Register(command.Command{
				ID:      "sidebar.overflow." + panelID,
				Title:   titles[i],
				Handler: func() { app.Sidebar.SetActivePanel(panelID) },
			})
		}
		openContextMenu(app, items, sx, sy)
	}

	app.Sidebar.OnPanelChange = func(id string) {
		if id == "debug" {
			app.refreshDebugConfiguration()
		}
		if id == "search" {
			app.applySearchHighlights()
		} else {
			app.EditorGroup.ClearSearch()
		}
		app.syncRepositoryObservation()
		if id == "outline" {
			app.RefreshSymbols()
		} else {
			app.stopSymbolsTask()
		}
		if id == "explorer" {
			app.Explorer.Reload()
		}
		if id == "plugins" && !app.registryRequested {
			app.PluginRefreshRegistry()
		}
	}

	revealSymbol := func(line, col int) {
		app.EditorGroup.GoToLine(line + 1)
		if app.EditorGroup.IsEditorActive() {
			editor := app.EditorGroup.Editor
			if editor.Cursor.Line < len(editor.Buf.Lines) {
				runes := []rune(editor.Buf.Lines[editor.Cursor.Line])
				if col > 0 && col <= len(runes) {
					editor.Cursor.Col = col
				}
			}
		}
	}
	app.Symbols.OnReveal = revealSymbol
	app.Symbols.OnJump = func(line, col int) {
		revealSymbol(line, col)
		app.Root.SetFocus(app.EditorGroup)
	}

	app.wireEditorGroupActivation(app.EditorGroup)

	app.EditorGroup.OnActiveTabChange = app.editorTabChanged

	app.EditorGroup.TabBar.OnTabClose = func(index int) {
		app.EditorGroup.SwitchTab(index)
		reg.Execute("tab.close")
	}

	app.EditorGroup.TabBar.OnTabUnpin = func(index int) {
		app.EditorGroup.SwitchTab(index)
		app.EditorGroup.TogglePinTab()
	}

	app.EditorGroup.TabBar.MoreButton.OnClick = func(sx, sy int) {
		openContextMenu(app, app.tabMoreMenu(), sx, sy)
	}

	app.EditorGroup.TabBar.OnTabRightClick = func(index, sx, sy int) {
		app.EditorGroup.SwitchTab(index)
		pinLabel := i18n.Text("menu.pinTab")
		if app.EditorGroup.IsActiveTabPinned() {
			pinLabel = i18n.Text("menu.unpinTab")
		}
		tabContextMenu := []ui.ContextMenuItem{
			{Label: i18n.Text("general.close"), Shortcut: app.KeyFor("tab.close"), Command: "tab.close"},
			{Label: i18n.Text("menu.closeOthers"), Command: "tab.closeOthers"},
			{Label: i18n.Text("menu.closeAll"), Command: "tab.closeAll"},
			{Label: i18n.Text("menu.closeAllSaved"), Command: "tab.closeAllSaved"},
			ui.MenuSep(),
			{Label: pinLabel, Shortcut: app.KeyFor("tab.pin"), Command: "tab.pin"},
		}
		if app.EditorGroup.CanMoveActiveTab(-1) {
			tabContextMenu = append(tabContextMenu, ui.ContextMenuItem{Label: i18n.Text("menu.moveTabLeft"), Command: "tab.moveLeft"})
		}
		if app.EditorGroup.CanMoveActiveTab(1) {
			tabContextMenu = append(tabContextMenu, ui.ContextMenuItem{Label: i18n.Text("menu.moveTabRight"), Command: "tab.moveRight"})
		}
		tabContextMenu = append(tabContextMenu,
			ui.MenuSep(),
			ui.ContextMenuItem{Label: i18n.Text("menu.copyAbsolutePath"), Command: "file.copyAbsolutePath"},
			ui.ContextMenuItem{Label: i18n.Text("menu.copyRelativePath"), Command: "file.copyRelativePath"},
			ui.ContextMenuItem{Label: i18n.Text("menu.revealInExplorer"), Command: "explorer.revealActiveFile"},
		)
		if app.EditorGroup.IsEditorActive() {
			tabContextMenu = append(tabContextMenu, ui.ContextMenuItem{Label: i18n.Text("menu.openToSide"), Command: "editor.openToSide"})
		}
		tabContextMenu = app.withActiveDiffViewSubmenu(tabContextMenu)
		openContextMenu(app, tabContextMenu, sx, sy)
	}

	app.Explorer.OnAction = func(id string) { reg.Execute(id) }
	app.Explorer.OnOpenFile = func(path string) {
		if err := app.EditorGroup.OpenFile(path); err != nil {
			return
		}
		app.FocusEditorIfEnabled()
	}
	app.Explorer.OnRightClick = func(node *widgets.TreeNode, sx, sy int) {
		app.ExplorerContextNode = node
		items := []ui.ContextMenuItem{
			{Label: i18n.Text("general.open"), Command: "explorer.open"},
			app.appsOpenWithMenu(),
			{Label: i18n.Text("menu.openToSide"), Command: "explorer.openToSide", Disabled: node.Expandable},
			ui.MenuSep(),
			{Label: i18n.Text("general.newFile"), Command: "explorer.newFile"},
			{Label: i18n.Text("general.newFolder"), Command: "explorer.newFolder"},
			ui.MenuSep(),
			{Label: i18n.Text("menu.copyAbsolutePath"), Command: "explorer.copyAbsolutePath"},
			{Label: i18n.Text("menu.copyRelativePath"), Command: "explorer.copyRelativePath"},
			ui.MenuSep(),
			{Label: i18n.Text("menu.revealInFileManager"), Command: "explorer.reveal"},
			{Label: i18n.Text("menu.openInTerminal"), Command: "explorer.openInTerminal"},
			ui.MenuSep(),
			{Label: i18n.Text("general.rename"), Command: "explorer.rename"},
			{Label: i18n.Text("general.duplicate"), Command: "explorer.duplicate"},
			{Label: i18n.Text("general.delete"), Command: "explorer.delete"},
			ui.MenuSep(),
			{Label: i18n.Text("general.expandAll"), Command: "explorer.expandAll"},
			{Label: i18n.Text("general.collapseAll"), Command: "explorer.collapseAll"},
		}
		openContextMenu(app, items, sx, sy)
	}
	app.Explorer.OnRootMenu = func(node *widgets.TreeNode, sx, sy int) {
		app.ExplorerContextNode = node
		favorite := ui.ContextMenuItem{Label: i18n.Text("menu.addToFavorites"), Command: "welcome.addFavorite"}
		if app.favoriteIndex(node.ID) >= 0 {
			favorite = ui.ContextMenuItem{Label: i18n.Text("menu.removeFromFavorites"), Command: "welcome.removeFavorite"}
		}
		items := []ui.ContextMenuItem{
			{Label: i18n.Text("general.refresh"), Command: "explorer.refresh"},
			app.appsOpenWithMenu(),
			{Label: i18n.Text("menu.copyPath"), Command: "explorer.copyAbsolutePath"},
			{Label: i18n.Text("menu.openInTerminal"), Command: "explorer.openInTerminal"},
			ui.MenuSep(),
			favorite,
			{Label: i18n.Text("menu.removeFromWorkspace"), Command: "explorer.removeRoot"},
			ui.MenuSep(),
			{Label: i18n.Text("general.expandAll"), Command: "explorer.expandAll"},
			{Label: i18n.Text("general.collapseAll"), Command: "explorer.collapseAll"},
		}
		openContextMenu(app, items, sx, sy)
	}

	app.Search.OnClear = func() {
		app.EditorGroup.ClearSearch()
	}
	app.Search.PostBatch = func(batch *ui.SearchBatch) {
		_ = app.Screen.PostEvent(tcell.NewEventInterrupt(batch))
	}
	app.Search.DiffSources = app.DiffSearchSources
	app.Search.OnOpenMatch = app.NavigateToSearchMatch
	app.Search.OnPreview = app.PreviewSearchReplace
	app.Search.OnReplaceRequest = app.startSearchReplace
	app.Search.OnCancelReplace = app.cancelSearchReplace

	app.Changes.OnRightClick = app.ShowChangesFileContextMenu
	app.Changes.OnRightClickFolder = app.ShowChangesFolderContextMenu
	app.Changes.OnPanelMenu = app.ShowChangesContextMenu

	app.Changes.OnOpenFile = func(path string) {
		if err := app.EditorGroup.OpenFile(path); err != nil {
			return
		}
		app.EditorGroup.DisableInlineDiff(path)
		app.FocusEditorIfEnabled()
	}
	app.Changes.OnOpenDiff = app.OpenChangeDiff
	app.Changes.OnRefreshed = app.onChangesRefreshed
	app.Changes.OnOpenCommitDiff = app.OpenCommitDiff
	app.Changes.OnOpenCommit = app.OpenCommitDetail
	app.Changes.OnOpenPRDetail = app.OpenPRDetail
	app.Changes.OnOpenPRDiff = func(group *ui.ChangesGroup, status git.FileStatus, extended bool) {
		app.OpenPRDiff(group, status, extended)
	}
	app.Changes.OnPRGroupMenu = app.ShowPRGroupMenu
	app.Changes.OnRefreshPR = app.FetchAndOpenPR
	app.Changes.OnGroupMenu = app.ShowGroupMenu
	app.Changes.OnCommit = app.CommitChanges
	app.Changes.OnConfirmDiscard = app.ConfirmDiscard
	app.Changes.OnError = app.StatusError
	app.Changes.OnRefresh = app.RefreshChanges
	app.Changes.OnStatusChanged = func() {
		app.invalidateAllRepositories(RepositoryWorktree)
	}
	app.Changes.OnHistoryResult = func(err error) {
		if app.Repository != nil {
			app.Repository.HandleHistory(err)
		}
	}
	app.Changes.Split.OnResizeEnd = app.persistCommitHistoryHeight

	app.ContentSplit.OnResize = app.resizePanel
	app.ContentSplit.OnResizeEnd = app.finishPanelResize

	app.ContentSplit.OnTopClick = app.focusEditorAfterClick

	app.ContentSplit.OnBottomClick = func() {
		if app.Root.HasModalOverlay() || app.Root.FocusClaimed() {
			return
		}
		if w := app.BottomPanel.ActiveWidget(); w != nil {
			app.Root.SetFocus(w)
		}
	}

	app.SplitPanel.OnResize = app.resizeSidebar
	app.SplitPanel.OnResizeEnd = app.persistSidebarLayout

	app.BottomPanel.Tabs.Config.OnTabClick = func(index int) {
		panels := app.BottomPanel.PanelIDs()
		if index >= 0 && index < len(panels) {
			app.BottomPanel.SetActivePanel(panels[index])
			if w := app.BottomPanel.ActiveWidget(); w != nil {
				app.Root.SetFocus(w)
			}
		}
	}

	app.BottomPanel.Tabs.Config.OnOverflow = func(sx, sy int) {
		ids, titles := app.BottomPanel.HiddenTabs()
		var items []ui.ContextMenuItem
		for i, id := range ids {
			panelID := id
			commandID := "bottom.overflow." + panelID
			items = append(items, ui.ContextMenuItem{Label: titles[i], Command: commandID})
			reg.Register(command.Command{
				ID:    commandID,
				Title: titles[i],
				Handler: func() {
					app.BottomPanel.SetActivePanel(panelID)
					app.FocusPanel()
				},
			})
		}
		openContextMenu(app, items, sx, sy)
	}

	app.TerminalPanel.TabBar.OnRightClick = func(index, sx, sy int) {
		app.TerminalPanel.SetActive(index)
		app.Root.SetFocus(app.TerminalPanel)
		items := []ui.ContextMenuItem{
			{Label: i18n.Text("menu.closeTerminal"), Command: "terminal.close"},
			{Label: i18n.Text("menu.closeAllTerminals"), Command: "terminal.closeAll"},
		}
		openContextMenu(app, items, sx, sy)
	}

	app.BottomPanel.Tabs.Config.Actions = []widgets.TabAction{
		{Icon: string(icons.Rune(icons.Add)), OnClick: func(_, _ int) {
			reg.Execute("terminal.new")
		}},
		{Icon: string(icons.Rune(icons.More)), OnClick: func(sx, sy int) {
			dock := ui.ContextMenuItem{Label: i18n.Text("menu.dockRight"), Command: "panel.dockRight"}
			if app.ContentSplit.Position == ui.SplitRight {
				dock = ui.ContextMenuItem{Label: i18n.Text("menu.dockBottom"), Command: "panel.dockBottom"}
			}
			items := []ui.ContextMenuItem{
				{Label: i18n.Text("menu.newTerminal"), Command: "terminal.new"},
				ui.MenuSep(),
				{Label: i18n.Text("menu.closeTerminal"), Command: "terminal.close"},
				{Label: i18n.Text("menu.closeAllTerminals"), Command: "terminal.closeAll"},
				ui.MenuSep(),
				dock,
				{Label: i18n.Text("menu.closePanel"), Command: "panel.toggle"},
			}
			openContextMenu(app, items, sx, sy)
		}},
	}
}
