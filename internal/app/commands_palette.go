package app

import (
	"slices"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func (a *App) OpenCommandPalette(fileMode bool, initialText ...string) {
	if a.Root.HasModalOverlay() {
		return
	}
	previousFocus := a.Root.Focused
	palette := ui.NewSelectDialogWidget(a.Reg.List())
	palette.SetRecentCommands(a.State.RecentCommands)
	palette.Input.ControlModifiers = &a.Root.ControlModifiers
	palette.Borders = a.Borders
	palette.Icons = a.Settings.Appearance.Icons
	a.prepareQuickOpen(palette)
	if len(initialText) > 0 {
		palette.Input.SetText(initialText[0])
	} else if fileMode {
		palette.Input.SetText("")
	}
	palette.OnExecute = func(id string) {
		a.DismissDialog()
		if (id == "terminal.sendNextKey" || id == "terminal.toggleShortcutForwarding") && previousFocus != nil {
			a.Root.SetFocus(previousFocus)
		}
		a.rememberRecentCommand(id)
		a.Reg.Execute(id)
	}
	palette.OnOpenFile = func(absPath string) {
		a.DismissDialog()
		a.EditorGroup.OpenFile(absPath)
		a.FocusEditor()
	}
	palette.OnGoToLine = func(line int) {
		a.DismissDialog()
		a.EditorGroup.GoToLine(line)
		a.FocusEditor()
	}
	palette.OnDismiss = func() {
		a.DismissDialog()
	}
	a.ShowDialog(palette)
}

const maxRecentCommands = 10

// rememberRecentCommand merges into the list on disk, so another ttt window's
// commands are kept.
func (a *App) rememberRecentCommand(id string) {
	disk := config.LoadState()
	recent := pushRecent(disk.RecentCommands, []string{id}, maxRecentCommands)
	a.State.RecentCommands = recent
	if slices.Equal(recent, disk.RecentCommands) {
		return
	}
	disk.RecentCommands = recent
	if err := config.SaveState(disk); err != nil {
		a.StatusError(i18n.Format("status.failedToSaveRecentCommands", i18n.Args{"error": err.Error()}))
	}
}

func (a *App) ShowThemePicker() {
	names := config.ListThemes()
	if len(names) == 0 {
		return
	}
	items := make([]widgets.SelectItem, len(names))
	for i, name := range names {
		items[i] = widgets.SelectItem{ID: name, Label: name}
	}
	originalStyleMap := a.Screen.GetStyleMap()
	originalTokenTheme := highlight.CurrentTokenTheme()
	originalPalette := *a.Palette
	applyTheme := func(theme config.ThemeConfig) {
		ApplyThemeStyles(a.Screen, theme, WithTransparentBackground(a.Settings.Editor.TransparentBackground), WithEditorSelectionStyle(a.Settings.Editor.SelectionStyle), WithPluginSelectionStyle(a.Settings.Plugins.SelectionStyle))
		*a.Palette = BuildTerminalPalette(theme, WithTransparentBackground(a.Settings.Editor.TransparentBackground), WithTerminalSelectionStyle(a.Settings.Terminal.SelectionStyle))
		*a.Borders = BuildBorderSet(theme.Borders)
		a.ApplyBorderStyle()
		a.Renderer.Clear()
		a.invalidateImageLayer()
	}
	sel := widgets.NewSelectWidget(widgets.SelectConfig{
		Items:       items,
		ShowDivider: true,
		OnChange: func(name string) {
			theme, err := config.LoadTheme(name)
			if err != nil {
				return
			}
			applyTheme(theme)
		},
		OnSelect: func(name string) {
			a.DismissDialog()
			theme, err := config.LoadTheme(name)
			if err != nil {
				return
			}
			applyTheme(theme)
			a.Settings.Theme = name
			a.persistSettings()
		},
		OnDismiss: func() {
			a.DismissDialog()
			a.Screen.SetStyleMap(originalStyleMap)
			highlight.SetTokenTheme(originalTokenTheme)
			*a.Palette = originalPalette
			a.Renderer.Clear()
			a.invalidateImageLayer()
		},
	})

	dialog := widgets.NewDialogWidget(50)
	dialog.Title = i18n.Text("options.selectTheme")
	dialog.Borders = *a.Borders
	dialog.SetContent(sel)
	dialog.OnDismiss = sel.Config.OnDismiss
	dialog.Build()

	adapter := ui.NewWidgetAdapter(dialog)
	a.ShowDialog(adapter)
}

func (a *App) ShowIndentPicker() {
	a.showIndentDialog(i18n.Text("options.fileIndentation"), func(useTabs bool, tabSize int) {
		a.EditorGroup.SetTabSize(tabSize)
		a.EditorGroup.SetUseTabs(useTabs)
	})
}

func (a *App) ShowIndentSettings() {
	a.showIndentDialog(i18n.Text("options.editorIndentation"), func(useTabs bool, tabSize int) {
		a.Settings.Editor.TabSize = tabSize
		a.Settings.Editor.InsertSpaces = !useTabs
		a.EditorGroup.TabSize = tabSize
		a.EditorGroup.InsertSpaces = !useTabs
		a.EditorGroup.SetTabSize(tabSize)
		a.EditorGroup.SetUseTabs(useTabs)
		a.persistSettings()
	})
}

func (a *App) ShowEolPicker() {
	items := []widgets.SelectItem{
		{ID: "lf", Label: "LF"},
		{ID: "crlf", Label: "CRLF"},
	}
	a.ShowSelectDialog(i18n.Text("options.lineEnding"), items, func(id string) {
		buf := a.EditorGroup.ActiveBuffer()
		if buf == nil {
			return
		}
		switch id {
		case "lf":
			buf.LineEnding = "\n"
		case "crlf":
			buf.LineEnding = "\r\n"
		}
		buf.Dirty = true
	}, nil)
}

func (a *App) ShowLanguagePicker() {
	if a.Root.HasOverlay() || a.EditorGroup.ActiveBuffer() == nil {
		return
	}
	items := []widgets.SelectItem{
		{ID: "", Label: i18n.Text("options.autoDetect")},
		{ID: highlight.PlainText, Label: i18n.Text("options.plainText")},
	}
	for _, language := range highlight.Languages() {
		items = append(items, widgets.SelectItem{ID: language.ID, Label: language.Name})
	}
	for i := range items {
		if items[i].ID == a.EditorGroup.ActiveLanguageSelection() {
			items[i].Label += " (current)"
		}
	}
	a.ShowSelectDialog(i18n.Text("options.changeLanguageMode"), items, func(id string) {
		if a.EditorGroup.ActiveBuffer() == nil {
			return
		}
		a.EditorGroup.SetActiveLanguage(id)
		a.SyncLanguageSegment()
		if !a.EditorGroup.SyntaxHighlight {
			a.StatusNotify(i18n.Text("status.syntaxHighlightingIsDisabled"))
		}
	}, nil)
}

func registerPaletteCommands(app *App) {
	app.Reg.Register(command.Command{ID: "language.select", Title: i18n.Text("command.changeDisplayLanguage"), Keywords: []string{"language", "locale", "translation", "display"}, Handler: app.ShowDisplayLanguagePicker})
	reg := app.Reg

	reg.Register(command.Command{
		ID: "command.palette", Title: i18n.Text("command.commandPalette"),
		Handler: func() { app.OpenCommandPalette(false) },
	})

	reg.Register(command.Command{
		ID: "file.quickOpen", Title: i18n.Text("command.goToFile"),
		Keywords: []string{"file", "navigate", "open"},
		Handler:  func() { app.OpenCommandPalette(true) },
	})

	reg.Register(command.Command{
		ID: "editor.goToLine", Title: i18n.Text("command.goToLine"),
		Keywords: []string{"editor", "navigate", "jump"},
		Handler:  func() { app.OpenCommandPalette(false, ":") },
	})

	reg.Register(command.Command{
		ID: "theme.switch", Title: i18n.Text("command.switchTheme"),
		Keywords: []string{"preferences", "settings", "colors", "appearance"},
		Handler:  app.ShowThemePicker,
	})

	app.Status.SetSegment(view.StatusSegment{ID: "indent", Side: "right", Priority: 200, OnClick: app.ShowIndentPicker})
	app.Status.SetSegment(view.StatusSegment{ID: "eol", Side: "right", Priority: 400, OnClick: app.ShowEolPicker})

	reg.Register(command.Command{
		ID: "editor.indentation", Title: i18n.Text("command.changeIndentation"),
		Keywords: []string{"editor", "preferences", "settings", "spaces", "tabs"},
		Handler:  app.ShowIndentPicker,
	})

	reg.Register(command.Command{
		ID: "editor.lineEnding", Title: i18n.Text("command.changeLineEnding"),
		Keywords: []string{"editor", "preferences", "settings", "eol", "crlf", "lf"},
		Handler:  app.ShowEolPicker,
	})

	reg.Register(command.Command{
		ID: "editor.changeLanguage", Title: i18n.Text("command.changeLanguageMode"),
		Keywords: []string{"language", "syntax", "highlighting", "mode", "filetype"},
		Handler:  app.ShowLanguagePicker,
	})

	reg.Register(command.Command{
		ID: "settings.openUI", Title: i18n.Text("command.settingsOpenEditorSettings"),
		Keywords: []string{"preferences", "settings", "configuration", "options", "ui"},
		Handler:  app.ShowSettings,
	})

	reg.Register(command.Command{
		ID: "settings.apply", Title: i18n.Text("command.settingsApplyChanges"),
		Keywords: []string{"preferences", "settings", "save"},
		Handler:  app.ApplySettingsView,
	})

	reg.Register(command.Command{
		ID: "settings.cancel", Title: i18n.Text("command.settingsDiscardChanges"),
		Keywords: []string{"preferences", "settings", "revert", "cancel", "close"},
		Handler:  app.CancelSettingsView,
	})

	reg.Register(command.Command{
		ID: "settings.open", Title: i18n.Text("command.settingsOpenSettingsJson"),
		Keywords: []string{"preferences", "settings", "configuration", "options"},
		Handler:  app.OpenSettingsFile,
	})

	reg.Register(command.Command{
		ID: "keybindings.open", Title: i18n.Text("command.settingsOpenKeybindingsJson"),
		Keywords: []string{"preferences", "settings", "hotkeys", "keymap"},
		Handler: func() {
			path := config.ConfigFilePath("keybindings.json")
			config.EnsureConfigFile(path, "{}\n")
			app.EditorGroup.OpenFile(path)
		},
	})
}
