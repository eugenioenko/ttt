package app

import (
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func (a *App) ToggleLineNumbers() {
	a.Settings.Editor.LineNumbers = !a.Settings.Editor.LineNumbers
	a.EditorGroup.LineNumbers = a.Settings.Editor.LineNumbers
	if a.EditorGroup.Editor != nil {
		a.EditorGroup.Editor.LineNumbers = a.Settings.Editor.LineNumbers
	}
	config.SaveSettings(*a.Settings)
}

func (a *App) ToggleWordWrap() {
	a.Settings.Editor.WordWrap = !a.Settings.Editor.WordWrap
	a.EditorGroup.WordWrap = a.Settings.Editor.WordWrap
	if a.EditorGroup.Editor != nil {
		a.EditorGroup.Editor.WordWrap = a.Settings.Editor.WordWrap
	}
	config.SaveSettings(*a.Settings)
}

func (a *App) ToggleSyntaxHighlight() {
	enabled := !a.Settings.Editor.IsSyntaxHighlightEnabled()
	a.Settings.Editor.SyntaxHighlight = &enabled
	config.SaveSettings(*a.Settings)
	a.StatusNotify("Restart to apply syntax highlight changes")
}

func (a *App) ToggleBracketPairColorization() {
	a.Settings.Editor.BracketPairColorization = !a.Settings.Editor.BracketPairColorization
	a.EditorGroup.BracketPairColorization = a.Settings.Editor.BracketPairColorization
	if a.EditorGroup.Editor != nil {
		a.EditorGroup.Editor.BracketPairColorization = a.Settings.Editor.BracketPairColorization
		a.EditorGroup.Editor.InvalidateBracketColors()
	}
	config.SaveSettings(*a.Settings)
}

func (a *App) ToggleLSP() {
	enabled := !a.Settings.LSP.IsEnabled()
	a.Settings.LSP.Enabled = &enabled
	config.SaveSettings(*a.Settings)
}

func (a *App) ToggleGitGutter() {
	enabled := !a.Settings.Editor.IsGitGutterEnabled()
	a.Settings.Editor.GitGutter = &enabled
	config.SaveSettings(*a.Settings)
	if enabled {
		a.RequestGitGutterForActiveFile()
	} else if a.EditorGroup.Editor != nil {
		a.EditorGroup.Editor.LineChanges = nil
	}
}

func (a *App) SetGutterStyle(style string) {
	a.Settings.Editor.GutterStyle = style
	a.EditorGroup.GutterStyle = style
	if a.EditorGroup.Editor != nil {
		a.EditorGroup.Editor.GutterStyle = style
	}
	config.SaveSettings(*a.Settings)
}

func (a *App) ShowGutterStylePicker() {
	items := []widgets.SelectItem{
		{ID: "minimal", Label: "Minimal"},
		{ID: "compact", Label: "Compact"},
		{ID: "extended", Label: "Extended"},
	}
	a.ShowSelectDialog("Gutter Style", items, func(id string) {
		a.SetGutterStyle(id)
	}, nil)
}

func (a *App) SetBorderStyle(style string) {
	a.Settings.Editor.BorderStyle = style
	a.ApplyBorderStyle()
	config.SaveSettings(*a.Settings)
}

func (a *App) ApplyBorderStyle() {
	style := a.Settings.Editor.BorderStyle
	switch style {
	case "default", "theme", "":
		// use whatever BuildBorderSet resolved from the theme (defaults to rounded)
	case "rounded":
		*a.Borders = term.RoundedBorderSet()
	case "sharp":
		*a.Borders = term.SingleBorderSet()
	case "double":
		*a.Borders = term.DoubleBorderSet()
	case "bold":
		*a.Borders = term.BoldBorderSet()
	case "ascii":
		*a.Borders = term.AsciiBorderSet()
case "none":
		*a.Borders = term.NoneBorderSet()
	}
}

func (a *App) ShowBorderStylePicker() {
	items := []widgets.SelectItem{
		{ID: "default", Label: "Default"},
		{ID: "rounded", Label: "Rounded"},
		{ID: "sharp", Label: "Sharp"},
		{ID: "double", Label: "Double"},
		{ID: "bold", Label: "Bold"},
		{ID: "ascii", Label: "ASCII"},
		{ID: "none", Label: "None"},
	}
	a.ShowSelectDialog("Border Style", items, func(id string) {
		a.SetBorderStyle(id)
	}, nil)
}

func (a *App) ToggleCmdAsCtrl() {
	a.Settings.Experimental.CmdAsCtrl = !a.Settings.Experimental.CmdAsCtrl
	a.Root.CmdAsCtrl = a.Settings.Experimental.CmdAsCtrl
	config.SaveSettings(*a.Settings)
}

func (a *App) BuildOptionsMenu() []ui.ContextMenuItem {
	lineNumbersChecked := ui.MenuUnchecked
	if a.Settings.Editor.LineNumbers {
		lineNumbersChecked = ui.MenuChecked
	}

	wordWrapChecked := ui.MenuUnchecked
	if a.Settings.Editor.WordWrap {
		wordWrapChecked = ui.MenuChecked
	}

	bracketColorChecked := ui.MenuUnchecked
	if a.Settings.Editor.BracketPairColorization {
		bracketColorChecked = ui.MenuChecked
	}

	lspChecked := ui.MenuUnchecked
	if a.Settings.LSP.IsEnabled() {
		lspChecked = ui.MenuChecked
	}

	gitGutterChecked := ui.MenuUnchecked
	if a.Settings.Editor.IsGitGutterEnabled() {
		gitGutterChecked = ui.MenuChecked
	}

	cmdAsCtrlChecked := ui.MenuUnchecked
	if a.Settings.Experimental.CmdAsCtrl {
		cmdAsCtrlChecked = ui.MenuChecked
	}

	items := []ui.ContextMenuItem{
		{Label: "Line Numbers", Command: "options.toggleLineNumbers", Checked: lineNumbersChecked},
		{Label: "Word Wrap", Command: "options.toggleWordWrap", Checked: wordWrapChecked},
		{Label: "Bracket Colors", Command: "options.toggleBracketColors", Checked: bracketColorChecked},
		{Label: "LSP Code Assist", Command: "options.toggleLSP", Checked: lspChecked},
		{Label: "Git Gutter", Command: "options.toggleGitGutter", Checked: gitGutterChecked},
		ui.MenuSep(),
		{Label: "Gutter Style", Command: "options.gutterStyle"},
		{Label: "Border Style", Command: "options.borderStyle"},
		{Label: "Indentation", Command: "options.indentation"},
		ui.MenuSep(),
		{Label: "Cmd as Ctrl", Command: "options.toggleCmdAsCtrl", Checked: cmdAsCtrlChecked},
		ui.MenuSep(),
		{Label: "Switch Theme", Command: "theme.switch"},
		ui.MenuSep(),
		{Label: "Open Settings", Command: "settings.open"},
	}
	return items
}

func registerOptionsCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "options.toggleSyntaxHighlight", Title: "Toggle Syntax Highlight",
		Keywords: []string{"preferences", "settings", "editor", "view", "performance"},
		Handler:  app.ToggleSyntaxHighlight,
	})

	reg.Register(command.Command{
		ID: "options.toggleLineNumbers", Title: "Toggle Line Numbers",
		Keywords: []string{"preferences", "settings", "editor", "view"},
		Handler:  app.ToggleLineNumbers,
	})

	reg.Register(command.Command{
		ID: "options.toggleWordWrap", Title: "Toggle Word Wrap",
		Keywords: []string{"preferences", "settings", "editor", "view"},
		Handler:  app.ToggleWordWrap,
	})

	reg.Register(command.Command{
		ID: "options.toggleBracketColors", Title: "Toggle Bracket Pair Colorization",
		Handler: app.ToggleBracketPairColorization,
	})

	reg.Register(command.Command{
		ID: "options.toggleLSP", Title: "Toggle LSP",
		Keywords: []string{"preferences", "settings", "language", "server", "autocomplete"},
		Handler:  app.ToggleLSP,
	})

	reg.Register(command.Command{
		ID: "options.toggleGitGutter", Title: "Toggle Git Gutter",
		Keywords: []string{"preferences", "settings", "editor", "view", "git"},
		Handler:  app.ToggleGitGutter,
	})

	reg.Register(command.Command{
		ID: "options.gutterStyle", Title: "Change Gutter Style",
		Keywords: []string{"preferences", "settings", "editor", "view"},
		Handler:  app.ShowGutterStylePicker,
	})

	reg.Register(command.Command{
		ID: "options.borderStyle", Title: "Change Border Style",
		Keywords: []string{"preferences", "settings", "editor", "view", "borders", "rounded", "sharp"},
		Handler:  app.ShowBorderStylePicker,
	})

	reg.Register(command.Command{
		ID: "options.indentation", Title: "Editor Indentation",
		Keywords: []string{"preferences", "settings", "editor", "indentation", "tabs", "spaces"},
		Handler:  app.ShowIndentSettings,
	})

	reg.Register(command.Command{
		ID: "options.toggleCmdAsCtrl", Title: "Experimental: Toggle Cmd as Ctrl",
		Keywords: []string{"preferences", "settings", "experimental", "macos", "cmd", "super", "meta"},
		Handler:  app.ToggleCmdAsCtrl,
	})
}
