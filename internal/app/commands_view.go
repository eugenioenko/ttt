package app

import (
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func (a *App) ToggleTerminal() {
	if !a.ContentSplit.ShowBottom {
		a.ensureUsablePanelSize()
		a.showTerminalPanel()
		resizeTerminals(a)
	} else {
		a.HideBottomPanel()
	}
}

func (a *App) ToggleTerminalFullscreen() {
	r := a.ContentSplit.GetRect()
	fullH := r.H - 1
	if a.ContentSplit.ShowBottom && a.ContentSplit.BottomH >= fullH {
		a.HideBottomPanel()
	} else {
		a.EditorGroup.InvalidatePointerInteraction()
		a.ContentSplit.BottomH = fullH
		a.showTerminalPanel()
		resizeTerminals(a)
	}
}

func (a *App) focusedRegion() string {
	f := a.Root.Focused
	// Plugin row menu restores focus to InstalledTree, not the panel adapter.
	inPlugins := a.PluginsPanel != nil && f == a.PluginsPanel.InstalledTree
	if f == a.Sidebar || f == a.Sidebar.ActiveWidget() || inPlugins {
		return "sidebar"
	}
	if f == a.BottomPanel || f == a.BottomPanel.ActiveWidget() {
		return "bottom"
	}
	return "editor"
}

func (a *App) focusNextGroup() { a.cycleFocusGroup(1) }
func (a *App) focusPrevGroup() { a.cycleFocusGroup(-1) }

func (a *App) cycleFocusGroup(direction int) {
	if a.Root.HasModalOverlay() {
		return
	}
	var groups []ui.Widget
	for _, group := range a.editorGroups() {
		groups = append(groups, group)
	}
	if a.Sidebar.Visible {
		groups = append(groups, a.Sidebar.ActiveWidget())
	}
	if a.ContentSplit.ShowBottom {
		groups = append(groups, a.BottomPanel.ActiveWidget())
	}
	for i, group := range groups {
		if group == a.Root.Focused || (a.focusedRegion() == "sidebar" && group == a.Sidebar.ActiveWidget()) || (a.focusedRegion() == "bottom" && group == a.BottomPanel.ActiveWidget()) {
			a.Root.SetFocus(groups[(i+direction+len(groups))%len(groups)])
			return
		}
	}
	a.FocusEditor()
}

func (a *App) ScrollUp()       { a.scrollEditor(-1) }
func (a *App) ScrollDown()     { a.scrollEditor(1) }
func (a *App) ScrollLineUp()   { a.EditorGroup.ScrollViewportLines(-1) }
func (a *App) ScrollLineDown() { a.EditorGroup.ScrollViewportLines(1) }

func (a *App) scrollEditor(direction int) {
	if lines := a.Settings.Editor.ScrollLines; lines > 0 {
		a.EditorGroup.ScrollLines(direction * lines)
		return
	}
	a.EditorGroup.ScrollHalfPage(direction)
}

func (a *App) contextNextTab() {
	switch a.focusedRegion() {
	case "sidebar":
		a.Sidebar.NextPanel()
		if w := a.Sidebar.ActiveWidget(); w != nil {
			a.Root.SetFocus(w)
		}
	case "bottom":
		a.BottomPanel.NextPanel()
		if w := a.BottomPanel.ActiveWidget(); w != nil {
			a.Root.SetFocus(w)
		}
	default:
		a.EditorGroup.NextTab()
	}
}

func (a *App) contextPrevTab() {
	switch a.focusedRegion() {
	case "sidebar":
		a.Sidebar.PrevPanel()
		if w := a.Sidebar.ActiveWidget(); w != nil {
			a.Root.SetFocus(w)
		}
	case "bottom":
		a.BottomPanel.PrevPanel()
		if w := a.BottomPanel.ActiveWidget(); w != nil {
			a.Root.SetFocus(w)
		}
	default:
		a.EditorGroup.PrevTab()
	}
}

func (a *App) focusTerminal() {
	if !a.ContentSplit.ShowBottom {
		a.ensureUsablePanelSize()
		a.showTerminalPanel()
	}
	a.BottomPanel.SetActivePanel("terminal")
	if w := a.BottomPanel.ActiveWidget(); w != nil {
		a.Root.SetFocus(w)
	}
}

func (a *App) FocusPanel() {
	if !a.ContentSplit.ShowBottom {
		a.ContentSplit.ShowBottom = true
	}
	if w := a.BottomPanel.ActiveWidget(); w != nil {
		a.Root.SetFocus(w)
	}
}

func (a *App) ShowKeybindings() {
	a.showKeybindings("")
}

func (a *App) showKeybindings(commandID string) {
	defaults := make(map[string]string)
	for _, kb := range config.DefaultKeybindings() {
		defaults[kb.Command] = FormatKeyBinding(kb.Key, a.Root.ControlModifiers)
	}

	w := ui.NewKeybindingsWidget(a.Reg.List())
	if cmd, ok := a.Reg.Get(commandID); ok {
		w.SetFilter(cmd.Title)
	}
	w.ControlModifiers = &a.Root.ControlModifiers
	w.Borders = a.Borders
	w.GetShortcut = func(cmdID string) string {
		for _, kb := range a.Keybindings {
			if kb.Command == cmdID {
				return FormatKeyBinding(kb.Key, a.Root.ControlModifiers)
			}
		}
		return ""
	}
	w.GetDefault = func(cmdID string) string {
		return defaults[cmdID]
	}
	w.OnEdit = func(cmdID string, newKey string) {
		filtered := make([]config.KeyBinding, 0, len(a.Keybindings))
		for _, kb := range a.Keybindings {
			if kb.Key != newKey && kb.Command != cmdID {
				filtered = append(filtered, kb)
			}
		}
		filtered = append(filtered, config.KeyBinding{Key: newKey, Command: cmdID})
		a.Keybindings = filtered
		config.SaveKeybindings(a.Keybindings)
		a.RebindKeys()
	}
	w.OnReset = func(cmdID string) {
		var defaultKey string
		for _, kb := range config.DefaultKeybindings() {
			if kb.Command == cmdID {
				defaultKey = kb.Key
				break
			}
		}
		filtered := make([]config.KeyBinding, 0, len(a.Keybindings))
		for _, kb := range a.Keybindings {
			if kb.Command == cmdID {
				continue
			}
			if defaultKey != "" && kb.Key == defaultKey {
				continue
			}
			filtered = append(filtered, kb)
		}
		if defaultKey != "" {
			filtered = append(filtered, config.KeyBinding{Key: defaultKey, Command: cmdID})
		}
		a.Keybindings = filtered
		config.SaveKeybindings(a.Keybindings)
		a.RebindKeys()
	}
	w.OnClear = func(cmdID string) {
		filtered := make([]config.KeyBinding, 0, len(a.Keybindings))
		for _, kb := range a.Keybindings {
			if kb.Command != cmdID {
				filtered = append(filtered, kb)
			}
		}
		a.Keybindings = filtered
		config.SaveKeybindings(a.Keybindings)
		a.RebindKeys()
	}
	w.OnHelp = func() {
		content := widgets.NewKeyValueListWidget([]widgets.KeyValueEntry{
			{Key: "Mod", Value: i18n.Text("keybindings.modifierEither")},
			{Key: "Super", Value: i18n.Text("keybindings.modifierSuper")},
			{Key: "Enter", Value: i18n.Text("keybindings.editSelectedShortcut")},
			{Key: "Backspace", Value: i18n.Text("keybindings.resetToDefault")},
			{Key: "Delete", Value: i18n.Text("keybindings.clearShortcut")},
			{Key: "Up / k", Value: i18n.Text("keybindings.moveUp")},
			{Key: "Down / j", Value: i18n.Text("keybindings.moveDown")},
			{Key: "Esc", Value: i18n.Text("general.close")},
		})
		dialog := widgets.NewDialogWidget(50)
		dialog.Title = i18n.Text("keybindings.keyboardShortcutsHelp")
		dialog.Borders = *a.Borders
		dialog.SetContent(content)
		dialog.Buttons = []widgets.DialogButton{
			{Label: i18n.Text("general.closeButton"), Handler: func() {
				a.Root.PopOverlay()
				a.Root.SetFocus(w)
			}},
		}
		dialog.OnDismiss = func() {
			a.Root.PopOverlay()
			a.Root.SetFocus(w)
		}
		dialog.Build()
		adapter := ui.NewWidgetAdapter(dialog)
		a.Root.PushOverlay(ui.Overlay{Widget: adapter, Modal: true})
		a.Root.SetFocus(adapter)
	}
	w.OnDismiss = func() {
		a.DismissDialog()
	}
	a.ShowDialog(w)
}

func (a *App) OpenGlobalSearch() {
	a.prefillSearchInput(a.Search.Input)
	a.ShowPanel("search", a.Search)
}

func (a *App) ToggleSearchReplace() {
	if a.Sidebar.Visible && a.Sidebar.ActivePanel == "search" {
		a.Search.ToggleReplaceMode()
	} else {
		a.Search.SetReplaceMode(true)
	}
	a.OpenGlobalSearch()
}

func registerViewCommands(app *App) {
	app.Reg.Register(command.Command{ID: "editor.split", Title: i18n.Text("command.splitEditor"), Handler: app.SplitEditor})
	app.Reg.Register(command.Command{ID: "editor.splitDown", Title: i18n.Text("command.splitEditorDown"), Handler: app.SplitEditorDown})
	app.Reg.Register(command.Command{ID: "editor.closeSplit", Title: i18n.Text("command.closeSplit"), Handler: app.CloseEditorSplit})
	app.Reg.Register(command.Command{ID: "editor.equalizeGroups", Title: i18n.Text("command.equalizeGroups"), Handler: app.EqualizeEditorGroups})
	app.Reg.Register(command.Command{ID: "focus.editorLeft", Title: i18n.Text("command.focusEditorLeft"), Handler: app.FocusEditorLeft})
	app.Reg.Register(command.Command{ID: "focus.editorRight", Title: i18n.Text("command.focusEditorRight"), Handler: app.FocusEditorRight})
	app.Reg.Register(command.Command{ID: "focus.editorUp", Title: i18n.Text("command.focusEditorUp"), Handler: app.FocusEditorUp})
	app.Reg.Register(command.Command{ID: "focus.editorDown", Title: i18n.Text("command.focusEditorDown"), Handler: app.FocusEditorDown})
	reg := app.Reg

	reg.Register(command.Command{
		ID: "sidebar.toggle", Title: i18n.Text("command.viewToggleSidebar"),
		Keywords: []string{"view", "panel", "show", "hide"},
		Handler:  app.ToggleSidebar,
	})

	reg.Register(command.Command{
		ID: menuBarToggleCommand, Title: i18n.Text("command.viewToggleMenuBar"),
		Keywords: []string{"view", "menu", "toolbar", "show", "hide"},
		Handler:  app.ToggleMenuBar,
	})

	reg.Register(command.Command{
		ID: "sidebar.explorer", Title: i18n.Text("command.showExplorer"),
		Keywords: []string{"view", "file", "tree", "browser"},
		Handler: func() {
			app.ShowPanel("explorer", app.Explorer.Adapter)
		},
	})

	reg.Register(command.Command{
		ID: "sidebar.search", Title: i18n.Text("command.showSearch"),
		Keywords: []string{"view", "search", "find", "grep"},
		Handler:  app.OpenGlobalSearch,
	})

	reg.Register(command.Command{
		ID: "sidebar.searchReplace", Title: i18n.Text("command.searchAndReplaceInFiles"),
		Keywords: []string{"view", "search", "find", "replace", "substitute"},
		Handler:  app.ToggleSearchReplace,
	})

	reg.Register(command.Command{
		ID: "sidebar.changes", Title: i18n.Text("command.showChanges"),
		Keywords: []string{"view", "git", "diff", "source control"},
		Handler: func() {
			app.ShowPanel("changes", app.Changes.Adapter)
		},
	})

	reg.Register(command.Command{
		ID: "sidebar.outline", Title: i18n.Text("command.showOutline"),
		Keywords: []string{"view", "symbols", "structure", "functions", "browser", "headings"},
		Handler: func() {
			app.ShowPanel("outline", app.Symbols.Adapter)
		},
	})

	reg.Register(command.Command{
		ID: "sidebar.apps", Title: i18n.Text("command.showApps"),
		Keywords: []string{"view", "apps", "applications", "launch", "programs"},
		Handler:  app.ShowApps,
	})
	reg.Register(command.Command{ID: "apps.add", Title: i18n.Text("apps.add"), Handler: app.AddApp})
	reg.Register(command.Command{ID: "apps.addOpenWith", Hidden: true, Handler: app.AddOpenWithApp})
	reg.Register(command.Command{ID: "apps.openCurrentFileWith", Title: i18n.Text("command.openCurrentFileWith"), Keywords: []string{"file", "open", "apps", "preview", "external"}, Handler: app.OpenCurrentFileWith})

	reg.Register(command.Command{
		ID: "sidebar.wider", Title: i18n.Text("command.viewIncreaseSidebarWidth"),
		Keywords: []string{"view", "resize"},
		Handler: func() {
			app.SetSidebarWidth(app.SplitPanel.DividerPos + 1)
		},
	})

	reg.Register(command.Command{
		ID: "sidebar.narrower", Title: i18n.Text("command.viewDecreaseSidebarWidth"),
		Keywords: []string{"view", "resize"},
		Handler: func() {
			if app.Sidebar.Visible {
				app.SetSidebarWidth(app.SplitPanel.DividerPos - 1)
			}
		},
	})

	reg.Register(command.Command{
		ID: "sidebar.focus", Title: i18n.Text("command.viewFocusSidebar"),
		Keywords: []string{"view"},
		Handler:  app.FocusSidebar,
	})

	reg.Register(command.Command{
		ID: "view.scrollUp", Title: i18n.Text("command.scrollUp"),
		Keywords: []string{"view", "scroll", "page", "half", "up"},
		Handler:  app.ScrollUp,
	})

	reg.Register(command.Command{
		ID: "view.scrollDown", Title: i18n.Text("command.scrollDown"),
		Keywords: []string{"view", "scroll", "page", "half", "down"},
		Handler:  app.ScrollDown,
	})

	reg.Register(command.Command{
		ID: "view.scrollLineUp", Title: i18n.Text("command.scrollLineUp"),
		Keywords: []string{"view", "scroll", "line", "viewport", "up"},
		Handler:  app.ScrollLineUp,
	})

	reg.Register(command.Command{
		ID: "view.scrollLineDown", Title: i18n.Text("command.scrollLineDown"),
		Keywords: []string{"view", "scroll", "line", "viewport", "down"},
		Handler:  app.ScrollLineDown,
	})

	reg.Register(command.Command{
		ID: "sidebar.movePanelLeft", Title: i18n.Text("command.viewMoveSidebarPanelLeft"),
		Keywords: []string{"view", "sidebar", "panel", "tab", "reorder", "left"},
		Handler:  func() { app.MoveActiveSidebarPanel(-1) },
	})

	reg.Register(command.Command{
		ID: "sidebar.movePanelRight", Title: i18n.Text("command.viewMoveSidebarPanelRight"),
		Keywords: []string{"view", "sidebar", "panel", "tab", "reorder", "right"},
		Handler:  func() { app.MoveActiveSidebarPanel(1) },
	})

	reg.Register(command.Command{
		ID: "panel.toggle", Title: i18n.Text("command.viewTogglePanel"),
		Keywords: []string{"view", "bottom", "show", "hide"},
		Handler:  app.ToggleBottomPanel,
	})

	reg.Register(command.Command{
		ID: "panel.focus", Title: i18n.Text("command.viewFocusPanel"),
		Keywords: []string{"view", "bottom"},
		Handler:  app.FocusPanel,
	})

	reg.Register(command.Command{
		ID: "panel.show", Title: i18n.Text("command.viewShowPanelTab"),
		Keywords: []string{"view", "bottom", "tab", "switch"},
		Handler: func() {
			panels := app.BottomPanel.PanelEntries()
			if len(panels) == 0 {
				return
			}
			var items []widgets.SelectItem
			for _, p := range panels {
				items = append(items, widgets.SelectItem{ID: p.ID, Label: p.Title})
			}
			app.ShowSelectDialog(i18n.Text("keybindings.showPanel"), items, func(id string) {
				app.BottomPanel.SetActivePanel(id)
				app.FocusPanel()
			}, nil)
		},
	})

	reg.Register(command.Command{
		ID: "panel.taller", Title: i18n.Text("command.viewIncreasePanelSize"),
		Keywords: []string{"view", "resize", "bottom", "right", "height", "width"},
		Handler: func() {
			if !app.ContentSplit.ShowBottom {
				app.ShowBottomPanel()
			}
			app.ContentSplit.ResizePanel(1)
			app.persistPanelSize()
			resizeTerminals(app)
		},
	})

	reg.Register(command.Command{
		ID: "panel.shorter", Title: i18n.Text("command.viewDecreasePanelSize"),
		Keywords: []string{"view", "resize", "bottom", "right", "height", "width"},
		Handler: func() {
			if !app.ContentSplit.ShowBottom {
				return
			}
			app.ContentSplit.ResizePanel(-1)
			app.persistPanelSize()
			resizeTerminals(app)
		},
	})

	reg.Register(command.Command{
		ID: "panel.dockRight", Title: i18n.Text("command.viewDockPanelRight"),
		Keywords: []string{"view", "panel", "terminal", "dock", "position", "side"},
		Handler:  func() { app.SetPanelPosition(ui.SplitRight) },
	})

	reg.Register(command.Command{
		ID: "panel.dockBottom", Title: i18n.Text("command.viewDockPanelBottom"),
		Keywords: []string{"view", "panel", "terminal", "dock", "position", "down"},
		Handler:  func() { app.SetPanelPosition(ui.SplitBottom) },
	})

	reg.Register(command.Command{
		ID: "focus.nextGroup", Title: i18n.Text("command.viewFocusNextGroup"),
		Keywords: []string{"focus", "panel", "sidebar", "editor"},
		Handler:  app.focusNextGroup,
	})

	reg.Register(command.Command{
		ID: "focus.prevGroup", Title: i18n.Text("command.viewFocusPreviousGroup"),
		Keywords: []string{"focus", "panel", "sidebar", "editor"},
		Handler:  app.focusPrevGroup,
	})

	reg.Register(command.Command{
		ID: "focus.terminal", Title: i18n.Text("command.viewFocusTerminal"),
		Keywords: []string{"focus", "terminal", "shell"},
		Handler:  app.focusTerminal,
	})

	reg.Register(command.Command{
		ID: "terminal.new", Title: i18n.Text("command.terminalNewTerminal"),
		Keywords: []string{"terminal", "shell", "console", "bash"},
		Handler:  app.SpawnTerminal,
	})
	reg.Register(command.Command{
		ID: "terminal.next", Title: i18n.Text("command.terminalNext"),
		Keywords: []string{"terminal", "shell", "tab", "next"},
		Handler:  app.NextTerminal,
	})
	reg.Register(command.Command{
		ID: "terminal.prev", Title: i18n.Text("command.terminalPrev"),
		Keywords: []string{"terminal", "shell", "tab", "previous"},
		Handler:  app.PrevTerminal,
	})
	reg.Register(command.Command{
		ID: "terminal.sendNextKey", Title: i18n.Text("command.terminalSendNextKey"),
		Keywords: []string{"terminal", "key", "shortcut", "forward"},
		Handler:  app.SendNextTerminalKey,
	})
	reg.Register(command.Command{
		ID: "terminal.toggleShortcutForwarding", Title: i18n.Text("command.terminalToggleShortcutForwarding"),
		Keywords: []string{"terminal", "key", "shortcut", "forward", "toggle"},
		Handler:  app.ToggleTerminalShortcutForwarding,
	})

	reg.Register(command.Command{
		ID: "terminal.newInEditor", Title: i18n.Text("command.terminalNewEditorTerminal"),
		Keywords: []string{"terminal", "shell", "console", "tab", "editor"},
		Handler:  app.SpawnEditorTerminal,
	})

	reg.Register(command.Command{
		ID: "terminal.toggle", Title: i18n.Text("command.terminalToggleTerminal"),
		Keywords: []string{"terminal", "shell", "console", "bash"},
		Handler:  app.ToggleTerminal,
	})

	reg.Register(command.Command{
		ID: "terminal.fullscreen", Title: i18n.Text("command.terminalToggleFullscreen"),
		Keywords: []string{"terminal", "shell", "maximize"},
		Handler:  app.ToggleTerminalFullscreen,
	})

	reg.Register(command.Command{
		ID: "terminal.close", Title: i18n.Text("command.terminalClose"),
		Keywords: []string{"terminal", "shell", "kill"},
		Handler:  app.CloseActiveTerminal,
	})

	reg.Register(command.Command{
		ID: "terminal.closeAll", Title: i18n.Text("command.terminalCloseAll"),
		Keywords: []string{"terminal", "shell"},
		Handler:  app.CloseAllTerminals,
	})

	reg.Register(command.Command{
		ID: "view.keybindings", Title: i18n.Text("command.keyboardShortcuts"),
		Keywords: []string{"view", "keybindings", "shortcuts", "remap", "rebind", "hotkeys"},
		Handler:  app.ShowKeybindings,
	})

	reg.Register(command.Command{
		ID: "about", Title: i18n.Text("command.aboutTTTEditor"),
		Keywords: []string{"help", "version", "info"},
		Handler:  app.ShowAbout,
	})
	reg.Register(command.Command{
		ID: "update.check", Title: i18n.Text("update.checkForUpdates"),
		Keywords: []string{"help", "version", "update", "upgrade"},
		Handler:  app.CheckForUpdates,
	})

}
