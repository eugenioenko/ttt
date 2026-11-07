package app

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v3"
)

var menuBarLabels = []string{
	"menu.file", "menu.edit", "menu.selection", "menu.view", "menu.options", "menu.help",
}

type menuEntry struct {
	Label   i18n.Message
	Command string
}

func menuItems(entries []menuEntry) []ui.ContextMenuItem {
	items := make([]ui.ContextMenuItem, len(entries))
	for i, entry := range entries {
		if entry.Command == "" {
			items[i] = ui.MenuSep()
		} else {
			items[i] = ui.ContextMenuItem{Label: entry.Label.Text(), Command: entry.Command}
		}
	}
	return items
}

var menuBarMenus = [][]menuEntry{
	// File
	{
		{Label: i18n.Key("general.newFile"), Command: "file.new"},
		{},
		{Label: i18n.Key("general.save"), Command: "file.save"},
		{Label: i18n.Key("menu.saveAs"), Command: "file.saveAs"},
		{},
		{Label: i18n.Key("general.openFile"), Command: "file.open"},
		{Label: i18n.Key("general.openFolder"), Command: "workspace.openFolder"},
		{Label: i18n.Key("general.addFolder"), Command: "workspace.addFolder"},
		{},
		{Label: i18n.Key("general.openWorkspace"), Command: "workspace.open"},
		{Label: i18n.Key("general.saveWorkspace"), Command: "workspace.save"},
		{Label: i18n.Key("general.closeWorkspace"), Command: "workspace.close"},
		{},
		{Label: i18n.Key("menu.openPRDiff"), Command: "pr.openDiff"},
		{},
		{Label: i18n.Key("general.quit"), Command: "editor.quit"},
	},
	// Edit
	{
		{Label: i18n.Key("general.undo"), Command: "editor.undo"},
		{Label: i18n.Key("general.redo"), Command: "editor.redo"},
		{},
		{Label: i18n.Key("general.cut"), Command: "editor.cut"},
		{Label: i18n.Key("general.copy"), Command: "editor.copy"},
		{Label: i18n.Key("general.paste"), Command: "editor.paste"},
		{},
		{Label: i18n.Key("general.find"), Command: "search.find"},
		{Label: i18n.Key("general.replace"), Command: "search.replace"},
	},
	// Selection
	{
		{Label: i18n.Key("menu.selectAll"), Command: "editor.selectAll"},
		{},
		{Label: i18n.Key("menu.addCursorAbove"), Command: "multicursor.addCursorAbove"},
		{Label: i18n.Key("menu.addCursorBelow"), Command: "multicursor.addCursorBelow"},
		{Label: i18n.Key("menu.addNextOccurrence"), Command: "multicursor.selectNext"},
		{Label: i18n.Key("menu.selectAllOccurrences"), Command: "multicursor.selectAll"},
		{Label: i18n.Key("menu.undoLastCursor"), Command: "multicursor.undoCursor"},
	},
	// View
	{
		{Label: i18n.Key("menu.commandPalette"), Command: "command.palette"},
		{Label: i18n.Key("menu.quickOpen"), Command: "file.quickOpen"},
		{},
		{Label: i18n.Key("menu.explore"), Command: "sidebar.explorer"},
		{Label: i18n.Key("general.find"), Command: "sidebar.search"},
		{Label: i18n.Key("general.replace"), Command: "sidebar.searchReplace"},
		{Label: i18n.Key("menu.changes"), Command: "sidebar.changes"},
		{Label: i18n.Key("menu.outline"), Command: "sidebar.outline"},
		{},
		{Label: i18n.Key("menu.toggleSidebar"), Command: "sidebar.toggle"},
		{Label: i18n.Key("menu.toggleTerminal"), Command: "terminal.toggle"},
		{Label: i18n.Key("menu.newTerminal"), Command: "terminal.new"},
		{},
		{Label: i18n.Key("menu.theme"), Command: "theme.switch"},
		{Label: i18n.Key("menu.keybindings"), Command: "view.keybindings"},
		{Label: i18n.Key("general.settings"), Command: "settings.openUI"},
	},
	// Options (placeholder — replaced dynamically by openMenuBarDropdown)
	nil,
	// Help
	{
		{Label: i18n.Key("menu.about"), Command: "about"},
	},
}

var editorContextMenu = []menuEntry{
	{Label: i18n.Key("menu.goToDefinition"), Command: "editor.goToDefinition"},
	{Label: i18n.Key("menu.goToTypeDefinition"), Command: "editor.goToTypeDefinition"},
	{Label: i18n.Key("menu.goToImplementation"), Command: "editor.goToImplementation"},
	{Label: i18n.Key("menu.findAllReferences"), Command: "editor.findReferences"},
	{Label: i18n.Key("menu.renameSymbol"), Command: "editor.rename"},
	{},
	{Label: i18n.Key("menu.formatDocumentLSP"), Command: "editor.formatDocument"},
	{Label: i18n.Key("menu.formatDocumentExternal"), Command: "editor.formatExternal"},
	{Label: i18n.Key("menu.formatSelection"), Command: "editor.formatSelection"},
	{},
	{Label: i18n.Key("general.undo"), Command: "editor.undo"},
	{Label: i18n.Key("general.redo"), Command: "editor.redo"},
	{},
	{Label: i18n.Key("general.cut"), Command: "editor.cut"},
	{Label: i18n.Key("general.copy"), Command: "editor.copy"},
	{Label: i18n.Key("general.paste"), Command: "editor.paste"},
	{},
	{Label: i18n.Key("menu.selectAll"), Command: "editor.selectAll"},
	{},
	{Label: i18n.Key("general.find"), Command: "search.find"},
	{Label: i18n.Key("general.replace"), Command: "search.replace"},
	{Label: i18n.Key("menu.goToLine"), Command: "editor.goToLine"},
}

var diffContextMenu = []menuEntry{
	{Label: i18n.Key("general.copy"), Command: "editor.copy"},
	{},
	{Label: i18n.Key("general.find"), Command: "search.find"},
}

var commitDetailContextMenu = []menuEntry{
	{Label: i18n.Key("general.copy"), Command: "editor.copy"},
	{},
	{Label: i18n.Key("menu.expandAllFiles"), Command: "changes.expandAllCommitDetail"},
	{Label: i18n.Key("menu.collapseAllFiles"), Command: "changes.collapseAllCommitDetail"},
}

var textContentContextMenu = []menuEntry{
	{Label: i18n.Key("general.copy"), Command: "editor.copy"},
	{Label: i18n.Key("menu.selectAll"), Command: "editor.selectAll"},
}

var terminalContextMenu = []menuEntry{
	{Label: i18n.Key("general.copy"), Command: "editor.copy"},
	{Label: i18n.Key("general.paste"), Command: "editor.paste"},
}

var changesContextMenuStaged = []menuEntry{
	{Label: i18n.Key("menu.openChanges"), Command: "changes.openDiff"},
	{Label: i18n.Key("menu.openFullDiff"), Command: "changes.openExtendedDiff"},
	{Label: i18n.Key("general.openFile"), Command: "changes.openFile"},
	{},
	{Label: i18n.Key("menu.unstage"), Command: "changes.unstage"},
}

var changesContextMenuUnstaged = []menuEntry{
	{Label: i18n.Key("menu.openChanges"), Command: "changes.openDiff"},
	{Label: i18n.Key("menu.openFullDiff"), Command: "changes.openExtendedDiff"},
	{Label: i18n.Key("general.openFile"), Command: "changes.openFile"},
	{},
	{Label: i18n.Key("menu.stage"), Command: "changes.stage"},
	{Label: i18n.Key("menu.discardChanges"), Command: "changes.discard"},
}

func (a *App) BuildActiveDiffViewMenu() []ui.ContextMenuItem {
	var items []ui.ContextMenuItem
	modeSurface := a.EditorGroup.ActiveDiffModeSurface()
	contextSurface := a.EditorGroup.ActiveDiffContextSurface()
	if modeSurface != nil {
		items = append(items,
			ui.ContextMenuItem{Label: i18n.Text("menu.split"), Command: "diff.splitView", Checked: menuChecked(modeSurface.Mode() == ui.DiffModeSplit)},
			ui.ContextMenuItem{Label: i18n.Text("menu.unified"), Command: "diff.unifiedView", Checked: menuChecked(modeSurface.Mode() == ui.DiffModeUnified)},
		)
	}
	if contextSurface != nil {
		if len(items) > 0 {
			items = append(items, ui.MenuSep())
		}
		items = append(items,
			ui.ContextMenuItem{Label: i18n.Text("menu.changesOnly"), Command: "diff.changesOnlyView", Checked: menuChecked(contextSurface.ContextMode() == ui.DiffContextChangesOnly)},
			ui.ContextMenuItem{Label: i18n.Text("menu.fullFile"), Command: "diff.fullFileView", Checked: menuChecked(contextSurface.ContextMode() == ui.DiffContextFullFile)},
		)
	}
	if modeSurface != nil {
		if len(items) > 0 {
			items = append(items, ui.MenuSep())
		}
		items = append(items, ui.ContextMenuItem{Label: i18n.Text("menu.wrapLines"), Command: "diff.toggleWrap", Checked: menuChecked(modeSurface.WrapMode() == ui.DiffWrapOn)})
	}
	return items
}

func (a *App) withActiveDiffViewMenu(items []ui.ContextMenuItem) []ui.ContextMenuItem {
	controls := a.BuildActiveDiffViewMenu()
	if len(controls) == 0 {
		return items
	}
	combined := make([]ui.ContextMenuItem, 0, len(items)+1+len(controls))
	combined = append(combined, items...)
	combined = append(combined, ui.MenuSep())
	return append(combined, controls...)
}

func (a *App) withActiveDiffViewSubmenu(items []ui.ContextMenuItem) []ui.ContextMenuItem {
	controls := a.BuildActiveDiffViewMenu()
	if len(controls) == 0 {
		return items
	}
	combined := make([]ui.ContextMenuItem, 0, len(items)+2)
	combined = append(combined, items...)
	combined = append(combined, ui.MenuSep(), ui.ContextMenuItem{Label: i18n.Text("menu.diffView"), Submenu: controls})
	return combined
}

func resolveShortcuts(reg *command.Registry, items []ui.ContextMenuItem) []ui.ContextMenuItem {
	resolved := make([]ui.ContextMenuItem, len(items))
	for i, item := range items {
		resolved[i] = item
		if len(item.Submenu) > 0 {
			resolved[i].Submenu = resolveShortcuts(reg, item.Submenu)
		}
		if item.Command != "" {
			if cmd, ok := reg.Get(item.Command); ok && cmd.Shortcut != "" {
				resolved[i].Shortcut = cmd.Shortcut
			}
		}
	}
	return resolved
}

// Menus take focus while they are open, so closing one has to hand focus back —
// otherwise key input goes to a menu widget that is no longer on screen.
// captureMenuFocus is a no-op when a menu is already focused: navigating from
// one dropdown to the next re-enters here, and the original target must survive.
func (a *App) captureMenuFocus() {
	if _, inMenu := a.Root.Focused.(*ui.ContextMenuWidget); inMenu {
		return
	}
	a.menuReturnFocus = a.Root.Focused
}

// restoreMenuFocus runs before a selected command, so a command that focuses
// something itself (e.g. "Show Explorer") still wins.
func (a *App) restoreMenuFocus() {
	target := a.menuReturnFocus
	a.menuReturnFocus = nil
	if target == nil {
		a.FocusEditor()
		return
	}
	a.Root.SetFocus(target)
}

func openContextMenu(app *App, items []ui.ContextMenuItem, x, y int) {
	reg := app.Reg
	app.captureMenuFocus()
	menu := ui.NewContextMenuWidget(resolveShortcuts(reg, items), x, y)
	menu.Borders = app.Borders
	menu.Icons = app.Settings.Appearance.Icons
	menu.OnExec = func(cmd string) {
		app.Root.PopOverlay()
		app.restoreMenuFocus()
		reg.Execute(cmd)
		app.ExplorerContextNode = nil
	}
	menu.OnDismiss = func() {
		app.ExplorerContextNode = nil
		app.Root.PopOverlay()
		app.restoreMenuFocus()
	}
	app.Root.PushOverlay(ui.Overlay{Widget: menu, Modal: true})
	app.Root.SetFocus(menu)
}

const menuOptionsIndex = 4

func openMenuBarDropdown(app *App, index int) {
	if index < 0 || index >= len(menuBarMenus) {
		return
	}
	reg := app.Reg
	app.captureMenuFocus()
	app.MenuBar.Selected = index
	anchorX := app.MenuBar.ItemAnchorX(index)
	// The menu.* shortcuts stay live while the bar is hidden; with no bar to
	// hang under, the dropdown floats from the top edge instead.
	anchorY := 1
	if !app.MenuBar.Visible {
		anchorY = 0
	}
	items := menuItems(menuBarMenus[index])
	if index == menuOptionsIndex {
		items = app.BuildOptionsMenu()
	}
	menu := ui.NewContextMenuWidget(resolveShortcuts(reg, items), anchorX, anchorY)
	menu.Borders = app.Borders
	menu.Icons = app.Settings.Appearance.Icons
	menu.OnExec = func(cmd string) {
		app.Root.PopOverlay()
		app.MenuBar.Selected = -1
		app.restoreMenuFocus()
		reg.Execute(cmd)
	}
	menu.OnDismiss = func() {
		app.Root.PopOverlay()
		app.MenuBar.Selected = -1
		app.restoreMenuFocus()
	}
	menu.OnNavigate = func(dir int) {
		app.Root.PopOverlay()
		next := (index + dir + len(menuBarMenus)) % len(menuBarMenus)
		openMenuBarDropdown(app, next)
	}
	menu.OnMouseOutside = func(ev tcell.Event) {
		mev, ok := ev.(*tcell.EventMouse)
		if !ok {
			return
		}
		mx, my := mev.Position()
		r := app.MenuBar.GetRect()
		if my != r.Y {
			return
		}
		localX := mx - r.X
		for i, span := range app.MenuBar.ItemSpans() {
			if localX >= span.Start && localX < span.End && i != index {
				app.Root.PopOverlay()
				openMenuBarDropdown(app, i)
				return
			}
		}
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
					result := w.HandleEvent(ev)
					if result == ui.EventIgnored && app.Sidebar.ActivePanel == "changes" {
						app.ShowChangesContextMenu(mx, my)
					}
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

	if app.ContentSplit != nil && app.ContentSplit.ShowBottom {
		divY := app.ContentSplit.DividerScreenY()
		if divY >= 0 && my > divY {
			if app.BottomPanel != nil && app.BottomPanel.ActivePanel == "terminal" {
				if app.TerminalPanel != nil {
					app.Root.SetFocus(app.TerminalPanel)
				}
				openContextMenu(app, menuItems(terminalContextMenu), mx, my)
			}
			return
		}
	}

	if content := app.EditorGroup.ActiveTextSelection(); content != nil {
		if w, ok := content.(ui.Widget); ok && w.HandleEvent(tcell.NewEventMouse(mx, my, tcell.Button2, 0)) != ui.EventIgnored {
			return
		}
		openContextMenu(app, menuItems(textContentContextMenu), mx, my)
	} else if app.EditorGroup.ActiveCommitDetailWidget() != nil {
		openContextMenu(app, app.withActiveDiffViewMenu(menuItems(commitDetailContextMenu)), mx, my)
	} else if app.EditorGroup.ActiveDiffModeSurface() != nil || app.EditorGroup.ActiveDiffContextSurface() != nil {
		openContextMenu(app, app.withActiveDiffViewMenu(menuItems(diffContextMenu)), mx, my)
	} else {
		openEditorContextMenu(app, mx, my)
	}
}

// openEditorContextMenu shows the editor right-click menu, appending any items
// contributed by enabled plugins. Plugin item selections are dispatched through
// their own OnSelect callbacks (not the global command registry) using
// synthetic command ids.
func openEditorContextMenu(app *App, mx, my int) {
	line, col, word, ok := app.EditorGroup.PositionAt(mx, my)

	items := menuItems(editorContextMenu)

	callbacks := map[string]func(){}
	if ok && app.PluginManager != nil {
		for _, p := range app.PluginManager.Plugins() {
			if !p.Enabled {
				continue
			}
			entries := p.EditorContextMenuItems(line+1, col+1, word)
			if len(entries) == 0 {
				continue
			}
			items = append(items, ui.MenuSep())
			for _, e := range entries {
				if e.Separator {
					items = append(items, ui.MenuSep())
					continue
				}
				id := fmt.Sprintf("__plugin_ctx_%d", len(callbacks))
				callbacks[id] = e.OnSelect
				items = append(items, ui.ContextMenuItem{Label: e.Label, Command: id})
			}
		}
	}

	reg := app.Reg
	app.captureMenuFocus()
	menu := ui.NewContextMenuWidget(resolveShortcuts(reg, items), mx, my)
	menu.Borders = app.Borders
	menu.Icons = app.Settings.Appearance.Icons
	menu.OnExec = func(cmd string) {
		app.Root.PopOverlay()
		app.restoreMenuFocus()
		if cb, ok := callbacks[cmd]; ok {
			if cb != nil {
				cb()
			}
			return
		}
		reg.Execute(cmd)
	}
	menu.OnDismiss = func() {
		app.Root.PopOverlay()
		app.restoreMenuFocus()
	}
	app.Root.PushOverlay(ui.Overlay{Widget: menu, Modal: true})
	app.Root.SetFocus(menu)
}
