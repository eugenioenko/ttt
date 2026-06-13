package app

import (
	"fmt"
	"strconv"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) OpenCommandPalette(fileMode bool, initialText ...string) {
	palette := ui.NewCommandPaletteWidget(a.Reg.List())
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
		a.PushNavHistory()
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
	var cmds []command.Command
	for _, name := range names {
		cmds = append(cmds, command.Command{ID: name, Title: name})
	}
	picker := ui.NewCommandPaletteWidget(cmds)
	picker.Borders = a.Borders
	originalStyleMap := a.Screen.GetStyleMap()
	originalPalette := *a.Palette
	applyTheme := func(theme config.ThemeConfig) {
		a.Screen.SetStyleMap(BuildStyleMap(theme))
		*a.Palette = BuildTerminalPalette(theme)
		*a.Borders = BuildBorderSet(theme.Borders)
		a.Renderer.Clear()
	}
	picker.OnSelectionChange = func(name string) {
		theme, err := config.LoadTheme(name)
		if err != nil {
			return
		}
		applyTheme(theme)
	}
	picker.OnExecute = func(name string) {
		a.DismissDialog()
		theme, err := config.LoadTheme(name)
		if err != nil {
			return
		}
		applyTheme(theme)
		a.Settings.Theme = name
		config.SaveSettings(*a.Settings)
	}
	picker.OnDismiss = func() {
		a.DismissDialog()
		a.Screen.SetStyleMap(originalStyleMap)
		*a.Palette = originalPalette
		a.Renderer.Clear()
	}
	a.ShowDialog(picker)
}

func (a *App) ShowIndentPicker() {
	var cmds []command.Command
	sizes := []int{1, 2, 3, 4, 6, 8}
	for _, s := range sizes {
		label := fmt.Sprintf("Spaces: %d", s)
		cmds = append(cmds, command.Command{ID: strconv.Itoa(s), Title: label})
	}
	cmds = append(cmds, command.Command{ID: "tabs", Title: "Indent Using Tabs"})
	cmds = append(cmds, command.Command{ID: "detect", Title: "Detect from Content"})
	a.ShowPicker(cmds, func(id string) {
		if id == "detect" {
			if a.EditorGroup.Editor != nil && a.EditorGroup.Editor.Buf != nil {
				if info := buffer.DetectIndent(a.EditorGroup.Editor.Buf.Lines); info.Size > 0 {
					a.EditorGroup.SetTabSize(info.Size)
				}
			}
		} else if id == "tabs" {
			a.EditorGroup.SetTabSize(4)
		} else if size, err := strconv.Atoi(id); err == nil {
			a.EditorGroup.SetTabSize(size)
		}
	})
}

func (a *App) ShowEolPicker() {
	cmds := []command.Command{
		{ID: "lf", Title: "LF"},
		{ID: "crlf", Title: "CRLF"},
	}
	a.ShowPicker(cmds, func(id string) {
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
	})
}

func registerPaletteCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "command.palette", Title: "Command Palette",
		Handler: func() { app.OpenCommandPalette(false) },
	})

	reg.Register(command.Command{
		ID: "file.quickOpen", Title: "Go to File",
		Handler: func() { app.OpenCommandPalette(true) },
	})

	reg.Register(command.Command{
		ID: "editor.goToLine", Title: "Go to Line",
		Handler: func() { app.OpenCommandPalette(false, ":") },
	})

	reg.Register(command.Command{
		ID: "theme.switch", Title: "Switch Theme",
		Handler: app.ShowThemePicker,
	})

	app.StatusBar.OnIndentClick = app.ShowIndentPicker
	app.StatusBar.OnEolClick = app.ShowEolPicker

	reg.Register(command.Command{
		ID: "editor.indentation", Title: "Change Indentation",
		Handler: app.ShowIndentPicker,
	})

	reg.Register(command.Command{
		ID: "editor.lineEnding", Title: "Change Line Ending",
		Handler: app.ShowEolPicker,
	})

	reg.Register(command.Command{
		ID: "settings.open", Title: "Preferences: Open Settings",
		Handler: func() {
			path := config.ConfigFilePath("settings.json")
			config.EnsureConfigFile(path, "{}\n")
			app.EditorGroup.OpenFile(path)
		},
	})

	reg.Register(command.Command{
		ID: "keybindings.open", Title: "Preferences: Open Keyboard Shortcuts",
		Handler: func() {
			path := config.ConfigFilePath("keybindings.json")
			config.EnsureConfigFile(path, "{}\n")
			app.EditorGroup.OpenFile(path)
		},
	})
}
