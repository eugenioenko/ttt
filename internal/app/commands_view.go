package app

import (
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func (a *App) ToggleTerminal() {
	if !a.ContentSplit.ShowBottom {
		r := a.ContentSplit.GetRect()
		maxH := r.H - 4
		if a.ContentSplit.BottomH <= 1 || a.ContentSplit.BottomH > maxH {
			a.ContentSplit.BottomH = min(r.H/2, maxH)
		}
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
	if f == a.Sidebar || f == a.Sidebar.ActiveWidget() {
		return "sidebar"
	}
	if f == a.BottomPanel || f == a.BottomPanel.ActiveWidget() {
		return "bottom"
	}
	return "editor"
}

func (a *App) focusNextGroup() {
	if a.Root.HasModalOverlay() {
		return
	}
	regions := []string{"editor"}
	if a.Sidebar.Visible {
		regions = append(regions, "sidebar")
	}
	if a.ContentSplit.ShowBottom {
		regions = append(regions, "bottom")
	}
	current := a.focusedRegion()
	for i, r := range regions {
		if r == current {
			next := regions[(i+1)%len(regions)]
			a.focusRegion(next)
			return
		}
	}
	a.FocusEditor()
}

func (a *App) focusPrevGroup() {
	if a.Root.HasModalOverlay() {
		return
	}
	regions := []string{"editor"}
	if a.Sidebar.Visible {
		regions = append(regions, "sidebar")
	}
	if a.ContentSplit.ShowBottom {
		regions = append(regions, "bottom")
	}
	current := a.focusedRegion()
	for i, r := range regions {
		if r == current {
			prev := i - 1
			if prev < 0 {
				prev = len(regions) - 1
			}
			a.focusRegion(regions[prev])
			return
		}
	}
	a.FocusEditor()
}

func (a *App) focusRegion(region string) {
	switch region {
	case "editor":
		a.FocusEditor()
	case "sidebar":
		a.FocusSidebar()
	case "bottom":
		a.FocusPanel()
	}
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
		r := a.ContentSplit.GetRect()
		maxH := r.H - 4
		if a.ContentSplit.BottomH <= 1 || a.ContentSplit.BottomH > maxH {
			a.ContentSplit.BottomH = min(r.H/2, maxH)
		}
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
	defaults := make(map[string]string)
	for _, kb := range config.DefaultKeybindings() {
		defaults[kb.Command] = FormatKeyBinding(kb.Key)
	}

	w := ui.NewKeybindingsWidget(a.Reg.List())
	w.Borders = a.Borders
	w.GetShortcut = func(cmdID string) string {
		for _, kb := range a.Keybindings {
			if kb.Command == cmdID {
				return FormatKeyBinding(kb.Key)
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
			{Key: "Enter", Value: "Edit selected shortcut"},
			{Key: "Backspace", Value: "Reset to default"},
			{Key: "Delete", Value: "Clear shortcut"},
			{Key: "Up / k", Value: "Move up"},
			{Key: "Down / j", Value: "Move down"},
			{Key: "Esc", Value: "Close"},
		})
		dialog := widgets.NewDialogWidget(50)
		dialog.Title = "Keyboard Shortcuts Help"
		dialog.Borders = *a.Borders
		dialog.SetContent(content)
		dialog.Buttons = []widgets.DialogButton{
			{Label: "&Close", Handler: func() {
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

func (a *App) ToggleSearchReplace() {
	if a.Sidebar.Visible && a.Sidebar.ActivePanel == "search" {
		a.Search.ToggleReplaceMode()
	} else {
		a.Search.SetReplaceMode(true)
		a.ShowPanel("search", a.Search)
	}
	a.Root.SetFocus(a.Search)
}

func registerViewCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "sidebar.toggle", Title: "View: Toggle Sidebar",
		Keywords: []string{"view", "panel", "show", "hide"},
		Handler:  app.ToggleSidebar,
	})

	reg.Register(command.Command{
		ID: menuBarToggleCommand, Title: "View: Toggle Menu Bar",
		Keywords: []string{"view", "menu", "toolbar", "show", "hide"},
		Handler:  app.ToggleMenuBar,
	})

	reg.Register(command.Command{
		ID: "sidebar.explorer", Title: "Show Explorer",
		Keywords: []string{"view", "file", "tree", "browser"},
		Handler: func() {
			app.ShowPanel("explorer", app.Explorer.Adapter)
		},
	})

	reg.Register(command.Command{
		ID: "sidebar.search", Title: "Show Search",
		Keywords: []string{"view", "search", "find", "grep"},
		Handler: func() {
			app.ShowPanel("search", app.Search)
		},
	})

	reg.Register(command.Command{
		ID: "sidebar.searchReplace", Title: "Search and Replace in Files",
		Keywords: []string{"view", "search", "find", "replace", "substitute"},
		Handler:  app.ToggleSearchReplace,
	})

	reg.Register(command.Command{
		ID: "sidebar.changes", Title: "Show Changes",
		Keywords: []string{"view", "git", "diff", "source control"},
		Handler: func() {
			app.ShowPanel("changes", app.Changes.Adapter)
		},
	})

	reg.Register(command.Command{
		ID: "sidebar.outline", Title: "Show Outline",
		Keywords: []string{"view", "symbols", "structure", "functions", "browser", "headings"},
		Handler: func() {
			app.ShowPanel("outline", app.Symbols.Adapter)
		},
	})

	reg.Register(command.Command{
		ID: "sidebar.wider", Title: "View: Increase Sidebar Width",
		Keywords: []string{"view", "resize"},
		Handler: func() {
			app.SetSidebarWidth(app.SplitPanel.DividerPos + 1)
		},
	})

	reg.Register(command.Command{
		ID: "sidebar.narrower", Title: "View: Decrease Sidebar Width",
		Keywords: []string{"view", "resize"},
		Handler: func() {
			if app.Sidebar.Visible {
				app.SetSidebarWidth(app.SplitPanel.DividerPos - 1)
			}
		},
	})

	reg.Register(command.Command{
		ID: "sidebar.focus", Title: "View: Focus Sidebar",
		Keywords: []string{"view"},
		Handler:  app.FocusSidebar,
	})

	reg.Register(command.Command{
		ID: "sidebar.movePanelLeft", Title: "View: Move Sidebar Panel Left",
		Keywords: []string{"view", "sidebar", "panel", "tab", "reorder", "left"},
		Handler:  func() { app.MoveActiveSidebarPanel(-1) },
	})

	reg.Register(command.Command{
		ID: "sidebar.movePanelRight", Title: "View: Move Sidebar Panel Right",
		Keywords: []string{"view", "sidebar", "panel", "tab", "reorder", "right"},
		Handler:  func() { app.MoveActiveSidebarPanel(1) },
	})

	reg.Register(command.Command{
		ID: "panel.toggle", Title: "View: Toggle Panel",
		Keywords: []string{"view", "bottom", "show", "hide"},
		Handler:  app.ToggleBottomPanel,
	})

	reg.Register(command.Command{
		ID: "panel.focus", Title: "View: Focus Panel",
		Keywords: []string{"view", "bottom"},
		Handler:  app.FocusPanel,
	})

	reg.Register(command.Command{
		ID: "panel.show", Title: "View: Show Panel Tab",
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
			app.ShowSelectDialog("Show Panel", items, func(id string) {
				app.BottomPanel.SetActivePanel(id)
				app.FocusPanel()
			}, nil)
		},
	})

	reg.Register(command.Command{
		ID: "panel.taller", Title: "View: Increase Panel Height",
		Keywords: []string{"view", "resize", "bottom"},
		Handler: func() {
			if !app.ContentSplit.ShowBottom {
				app.ShowBottomPanel()
			}
			app.ContentSplit.BottomH++
		},
	})

	reg.Register(command.Command{
		ID: "panel.shorter", Title: "View: Decrease Panel Height",
		Keywords: []string{"view", "resize", "bottom"},
		Handler: func() {
			if app.ContentSplit.ShowBottom && app.ContentSplit.BottomH > 1 {
				app.ContentSplit.BottomH--
			}
		},
	})

	reg.Register(command.Command{
		ID: "focus.nextGroup", Title: "View: Focus Next Group",
		Keywords: []string{"focus", "panel", "sidebar", "editor"},
		Handler:  app.focusNextGroup,
	})

	reg.Register(command.Command{
		ID: "focus.prevGroup", Title: "View: Focus Previous Group",
		Keywords: []string{"focus", "panel", "sidebar", "editor"},
		Handler:  app.focusPrevGroup,
	})

	reg.Register(command.Command{
		ID: "focus.terminal", Title: "View: Focus Terminal",
		Keywords: []string{"focus", "terminal", "shell"},
		Handler:  app.focusTerminal,
	})

	reg.Register(command.Command{
		ID: "terminal.new", Title: "Terminal: New Terminal",
		Keywords: []string{"terminal", "shell", "console", "bash"},
		Handler:  app.SpawnTerminal,
	})

	reg.Register(command.Command{
		ID: "terminal.toggle", Title: "Terminal: Toggle Terminal",
		Keywords: []string{"terminal", "shell", "console", "bash"},
		Handler:  app.ToggleTerminal,
	})

	reg.Register(command.Command{
		ID: "terminal.fullscreen", Title: "Terminal: Toggle Fullscreen",
		Keywords: []string{"terminal", "shell", "maximize"},
		Handler:  app.ToggleTerminalFullscreen,
	})

	reg.Register(command.Command{
		ID: "terminal.closeAll", Title: "Terminal: Close All",
		Keywords: []string{"terminal", "shell"},
		Handler:  app.CloseAllTerminals,
	})

	reg.Register(command.Command{
		ID: "view.keybindings", Title: "Keyboard Shortcuts",
		Keywords: []string{"view", "keybindings", "shortcuts", "remap", "rebind", "hotkeys"},
		Handler:  app.ShowKeybindings,
	})

	reg.Register(command.Command{
		ID: "about", Title: "About TTT Editor",
		Keywords: []string{"help", "version", "info"},
		Handler: func() {
			app.ShowInfoDialogEx("About TTT Editor", []widgets.KeyValueEntry{
				{Key: "Version", Value: app.Version},
				{Key: "Website", Value: "https://tttedit.dev"},
				{Key: "GitHub", Value: "https://github.com/eugenioenko/ttt"},
			}, true)
		},
	})

}
