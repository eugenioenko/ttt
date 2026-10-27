package app

import (
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"time"
)

func configuredDiffMode(mode string) ui.DiffMode {
	if mode == config.DiffModeUnified {
		return ui.DiffModeUnified
	}
	return ui.DiffModeSplit
}

func configuredDiffContext(contextMode string) ui.DiffContextMode {
	if contextMode == config.DiffContextFull {
		return ui.DiffContextFullFile
	}
	return ui.DiffContextChangesOnly
}

func (a *App) ReloadSettings() {
	s := config.LoadSettings()
	a.ApplySettings(s)
	a.StatusNotify("Settings reloaded")
}

func (a *App) applyIcons(ap config.AppearanceSettings) {
	for _, warning := range icons.Configure(ap.Icons, ap.Overrides) {
		a.LogOutput("warn", "settings", warning)
	}
	collapsed, expanded := icons.Rune(icons.ChevronRight), icons.Rune(icons.ChevronDown)
	var trees []*widgets.TreeWidget
	if a.Explorer != nil {
		trees = append(trees, a.Explorer.Tree)
	}
	if a.Changes != nil {
		trees = append(trees, a.Changes.Tree, a.Changes.CommitLog)
	}
	if a.Symbols != nil {
		trees = append(trees, a.Symbols.Tree)
	}
	for _, tree := range trees {
		if tree != nil {
			tree.Config.ChevronCollapsed = collapsed
			tree.Config.ChevronExpanded = expanded
		}
	}
	if a.Search != nil {
		a.Search.ChevronCollapsed = collapsed
		a.Search.ChevronExpanded = expanded
	}
	if a.EditorGroup != nil {
		a.EditorGroup.SetChevrons(collapsed, expanded)
	}
}

// ApplySettings is the single live-apply path: anything that can take effect
// without a restart belongs here, so every caller produces identical results.
func (a *App) ApplySettings(s config.Settings) {
	// Side effects below are keyed off what actually changed: ApplySettings runs
	// on every option toggle, and refetching the git gutter or rebuilding bracket
	// colors each time would be wasted work. The baseline is the last applied
	// value, not *a.Settings — callers such as the Options toggles mutate that
	// before calling in, so reading it here would always compare s against itself.
	prev := a.appliedSettings
	a.appliedSettings = s
	*a.Settings = s
	if !s.Welcome.IsRecentFoldersEnabled() {
		a.clearRecentFolders()
	}
	a.refreshWelcome()
	for _, tab := range a.Terminals {
		tab.Widget.CtrlVPastes = s.Terminal.CtrlVPastes
	}

	highlight.SetTimeLimit(time.Duration(s.Editor.HighlightTimeLimit) * time.Millisecond)
	// Apply editor settings to the editor group and active editor
	a.EditorGroup.TabSize = s.Editor.TabSize
	a.EditorGroup.SetFileIconMode(s.Appearance.Icons)
	a.EditorGroup.InsertSpaces = s.Editor.InsertSpaces
	a.EditorGroup.LineNumbers = s.Editor.LineNumbers
	a.EditorGroup.GutterStyle = s.Editor.GutterStyle
	a.EditorGroup.InsertFinalNewline = s.Editor.InsertFinalNewline
	a.EditorGroup.ShowTrailingNewline = s.Editor.IsShowTrailingNewlineEnabled()
	a.EditorGroup.TrimTrailingWhitespace = s.Editor.TrimTrailingWhitespace
	a.EditorGroup.WordWrap = s.Editor.WordWrap
	a.EditorGroup.WordWrapColumn = s.Editor.WordWrapColumn
	a.EditorGroup.LanguageOverrides = s.Editor.LanguageOverrides
	a.EditorGroup.WordWrapMaxBytes = s.Editor.WordWrapMaxBytes()
	a.EditorGroup.SetDiffDefaults(configuredDiffMode(s.Editor.DiffMode), configuredDiffContext(s.Editor.DiffContext), s.Editor.DiffWordWrap)
	a.EditorGroup.SetMarkdownMaxWidth(s.Markdown.WrapWidth)
	a.EditorGroup.SetDiffHighContrast(s.Editor.DiffHighContrast)
	a.EditorGroup.SetDiffCollapsedEmphasis(s.Editor.DiffCollapsedEmphasis)
	a.EditorGroup.SetDiffSigns(s.Editor.DiffSigns)
	a.EditorGroup.SetDiffSignsColor(s.Editor.DiffSignsColor)
	a.EditorGroup.SetDiffGutterHighlight(s.Editor.DiffGutterHighlight)
	a.EditorGroup.SetDiffColorGutterNumbers(s.Editor.DiffColorGutterNumbers)
	a.EditorGroup.SetDiffScrollMarks(s.Editor.DiffScrollbarGitMarks, s.Appearance.ScrollbarGlyphs == config.ScrollbarGlyphsLegacy)
	a.EditorGroup.BracketPairColorization = s.Editor.BracketPairColorization
	a.EditorGroup.UndoDeleteCursorStart = s.Editor.UndoDeleteCursorStart
	a.EditorGroup.ApplyUndoDeleteCursorStart(s.Editor.UndoDeleteCursorStart)
	a.EditorGroup.SetImageProtocol(s.Image.Protocol)
	if a.Sidebar != nil {
		a.Sidebar.SetPanelOrder(s.Sidebar.PanelOrder)
	}

	if a.EditorGroup.Editor != nil {
		a.EditorGroup.Editor.TabSize = s.Editor.TabSize
		a.EditorGroup.Editor.LineNumbers = s.Editor.LineNumbers
		a.EditorGroup.Editor.GutterStyle = s.Editor.GutterStyle
		a.EditorGroup.Editor.LegacyScrollbarGlyphs = s.Appearance.ScrollbarGlyphs == config.ScrollbarGlyphsLegacy
		a.EditorGroup.Editor.ScrollbarGitMarks = s.Editor.IsScrollbarGitMarksEnabled()
		a.EditorGroup.Editor.AutoDedent = s.Editor.IsAutoDedentEnabled()
		a.EditorGroup.Editor.AutoIndent = s.Editor.IsAutoIndentEnabled()
		a.EditorGroup.Editor.BracketPairColorization = s.Editor.BracketPairColorization
		if s.Editor.BracketPairColorization != prev.Editor.BracketPairColorization {
			a.EditorGroup.Editor.InvalidateBracketColors()
		}
	}

	a.applyMenuBarVisibility(s.Editor.IsMenuBarVisible())

	// Apply cursor style
	if a.Screen != nil {
		a.Screen.SetCursorStyle(term.ParseCursorStyle(s.Editor.CursorStyle))
	}

	// Apply search debounce
	a.Search.Debounce.DelayMs = s.Search.Debounce
	a.Search.Icons = s.Appearance.Icons
	if s.LSP.IsHoverEnabled() != prev.LSP.IsHoverEnabled() || s.LSP.IsEnabled() != prev.LSP.IsEnabled() {
		a.DismissHover()
	}

	if s.Editor.IsGitGutterEnabled() != prev.Editor.IsGitGutterEnabled() {
		if !s.Editor.IsGitGutterEnabled() && a.EditorGroup.Editor != nil {
			a.EditorGroup.Editor.LineChanges = nil
		}
		a.RequestGitGutterForActiveFile()
	}

	a.applyIcons(s.Appearance)
	a.applyGrammarSettings(s.Editor.Grammars)
	a.EditorGroup.ApplyFileWordWrap()

	if a.Explorer != nil && (a.Explorer.Settings != s.Explorer || a.Explorer.Icons != s.Appearance.Icons) {
		a.Explorer.Settings = s.Explorer
		a.Explorer.Icons = s.Appearance.Icons
		a.Explorer.Reload()
	}
	if a.Changes != nil {
		a.Changes.SetFileView(s.Git.FileView)
		a.Changes.SetIcons(s.Appearance.Icons)
	}
	if a.Symbols != nil {
		a.Symbols.SetIcons(s.Appearance.Icons)
	}

	// An empty theme name means the built-in default, and must still be applied —
	// otherwise switching back to it leaves the previous theme's colors on screen.
	var themeBorders *term.BorderSet
	if a.Screen != nil {
		theme, ok := config.DefaultTheme(), s.Theme == ""
		if !ok {
			loaded, err := config.LoadTheme(s.Theme)
			theme, ok = loaded, err == nil
		}
		if ok {
			ApplyThemeStyles(a.Screen, theme, WithTransparentBackground(s.Editor.TransparentBackground), WithEditorSelectionStyle(s.Editor.SelectionStyle), WithPluginSelectionStyle(s.Plugins.SelectionStyle))
			*a.Palette = BuildTerminalPalette(theme, WithTransparentBackground(s.Editor.TransparentBackground), WithTerminalSelectionStyle(s.Terminal.SelectionStyle))
			borders := BuildBorderSet(theme.Borders)
			*a.Borders = borders
			themeBorders = &borders
			a.Renderer.Clear()
			a.invalidateImageLayer()
		}
	} else if a.Palette != nil {
		theme, ok := config.DefaultTheme(), s.Theme == ""
		if !ok {
			loaded, err := config.LoadTheme(s.Theme)
			theme, ok = loaded, err == nil
		}
		if ok {
			*a.Palette = BuildTerminalPalette(theme, WithTransparentBackground(s.Editor.TransparentBackground), WithTerminalSelectionStyle(s.Terminal.SelectionStyle))
		}
	}

	// Overrides what the theme resolved, so it must run last and unconditionally.
	// Passing the borders just built avoids reloading the theme from disk.
	a.applyBorderStyle(themeBorders)
}

func registerSettingsCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "settings.reload", Title: "Reload Settings",
		Handler: app.ReloadSettings,
	})
}
