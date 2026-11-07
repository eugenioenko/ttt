package app

import (
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
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

func (a *App) ToggleDiffSigns() {
	a.Settings.Editor.DiffSigns = !a.Settings.Editor.DiffSigns
	a.SaveAndApplySettings()
}

func (a *App) ToggleDiffSignsColor() {
	a.Settings.Editor.DiffSignsColor = !a.Settings.Editor.DiffSignsColor
	a.SaveAndApplySettings()
}

func (a *App) ToggleDiffGutterHighlight() {
	a.Settings.Editor.DiffGutterHighlight = !a.Settings.Editor.DiffGutterHighlight
	a.SaveAndApplySettings()
}

func (a *App) ToggleDiffColorGutterNumbers() {
	a.Settings.Editor.DiffColorGutterNumbers = !a.Settings.Editor.DiffColorGutterNumbers
	a.SaveAndApplySettings()
}

func (a *App) ToggleDiffCollapsedEmphasis() {
	a.Settings.Editor.DiffCollapsedEmphasis = !a.Settings.Editor.DiffCollapsedEmphasis
	a.SaveAndApplySettings()
}

func (a *App) ToggleDiffScrollbarGitMarks() {
	a.Settings.Editor.DiffScrollbarGitMarks = !a.Settings.Editor.DiffScrollbarGitMarks
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

func (a *App) ToggleFileIcons() {
	a.Settings.Appearance.Icons = toggledIconMode(a.Settings.Appearance.Icons)
	a.SaveAndApplySettings()
}

func toggledIconMode(mode string) string {
	if mode == config.IconsNone {
		return config.IconsNerdFont
	}
	return config.IconsNone
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
	a.StatusNotify(i18n.Text("status.restartToApplySyntaxHighlightChanges"))
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

func (a *App) ToggleLSPHover() {
	enabled := !a.Settings.LSP.IsHoverEnabled()
	a.Settings.LSP.Hover = &enabled
	if !enabled {
		a.DismissHover()
	}
	a.SaveAndApplySettings()
}

func (a *App) ToggleGitGutter() {
	enabled := !a.Settings.Editor.IsGitGutterEnabled()
	a.Settings.Editor.GitGutter = &enabled
	a.SaveAndApplySettings()
}

func (a *App) ToggleScrollbarGitMarks() {
	enabled := !a.Settings.Editor.IsScrollbarGitMarksEnabled()
	a.Settings.Editor.ScrollbarGitMarks = &enabled
	a.SaveAndApplySettings()
}

func (a *App) SetGutterStyle(style string) {
	a.Settings.Editor.GutterStyle = style
	a.SaveAndApplySettings()
}

// Display labels for the gutter and border style values in config.
func styleLabels() map[string]string {
	return map[string]string{
		"minimal":    i18n.Text("options.minimal"),
		"compact":    i18n.Text("options.compact"),
		"extended":   i18n.Text("options.extended"),
		"default":    i18n.Text("options.default"),
		"rounded":    i18n.Text("options.rounded"),
		"sharp":      i18n.Text("options.sharp"),
		"double":     i18n.Text("options.double"),
		"bold":       i18n.Text("options.bold"),
		"ascii":      i18n.Text("options.aSCII"),
		"none":       i18n.Text("options.none"),
		"background": i18n.Text("options.background"),
		"invert":     i18n.Text("options.invert"),
	}
}

func selectionStyleItems() []widgets.SelectItem {
	labels := styleLabels()
	items := make([]widgets.SelectItem, 0, len(config.SelectionStyles))
	for _, id := range config.SelectionStyles {
		items = append(items, widgets.SelectItem{ID: id, Label: labels[id]})
	}
	return items
}

func gutterStyleItems() []widgets.SelectItem {
	labels := styleLabels()
	items := make([]widgets.SelectItem, 0, len(config.GutterStyles))
	for _, id := range config.GutterStyles {
		items = append(items, widgets.SelectItem{ID: id, Label: labels[id]})
	}
	return items
}

func borderStyleItems() []widgets.SelectItem {
	labels := styleLabels()
	items := make([]widgets.SelectItem, 0, len(config.BorderStyles))
	for _, id := range config.BorderStyles {
		// "theme" is accepted in settings.json but behaves identically to
		// "default", so it is not offered as a separate choice.
		if id == "theme" {
			continue
		}
		items = append(items, widgets.SelectItem{ID: id, Label: labels[id]})
	}
	return items
}

func diffModeItems() []widgets.SelectItem {
	return []widgets.SelectItem{
		{ID: config.DiffModeSplit, Label: i18n.Text("menu.split")},
		{ID: config.DiffModeUnified, Label: i18n.Text("menu.unified")},
	}
}

func diffContextItems() []widgets.SelectItem {
	return []widgets.SelectItem{
		{ID: config.DiffContextChanges, Label: i18n.Text("menu.changesOnly")},
		{ID: config.DiffContextFull, Label: i18n.Text("menu.fullFile")},
	}
}

func (a *App) ShowGutterStylePicker() {
	a.ShowSelectDialog(i18n.Text("options.gutterStyle"), gutterStyleItems(), func(id string) {
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
	a.ShowSelectDialog(i18n.Text("options.borderStyle"), borderStyleItems(), func(id string) {
		a.SetBorderStyle(id)
	}, nil)
}

func (a *App) SetEditorSelectionStyle(style string) {
	a.Settings.Editor.SelectionStyle = style
	a.SaveAndApplySettings()
}

func (a *App) SetTerminalSelectionStyle(style string) {
	a.Settings.Terminal.SelectionStyle = style
	a.SaveAndApplySettings()
}

func (a *App) ShowEditorSelectionStylePicker() {
	if a.Root.HasOverlay() {
		return
	}
	a.ShowSelectDialog(i18n.Text("options.editorSelectionStyle"), selectionStyleItems(), func(id string) {
		a.SetEditorSelectionStyle(id)
	}, nil)
}

func (a *App) ShowTerminalSelectionStylePicker() {
	if a.Root.HasOverlay() {
		return
	}
	a.ShowSelectDialog(i18n.Text("options.terminalSelectionStyle"), selectionStyleItems(), func(id string) {
		a.SetTerminalSelectionStyle(id)
	}, nil)
}

func (a *App) ShowDiffViewModePicker() {
	a.ShowSelectDialog(i18n.Text("options.diffViewMode"), diffModeItems(), func(id string) {
		if id == config.DiffModeUnified {
			a.UseUnifiedDiffByDefault()
			return
		}
		a.UseSplitDiffByDefault()
	}, nil)
}

func (a *App) ShowDiffContextPicker() {
	a.ShowSelectDialog(i18n.Text("options.diffContext"), diffContextItems(), func(id string) {
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

	fontIconsChecked := menuChecked(a.Settings.Appearance.Icons == config.IconsNerdFont)

	items := []ui.ContextMenuItem{
		{Label: i18n.Text("menu.lineNumbers"), Command: "options.toggleLineNumbers", Checked: lineNumbersChecked},
		{Label: i18n.Text("menu.wordWrap"), Command: "options.toggleWordWrap", Checked: wordWrapChecked},
		{Label: i18n.Text("menu.autoIndent"), Command: "options.toggleAutoIndent", Checked: autoIndentChecked},
		{Label: i18n.Text("menu.autoDedent"), Command: "options.toggleAutoDedent", Checked: autoDedentChecked},
		{Label: i18n.Text("menu.syntaxHighlight"), Command: "options.toggleSyntaxHighlight", Checked: syntaxChecked},
		{Label: i18n.Text("menu.bracketColors"), Command: "options.toggleBracketColors", Checked: bracketColorChecked},
		{Label: i18n.Text("menu.lSPCodeAssist"), Command: "options.toggleLSP", Checked: lspChecked},
		{Label: i18n.Text("menu.lSPHover"), Command: "options.toggleLSPHover", Checked: menuChecked(a.Settings.LSP.IsHoverEnabled())},
		{Label: i18n.Text("menu.gitGutter"), Command: "options.toggleGitGutter", Checked: gitGutterChecked},
		{Label: i18n.Text("menu.scrollbarGitMarks"), Command: "options.toggleScrollbarGitMarks", Checked: menuChecked(a.Settings.Editor.IsScrollbarGitMarksEnabled())},
		{Label: i18n.Text("menu.fontIcons"), Command: "options.toggleFontIcons", Checked: fontIconsChecked},
		{Label: i18n.Text("menu.menuBar"), Command: menuBarToggleCommand, Checked: menuBarChecked},
		{Label: i18n.Text("menu.transparentBG"), Command: "options.toggleTransparentBackground", Checked: transparentBgChecked},
		ui.MenuSep(),
		{Label: i18n.Text("menu.diffViews"), Submenu: a.BuildDiffViewOptions()},
		{Label: i18n.Text("menu.gitFiles"), Submenu: a.BuildGitFileOptions()},
		ui.MenuSep(),
		{Label: i18n.Text("menu.gutterStyle"), Command: "options.gutterStyle"},
		{Label: i18n.Text("menu.borderStyle"), Command: "options.borderStyle"},
		{Label: i18n.Text("menu.indentation"), Command: "options.indentation"},
		ui.MenuSep(),
		{Label: i18n.Text("general.settings"), Command: "settings.openUI"},
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
		{Label: i18n.Text("menu.split"), Command: "options.useSplitDiff", Checked: menuChecked(a.Settings.Editor.DiffMode != config.DiffModeUnified)},
		{Label: i18n.Text("menu.unified"), Command: "options.useUnifiedDiff", Checked: menuChecked(a.Settings.Editor.DiffMode == config.DiffModeUnified)},
		ui.MenuSep(),
		{Label: i18n.Text("menu.changesOnly"), Command: "options.useChangesOnlyDiff", Checked: menuChecked(a.Settings.Editor.DiffContext != config.DiffContextFull)},
		{Label: i18n.Text("menu.fullFile"), Command: "options.useFullFileDiff", Checked: menuChecked(a.Settings.Editor.DiffContext == config.DiffContextFull)},
		ui.MenuSep(),
		{Label: i18n.Text("menu.wrapLines"), Command: "options.toggleDiffWordWrap", Checked: menuChecked(a.Settings.Editor.DiffWordWrap)},
		{Label: i18n.Text("menu.highContrast"), Command: "options.toggleDiffHighContrast", Checked: menuChecked(a.Settings.Editor.DiffHighContrast)},
		{Label: i18n.Text("menu.gutterSigns"), Command: "options.toggleDiffSigns", Checked: menuChecked(a.Settings.Editor.DiffSigns)},
		{Label: i18n.Text("menu.coloredGutterSigns"), Command: "options.toggleDiffSignsColor", Checked: menuChecked(a.Settings.Editor.DiffSignsColor)},
		{Label: i18n.Text("menu.gutterHighlight"), Command: "options.toggleDiffGutterHighlight", Checked: menuChecked(a.Settings.Editor.DiffGutterHighlight)},
		{Label: i18n.Text("menu.coloredGutterNumbers"), Command: "options.toggleDiffColorGutterNumbers", Checked: menuChecked(a.Settings.Editor.DiffColorGutterNumbers)},
		{Label: i18n.Text("menu.emphasizeCollapsedRows"), Command: "options.toggleDiffCollapsedEmphasis", Checked: menuChecked(a.Settings.Editor.DiffCollapsedEmphasis)},
		{Label: i18n.Text("menu.scrollbarGitMarks"), Command: "options.toggleDiffScrollbarGitMarks", Checked: menuChecked(a.Settings.Editor.DiffScrollbarGitMarks)},
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
		{Label: i18n.Text("menu.tree"), Command: "options.useGitFileTree", Checked: menuChecked(a.Settings.Git.FileView == config.GitFileViewTree)},
		{Label: i18n.Text("menu.list"), Command: "options.useGitFileList", Checked: menuChecked(a.Settings.Git.FileView != config.GitFileViewTree)},
		{Label: i18n.Text("menu.fileIcons"), Command: "options.toggleFontIcons", Checked: menuChecked(a.Settings.Appearance.Icons == config.IconsNerdFont)},
		ui.MenuSep(),
		{Label: i18n.Text("general.expandAll"), Command: expandCommand},
		{Label: i18n.Text("general.collapseAll"), Command: collapseCommand},
	}
}

func registerOptionsCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "options.toggleSyntaxHighlight", Title: i18n.Text("command.toggleSyntaxHighlight"),
		Keywords: []string{"preferences", "settings", "editor", "view", "performance"},
		Handler:  app.ToggleSyntaxHighlight,
	})

	reg.Register(command.Command{
		ID: "options.toggleLineNumbers", Title: i18n.Text("command.toggleLineNumbers"),
		Keywords: []string{"preferences", "settings", "editor", "view"},
		Handler:  app.ToggleLineNumbers,
	})

	reg.Register(command.Command{
		ID: "options.toggleWordWrap", Title: i18n.Text("command.toggleWordWrap"),
		Keywords: []string{"preferences", "settings", "editor", "view"},
		Handler:  app.ToggleWordWrap,
	})

	reg.Register(command.Command{
		ID: "options.diffViewMode", Title: i18n.Text("command.changeDiffViewMode"),
		Keywords: []string{"preferences", "settings", "git", "diff", "split", "unified", "mode"},
		Handler:  app.ShowDiffViewModePicker,
	})

	reg.Register(command.Command{
		ID: "options.diffContext", Title: i18n.Text("command.changeDiffContext"),
		Keywords: []string{"preferences", "settings", "git", "diff", "changes", "full", "context"},
		Handler:  app.ShowDiffContextPicker,
	})

	reg.Register(command.Command{
		ID: "options.useSplitDiff", Title: i18n.Text("command.useSplitDiffByDefault"),
		Keywords: []string{"preferences", "settings", "git", "diff", "split", "default"},
		Handler:  app.UseSplitDiffByDefault,
	})

	reg.Register(command.Command{
		ID: "options.useUnifiedDiff", Title: i18n.Text("command.useUnifiedDiffByDefault"),
		Keywords: []string{"preferences", "settings", "git", "diff", "unified", "default"},
		Handler:  app.UseUnifiedDiffByDefault,
	})

	reg.Register(command.Command{
		ID: "options.useChangesOnlyDiff", Title: i18n.Text("command.showChangesOnlyByDefault"),
		Keywords: []string{"preferences", "settings", "git", "diff", "compact", "context", "default"},
		Handler:  app.UseChangesOnlyDiffByDefault,
	})

	reg.Register(command.Command{
		ID: "options.useFullFileDiff", Title: i18n.Text("command.showFullFileDiffByDefault"),
		Keywords: []string{"preferences", "settings", "git", "diff", "extended", "context", "default"},
		Handler:  app.UseFullFileDiffByDefault,
	})

	reg.Register(command.Command{
		ID: "options.toggleDiffWordWrap", Title: i18n.Text("command.toggleDiffWordWrapDefault"),
		Keywords: []string{"preferences", "settings", "git", "diff", "wrap", "default"},
		Handler:  app.ToggleDiffWordWrapDefault,
	})

	reg.Register(command.Command{
		ID: "options.toggleDiffHighContrast", Title: i18n.Text("command.toggleHighContrastDiffs"),
		Keywords: []string{"preferences", "settings", "git", "diff", "contrast", "color", "accessibility"},
		Handler:  app.ToggleDiffHighContrast,
	})

	reg.Register(command.Command{
		ID: "options.toggleDiffSigns", Title: i18n.Text("command.toggleDiffGutterSigns"),
		Keywords: []string{"preferences", "settings", "git", "diff", "gutter", "plus", "minus", "signs"},
		Handler:  app.ToggleDiffSigns,
	})

	reg.Register(command.Command{
		ID: "options.toggleDiffSignsColor", Title: i18n.Text("command.toggleColoredDiffGutterSigns"),
		Keywords: []string{"preferences", "settings", "git", "diff", "gutter", "plus", "minus", "signs", "color"},
		Handler:  app.ToggleDiffSignsColor,
	})

	reg.Register(command.Command{
		ID: "options.toggleDiffGutterHighlight", Title: i18n.Text("command.toggleDiffGutterHighlight"),
		Keywords: []string{"preferences", "settings", "git", "diff", "gutter", "line numbers", "highlight", "color"},
		Handler:  app.ToggleDiffGutterHighlight,
	})

	reg.Register(command.Command{
		ID: "options.toggleDiffColorGutterNumbers", Title: i18n.Text("command.toggleColoredDiffGutterNumbers"),
		Keywords: []string{"preferences", "settings", "git", "diff", "gutter", "line numbers", "color"},
		Handler:  app.ToggleDiffColorGutterNumbers,
	})

	reg.Register(command.Command{
		ID: "options.toggleDiffCollapsedEmphasis", Title: i18n.Text("command.toggleCollapsedDiffRowEmphasis"),
		Keywords: []string{"preferences", "settings", "git", "diff", "collapsed", "omitted", "visibility"},
		Handler:  app.ToggleDiffCollapsedEmphasis,
	})

	reg.Register(command.Command{
		ID: "options.toggleDiffScrollbarGitMarks", Title: i18n.Text("command.toggleDiffScrollbarGitMarks"),
		Keywords: []string{"preferences", "settings", "git", "diff", "scrollbar", "marks", "changes"},
		Handler:  app.ToggleDiffScrollbarGitMarks,
	})

	reg.Register(command.Command{
		ID: "options.useGitFileTree", Title: i18n.Text("command.viewGitFilesAsTree"),
		Keywords: []string{"preferences", "settings", "git", "changes", "history", "files", "tree"},
		Handler:  app.UseTreeGitFileView,
	})

	reg.Register(command.Command{
		ID: "options.useGitFileList", Title: i18n.Text("command.viewGitFilesAsList"),
		Keywords: []string{"preferences", "settings", "git", "changes", "history", "files", "flat", "list"},
		Handler:  app.UseListGitFileView,
	})

	reg.Register(command.Command{
		ID: "options.toggleFontIcons", Title: i18n.Text("command.toggleFontIcons"),
		Keywords: []string{"preferences", "settings", "explorer", "git", "files", "icons", "nerd font"},
		Handler:  app.ToggleFileIcons,
	})

	reg.Register(command.Command{
		ID: "options.toggleAutoIndent", Title: i18n.Text("command.toggleAutoIndent"),
		Keywords: []string{"preferences", "settings", "editor", "indentation", "indent"},
		Handler:  app.ToggleAutoIndent,
	})

	reg.Register(command.Command{
		ID: "options.toggleAutoDedent", Title: i18n.Text("command.toggleAutoDedent"),
		Keywords: []string{"preferences", "settings", "editor", "indentation", "dedent", "bracket"},
		Handler:  app.ToggleAutoDedent,
	})

	reg.Register(command.Command{
		ID: "options.toggleShowTrailingNewline", Title: i18n.Text("command.toggleShowTrailingNewline"),
		Keywords: []string{"preferences", "settings", "editor", "newline", "trailing", "phantom"},
		Handler:  app.ToggleShowTrailingNewline,
	})

	reg.Register(command.Command{
		ID: "options.toggleBracketColors", Title: i18n.Text("command.toggleBracketPairColorization"),
		Handler: app.ToggleBracketPairColorization,
	})

	reg.Register(command.Command{
		ID: "options.toggleTransparentBackground", Title: i18n.Text("command.toggleTransparentBackground"),
		Keywords: []string{"preferences", "settings", "editor", "view", "background", "transparent", "terminal"},
		Handler:  app.ToggleTransparentBackground,
	})

	reg.Register(command.Command{
		ID: "options.toggleLSP", Title: i18n.Text("command.toggleLSP"),
		Keywords: []string{"preferences", "settings", "language", "server", "autocomplete"},
		Handler:  app.ToggleLSP,
	})

	reg.Register(command.Command{
		ID: "options.toggleLSPHover", Title: i18n.Text("command.toggleLSPHover"),
		Keywords: []string{"preferences", "settings", "language", "server", "hover", "mouse", "tooltip"},
		Handler:  app.ToggleLSPHover,
	})

	reg.Register(command.Command{
		ID: "options.toggleGitGutter", Title: i18n.Text("command.toggleGitGutter"),
		Keywords: []string{"preferences", "settings", "editor", "view", "git"},
		Handler:  app.ToggleGitGutter,
	})

	reg.Register(command.Command{
		ID: "options.toggleScrollbarGitMarks", Title: i18n.Text("command.toggleScrollbarGitMarks"),
		Keywords: []string{"preferences", "settings", "editor", "view", "git", "scrollbar", "marks", "changes"},
		Handler:  app.ToggleScrollbarGitMarks,
	})

	reg.Register(command.Command{
		ID: "options.gutterStyle", Title: i18n.Text("command.changeGutterStyle"),
		Keywords: []string{"preferences", "settings", "editor", "view"},
		Handler:  app.ShowGutterStylePicker,
	})

	reg.Register(command.Command{
		ID: "options.borderStyle", Title: i18n.Text("command.changeBorderStyle"),
		Keywords: []string{"preferences", "settings", "editor", "view", "borders", "rounded", "sharp"},
		Handler:  app.ShowBorderStylePicker,
	})

	reg.Register(command.Command{
		ID: "options.editorSelectionStyle", Title: i18n.Text("command.changeEditorSelectionStyle"),
		Keywords: []string{"preferences", "settings", "editor", "view", "selection", "invert", "highlight"},
		Handler:  app.ShowEditorSelectionStylePicker,
	})

	reg.Register(command.Command{
		ID: "options.terminalSelectionStyle", Title: i18n.Text("command.changeTerminalSelectionStyle"),
		Keywords: []string{"preferences", "settings", "terminal", "view", "selection", "invert", "highlight"},
		Handler:  app.ShowTerminalSelectionStylePicker,
	})

	reg.Register(command.Command{
		ID: "options.indentation", Title: i18n.Text("command.editorIndentation"),
		Keywords: []string{"preferences", "settings", "editor", "indentation", "tabs", "spaces"},
		Handler:  app.ShowIndentSettings,
	})
}
