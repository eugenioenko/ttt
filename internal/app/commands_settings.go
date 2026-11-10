package app

import (
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
	"reflect"
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

func (a *App) settingsFilePath() string {
	if a.settingsFile != "" {
		return a.settingsFile
	}
	return config.ConfigFilePath("settings.json")
}

func (a *App) readSettings() (config.Settings, error) {
	if a.settingsFile != "" {
		return config.ReadSettingsFile(a.settingsFilePath())
	}
	return config.ReadSettings()
}

func (a *App) saveSettings(s config.Settings) error {
	return config.SaveSettingsFile(a.settingsFilePath(), s)
}

func (a *App) OpenSettingsFile() {
	path := a.settingsFilePath()
	config.EnsureConfigFile(path, "{}\n")
	a.EditorGroup.OpenFile(path)
}

func (a *App) ReloadSettings() {
	s, err := a.readSettings()
	if err != nil {
		a.StatusWarn(i18n.Format("settings.loadFailed", i18n.Args{"error": err.Error()}))
		return
	}
	a.ApplySettings(s)
	a.StatusNotify(i18n.Text("status.settingsReloaded"))
}

func (a *App) applySidebarTabStyle(style string) {
	if a.Sidebar == nil {
		return
	}
	a.Sidebar.SetTabStyle(style)
	a.Sidebar.SetPanelIcon("explorer", string(icons.Rune(icons.PanelExplorer)))
	a.Sidebar.SetPanelIcon("search", string(icons.Rune(icons.PanelSearch)))
	a.Sidebar.SetPanelIcon("changes", string(icons.Rune(icons.PanelChanges)))
	a.Sidebar.SetPanelIcon("outline", string(icons.Rune(icons.PanelOutline)))
	a.Sidebar.SetPanelIcon("plugins", string(icons.Rune(icons.PanelPlugins)))
	a.Sidebar.SetPanelIcon(appsPanelID, string(icons.Rune(icons.PanelApps)))
	a.ApplyPluginPanelIcons()
}

func (a *App) applyIcons(ap config.AppearanceSettings) {
	for _, warning := range icons.Configure(ap.Icons, ap.Overrides) {
		a.LogOutput("warn", "settings", warning)
	}
	if a.Apps != nil {
		a.Apps.mode = ap.Icons
		a.Apps.Filter(a.Apps.SearchInput.Text())
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
		for _, group := range a.editorGroups() {
			group.SetChevrons(collapsed, expanded)
		}
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
	if s.Language.Locale != a.language.requested {
		a.language.requested = s.Language.Locale
		if locale, _ := i18n.NormalizeLocale(s.Language.Locale); locale != a.language.locale {
			a.StatusNotify(i18n.Text("status.languageRestartRequired"))
		}
	}
	if !s.Language.IsAutoUpdateEnabled() && a.language.updates != nil {
		a.language.updates.Stop()
	}
	*a.Settings = s
	a.Root.SetControlModifiers(controlModifiers(s.Keyboard.ModifierKey))
	if prev.Terminal.ForwardShortcuts != s.Terminal.ForwardShortcuts {
		a.Root.SetForwardShortcuts(s.Terminal.ForwardShortcuts)
	}
	if a.Reg != nil {
		for _, kb := range a.Keybindings {
			if len(kb.Steps) == 0 {
				continue
			}
			a.Reg.SetShortcut(kb.Command, FormatKeyBinding(kb.Key, a.Root.ControlModifiers))
		}
		if a.PluginManager != nil {
			for _, p := range a.PluginManager.Plugins() {
				for _, kb := range p.PluginKeybindings {
					if _, err := config.ParseKeyString(kb.Key); err != nil {
						continue
					}
					a.Reg.SetShortcut(kb.Command, FormatKeyBinding(kb.Key, a.Root.ControlModifiers))
				}
			}
		}
	}
	a.refreshSettingsShortcuts()
	if !s.Welcome.IsRecentFoldersEnabled() {
		a.clearRecentFolders()
	}
	a.refreshWelcome()
	a.TerminalPanel.PaddingX = s.Terminal.PaddingX
	if a.Reg != nil && a.Sidebar != nil && !reflect.DeepEqual(prev.Applications, s.Applications) {
		a.syncApps()
	}
	for _, tab := range a.allTerminals() {
		tab.Widget.CtrlVPastes = s.Terminal.CtrlVPastes
		tab.Widget.SuperShortcuts = a.Root.ControlModifiers&tcell.ModMeta != 0
		if tab.Tab != nil {
			tab.Tab.PaddingX = s.Terminal.PaddingX
			if tab.Widget.PaddingX != nil {
				tab.Tab.PaddingX = max(*tab.Widget.PaddingX, 0)
			}
		}
	}

	highlight.SetTimeLimit(time.Duration(s.Editor.HighlightTimeLimit) * time.Millisecond)
	for _, group := range a.editorGroups() {
		applyEditorGroupSettings(group, s, prev)
	}
	if a.Sidebar != nil {
		a.Sidebar.SetPanelOrder(s.Sidebar.PanelOrder)
	}

	a.applyMenuBarVisibility(s.Editor.IsMenuBarVisible())

	// Apply cursor style
	if a.Screen != nil {
		a.Screen.SetCursorStyle(term.ParseCursorStyle(s.Editor.CursorStyle))
		a.Screen.SetPointerShapesEnabled(s.Appearance.PointerShapesEnabled())
	}

	// Apply search debounce
	a.Search.Debounce.DelayMs = s.Search.Debounce
	if s.Search.Debounce != prev.Search.Debounce {
		a.setExplorerSearchDebounce(s.Search.Debounce)
	}
	a.Search.Icons = s.Appearance.Icons
	if s.LSP.IsHoverEnabled() != prev.LSP.IsHoverEnabled() || s.LSP.IsEnabled() != prev.LSP.IsEnabled() {
		a.DismissHover()
	}

	if s.Editor.IsGitGutterEnabled() != prev.Editor.IsGitGutterEnabled() {
		if !s.Editor.IsGitGutterEnabled() && a.EditorGroup.Editor != nil {
			for _, group := range a.editorGroups() {
				group.Editor.LineChanges = nil
			}
		}
		a.RequestGitGutterForActiveFile()
	}

	a.applyIcons(s.Appearance)
	a.applySidebarTabStyle(s.Sidebar.TabStyle)
	a.applyGrammarSettings(s.Editor.Grammars)
	for _, group := range a.editorGroups() {
		group.ApplyFileWordWrap()
	}

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
			if a.Renderer != nil {
				a.Renderer.Clear()
			}
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
	reg.Register(command.Command{ID: "session.clear", Title: i18n.Text("session.clearCommand"), Handler: app.ClearEditorSessions})

	reg.Register(command.Command{
		ID: "settings.reload", Title: i18n.Text("command.reloadSettings"),
		Handler: app.ReloadSettings,
	})
}

func applyEditorGroupSettings(group *ui.EditorGroupWidget, s, prev config.Settings) {
	group.TabSize = s.Editor.TabSize
	group.SetFileIconMode(s.Appearance.Icons)
	group.InsertSpaces = s.Editor.InsertSpaces
	group.LineNumbers = s.Editor.LineNumbers
	group.SetIndentGuides(s.Editor.IndentGuides)
	group.GutterStyle = s.Editor.GutterStyle
	group.InsertFinalNewline = s.Editor.InsertFinalNewline
	group.ShowTrailingNewline = s.Editor.IsShowTrailingNewlineEnabled()
	group.TrimTrailingWhitespace = s.Editor.TrimTrailingWhitespace
	group.WordWrap = s.Editor.WordWrap
	group.WordWrapColumn = s.Editor.WordWrapColumn
	group.WordWrapBreak = s.Editor.WordWrapBreak
	group.LanguageOverrides = s.Editor.LanguageOverrides
	group.WordWrapMaxBytes = s.Editor.WordWrapMaxBytes()
	group.SetDiffDefaults(configuredDiffMode(s.Editor.DiffMode), configuredDiffContext(s.Editor.DiffContext), s.Editor.DiffWordWrap)
	group.SetMarkdownMaxWidth(s.Markdown.WrapWidth)
	group.SetDiffHighContrast(s.Editor.DiffHighContrast)
	group.SetDiffCollapsedEmphasis(s.Editor.DiffCollapsedEmphasis)
	group.SetDiffSigns(s.Editor.DiffSigns)
	group.SetDiffSignsColor(s.Editor.DiffSignsColor)
	group.SetDiffGutterHighlight(s.Editor.DiffGutterHighlight)
	group.SetDiffColorGutterNumbers(s.Editor.DiffColorGutterNumbers)
	group.SetDiffScrollMarks(s.Editor.DiffScrollbarGitMarks, s.Appearance.ScrollbarGlyphs == config.ScrollbarGlyphsLegacy)
	group.BracketPairColorization = s.Editor.BracketPairColorization
	group.UndoDeleteCursorStart = s.Editor.UndoDeleteCursorStart
	group.ApplyUndoDeleteCursorStart(s.Editor.UndoDeleteCursorStart)
	group.SetImageProtocol(s.Image.Protocol)

	if group.Editor != nil {
		group.Editor.TabSize = s.Editor.TabSize
		group.Editor.LineNumbers = s.Editor.LineNumbers
		group.Editor.IndentGuides = s.Editor.IndentGuides
		group.Editor.GutterStyle = s.Editor.GutterStyle
		group.Editor.LegacyScrollbarGlyphs = s.Appearance.ScrollbarGlyphs == config.ScrollbarGlyphsLegacy
		group.Editor.ScrollbarGitMarks = s.Editor.IsScrollbarGitMarksEnabled()
		group.Editor.AutoDedent = s.Editor.IsAutoDedentEnabled()
		group.Editor.AutoIndent = s.Editor.IsAutoIndentEnabled()
		group.Editor.AutoClosing = s.Editor.AutoClosing
		group.Editor.BracketPairColorization = s.Editor.BracketPairColorization
		if s.Editor.BracketPairColorization != prev.Editor.BracketPairColorization {
			group.Editor.InvalidateBracketColors()
		}
	}
}
