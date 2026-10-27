package config

import (
	"encoding/json"
	"os"
	"slices"
)

// Validated by normalizeSettings and used to populate the settings UI pickers.
var (
	GutterStyles        = []string{"minimal", "compact", "extended"}
	BorderStyles        = []string{"default", "theme", "rounded", "sharp", "double", "bold", "ascii", "none"}
	DiffModes           = []string{"split", "unified"}
	DiffContexts        = []string{"changes", "full"}
	GitFileViews        = []string{"tree", "list"}
	IconModes           = []string{IconsNerdFont, IconsNone}
	ScrollbarGlyphModes = []string{ScrollbarGlyphsBlocks, ScrollbarGlyphsLegacy}
	SelectionStyles     = []string{SelectionStyleBackground, SelectionStyleInvert}
)

const (
	DiffModeSplit            = "split"
	DiffModeUnified          = "unified"
	DiffContextChanges       = "changes"
	DiffContextFull          = "full"
	GitFileViewTree          = "tree"
	GitFileViewList          = "list"
	IconsNone                = "none"
	IconsNerdFont            = "nerd-font"
	ScrollbarGlyphsBlocks    = "blocks"
	ScrollbarGlyphsLegacy    = "legacy"
	SelectionStyleBackground = "background"
	SelectionStyleInvert     = "invert"
)

type TerminalSettings struct {
	Shell string `json:"shell,omitempty"`
	// Args are passed to Shell unchanged, so a tab can run a program other than
	// an interactive shell (e.g. Shell="pi", Args=["--model", "..."]).
	Args           []string `json:"args,omitempty"`
	Scrollback     int      `json:"scrollback,omitempty"`
	SelectionStyle string   `json:"selectionStyle"`
	CtrlVPastes    bool     `json:"ctrlVPastes"`
}

func DefaultTerminalSettings() TerminalSettings {
	return TerminalSettings{
		Scrollback:     1000,
		SelectionStyle: SelectionStyleBackground,
		CtrlVPastes:    true,
	}
}

type AutocompleteSettings struct {
	Enabled       bool `json:"enabled"`
	AutoSuggest   bool `json:"autoSuggest"`
	Debounce      int  `json:"debounce"`
	SignatureHelp bool `json:"signatureHelp"`
}

func DefaultAutocompleteSettings() AutocompleteSettings {
	return AutocompleteSettings{
		Enabled:       true,
		AutoSuggest:   true,
		Debounce:      150,
		SignatureHelp: true,
	}
}

type LSPServerConfig struct {
	Command   []string          `json:"command"`
	Languages map[string]string `json:"languages,omitempty"`
}

type LSPSettings struct {
	Enabled            *bool                      `json:"enabled,omitempty"`
	Hover              *bool                      `json:"hover,omitempty"`
	Servers            map[string]LSPServerConfig `json:"servers,omitempty"`
	SaveOnRename       bool                       `json:"saveOnRename"`
	CodeActionsOnSave  []string                   `json:"codeActionsOnSave,omitempty"`
	HoverDelay         int                        `json:"hoverDelay,omitempty"`
	NotifyAvailability *bool                      `json:"notifyAvailability,omitempty"`
}

func (l LSPSettings) ShouldNotifyAvailability() bool {
	return l.NotifyAvailability == nil || *l.NotifyAvailability
}

func (l LSPSettings) IsEnabled() bool {
	return l.Enabled == nil || *l.Enabled
}

func (l LSPSettings) IsHoverEnabled() bool {
	return l.Hover == nil || *l.Hover
}

func DefaultLSPSettings() LSPSettings {
	return LSPSettings{
		HoverDelay: 500,
		Servers:    make(map[string]LSPServerConfig),
	}
}

type EditorSettings struct {
	TabSize           int                               `json:"tabSize"`
	InsertSpaces      bool                              `json:"insertSpaces"`
	WordWrap          bool                              `json:"wordWrap"`
	WordWrapColumn    int                               `json:"wordWrapColumn"`
	LanguageOverrides map[string]EditorLanguageOverride `json:"languageOverrides,omitempty"`
	// HighlightTimeLimit caps tokenizing one line, in milliseconds; 0 keeps the
	// built-in limit. Unlisted: raise it only where every line must finish, such
	// as slow CI runners.
	HighlightTimeLimit      int                `json:"highlightTimeLimit,omitempty"`
	DiffMode                string             `json:"diffMode"`
	DiffContext             string             `json:"diffContext"`
	DiffWordWrap            bool               `json:"diffWordWrap"`
	DiffHighContrast        bool               `json:"diffHighContrast"`
	DiffCollapsedEmphasis   bool               `json:"diffEmphasizeCollapsedRows"`
	DiffSigns               bool               `json:"diffSigns"`
	DiffSignsColor          bool               `json:"diffSignsColor"`
	DiffGutterHighlight     bool               `json:"diffGutterHighlight"`
	DiffColorGutterNumbers  bool               `json:"diffColorGutterNumbers"`
	DiffScrollbarGitMarks   bool               `json:"diffScrollbarGitMarks"`
	LineNumbers             bool               `json:"lineNumbers"`
	CursorStyle             string             `json:"cursorStyle,omitempty"`
	SelectionStyle          string             `json:"selectionStyle"`
	FormatOnSave            bool               `json:"formatOnSave"`
	InsertFinalNewline      bool               `json:"insertFinalNewline"`
	TrimTrailingWhitespace  bool               `json:"trimTrailingWhitespace"`
	FocusOnOpen             bool               `json:"focusOnOpen"`
	SyntaxHighlight         *bool              `json:"syntaxHighlight,omitempty"`
	GitGutter               *bool              `json:"gitGutter,omitempty"`
	ScrollbarGitMarks       *bool              `json:"scrollbarGitMarks,omitempty"`
	AutoDedent              *bool              `json:"autoDedent,omitempty"`
	AutoIndent              *bool              `json:"autoIndent,omitempty"`
	GutterStyle             string             `json:"gutterStyle,omitempty"`
	BorderStyle             string             `json:"borderStyle,omitempty"`
	BracketPairColorization bool               `json:"bracketPairColorization"`
	ShowTrailingNewline     *bool              `json:"showTrailingNewline,omitempty"`
	MenuBar                 *bool              `json:"menuBar,omitempty"`
	UndoDeleteCursorStart   bool               `json:"undoDeleteCursorStart,omitempty"`
	TransparentBackground   bool               `json:"transparentBackground"`
	Grammars                []GrammarSetting   `json:"grammars,omitempty"`
	LargeFiles              *LargeFileSettings `json:"largeFiles,omitempty"`

	SaveAllOnQuit              bool `json:"saveAllOnQuit"`
	SaveAllOnQuitExceptUnnamed bool `json:"saveAllOnQuitExceptUnnamed"`
}

type EditorLanguageOverride struct {
	WordWrap       *bool `json:"wordWrap,omitempty"`
	WordWrapColumn *int  `json:"wordWrapColumn,omitempty"`
}

func (e EditorSettings) ResolveWordWrap(languageID string) (bool, int) {
	enabled, column := e.WordWrap, e.WordWrapColumn
	if o, ok := e.LanguageOverrides[languageID]; ok && languageID != "" {
		if o.WordWrap != nil {
			enabled = *o.WordWrap
		}
		if o.WordWrapColumn != nil {
			column = *o.WordWrapColumn
		}
	}
	return enabled, max(column, 0)
}

type LargeFileSettings struct {
	WordWrapMaxMB   *int `json:"wordWrapMaxMB,omitempty"`
	AutoReloadMaxMB *int `json:"autoReloadMaxMB,omitempty"`
}

// Path is relative to the config directory unless absolute.
type GrammarSetting struct {
	Path      string   `json:"path"`
	Language  string   `json:"language,omitempty"`
	FileTypes []string `json:"fileTypes,omitempty"`
	Plugin    string   `json:"plugin,omitempty"`
}

func (e EditorSettings) IsShowTrailingNewlineEnabled() bool {
	return e.ShowTrailingNewline == nil || *e.ShowTrailingNewline
}

func (e EditorSettings) IsMenuBarVisible() bool {
	return e.MenuBar == nil || *e.MenuBar
}

func (e EditorSettings) IsSyntaxHighlightEnabled() bool {
	return e.SyntaxHighlight == nil || *e.SyntaxHighlight
}

func (e EditorSettings) IsGitGutterEnabled() bool {
	return e.GitGutter == nil || *e.GitGutter
}

func (e EditorSettings) IsScrollbarGitMarksEnabled() bool {
	return e.ScrollbarGitMarks == nil || *e.ScrollbarGitMarks
}

func (e EditorSettings) IsAutoDedentEnabled() bool {
	return e.AutoDedent == nil || *e.AutoDedent
}

func (e EditorSettings) IsAutoIndentEnabled() bool {
	return e.AutoIndent == nil || *e.AutoIndent
}

const DefaultWordWrapMaxMB = 32
const DefaultAutoReloadMaxMB = 64

// WordWrapMaxBytes returns 0 when there is no limit.
func (e EditorSettings) WordWrapMaxBytes() int64 {
	var mb *int
	if e.LargeFiles != nil {
		mb = e.LargeFiles.WordWrapMaxMB
	}
	return megabytes(mb, DefaultWordWrapMaxMB)
}

func (e EditorSettings) AutoReloadMaxBytes() int64 {
	var mb *int
	if e.LargeFiles != nil {
		mb = e.LargeFiles.AutoReloadMaxMB
	}
	return megabytes(mb, DefaultAutoReloadMaxMB)
}

func megabytes(mb *int, def int) int64 {
	n := def
	if mb != nil {
		n = *mb
	}
	if n <= 0 {
		return 0
	}
	if int64(n) > ((1<<63)-1)>>20 {
		return (1 << 63) - 1
	}
	return int64(n) << 20
}

func DefaultEditorSettings() EditorSettings {
	return EditorSettings{
		TabSize:                 4,
		InsertSpaces:            true,
		DiffMode:                DiffModeSplit,
		DiffContext:             DiffContextChanges,
		DiffSignsColor:          true,
		DiffGutterHighlight:     true,
		DiffColorGutterNumbers:  false,
		LineNumbers:             true,
		SelectionStyle:          SelectionStyleBackground,
		InsertFinalNewline:      true,
		GutterStyle:             "compact",
		BorderStyle:             "default",
		BracketPairColorization: false,
	}
}

type SearchSettings struct {
	Debounce int `json:"debounce"`
}

func DefaultSearchSettings() SearchSettings {
	return SearchSettings{
		Debounce: 350,
	}
}

type QuickOpenSettings struct {
	ShowHidden     bool `json:"showHidden"`
	ShowGitIgnored bool `json:"showGitIgnored"`
}

type ExplorerSettings struct {
	ShowHidden         bool `json:"showHidden"`
	ShowGitIgnored     bool `json:"showGitIgnored"`
	GitStatusColors    bool `json:"gitStatusColors"`
	DimStagedGitColors bool `json:"dimStagedGitColors"`
	AutoReveal         bool `json:"autoReveal"`
}

// LegacyChevrons is the old appearance.chevrons setting, migrated into
// appearance.overrides on load.
type LegacyChevrons struct {
	Collapsed string `json:"collapsed"`
	Expanded  string `json:"expanded"`
}

type AppearanceSettings struct {
	Icons           string            `json:"icons"`
	Overrides       map[string]string `json:"overrides,omitempty"`
	Chevrons        *LegacyChevrons   `json:"chevrons,omitempty"`
	ScrollbarGlyphs string            `json:"scrollbarGlyphs"`
}

func DefaultAppearanceSettings() AppearanceSettings {
	return AppearanceSettings{
		Icons:           IconsNone,
		ScrollbarGlyphs: ScrollbarGlyphsBlocks,
	}
}

// migrateChevrons moves custom appearance.chevrons values into overrides. The
// old defaults are dropped so they don't override the chevrons forever.
func (a *AppearanceSettings) migrateChevrons() {
	if a.Chevrons == nil {
		return
	}
	for name, value := range map[string]string{"chevron-right": a.Chevrons.Collapsed, "chevron-down": a.Chevrons.Expanded} {
		if value == "" || value == "▶" || value == "▼" || a.Overrides[name] != "" {
			continue
		}
		if a.Overrides == nil {
			a.Overrides = map[string]string{}
		}
		a.Overrides[name] = value
	}
	a.Chevrons = nil
}

type SidebarSettings struct {
	PanelOrder          []string `json:"panelOrder,omitempty"`
	Width               int      `json:"width,omitempty"`
	CommitHistoryHeight int      `json:"commitHistoryHeight,omitempty"`
}

func DefaultSidebarSettings() SidebarSettings {
	return SidebarSettings{}
}

type PanelSettings struct {
	Position string `json:"position,omitempty"`
}

type GitSettings struct {
	FileView string `json:"fileView"`
}

func DefaultGitSettings() GitSettings {
	return GitSettings{FileView: GitFileViewList}
}

func DefaultExplorerSettings() ExplorerSettings {
	return ExplorerSettings{
		ShowHidden:      true,
		ShowGitIgnored:  true,
		GitStatusColors: true,
	}
}

type PluginSettings struct {
	Enabled        *bool  `json:"enabled,omitempty"`
	SelectionStyle string `json:"selectionStyle,omitempty"`
}

func (p PluginSettings) IsEnabled() bool {
	return p.Enabled == nil || *p.Enabled
}

type MarkdownSettings struct {
	WrapWidth int `json:"wrapWidth,omitempty"`
}

func DefaultMarkdownSettings() MarkdownSettings {
	return MarkdownSettings{
		WrapWidth: 80,
	}
}

const (
	ImageProtocolAuto  = "auto"
	ImageProtocolKitty = "kitty"
	ImageProtocolNone  = "none"
)

type ImageSettings struct {
	Protocol string `json:"protocol"`
}

func DefaultImageSettings() ImageSettings {
	return ImageSettings{
		Protocol: ImageProtocolAuto,
	}
}

type WelcomeSettings struct {
	// ShowOnHome shows the welcome page instead of opening $HOME when ttt
	// starts there with no arguments, as desktop launchers do.
	ShowOnHome bool `json:"showOnHome,omitempty"`
	// Favorites are folders listed on the welcome page to open in one step.
	Favorites     []string `json:"favorites,omitempty"`
	RecentFolders *bool    `json:"recentFolders,omitempty"`
}

func (w WelcomeSettings) IsRecentFoldersEnabled() bool {
	return w.RecentFolders == nil || *w.RecentFolders
}

type Settings struct {
	Version   int    `json:"version"`
	Theme     string `json:"theme,omitempty"`
	DebugMode bool   `json:"debugMode,omitempty"`
	// These sections must NOT use omitzero: their defaults are non-zero, so an
	// all-false/all-zero section would be omitted on save and silently revert to
	// the defaults on the next load.
	Appearance   AppearanceSettings   `json:"appearance"`
	Editor       EditorSettings       `json:"editor"`
	QuickOpen    QuickOpenSettings    `json:"quickOpen"`
	Search       SearchSettings       `json:"search"`
	Explorer     ExplorerSettings     `json:"explorer"`
	Sidebar      SidebarSettings      `json:"sidebar,omitzero"`
	Panel        PanelSettings        `json:"panel,omitzero"`
	Git          GitSettings          `json:"git"`
	Terminal     TerminalSettings     `json:"terminal"`
	LSP          LSPSettings          `json:"lsp"`
	Autocomplete AutocompleteSettings `json:"autocomplete"`
	Markdown     MarkdownSettings     `json:"markdown"`
	Image        ImageSettings        `json:"image"`
	// Plugins is safe: each field's zero value (a nil *bool, an empty string)
	// means its default, so the zero value and "unset" mean the same thing.
	Plugins    PluginSettings    `json:"plugins,omitzero"`
	Welcome    WelcomeSettings   `json:"welcome,omitzero"`
	Formatters map[string]string `json:"formatters,omitempty"`
	// Extra holds top-level keys that are not part of the core schema — chiefly
	// plugin-namespaced settings (e.g. "vim"). Without this, json.Unmarshal into
	// the struct would silently drop them, making ttt.settings.get/set unusable
	// for plugins. It is populated/emitted by the custom (Un)MarshalJSON below.
	Extra map[string]json.RawMessage `json:"-"`
}

// knownSettingsKeys is the set of top-level JSON keys owned by the core schema.
// Any other top-level key is preserved via Settings.Extra.
var knownSettingsKeys = map[string]bool{
	"version": true, "theme": true, "debugMode": true, "appearance": true, "editor": true,
	"search": true, "quickOpen": true, "explorer": true, "sidebar": true, "panel": true, "git": true, "terminal": true, "lsp": true,
	"autocomplete": true, "markdown": true, "image": true, "plugins": true, "formatters": true, "welcome": true,
}

func (s Settings) MarshalJSON() ([]byte, error) {
	type alias Settings
	base, err := json.Marshal(alias(s))
	if err != nil {
		return nil, err
	}
	if len(s.Extra) == 0 {
		// Byte-identical to the plain struct encoding — keeps field ordering
		// stable for the settings-roundtrip test when no plugin keys are present.
		return base, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(base, &m); err != nil {
		return nil, err
	}
	for k, v := range s.Extra {
		if !knownSettingsKeys[k] {
			m[k] = v
		}
	}
	return json.Marshal(m)
}

func (s *Settings) UnmarshalJSON(data []byte) error {
	type alias Settings
	aux := (*alias)(s)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	for k, v := range m {
		if !knownSettingsKeys[k] {
			if s.Extra == nil {
				s.Extra = make(map[string]json.RawMessage)
			}
			s.Extra[k] = v
		}
	}
	return nil
}

func DefaultSettings() Settings {
	return Settings{
		Version:      1,
		Appearance:   DefaultAppearanceSettings(),
		Editor:       DefaultEditorSettings(),
		Search:       DefaultSearchSettings(),
		Explorer:     DefaultExplorerSettings(),
		Sidebar:      DefaultSidebarSettings(),
		Git:          DefaultGitSettings(),
		Terminal:     DefaultTerminalSettings(),
		LSP:          DefaultLSPSettings(),
		Autocomplete: DefaultAutocompleteSettings(),
		Markdown:     DefaultMarkdownSettings(),
		Image:        DefaultImageSettings(),
		Plugins:      PluginSettings{SelectionStyle: SelectionStyleInvert},
	}
}

func (s Settings) FormatterForExt(ext string) string {
	if s.Formatters == nil {
		return ""
	}
	return s.Formatters[ext]
}

func normalizeSettings(s *Settings) {
	s.Editor.WordWrapColumn = max(s.Editor.WordWrapColumn, 0)
	for language, o := range s.Editor.LanguageOverrides {
		if o.WordWrapColumn != nil && *o.WordWrapColumn < 0 {
			column := 0
			o.WordWrapColumn = &column
			s.Editor.LanguageOverrides[language] = o
		}
	}
	if !slices.Contains(GutterStyles, s.Editor.GutterStyle) {
		s.Editor.GutterStyle = "compact"
	}
	if !slices.Contains(BorderStyles, s.Editor.BorderStyle) {
		s.Editor.BorderStyle = "default"
	}
	if !slices.Contains(SelectionStyles, s.Editor.SelectionStyle) {
		s.Editor.SelectionStyle = SelectionStyleBackground
	}
	if !slices.Contains(SelectionStyles, s.Terminal.SelectionStyle) {
		s.Terminal.SelectionStyle = SelectionStyleBackground
	}
	if !slices.Contains(SelectionStyles, s.Plugins.SelectionStyle) {
		s.Plugins.SelectionStyle = SelectionStyleInvert
	}
	seenPanels := make(map[string]bool)
	panelOrder := s.Sidebar.PanelOrder[:0]
	for _, id := range s.Sidebar.PanelOrder {
		if id == "" || seenPanels[id] {
			continue
		}
		seenPanels[id] = true
		panelOrder = append(panelOrder, id)
	}
	s.Sidebar.PanelOrder = panelOrder
	if s.Sidebar.Width < 0 {
		s.Sidebar.Width = 0
	}
	if s.Sidebar.CommitHistoryHeight < 0 {
		s.Sidebar.CommitHistoryHeight = 0
	}
	s.Appearance.migrateChevrons()
	if !slices.Contains(DiffModes, s.Editor.DiffMode) {
		s.Editor.DiffMode = DiffModeSplit
	}
	if !slices.Contains(DiffContexts, s.Editor.DiffContext) {
		s.Editor.DiffContext = DiffContextChanges
	}
	if !slices.Contains(GitFileViews, s.Git.FileView) {
		s.Git.FileView = GitFileViewList
	}
	if !slices.Contains([]string{ImageProtocolAuto, ImageProtocolKitty, ImageProtocolNone}, s.Image.Protocol) {
		s.Image.Protocol = ImageProtocolAuto
	}
	if !slices.Contains(IconModes, s.Appearance.Icons) {
		s.Appearance.Icons = IconsNone
	}
	if !slices.Contains(ScrollbarGlyphModes, s.Appearance.ScrollbarGlyphs) {
		s.Appearance.ScrollbarGlyphs = ScrollbarGlyphsBlocks
	}
}

func LoadSettings() Settings {
	s := DefaultSettings()
	paths := configPaths()
	if data, err := readFirst(paths, "settings.json"); err == nil {
		json.Unmarshal(data, &s)
	}
	normalizeSettings(&s)
	return s
}

func SaveSettings(s Settings) error {
	path := ConfigFilePath("settings.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}
