package app

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v3"
)

var menuBarLabels = []string{
	"menu.file", "menu.edit", "menu.selection", "menu.view", "menu.options", "menu.help",
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
		{Label: "Open PR Diff", Command: "pr.openDiff"},
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
		{Label: "Outline", Command: "sidebar.outline"},
		ui.MenuSep(),
		{Label: "Toggle Sidebar", Command: "sidebar.toggle"},
		{Label: "Toggle Terminal", Command: "terminal.toggle"},
		{Label: "New Terminal", Command: "terminal.new"},
		ui.MenuSep(),
		{Label: "Settings", Command: "settings.openUI"},
		{Label: "Keyboard Shortcuts", Command: "view.keybindings"},
	},
	// Options (placeholder — replaced dynamically by openMenuBarDropdown)
	nil,
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
	{Label: "Format Document (LSP)", Command: "editor.formatDocument"},
	{Label: "Format Document (External)", Command: "editor.formatExternal"},
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
	menu.OnExec = func(cmd string) {
		app.Root.PopOverlay()
		app.restoreMenuFocus()
		reg.Execute(cmd)
	}
	menu.OnDismiss = func() {
		app.Root.PopOverlay()
		app.restoreMenuFocus()
	}
	app.Root.PushOverlay(ui.Overlay{Widget: menu, Modal: true})
	app.Root.SetFocus(menu)
}

const (
	menuViewIndex    = 3
	menuOptionsIndex = 4
)

// BuildViewMenu adds per-surface presentation overrides only while a diff is
// active. Options holds the persisted defaults used by newly opened diffs;
// these View controls never rewrite those settings.
func (a *App) BuildViewMenu() []ui.ContextMenuItem {
	items := append([]ui.ContextMenuItem(nil), menuBarMenus[menuViewIndex]...)
	surface := a.EditorGroup.ActiveDiffModeSurface()
	if surface == nil {
		return items
	}
	splitChecked := ui.MenuUnchecked
	unifiedChecked := ui.MenuUnchecked
	if surface.Mode() == ui.DiffModeUnified {
		unifiedChecked = ui.MenuChecked
	} else {
		splitChecked = ui.MenuChecked
	}
	wrapChecked := ui.MenuUnchecked
	if surface.WrapMode() == ui.DiffWrapOn {
		wrapChecked = ui.MenuChecked
	}
	return append(items,
		ui.MenuSep(),
		ui.ContextMenuItem{Label: "Diff: Split", Command: "diff.splitView", Checked: splitChecked},
		ui.ContextMenuItem{Label: "Diff: Unified", Command: "diff.unifiedView", Checked: unifiedChecked},
		ui.ContextMenuItem{Label: "Diff: Wrap Lines", Command: "diff.toggleWrap", Checked: wrapChecked},
	)
}

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
	items := menuBarMenus[index]
	if index == menuViewIndex {
		items = app.BuildViewMenu()
	} else if index == menuOptionsIndex {
		items = app.BuildOptionsMenu()
	}
	menu := ui.NewContextMenuWidget(resolveShortcuts(reg, items), anchorX, anchorY)
	menu.Borders = app.Borders
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
		openEditorContextMenu(app, mx, my)
	}
}

// openEditorContextMenu shows the editor right-click menu, appending any items
// contributed by enabled plugins. Plugin item selections are dispatched through
// their own OnSelect callbacks (not the global command registry) using
// synthetic command ids.
func openEditorContextMenu(app *App, mx, my int) {
	line, col, word, ok := app.EditorGroup.PositionAt(mx, my)

	items := make([]ui.ContextMenuItem, len(editorContextMenu))
	copy(items, editorContextMenu)

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
