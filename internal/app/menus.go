package app

import (
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v2"
)

var menuBarLabels = []string{
	"menu.file", "menu.edit", "menu.selection", "menu.view", "menu.help",
}

var menuBarMenus = [][]ui.ContextMenuItem{
	// File
	{
		{Label: "New File", Command: "file.new"},
		ui.MenuSep(),
		{Label: "Save", Command: "file.save"},
		{Label: "Save As...", Command: "file.saveAs"},
		ui.MenuSep(),
		{Label: "Open Folder", Command: "workspace.openFolder"},
		{Label: "Add Folder", Command: "workspace.addFolder"},
		ui.MenuSep(),
		{Label: "Open Workspace", Command: "workspace.open"},
		{Label: "Save Workspace", Command: "workspace.save"},
		ui.MenuSep(),
		{Label: "Review PR", Command: "pr.review"},
		ui.MenuSep(),
		{Label: "Quit", Command: "editor.quit"},
	},
	// Edit
	{
		{Label: "Undo", Command: "editor.undo"},
		{Label: "Redo", Command: "editor.redo"},
		ui.MenuSep(),
		{Label: "Cut", Command: "editor.cut"},
		{Label: "Copy", Command: "editor.copy"},
		{Label: "Paste", Command: "editor.paste"},
		ui.MenuSep(),
		{Label: "Find", Command: "search.find"},
		{Label: "Replace", Command: "search.replace"},
	},
	// Selection
	{
		{Label: "Select All", Command: "editor.selectAll"},
		ui.MenuSep(),
		{Label: "Add Next Occurrence", Command: "multicursor.selectNext"},
		{Label: "Select All Occurrences", Command: "multicursor.selectAll"},
		{Label: "Undo Last Cursor", Command: "multicursor.undoCursor"},
	},
	// View
	{
		{Label: "Command Palette", Command: "command.palette"},
		{Label: "Quick Open", Command: "file.quickOpen"},
		ui.MenuSep(),
		{Label: "Explore", Command: "sidebar.explorer"},
		{Label: "Find", Command: "sidebar.search"},
		{Label: "Replace", Command: "sidebar.searchReplace"},
		{Label: "Changes", Command: "sidebar.changes"},
		ui.MenuSep(),
		{Label: "Toggle Sidebar", Command: "sidebar.toggle"},
		{Label: "Toggle Terminal", Command: "terminal.toggle"},
		{Label: "New Terminal", Command: "terminal.new"},
		ui.MenuSep(),
		{Label: "Navigate Back", Command: "navigate.back"},
		{Label: "Navigate Forward", Command: "navigate.forward"},
		ui.MenuSep(),
		{Label: "Switch Theme", Command: "theme.switch"},
		ui.MenuSep(),
		{Label: "Keyboard Tester", Command: "view.keyboardTester"},
	},
	// Help
	{
		{Label: "About", Command: "about"},
	},
}

var editorContextMenu = []ui.ContextMenuItem{
	{Label: "Go to Definition", Command: "editor.goToDefinition"},
	{Label: "Go to Type Definition", Command: "editor.goToTypeDefinition"},
	{Label: "Go to Implementation", Command: "editor.goToImplementation"},
	{Label: "Find All References", Command: "editor.findReferences"},
	{Label: "Rename Symbol", Command: "editor.rename"},
	ui.MenuSep(),
	{Label: "Format Document", Command: "editor.formatDocument"},
	{Label: "Format Selection", Command: "editor.formatSelection"},
	ui.MenuSep(),
	{Label: "Undo", Command: "editor.undo"},
	{Label: "Redo", Command: "editor.redo"},
	ui.MenuSep(),
	{Label: "Cut", Command: "editor.cut"},
	{Label: "Copy", Command: "editor.copy"},
	{Label: "Paste", Command: "editor.paste"},
	ui.MenuSep(),
	{Label: "Select All", Command: "editor.selectAll"},
	ui.MenuSep(),
	{Label: "Find", Command: "search.find"},
	{Label: "Replace", Command: "search.replace"},
	{Label: "Go to Line", Command: "editor.goToLine"},
	ui.MenuSep(),
	{Label: "Navigate Back", Command: "navigate.back"},
	{Label: "Navigate Forward", Command: "navigate.forward"},
}

var diffContextMenu = []ui.ContextMenuItem{
	{Label: "Copy", Command: "editor.copy"},
	ui.MenuSep(),
	{Label: "Find", Command: "search.find"},
}

var changesContextMenuStaged = []ui.ContextMenuItem{
	{Label: "Open Compact Diff", Command: "changes.openDiff"},
	{Label: "Open Extended Diff", Command: "changes.openExtendedDiff"},
	{Label: "Open File", Command: "changes.openFile"},
	ui.MenuSep(),
	{Label: "Unstage", Command: "changes.unstage"},
}

var changesContextMenuUnstaged = []ui.ContextMenuItem{
	{Label: "Open Compact Diff", Command: "changes.openDiff"},
	{Label: "Open Extended Diff", Command: "changes.openExtendedDiff"},
	{Label: "Open File", Command: "changes.openFile"},
	ui.MenuSep(),
	{Label: "Stage", Command: "changes.stage"},
	{Label: "Discard Changes", Command: "changes.discard"},
}

func resolveShortcuts(reg *command.Registry, items []ui.ContextMenuItem) []ui.ContextMenuItem {
	resolved := make([]ui.ContextMenuItem, len(items))
	for i, item := range items {
		resolved[i] = item
		if item.Command != "" {
			if cmd, ok := reg.Get(item.Command); ok && cmd.Shortcut != "" {
				resolved[i].Shortcut = cmd.Shortcut
			}
		}
	}
	return resolved
}

func openContextMenu(app *App, items []ui.ContextMenuItem, x, y int) {
	reg := app.Reg
	menu := ui.NewContextMenuWidget(resolveShortcuts(reg, items), x, y)
	menu.Borders = app.Borders
	menu.OnExec = func(cmd string) {
		app.Root.PopOverlay()
		reg.Execute(cmd)
	}
	menu.OnDismiss = func() {
		app.Root.PopOverlay()
	}
	app.Root.PushOverlay(ui.Overlay{Widget: menu, Modal: true})
	app.Root.SetFocus(menu)
}

func openMenuBarDropdown(app *App, index int) {
	if index < 0 || index >= len(menuBarMenus) {
		return
	}
	reg := app.Reg
	app.MenuBar.Selected = index
	anchorX := app.MenuBar.ItemAnchorX(index)
	menu := ui.NewContextMenuWidget(resolveShortcuts(reg, menuBarMenus[index]), anchorX, 1)
	menu.Borders = app.Borders
	menu.OnExec = func(cmd string) {
		app.Root.PopOverlay()
		app.MenuBar.Selected = -1
		reg.Execute(cmd)
	}
	menu.OnDismiss = func() {
		app.Root.PopOverlay()
		app.MenuBar.Selected = -1
	}
	menu.OnNavigate = func(dir int) {
		app.Root.PopOverlay()
		next := (index + dir + len(menuBarMenus)) % len(menuBarMenus)
		openMenuBarDropdown(app, next)
	}
	app.Root.PushOverlay(ui.Overlay{Widget: menu, Modal: true})
	app.Root.SetFocus(menu)
}

func handleRightClick(app *App, mx, my int) {
	panelRect := app.SplitPanel.GetRect()
	if my < panelRect.Y || my >= panelRect.Y+panelRect.H {
		return
	}

	if app.Sidebar.Visible {
		divX := app.SplitPanel.DividerScreenX()
		if mx < divX {
			sidebarR := app.Sidebar.GetRect()
			if my > sidebarR.Y+1 {
				ev := tcell.NewEventMouse(mx, my, tcell.Button2, 0)
				if w := app.Sidebar.ActiveWidget(); w != nil {
					w.HandleEvent(ev)
				}
			}
			return
		}
	}

	tabR := app.EditorGroup.TabBar.GetRect()
	if my >= tabR.Y && my < tabR.Y+tabR.H {
		ev := tcell.NewEventMouse(mx, my, tcell.Button2, 0)
		app.EditorGroup.TabBar.HandleEvent(ev)
		return
	}

	if app.EditorGroup.ActiveDiffWidget() != nil {
		openContextMenu(app, diffContextMenu, mx, my)
	} else {
		openContextMenu(app, editorContextMenu, mx, my)
	}
}
