package app

import (
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func (a *App) OpenCommandPalette(fileMode bool, initialText ...string) {
	if a.Root.HasModalOverlay() {
		return
	}
	palette := ui.NewSelectDialogWidget(a.Reg.List())
	palette.Borders = a.Borders
	palette.SetFiles(a.Workspace.Paths())
	if len(initialText) > 0 {
		palette.Input.SetText(initialText[0])
	} else if fileMode {
		palette.Input.SetText("")
	}
	palette.OnExecute = func(id string) {
		a.DismissDialog()
		a.Reg.Execute(id)
	}
	palette.OnOpenFile = func(absPath string) {
		a.DismissDialog()
		a.EditorGroup.OpenFile(absPath)
	}
	palette.OnGoToLine = func(line int) {
		a.DismissDialog()
		a.EditorGroup.GoToLine(line)
	}
	palette.OnDismiss = func() {
		a.DismissDialog()
	}
	a.ShowDialog(palette)
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
	originalPalette := *a.Palette
	applyTheme := func(theme config.ThemeConfig) {
		a.Screen.SetStyleMap(BuildStyleMap(theme, WithTransparentBackground(a.Settings.Editor.TransparentBackground)))
		*a.Palette = BuildTerminalPalette(theme, WithTransparentBackground(a.Settings.Editor.TransparentBackground))
		*a.Borders = BuildBorderSet(theme.Borders)
		a.ApplyBorderStyle()
		a.Renderer.Clear()
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
			config.SaveSettings(*a.Settings)
		},
		OnDismiss: func() {
			a.DismissDialog()
			a.Screen.SetStyleMap(originalStyleMap)
			*a.Palette = originalPalette
			a.Renderer.Clear()
		},
	})

	dialog := widgets.NewDialogWidget(50)
	dialog.Title = "Select Theme"
	dialog.Borders = *a.Borders
	dialog.SetContent(sel)
	dialog.OnDismiss = sel.Config.OnDismiss
	dialog.Build()

	adapter := ui.NewWidgetAdapter(dialog)
	a.ShowDialog(adapter)
}

func (a *App) ShowIndentPicker() {
	a.showIndentDialog("File Indentation", func(useTabs bool, tabSize int) {
		a.EditorGroup.SetTabSize(tabSize)
		a.EditorGroup.SetUseTabs(useTabs)
	})
}

func (a *App) ShowIndentSettings() {
	a.showIndentDialog("Editor Indentation", func(useTabs bool, tabSize int) {
		a.Settings.Editor.TabSize = tabSize
		a.Settings.Editor.InsertSpaces = !useTabs
		a.EditorGroup.TabSize = tabSize
		a.EditorGroup.InsertSpaces = !useTabs
		a.EditorGroup.SetTabSize(tabSize)
		a.EditorGroup.SetUseTabs(useTabs)
		config.SaveSettings(*a.Settings)
	})
}

func (a *App) ShowEolPicker() {
	items := []widgets.SelectItem{
		{ID: "lf", Label: "LF"},
		{ID: "crlf", Label: "CRLF"},
	}
	a.ShowSelectDialog("Line Ending", items, func(id string) {
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

func registerPaletteCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "command.palette", Title: "Command Palette",
		Handler: func() { app.OpenCommandPalette(false) },
	})

	reg.Register(command.Command{
		ID: "file.quickOpen", Title: "Go to File",
		Keywords: []string{"file", "navigate", "open"},
		Handler:  func() { app.OpenCommandPalette(true) },
	})

	reg.Register(command.Command{
		ID: "editor.goToLine", Title: "Go to Line",
		Keywords: []string{"editor", "navigate", "jump"},
		Handler:  func() { app.OpenCommandPalette(false, ":") },
	})

	reg.Register(command.Command{
		ID: "theme.switch", Title: "Switch Theme",
		Keywords: []string{"preferences", "settings", "colors", "appearance"},
		Handler:  app.ShowThemePicker,
	})

	app.Status.SetSegment(view.StatusSegment{ID: "indent", Side: "right", Priority: 200, OnClick: app.ShowIndentPicker})
	app.Status.SetSegment(view.StatusSegment{ID: "eol", Side: "right", Priority: 400, OnClick: app.ShowEolPicker})

	reg.Register(command.Command{
		ID: "editor.indentation", Title: "Change Indentation",
		Keywords: []string{"editor", "preferences", "settings", "spaces", "tabs"},
		Handler:  app.ShowIndentPicker,
	})

	reg.Register(command.Command{
		ID: "editor.lineEnding", Title: "Change Line Ending",
		Keywords: []string{"editor", "preferences", "settings", "eol", "crlf", "lf"},
		Handler:  app.ShowEolPicker,
	})

	reg.Register(command.Command{
		ID: "settings.openUI", Title: "Settings: Open Editor Settings",
		Keywords: []string{"preferences", "settings", "configuration", "options", "ui"},
		Handler:  app.ShowSettings,
	})

	reg.Register(command.Command{
		ID: "settings.apply", Title: "Settings: Apply Changes",
		Keywords: []string{"preferences", "settings", "save"},
		Handler:  app.ApplySettingsView,
	})

	reg.Register(command.Command{
		ID: "settings.cancel", Title: "Settings: Discard Changes",
		Keywords: []string{"preferences", "settings", "revert", "cancel", "close"},
		Handler:  app.CancelSettingsView,
	})

	reg.Register(command.Command{
		ID: "settings.open", Title: "Settings: Open settings.json",
		Keywords: []string{"preferences", "settings", "configuration", "options"},
		Handler: func() {
			path := config.ConfigFilePath("settings.json")
			config.EnsureConfigFile(path, "{}\n")
			app.EditorGroup.OpenFile(path)
		},
	})

	reg.Register(command.Command{
		ID: "keybindings.open", Title: "Settings: Open keybindings.json",
		Keywords: []string{"preferences", "settings", "hotkeys", "keymap"},
		Handler: func() {
			path := config.ConfigFilePath("keybindings.json")
			config.EnsureConfigFile(path, "{}\n")
			app.EditorGroup.OpenFile(path)
		},
	})
}
