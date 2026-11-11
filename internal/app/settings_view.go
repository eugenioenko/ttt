package app

import (
	"maps"
	"strconv"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

const (
	settingsTabID       = "settings"
	settingsLabelCols   = 30
	settingsControlCols = 34
)

type settingKind int

const (
	settingBool settingKind = iota
	settingInt
	settingString
	settingEnum
	// settingSection is a heading row that groups the fields below it.
	settingSection
	settingShortcut
)

type settingField struct {
	Label   string
	Kind    settingKind
	Restart bool
	Command string
	Options func(*config.Settings) []widgets.SelectItem
	// Min is the smallest accepted value for settingInt. Fields whose json tag
	// carries omitempty must set Min >= 1, since a stored 0 would be dropped on
	// save and silently revert to the default on the next load.
	Min int

	GetBool func(*config.Settings) bool
	SetBool func(*config.Settings, bool)

	GetString func(*config.Settings) string
	SetString func(*config.Settings, string)

	GetInt func(*config.Settings) int
	SetInt func(*config.Settings, int)
}

type settingsCategory struct {
	Title  string
	Fields []settingField
}

func boolPtr(b bool) *bool { return &b }

func fixedOptions(items func() []widgets.SelectItem) func(*config.Settings) []widgets.SelectItem {
	return func(*config.Settings) []widgets.SelectItem { return items() }
}

func section(title string) settingField { return settingField{Label: title, Kind: settingSection} }

// LSP servers and the formatters map are deliberately absent: both are
// structured config that a form handles badly, and stay JSON-only.
func settingsCategories() []settingsCategory {
	return []settingsCategory{
		{Title: i18n.Text("settings.general"), Fields: []settingField{
			section(i18n.Text("settings.startup")),
			{Label: i18n.Text("settings.displayLanguage"), Kind: settingEnum, Restart: true, Options: languageItems,
				GetString: func(s *config.Settings) string {
					if s.Language.Locale == "" {
						return "en"
					}
					return s.Language.Locale
				},
				SetString: func(s *config.Settings, v string) { s.Language.Locale = v }},
			{Label: i18n.Text("settings.autoUpdateLanguagePacks"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Language.IsAutoUpdateEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Language.AutoUpdate = boolPtr(v) }},
			{Label: i18n.Text("settings.reuseRunningEditor"), Kind: settingBool, Restart: true,
				GetBool: func(s *config.Settings) bool { return s.ReuseWindow },
				SetBool: func(s *config.Settings, v bool) { s.ReuseWindow = v }},
			{Label: i18n.Text("settings.welcomePageInHomeFolder"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Welcome.ShowOnHome },
				SetBool: func(s *config.Settings, v bool) { s.Welcome.ShowOnHome = v }},
			{Label: i18n.Text("settings.recentFolderHistory"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Welcome.IsRecentFoldersEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Welcome.RecentFolders = boolPtr(v) }},
			section(i18n.Text("settings.keyboard")),
			{Label: i18n.Text("settings.shortcutModifier"), Kind: settingEnum, Options: fixedOptions(keyboardModifierItems),
				GetString: func(s *config.Settings) string { return s.Keyboard.ModifierKey },
				SetString: func(s *config.Settings, v string) { s.Keyboard.ModifierKey = v }},
			{Label: i18n.Text("settings.forwardTerminalShortcuts"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Terminal.ForwardShortcuts },
				SetBool: func(s *config.Settings, v bool) { s.Terminal.ForwardShortcuts = v }},
			{Label: i18n.Text("settings.sendNextTerminalKey"), Kind: settingShortcut, Command: "terminal.sendNextKey"},
			section(i18n.Text("settings.plugins")),
			{Label: i18n.Text("settings.enablePlugins"), Kind: settingBool, Restart: true,
				GetBool: func(s *config.Settings) bool { return s.Plugins.IsEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Plugins.Enabled = boolPtr(v) }},
			{Label: i18n.Text("settings.selectionStyle"), Kind: settingEnum, Options: fixedOptions(selectionStyleItems),
				GetString: func(s *config.Settings) string { return s.Plugins.SelectionStyle },
				SetString: func(s *config.Settings, v string) { s.Plugins.SelectionStyle = v }},
			section(i18n.Text("settings.debugging")),
			{Label: i18n.Text("settings.debugMode"), Kind: settingBool, Restart: true,
				GetBool: func(s *config.Settings) bool { return s.DebugMode },
				SetBool: func(s *config.Settings, v bool) { s.DebugMode = v }},
		}},
		{Title: i18n.Text("settings.editor"), Fields: []settingField{
			section(i18n.Text("settings.indentation")),
			{Label: i18n.Text("settings.tabSize"), Kind: settingInt, Min: 1,
				GetInt: func(s *config.Settings) int { return s.Editor.TabSize },
				SetInt: func(s *config.Settings, v int) { s.Editor.TabSize = v }},
			{Label: i18n.Text("settings.insertSpaces"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.InsertSpaces },
				SetBool: func(s *config.Settings, v bool) { s.Editor.InsertSpaces = v }},
			{Label: i18n.Text("settings.autoIndent"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.IsAutoIndentEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Editor.AutoIndent = boolPtr(v) }},
			{Label: i18n.Text("settings.autoDedent"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.IsAutoDedentEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Editor.AutoDedent = boolPtr(v) }},
			{Label: i18n.Text("settings.previewTabs"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.PreviewTabsEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Editor.PreviewTabs = boolPtr(v) }},
			section(i18n.Text("settings.autoClosing")),
			{Label: i18n.Text("settings.autoCloseBrackets"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.AutoClosing.Brackets },
				SetBool: func(s *config.Settings, v bool) { s.Editor.AutoClosing.Brackets = v }},
			{Label: i18n.Text("settings.autoCloseQuotes"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.AutoClosing.Quotes },
				SetBool: func(s *config.Settings, v bool) { s.Editor.AutoClosing.Quotes = v }},
			{Label: i18n.Text("settings.skipGeneratedClosers"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.AutoClosing.SkipClosing },
				SetBool: func(s *config.Settings, v bool) { s.Editor.AutoClosing.SkipClosing = v }},
			{Label: i18n.Text("settings.deleteEmptyGeneratedPairs"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.AutoClosing.DeletePairs },
				SetBool: func(s *config.Settings, v bool) { s.Editor.AutoClosing.DeletePairs = v }},
			{Label: i18n.Text("settings.autoCloseInsideStrings"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.AutoClosing.InStrings },
				SetBool: func(s *config.Settings, v bool) { s.Editor.AutoClosing.InStrings = v }},
			{Label: i18n.Text("settings.autoCloseInPlainText"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.AutoClosing.PlainText },
				SetBool: func(s *config.Settings, v bool) { s.Editor.AutoClosing.PlainText = v }},
			section(i18n.Text("settings.display")),
			{Label: i18n.Text("settings.wordWrap"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.WordWrap },
				SetBool: func(s *config.Settings, v bool) { s.Editor.WordWrap = v }},
			{Label: i18n.Text("settings.wrapColumn0PaneWidth"), Kind: settingInt, Min: 0,
				GetInt: func(s *config.Settings) int { return s.Editor.WordWrapColumn },
				SetInt: func(s *config.Settings, v int) { s.Editor.WordWrapColumn = v }},
			{Label: i18n.Text("settings.wordWrapBreak"), Kind: settingEnum, Options: fixedOptions(wordWrapBreakItems),
				GetString: func(s *config.Settings) string { return s.Editor.WordWrapBreak },
				SetString: func(s *config.Settings, v string) { s.Editor.WordWrapBreak = v }},
			{Label: i18n.Text("settings.scrollLines"), Kind: settingInt, Min: 0,
				GetInt: func(s *config.Settings) int { return s.Editor.ScrollLines },
				SetInt: func(s *config.Settings, v int) { s.Editor.ScrollLines = v }},
			{Label: i18n.Text("settings.lineNumbers"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.LineNumbers },
				SetBool: func(s *config.Settings, v bool) { s.Editor.LineNumbers = v }},
			{Label: i18n.Text("settings.indentGuides"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.IndentGuides },
				SetBool: func(s *config.Settings, v bool) { s.Editor.IndentGuides = v }},
			{Label: i18n.Text("settings.showTrailingNewline"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.IsShowTrailingNewlineEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Editor.ShowTrailingNewline = boolPtr(v) }},
			section(i18n.Text("settings.onSave")),
			{Label: i18n.Text("settings.formatOnSave"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.FormatOnSave },
				SetBool: func(s *config.Settings, v bool) { s.Editor.FormatOnSave = v }},
			{Label: i18n.Text("settings.insertFinalNewline"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.InsertFinalNewline },
				SetBool: func(s *config.Settings, v bool) { s.Editor.InsertFinalNewline = v }},
			{Label: i18n.Text("settings.trimTrailingWhitespace"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.TrimTrailingWhitespace },
				SetBool: func(s *config.Settings, v bool) { s.Editor.TrimTrailingWhitespace = v }},
			section(i18n.Text("settings.onQuit")),
			{Label: i18n.Text("settings.showSaveAllQuitButton"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.SaveAllOnQuit },
				SetBool: func(s *config.Settings, v bool) { s.Editor.SaveAllOnQuit = v }},
			{Label: i18n.Text("settings.exceptWithUnnamedBuffers"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.SaveAllOnQuitExceptUnnamed },
				SetBool: func(s *config.Settings, v bool) { s.Editor.SaveAllOnQuitExceptUnnamed = v }},
			section(i18n.Text("settings.completion")),
			{Label: i18n.Text("settings.enableCompletion"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Autocomplete.Enabled },
				SetBool: func(s *config.Settings, v bool) { s.Autocomplete.Enabled = v }},
			{Label: i18n.Text("settings.suggestAsYouType"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Autocomplete.AutoSuggest },
				SetBool: func(s *config.Settings, v bool) { s.Autocomplete.AutoSuggest = v }},
			{Label: i18n.Text("settings.signatureHelp"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Autocomplete.SignatureHelp },
				SetBool: func(s *config.Settings, v bool) { s.Autocomplete.SignatureHelp = v }},
			{Label: i18n.Text("settings.debounceMs"), Kind: settingInt,
				GetInt: func(s *config.Settings) int { return s.Autocomplete.Debounce },
				SetInt: func(s *config.Settings, v int) { s.Autocomplete.Debounce = v }},
			section(i18n.Text("settings.behavior")),
			{Label: i18n.Text("settings.focusEditorOnOpen"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.FocusOnOpen },
				SetBool: func(s *config.Settings, v bool) { s.Editor.FocusOnOpen = v }},
		}},
		{Title: i18n.Text("settings.appearance"), Fields: []settingField{
			section(i18n.Text("settings.theme")),
			{Label: i18n.Text("settings.theme"), Kind: settingEnum, Options: fixedOptions(themeItems),
				GetString: func(s *config.Settings) string { return s.Theme },
				SetString: func(s *config.Settings, v string) { s.Theme = v }},
			{Label: i18n.Text("settings.transparentBackground"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.TransparentBackground },
				SetBool: func(s *config.Settings, v bool) { s.Editor.TransparentBackground = v }},
			section(i18n.Text("settings.window")),
			{Label: i18n.Text("settings.menuBar"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.IsMenuBarVisible() },
				SetBool: func(s *config.Settings, v bool) { s.Editor.MenuBar = boolPtr(v) }},
			{Label: i18n.Text("settings.pointerShapes"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Appearance.PointerShapesEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Appearance.PointerShapes = boolPtr(v) }},
			{Label: i18n.Text("settings.borderStyle"), Kind: settingEnum, Options: fixedOptions(borderStyleItems),
				GetString: func(s *config.Settings) string { return s.Editor.BorderStyle },
				SetString: func(s *config.Settings, v string) { s.Editor.BorderStyle = v }},
			section(i18n.Text("settings.code")),
			{Label: i18n.Text("settings.syntaxHighlight"), Kind: settingBool, Restart: true,
				GetBool: func(s *config.Settings) bool { return s.Editor.IsSyntaxHighlightEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Editor.SyntaxHighlight = boolPtr(v) }},
			{Label: i18n.Text("settings.bracketPairColors"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.BracketPairColorization },
				SetBool: func(s *config.Settings, v bool) { s.Editor.BracketPairColorization = v }},
			{Label: i18n.Text("settings.cursorStyle"), Kind: settingEnum, Options: fixedOptions(cursorStyleItems),
				GetString: func(s *config.Settings) string { return s.Editor.CursorStyle },
				SetString: func(s *config.Settings, v string) { s.Editor.CursorStyle = v }},
			{Label: i18n.Text("settings.selectionStyle"), Kind: settingEnum, Options: fixedOptions(selectionStyleItems),
				GetString: func(s *config.Settings) string { return s.Editor.SelectionStyle },
				SetString: func(s *config.Settings, v string) { s.Editor.SelectionStyle = v }},
			{Label: i18n.Text("settings.gutterStyle"), Kind: settingEnum, Options: fixedOptions(gutterStyleItems),
				GetString: func(s *config.Settings) string { return s.Editor.GutterStyle },
				SetString: func(s *config.Settings, v string) { s.Editor.GutterStyle = v }},
			{Label: i18n.Text("settings.gitGutter"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.IsGitGutterEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Editor.GitGutter = boolPtr(v) }},
			{Label: i18n.Text("settings.scrollbarGitMarks"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.IsScrollbarGitMarksEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Editor.ScrollbarGitMarks = boolPtr(v) }},
			{Label: i18n.Text("settings.scrollbarGlyphs"), Kind: settingEnum, Options: fixedOptions(scrollbarGlyphItems),
				GetString: func(s *config.Settings) string { return s.Appearance.ScrollbarGlyphs },
				SetString: func(s *config.Settings, v string) { s.Appearance.ScrollbarGlyphs = v }},
			section(i18n.Text("settings.markdown")),
			{Label: i18n.Text("settings.wrapWidth"), Kind: settingInt, Min: 1,
				GetInt: func(s *config.Settings) int { return s.Markdown.WrapWidth },
				SetInt: func(s *config.Settings, v int) { s.Markdown.WrapWidth = v }},
		}},
		{Title: i18n.Text("settings.explorer"), Fields: []settingField{
			section(i18n.Text("settings.files")),
			{Label: i18n.Text("settings.useTrash"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Explorer.UseTrash },
				SetBool: func(s *config.Settings, v bool) { s.Explorer.UseTrash = v }},
			{Label: i18n.Text("settings.hiddenFiles"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Explorer.ShowHidden },
				SetBool: func(s *config.Settings, v bool) { s.Explorer.ShowHidden = v }},
			{Label: i18n.Text("settings.gitIgnoredFiles"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Explorer.ShowGitIgnored },
				SetBool: func(s *config.Settings, v bool) { s.Explorer.ShowGitIgnored = v }},
			{Label: i18n.Text("settings.revealActiveFile"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Explorer.AutoReveal },
				SetBool: func(s *config.Settings, v bool) { s.Explorer.AutoReveal = v }},
			{Label: i18n.Text("settings.compactFolders"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Explorer.CompactFolders },
				SetBool: func(s *config.Settings, v bool) { s.Explorer.CompactFolders = v }},
			section(i18n.Text("settings.appearance")),
			{Label: i18n.Text("settings.gitStatusColors"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Explorer.GitStatusColors },
				SetBool: func(s *config.Settings, v bool) { s.Explorer.GitStatusColors = v }},
			{Label: i18n.Text("settings.dimStagedColors"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Explorer.DimStagedGitColors },
				SetBool: func(s *config.Settings, v bool) { s.Explorer.DimStagedGitColors = v }},
		}},
		{Title: i18n.Text("settings.sidebar"), Fields: []settingField{
			section(i18n.Text("settings.look")),
			{Label: i18n.Text("settings.tabStyle"), Kind: settingEnum, Options: fixedOptions(sidebarTabStyleItems),
				GetString: func(s *config.Settings) string { return s.Sidebar.TabStyle },
				SetString: func(s *config.Settings, v string) { s.Sidebar.TabStyle = v }},
			{Label: i18n.Text("settings.icons"), Kind: settingEnum, Options: fixedOptions(iconModeItems),
				GetString: func(s *config.Settings) string { return s.Appearance.Icons },
				SetString: func(s *config.Settings, v string) { s.Appearance.Icons = v }},
			{Label: i18n.Text("settings.chevronCollapsed"), Kind: settingString,
				GetString: func(s *config.Settings) string { return s.Appearance.Overrides["chevron-right"] },
				SetString: func(s *config.Settings, v string) { setIconOverride(s, "chevron-right", v) }},
			{Label: i18n.Text("settings.chevronExpanded"), Kind: settingString,
				GetString: func(s *config.Settings) string { return s.Appearance.Overrides["chevron-down"] },
				SetString: func(s *config.Settings, v string) { setIconOverride(s, "chevron-down", v) }},
			section(i18n.Text("settings.sourceControl")),
			{Label: i18n.Text("settings.fileView"), Kind: settingEnum, Options: fixedOptions(gitFileViewItems),
				GetString: func(s *config.Settings) string { return s.Git.FileView },
				SetString: func(s *config.Settings, v string) { s.Git.FileView = v }},
			section(i18n.Text("settings.quickOpen")),
			{Label: i18n.Text("settings.hiddenFiles"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.QuickOpen.ShowHidden },
				SetBool: func(s *config.Settings, v bool) { s.QuickOpen.ShowHidden = v }},
			{Label: i18n.Text("settings.gitIgnoredFiles"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.QuickOpen.ShowGitIgnored },
				SetBool: func(s *config.Settings, v bool) { s.QuickOpen.ShowGitIgnored = v }},
			section(i18n.Text("settings.search")),
			{Label: i18n.Text("settings.debounceMs"), Kind: settingInt,
				GetInt: func(s *config.Settings) int { return s.Search.Debounce },
				SetInt: func(s *config.Settings, v int) { s.Search.Debounce = v }},
		}},
		{Title: i18n.Text("settings.terminal"), Fields: []settingField{
			{Label: i18n.Text("settings.horizontalPadding"), Kind: settingInt,
				GetInt: func(s *config.Settings) int { return s.Terminal.PaddingX },
				SetInt: func(s *config.Settings, v int) { s.Terminal.PaddingX = v }},
			{Label: i18n.Text("settings.ctrlVPastes"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Terminal.CtrlVPastes },
				SetBool: func(s *config.Settings, v bool) { s.Terminal.CtrlVPastes = v }},
			{Label: i18n.Text("settings.shell"), Kind: settingString, Restart: true,
				GetString: func(s *config.Settings) string { return s.Terminal.Shell },
				SetString: func(s *config.Settings, v string) { s.Terminal.Shell = v }},
			{Label: i18n.Text("settings.shellIntegration"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Terminal.ShellIntegration },
				SetBool: func(s *config.Settings, v bool) { s.Terminal.ShellIntegration = v }},
			{Label: i18n.Text("settings.scrollback"), Kind: settingInt, Restart: true, Min: 1,
				GetInt: func(s *config.Settings) int { return s.Terminal.Scrollback },
				SetInt: func(s *config.Settings, v int) { s.Terminal.Scrollback = v }},
			{Label: i18n.Text("settings.selectionStyle"), Kind: settingEnum, Options: fixedOptions(selectionStyleItems),
				GetString: func(s *config.Settings) string { return s.Terminal.SelectionStyle },
				SetString: func(s *config.Settings, v string) { s.Terminal.SelectionStyle = v }},
		}},
		{Title: i18n.Text("settings.diff"), Fields: []settingField{
			{Label: i18n.Text("settings.diffMode"), Kind: settingEnum, Options: fixedOptions(diffModeItems),
				GetString: func(s *config.Settings) string { return s.Editor.DiffMode },
				SetString: func(s *config.Settings, v string) { s.Editor.DiffMode = v }},
			{Label: i18n.Text("settings.diffContext"), Kind: settingEnum, Options: fixedOptions(diffContextItems),
				GetString: func(s *config.Settings) string { return s.Editor.DiffContext },
				SetString: func(s *config.Settings, v string) { s.Editor.DiffContext = v }},
			{Label: i18n.Text("settings.diffWordWrap"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.DiffWordWrap },
				SetBool: func(s *config.Settings, v bool) { s.Editor.DiffWordWrap = v }},
			{Label: i18n.Text("settings.highContrastDiffs"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.DiffHighContrast },
				SetBool: func(s *config.Settings, v bool) { s.Editor.DiffHighContrast = v }},
			{Label: i18n.Text("settings.diffGutterSigns"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.DiffSigns },
				SetBool: func(s *config.Settings, v bool) { s.Editor.DiffSigns = v }},
			{Label: i18n.Text("settings.coloredDiffGutterSigns"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.DiffSignsColor },
				SetBool: func(s *config.Settings, v bool) { s.Editor.DiffSignsColor = v }},
			{Label: i18n.Text("settings.diffGutterHighlight"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.DiffGutterHighlight },
				SetBool: func(s *config.Settings, v bool) { s.Editor.DiffGutterHighlight = v }},
			{Label: i18n.Text("settings.coloredDiffGutterNumbers"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.DiffColorGutterNumbers },
				SetBool: func(s *config.Settings, v bool) { s.Editor.DiffColorGutterNumbers = v }},
			{Label: i18n.Text("settings.emphasizeCollapsedDiffRows"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.DiffCollapsedEmphasis },
				SetBool: func(s *config.Settings, v bool) { s.Editor.DiffCollapsedEmphasis = v }},
			{Label: i18n.Text("settings.diffScrollbarGitMarks"), Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.DiffScrollbarGitMarks },
				SetBool: func(s *config.Settings, v bool) { s.Editor.DiffScrollbarGitMarks = v }},
		}},
	}
}

func keyboardModifierItems() []widgets.SelectItem {
	return []widgets.SelectItem{
		{ID: config.ModifierControl, Label: i18n.Text("settings.modifierControl")},
		{ID: config.ModifierControlSuper, Label: i18n.Text("settings.modifierControlSuper")},
		{ID: config.ModifierSuper, Label: i18n.Text("settings.modifierSuper")},
	}
}

func gitFileViewItems() []widgets.SelectItem {
	return []widgets.SelectItem{
		{ID: config.GitFileViewTree, Label: i18n.Text("settings.tree")},
		{ID: config.GitFileViewList, Label: i18n.Text("settings.list")},
	}
}

func iconModeItems() []widgets.SelectItem {
	return []widgets.SelectItem{
		{ID: config.IconsNerdFont, Label: i18n.Text("settings.nerdFont")},
		{ID: config.IconsNone, Label: i18n.Text("settings.none")},
	}
}

func sidebarTabStyleItems() []widgets.SelectItem {
	return []widgets.SelectItem{
		{ID: config.SidebarTabStyleLabel, Label: i18n.Text("settings.label")},
		{ID: config.SidebarTabStyleLabelIcon, Label: i18n.Text("settings.labelIcon")},
		{ID: config.SidebarTabStyleIcon, Label: i18n.Text("settings.icon")},
	}
}

func scrollbarGlyphItems() []widgets.SelectItem {
	return []widgets.SelectItem{
		{ID: config.ScrollbarGlyphsBlocks, Label: i18n.Text("settings.blocks")},
		{ID: config.ScrollbarGlyphsLegacy, Label: i18n.Text("settings.legacyComputing")},
	}
}

func themeItems() []widgets.SelectItem {
	names := config.ListThemes()
	items := make([]widgets.SelectItem, 0, len(names)+1)
	items = append(items, widgets.SelectItem{ID: "", Label: i18n.Text("settings.default")})
	for _, n := range names {
		items = append(items, widgets.SelectItem{ID: n, Label: n})
	}
	return items
}

// Mirrors term.ParseCursorStyle. "" means unset and behaves as a blinking bar,
// so it is offered as "Default" rather than duplicated as "Bar".
func cursorStyleItems() []widgets.SelectItem {
	return []widgets.SelectItem{
		{ID: "", Label: i18n.Text("settings.default")},
		{ID: "bar", Label: i18n.Text("settings.barBlinking")},
		{ID: "steadyBar", Label: i18n.Text("settings.barSteady")},
		{ID: "block", Label: i18n.Text("settings.blockBlinking")},
		{ID: "steadyBlock", Label: i18n.Text("settings.blockSteady")},
		{ID: "underline", Label: i18n.Text("settings.underlineBlinking")},
		{ID: "steadyUnderline", Label: i18n.Text("settings.underlineSteady")},
	}
}

type settingsView struct {
	app        *App
	working    config.Settings
	baseline   config.Settings
	categories []settingsCategory
	adapter    *ui.WidgetAdapter
	status     *widgets.LabelWidget
	inputs     []func() string
	selects    []*widgets.SelectWidget
	shortcuts  []func()
	refresh    []func()
}

// Only changed form values override live settings; untouched fields may have
// changed through an Options toggle, theme selection, or settings reload.
func (v *settingsView) commitTo(s *config.Settings) {
	for _, cat := range v.categories {
		for _, f := range cat.Fields {
			switch f.Kind {
			case settingSection, settingShortcut:
			case settingBool:
				if value := f.GetBool(&v.working); value != f.GetBool(&v.baseline) {
					f.SetBool(s, value)
				}
			case settingInt:
				if value := f.GetInt(&v.working); value != f.GetInt(&v.baseline) {
					f.SetInt(s, value)
				}
			default:
				if value := f.GetString(&v.working); value != f.GetString(&v.baseline) {
					f.SetString(s, value)
				}
			}
		}
	}
}

func (v *settingsView) closeSelectsExcept(keep *widgets.SelectWidget) {
	for _, s := range v.selects {
		if s != keep {
			s.ClosePopup()
		}
	}
}

func (a *App) ShowSettings() {
	// Reopening while the tab is already open must not discard pending edits.
	if v := a.settingsView; v != nil {
		if !a.EditorGroup.SwitchToTabByPath(settingsTabID) {
			a.EditorGroup.OpenPluginTab(settingsTabID, i18n.Text("general.settings"), v.adapter)
		}
		a.FocusEditor()
		v.adapter.SetFocused(true)
		return
	}

	v := &settingsView{app: a, working: *a.Settings, baseline: *a.Settings, categories: settingsCategories()}
	a.settingsView = v

	tabItems := make([]widgets.TabItem, 0, len(v.categories))
	panes := make([]widgets.Widget, 0, len(v.categories))
	for _, cat := range v.categories {
		tabItems = append(tabItems, widgets.TabItem{ID: cat.Title, Label: cat.Title})
		panes = append(panes, v.buildPane(cat))
	}
	tabs := widgets.NewTabsWidget(widgets.TabsConfig{Items: tabItems, Align: "left"})
	tabbed := widgets.NewTabbedWidget(tabs, panes)
	tabbed.Fill = true

	v.status = widgets.NewLabelWidget(widgets.LabelConfig{Style: term.StyleMuted})
	cancelBtn := widgets.NewButtonWidget(widgets.ButtonConfig{Label: i18n.Text("general.cancel"), OnClick: v.cancel})
	applyBtn := widgets.NewButtonWidget(widgets.ButtonConfig{Label: i18n.Text("general.apply"), OnClick: v.apply})
	buttons := widgets.NewHStackWidget(v.status, cancelBtn, applyBtn)
	buttons.Gap = 1
	buttons.FixedHeight = 1
	buttons.Box.PaddingLeft = 1
	buttons.Box.PaddingRight = 1

	root := widgets.NewVStackWidget(
		tabbed,
		widgets.NewDividerWidget(widgets.DividerConfig{}),
		buttons,
	)

	// NewWidgetAdapter wires TabbedWidget.OnChange to rebuild focus on tab change.
	v.adapter = ui.NewWidgetAdapter(root)
	v.adapter.EnableScrollIntoView()

	a.EditorGroup.OpenPluginTab(settingsTabID, i18n.Text("general.settings"), v.adapter)
	a.FocusEditor()
	v.adapter.SetFocused(true)
}

func (v *settingsView) buildPane(cat settingsCategory) widgets.Widget {
	rows := make([]widgets.Widget, 0, len(cat.Fields))
	indent := 0
	for i, f := range cat.Fields {
		if f.Kind == settingSection {
			if i > 0 {
				rows = append(rows, widgets.NewLabelWidget(widgets.LabelConfig{}))
			}
			rows = append(rows, sectionHeading(f.Label))
			indent = 2
			continue
		}
		row := v.buildRow(cat.Title, f)
		row.Box.PaddingLeft = indent
		rows = append(rows, row)
	}
	stack := widgets.NewVStackWidget(rows...)
	stack.MeasureGrow = true
	stack.Box.PaddingLeft = 1
	stack.Box.PaddingTop = 1

	// The divider sits inside the pane so it reads as the tab strip's bottom
	// border, and stays put while the fields scroll under it.
	return widgets.NewVStackWidget(
		widgets.NewDividerWidget(widgets.DividerConfig{}),
		widgets.NewScrollViewWidget(stack),
	)
}

// sectionHeading draws a muted title followed by a rule to the pane's edge.
func sectionHeading(title string) widgets.Widget {
	name := widgets.NewLabelWidget(widgets.LabelConfig{Text: title, Style: term.StyleMuted})
	name.FixedWidth = textwidth.String(title)
	row := widgets.NewHStackWidget(name, widgets.NewDividerWidget(widgets.DividerConfig{}))
	row.Gap = 1
	row.FixedHeight = 1
	row.Box.PaddingRight = 2
	return row
}

// One row per setting: label in a fixed left column, control on the right.
// Each control keeps its native shape, so its type is readable at a glance.
func (v *settingsView) buildRow(category string, f settingField) *widgets.HStackWidget {
	label := f.Label
	if f.Restart {
		label += i18n.Text("settings.restart")
	}
	name := widgets.NewLabelWidget(widgets.LabelConfig{Text: label})
	name.FixedWidth = settingsLabelCols

	var control widgets.Widget
	switch f.Kind {
	case settingBool:
		control = v.boolControl(f)
	case settingEnum:
		control = v.enumControl(f)
	case settingShortcut:
		button := widgets.NewButtonWidget(widgets.ButtonConfig{
			OnClick: func() { v.app.showKeybindings(f.Command) },
		})
		refresh := func() {
			label := v.app.KeyFor(f.Command)
			if label == "" {
				label = i18n.Text("settings.setShortcut")
			}
			button.SetLabel(label)
		}
		refresh()
		v.shortcuts = append(v.shortcuts, refresh)
		control = button
	default:
		control = v.textControl(category, f)
	}

	row := widgets.NewHStackWidget(name, control)
	row.Gap = 2
	row.FixedHeight = 1
	return row
}

func (a *App) refreshSettingsShortcuts() {
	if a.settingsView != nil {
		for _, refresh := range a.settingsView.shortcuts {
			refresh()
		}
	}
}

func (v *settingsView) boolControl(f settingField) widgets.Widget {
	checkbox := widgets.NewCheckboxWidget(widgets.CheckboxConfig{
		Checked:  f.GetBool(&v.working),
		OnChange: func(checked bool) { f.SetBool(&v.working, checked) },
	})
	v.refresh = append(v.refresh, func() { checkbox.Config.Checked = f.GetBool(&v.working) })
	return checkbox
}

func (v *settingsView) enumControl(f settingField) widgets.Widget {
	var sel *widgets.SelectWidget
	sel = widgets.NewSelectWidget(widgets.SelectConfig{
		Items:       f.Options(&v.working),
		Collapsible: true,
		OnOpen:      func() { v.closeSelectsExcept(sel) },
		OnSelect: func(id string) {
			f.SetString(&v.working, id)
			sel.SetSelectedID(id)
		},
	})
	v.selects = append(v.selects, sel)
	sel.FixedWidth = settingsControlCols
	sel.SetSelectedID(f.GetString(&v.working))
	v.refresh = append(v.refresh, func() { sel.SetSelectedID(f.GetString(&v.working)) })
	return sel
}

// Text and numeric fields are parsed on Apply rather than per keystroke, so a
// half-typed value never reaches the working copy.
func (v *settingsView) textControl(category string, f settingField) widgets.Widget {
	current := ""
	if f.Kind == settingInt {
		current = strconv.Itoa(f.GetInt(&v.working))
	} else {
		current = f.GetString(&v.working)
	}

	// Borderless: InputWidget draws a "❯" prefix and recolours it on focus, which
	// is affordance enough for a one-line field.
	inp := widgets.NewInputWidget(widgets.InputConfig{})
	inp.SetText(current)
	v.refresh = append(v.refresh, func() {
		if f.Kind == settingInt {
			inp.SetText(strconv.Itoa(f.GetInt(&v.working)))
		} else {
			inp.SetText(f.GetString(&v.working))
		}
	})

	// Returns a description of the offending field, or "" when the value is good.
	v.inputs = append(v.inputs, func() string {
		text := inp.Text()
		if f.Kind == settingString {
			f.SetString(&v.working, text)
			return ""
		}
		n, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || n < f.Min {
			inp.SetText(strconv.Itoa(f.GetInt(&v.working)))
			return category + " → " + f.Label
		}
		f.SetInt(&v.working, n)
		return ""
	})
	return inp
}

func (v *settingsView) apply() {
	// Validate every field before committing, so one bad value does not hide the
	// rest and the user fixes them in a single pass.
	var bad []string
	for _, commit := range v.inputs {
		if msg := commit(); msg != "" {
			bad = append(bad, msg)
		}
	}
	if len(bad) > 0 {
		v.setStatus(i18n.Format("settings.invalidValueFor", i18n.Args{"items": strings.Join(bad, ", ")}))
		return
	}
	v.commitTo(v.app.Settings)
	if !v.app.SaveAndApplySettings() {
		v.setStatus(i18n.Text("settings.notSaved"))
		return
	}
	v.working = *v.app.Settings
	v.baseline = v.working
	for _, refresh := range v.refresh {
		refresh()
	}
	v.setStatus(i18n.Text("settings.settingsApplied"))
}

// Dropping the working copy is what discards unapplied edits, so it does not
// wait on OnContentTabClose: that hook only exists once App.Init has run, and
// clearing it here is idempotent with it.
func (v *settingsView) cancel() {
	v.app.settingsView = nil
	v.app.EditorGroup.ClosePluginTab(settingsTabID)
}

func (v *settingsView) setStatus(msg string) {
	if v.status != nil {
		v.status.Config.Text = msg
	}
}

func (a *App) ApplySettingsView() {
	if a.settingsView == nil {
		a.StatusNotify(i18n.Text("settings.noSettingsEditorOpen"))
		return
	}
	a.settingsView.apply()
}

func (a *App) CancelSettingsView() {
	if a.settingsView == nil {
		a.StatusNotify(i18n.Text("settings.noSettingsEditorOpen"))
		return
	}
	a.settingsView.cancel()
}

func (a *App) cleanupSettingsTab(id string) {
	if id == settingsTabID {
		a.settingsView = nil
	}
}

func setIconOverride(s *config.Settings, name, value string) {
	overrides := maps.Clone(s.Appearance.Overrides)
	if overrides == nil {
		overrides = map[string]string{}
	}
	if value == "" {
		delete(overrides, name)
	} else {
		overrides[name] = value
	}
	s.Appearance.Overrides = overrides
}
