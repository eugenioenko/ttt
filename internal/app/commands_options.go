package app

import (
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func (a *App) SaveAndApplySettings() {
	config.SaveSettings(*a.Settings)
	a.ApplySettings(*a.Settings)
}

func (a *App) ToggleLineNumbers() {
	a.Settings.Editor.LineNumbers = !a.Settings.Editor.LineNumbers
	a.SaveAndApplySettings()
}

func (a *App) ToggleWordWrap() {
	a.Settings.Editor.WordWrap = !a.Settings.Editor.WordWrap
	a.SaveAndApplySettings()
}

func (a *App) UseSplitDiffByDefault() {
	a.Settings.Editor.DiffMode = config.DiffModeSplit
	a.SaveAndApplySettings()
}

func (a *App) UseUnifiedDiffByDefault() {
	a.Settings.Editor.DiffMode = config.DiffModeUnified
	a.SaveAndApplySettings()
}

func (a *App) UseChangesOnlyDiffByDefault() {
	a.Settings.Editor.DiffContext = config.DiffContextChanges
	a.SaveAndApplySettings()
}

func (a *App) UseFullFileDiffByDefault() {
	a.Settings.Editor.DiffContext = config.DiffContextFull
	a.SaveAndApplySettings()
}

func (a *App) ToggleDiffWordWrapDefault() {
	a.Settings.Editor.DiffWordWrap = !a.Settings.Editor.DiffWordWrap
	a.SaveAndApplySettings()
}

func (a *App) ToggleDiffHighContrast() {
	a.Settings.Editor.DiffHighContrast = !a.Settings.Editor.DiffHighContrast
	a.SaveAndApplySettings()
}

func (a *App) ToggleDiffCollapsedEmphasis() {
	a.Settings.Editor.DiffCollapsedEmphasis = !a.Settings.Editor.DiffCollapsedEmphasis
	a.SaveAndApplySettings()
}

func (a *App) UseTreeGitFileView() {
	a.Settings.Git.FileView = config.GitFileViewTree
	a.SaveAndApplySettings()
}

func (a *App) UseListGitFileView() {
	a.Settings.Git.FileView = config.GitFileViewList
	a.SaveAndApplySettings()
}

func (a *App) ExpandAllGitFiles() {
	if detail := a.EditorGroup.ActiveCommitDetailWidget(); detail != nil {
		detail.ExpandAllFiles()
		return
	}
	if a.Sidebar.ActivePanel == "changes" {
		a.Changes.ExpandAll()
	}
}

func (a *App) CollapseAllGitFiles() {
	if detail := a.EditorGroup.ActiveCommitDetailWidget(); detail != nil {
		detail.CollapseAllFiles()
		return
	}
	if a.Sidebar.ActivePanel == "changes" {
		a.Changes.CollapseAll()
	}
}

func (a *App) ExpandAllChangesFiles() {
	if a.Changes != nil {
		a.Changes.ExpandAll()
	}
}

func (a *App) CollapseAllChangesFiles() {
	if a.Changes != nil {
		a.Changes.CollapseAll()
	}
}

func (a *App) ExpandAllCommitDetailFiles() {
	if detail := a.EditorGroup.ActiveCommitDetailWidget(); detail != nil {
		detail.ExpandAllFiles()
	}
}

func (a *App) CollapseAllCommitDetailFiles() {
	if detail := a.EditorGroup.ActiveCommitDetailWidget(); detail != nil {
		detail.CollapseAllFiles()
	}
}

func (a *App) ToggleAutoDedent() {
	enabled := !a.Settings.Editor.IsAutoDedentEnabled()
	a.Settings.Editor.AutoDedent = &enabled
	a.SaveAndApplySettings()
}

func (a *App) ToggleAutoIndent() {
	enabled := !a.Settings.Editor.IsAutoIndentEnabled()
	a.Settings.Editor.AutoIndent = &enabled
	a.SaveAndApplySettings()
}

func (a *App) ToggleShowTrailingNewline() {
	enabled := !a.Settings.Editor.IsShowTrailingNewlineEnabled()
	a.Settings.Editor.ShowTrailingNewline = &enabled
	a.SaveAndApplySettings()
}

func (a *App) ToggleSyntaxHighlight() {
	enabled := !a.Settings.Editor.IsSyntaxHighlightEnabled()
	a.Settings.Editor.SyntaxHighlight = &enabled
	a.SaveAndApplySettings()
	a.StatusNotify("Restart to apply syntax highlight changes")
}

func (a *App) ToggleBracketPairColorization() {
	a.Settings.Editor.BracketPairColorization = !a.Settings.Editor.BracketPairColorization
	a.SaveAndApplySettings()
}

func (a *App) ToggleTransparentBackground() {
	a.Settings.Editor.TransparentBackground = !a.Settings.Editor.TransparentBackground
	a.SaveAndApplySettings()
}

func (a *App) ToggleLSP() {
	enabled := !a.Settings.LSP.IsEnabled()
	a.Settings.LSP.Enabled = &enabled
	a.SaveAndApplySettings()
}

func (a *App) ToggleGitGutter() {
	enabled := !a.Settings.Editor.IsGitGutterEnabled()
	a.Settings.Editor.GitGutter = &enabled
	a.SaveAndApplySettings()
}

func (a *App) SetGutterStyle(style string) {
	a.Settings.Editor.GutterStyle = style
	a.SaveAndApplySettings()
}

// Display labels for the gutter and border style values in config.
var styleLabels = map[string]string{
	"minimal":  "Minimal",
	"compact":  "Compact",
	"extended": "Extended",
	"default":  "Default",
	"rounded":  "Rounded",
	"sharp":    "Sharp",
	"double":   "Double",
	"bold":     "Bold",
	"ascii":    "ASCII",
	"none":     "None",
}

func gutterStyleItems() []widgets.SelectItem {
	items := make([]widgets.SelectItem, 0, len(config.GutterStyles))
	for _, id := range config.GutterStyles {
		items = append(items, widgets.SelectItem{ID: id, Label: styleLabels[id]})
	}
	return items
}

func borderStyleItems() []widgets.SelectItem {
	items := make([]widgets.SelectItem, 0, len(config.BorderStyles))
	for _, id := range config.BorderStyles {
		// "theme" is accepted in settings.json but behaves identically to
		// "default", so it is not offered as a separate choice.
		if id == "theme" {
			continue
		}
		items = append(items, widgets.SelectItem{ID: id, Label: styleLabels[id]})
	}
	return items
}

func diffModeItems() []widgets.SelectItem {
	return []widgets.SelectItem{
		{ID: config.DiffModeSplit, Label: "Split"},
		{ID: config.DiffModeUnified, Label: "Unified"},
	}
}

func diffContextItems() []widgets.SelectItem {
	return []widgets.SelectItem{
		{ID: config.DiffContextChanges, Label: "Changes Only"},
		{ID: config.DiffContextFull, Label: "Full File"},
	}
}

func (a *App) ShowGutterStylePicker() {
	a.ShowSelectDialog("Gutter Style", gutterStyleItems(), func(id string) {
		a.SetGutterStyle(id)
	}, nil)
}

func (a *App) SetBorderStyle(style string) {
	a.Settings.Editor.BorderStyle = style
	a.SaveAndApplySettings()
}

func (a *App) ApplyBorderStyle() { a.applyBorderStyle(nil) }

// themeBorders, when non-nil, is the border set already resolved from the
// current theme, so the theme need not be reloaded from disk.
func (a *App) applyBorderStyle(themeBorders *term.BorderSet) {
	style := a.Settings.Editor.BorderStyle
	switch style {
	case "default", "theme", "":
		// Fall back to the theme's border set. Rebuilding it here (rather than
		// relying on whatever is currently in a.Borders) is what makes switching
		// from an explicit style back to "default" actually take effect.
		if themeBorders != nil {
			*a.Borders = *themeBorders
		} else if a.Settings.Theme != "" {
			if theme, err := config.LoadTheme(a.Settings.Theme); err == nil {
				*a.Borders = BuildBorderSet(theme.Borders)
			}
		}
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
	a.ShowSelectDialog("Border Style", borderStyleItems(), func(id string) {
		a.SetBorderStyle(id)
	}, nil)
}

func (a *App) ShowDiffViewModePicker() {
	a.ShowSelectDialog("Diff View Mode", diffModeItems(), func(id string) {
		if id == config.DiffModeUnified {
			a.UseUnifiedDiffByDefault()
			return
		}
		a.UseSplitDiffByDefault()
	}, nil)
}

func (a *App) ShowDiffContextPicker() {
	a.ShowSelectDialog("Diff Context", diffContextItems(), func(id string) {
		if id == config.DiffContextFull {
			a.UseFullFileDiffByDefault()
			return
		}
		a.UseChangesOnlyDiffByDefault()
	}, nil)
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

	autoIndentChecked := ui.MenuUnchecked
	if a.Settings.Editor.IsAutoIndentEnabled() {
		autoIndentChecked = ui.MenuChecked
	}

	autoDedentChecked := ui.MenuUnchecked
	if a.Settings.Editor.IsAutoDedentEnabled() {
		autoDedentChecked = ui.MenuChecked
	}

	lspChecked := ui.MenuUnchecked
	if a.Settings.LSP.IsEnabled() {
		lspChecked = ui.MenuChecked
	}

	gitGutterChecked := ui.MenuUnchecked
	if a.Settings.Editor.IsGitGutterEnabled() {
		gitGutterChecked = ui.MenuChecked
	}

	syntaxChecked := ui.MenuUnchecked
	if a.Settings.Editor.IsSyntaxHighlightEnabled() {
		syntaxChecked = ui.MenuChecked
	}

	menuBarChecked := ui.MenuUnchecked
	if a.Settings.Editor.IsMenuBarVisible() {
		menuBarChecked = ui.MenuChecked
	}

	transparentBgChecked := ui.MenuUnchecked
	if a.Settings.Editor.TransparentBackground {
		transparentBgChecked = ui.MenuChecked
	}

	items := []ui.ContextMenuItem{
		{Label: "Line Numbers", Command: "options.toggleLineNumbers", Checked: lineNumbersChecked},
		{Label: "Word Wrap", Command: "options.toggleWordWrap", Checked: wordWrapChecked},
		{Label: "Auto Indent", Command: "options.toggleAutoIndent", Checked: autoIndentChecked},
		{Label: "Auto Dedent", Command: "options.toggleAutoDedent", Checked: autoDedentChecked},
		{Label: "Syntax Highlight", Command: "options.toggleSyntaxHighlight", Checked: syntaxChecked},
		{Label: "Bracket Colors", Command: "options.toggleBracketColors", Checked: bracketColorChecked},
		{Label: "LSP Code Assist", Command: "options.toggleLSP", Checked: lspChecked},
		{Label: "Git Gutter", Command: "options.toggleGitGutter", Checked: gitGutterChecked},
		{Label: "Menu Bar", Command: menuBarToggleCommand, Checked: menuBarChecked},
		{Label: "Transparent BG", Command: "options.toggleTransparentBackground", Checked: transparentBgChecked},
		ui.MenuSep(),
		{Label: "Diff Views", Submenu: a.BuildDiffViewOptions()},
		{Label: "Git Files", Submenu: a.BuildGitFileOptions()},
		ui.MenuSep(),
		{Label: "Gutter Style", Command: "options.gutterStyle"},
		{Label: "Border Style", Command: "options.borderStyle"},
		{Label: "Indentation", Command: "options.indentation"},
		ui.MenuSep(),
		{Label: "Settings", Command: "settings.openUI"},
	}
	return items
}

func menuChecked(checked bool) int {
	if checked {
		return ui.MenuChecked
	}
	return ui.MenuUnchecked
}

func (a *App) BuildDiffViewOptions() []ui.ContextMenuItem {
	return []ui.ContextMenuItem{
		{Label: "Split", Command: "options.useSplitDiff", Checked: menuChecked(a.Settings.Editor.DiffMode != config.DiffModeUnified)},
		{Label: "Unified", Command: "options.useUnifiedDiff", Checked: menuChecked(a.Settings.Editor.DiffMode == config.DiffModeUnified)},
		ui.MenuSep(),
		{Label: "Changes Only", Command: "options.useChangesOnlyDiff", Checked: menuChecked(a.Settings.Editor.DiffContext != config.DiffContextFull)},
		{Label: "Full File", Command: "options.useFullFileDiff", Checked: menuChecked(a.Settings.Editor.DiffContext == config.DiffContextFull)},
		ui.MenuSep(),
		{Label: "Wrap Lines", Command: "options.toggleDiffWordWrap", Checked: menuChecked(a.Settings.Editor.DiffWordWrap)},
		{Label: "High Contrast", Command: "options.toggleDiffHighContrast", Checked: menuChecked(a.Settings.Editor.DiffHighContrast)},
		{Label: "Emphasize Collapsed Rows", Command: "options.toggleDiffCollapsedEmphasis", Checked: menuChecked(a.Settings.Editor.DiffCollapsedEmphasis)},
	}
}

func (a *App) BuildGitFileOptions() []ui.ContextMenuItem {
	return a.buildGitFileOptions("changes.expandAll", "changes.collapseAll")
}

func (a *App) BuildChangesGitFileOptions() []ui.ContextMenuItem {
	return a.buildGitFileOptions("changes.expandAllWorkingTree", "changes.collapseAllWorkingTree")
}

func (a *App) buildGitFileOptions(expandCommand, collapseCommand string) []ui.ContextMenuItem {
	return []ui.ContextMenuItem{
		{Label: "Tree", Command: "options.useGitFileTree", Checked: menuChecked(a.Settings.Git.FileView == config.GitFileViewTree)},
		{Label: "List", Command: "options.useGitFileList", Checked: menuChecked(a.Settings.Git.FileView != config.GitFileViewTree)},
		ui.MenuSep(),
		{Label: "Expand All", Command: expandCommand},
		{Label: "Collapse All", Command: collapseCommand},
	}
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
		ID: "options.diffViewMode", Title: "Change Diff View Mode",
		Keywords: []string{"preferences", "settings", "git", "diff", "split", "unified", "mode"},
		Handler:  app.ShowDiffViewModePicker,
	})

	reg.Register(command.Command{
		ID: "options.diffContext", Title: "Change Diff Context",
		Keywords: []string{"preferences", "settings", "git", "diff", "changes", "full", "context"},
		Handler:  app.ShowDiffContextPicker,
	})

	reg.Register(command.Command{
		ID: "options.useSplitDiff", Title: "Use Split Diff by Default",
		Keywords: []string{"preferences", "settings", "git", "diff", "split", "default"},
		Handler:  app.UseSplitDiffByDefault,
	})

	reg.Register(command.Command{
		ID: "options.useUnifiedDiff", Title: "Use Unified Diff by Default",
		Keywords: []string{"preferences", "settings", "git", "diff", "unified", "default"},
		Handler:  app.UseUnifiedDiffByDefault,
	})

	reg.Register(command.Command{
		ID: "options.useChangesOnlyDiff", Title: "Show Changes Only by Default",
		Keywords: []string{"preferences", "settings", "git", "diff", "compact", "context", "default"},
		Handler:  app.UseChangesOnlyDiffByDefault,
	})

	reg.Register(command.Command{
		ID: "options.useFullFileDiff", Title: "Show Full File Diff by Default",
		Keywords: []string{"preferences", "settings", "git", "diff", "extended", "context", "default"},
		Handler:  app.UseFullFileDiffByDefault,
	})

	reg.Register(command.Command{
		ID: "options.toggleDiffWordWrap", Title: "Toggle Diff Word Wrap Default",
		Keywords: []string{"preferences", "settings", "git", "diff", "wrap", "default"},
		Handler:  app.ToggleDiffWordWrapDefault,
	})

	reg.Register(command.Command{
		ID: "options.toggleDiffHighContrast", Title: "Toggle High Contrast Diffs",
		Keywords: []string{"preferences", "settings", "git", "diff", "contrast", "color", "accessibility"},
		Handler:  app.ToggleDiffHighContrast,
	})

	reg.Register(command.Command{
		ID: "options.toggleDiffCollapsedEmphasis", Title: "Toggle Collapsed Diff Row Emphasis",
		Keywords: []string{"preferences", "settings", "git", "diff", "collapsed", "omitted", "visibility"},
		Handler:  app.ToggleDiffCollapsedEmphasis,
	})

	reg.Register(command.Command{
		ID: "options.useGitFileTree", Title: "View Git Files as Tree",
		Keywords: []string{"preferences", "settings", "git", "changes", "history", "files", "tree"},
		Handler:  app.UseTreeGitFileView,
	})

	reg.Register(command.Command{
		ID: "options.useGitFileList", Title: "View Git Files as List",
		Keywords: []string{"preferences", "settings", "git", "changes", "history", "files", "flat", "list"},
		Handler:  app.UseListGitFileView,
	})

	reg.Register(command.Command{
		ID: "options.toggleAutoIndent", Title: "Toggle Auto Indent",
		Keywords: []string{"preferences", "settings", "editor", "indentation", "indent"},
		Handler:  app.ToggleAutoIndent,
	})

	reg.Register(command.Command{
		ID: "options.toggleAutoDedent", Title: "Toggle Auto Dedent",
		Keywords: []string{"preferences", "settings", "editor", "indentation", "dedent", "bracket"},
		Handler:  app.ToggleAutoDedent,
	})

	reg.Register(command.Command{
		ID: "options.toggleShowTrailingNewline", Title: "Toggle Show Trailing Newline",
		Keywords: []string{"preferences", "settings", "editor", "newline", "trailing", "phantom"},
		Handler:  app.ToggleShowTrailingNewline,
	})

	reg.Register(command.Command{
		ID: "options.toggleBracketColors", Title: "Toggle Bracket Pair Colorization",
		Handler: app.ToggleBracketPairColorization,
	})

	reg.Register(command.Command{
		ID: "options.toggleTransparentBackground", Title: "Toggle Transparent Background",
		Keywords: []string{"preferences", "settings", "editor", "view", "background", "transparent", "terminal"},
		Handler:  app.ToggleTransparentBackground,
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
}
