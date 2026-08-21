package config

import (
	"encoding/json"
	"os"
	"slices"
)

// Validated by normalizeSettings and used to populate the settings UI pickers.
var (
	GutterStyles = []string{"minimal", "compact", "extended"}
	BorderStyles = []string{"default", "theme", "rounded", "sharp", "double", "bold", "ascii", "none"}
	DiffModes    = []string{"split", "unified"}
	DiffContexts = []string{"changes", "full"}
	GitFileViews = []string{"tree", "list"}
)

const (
	DiffModeSplit      = "split"
	DiffModeUnified    = "unified"
	DiffContextChanges = "changes"
	DiffContextFull    = "full"
	GitFileViewTree    = "tree"
	GitFileViewList    = "list"
)

type TerminalSettings struct {
	Shell      string `json:"shell,omitempty"`
	Scrollback int    `json:"scrollback,omitempty"`
}

func DefaultTerminalSettings() TerminalSettings {
	return TerminalSettings{
		Scrollback: 1000,
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
	TabSize                 int    `json:"tabSize"`
	InsertSpaces            bool   `json:"insertSpaces"`
	WordWrap                bool   `json:"wordWrap"`
	DiffMode                string `json:"diffMode"`
	DiffContext             string `json:"diffContext"`
	DiffWordWrap            bool   `json:"diffWordWrap"`
	DiffHighContrast        bool   `json:"diffHighContrast,omitempty"`
	LineNumbers             bool   `json:"lineNumbers"`
	CursorStyle             string `json:"cursorStyle,omitempty"`
	FormatOnSave            bool   `json:"formatOnSave"`
	InsertFinalNewline      bool   `json:"insertFinalNewline"`
	TrimTrailingWhitespace  bool   `json:"trimTrailingWhitespace"`
	FocusOnOpen             bool   `json:"focusOnOpen"`
	SyntaxHighlight         *bool  `json:"syntaxHighlight,omitempty"`
	GitGutter               *bool  `json:"gitGutter,omitempty"`
	AutoDedent              *bool  `json:"autoDedent,omitempty"`
	AutoIndent              *bool  `json:"autoIndent,omitempty"`
	GutterStyle             string `json:"gutterStyle,omitempty"`
	BorderStyle             string `json:"borderStyle,omitempty"`
	BracketPairColorization bool   `json:"bracketPairColorization"`
	ShowTrailingNewline     *bool  `json:"showTrailingNewline,omitempty"`
	MenuBar                 *bool  `json:"menuBar,omitempty"`
	UndoDeleteCursorStart   bool   `json:"undoDeleteCursorStart,omitempty"`
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

func (e EditorSettings) IsAutoDedentEnabled() bool {
	return e.AutoDedent == nil || *e.AutoDedent
}

func (e EditorSettings) IsAutoIndentEnabled() bool {
	return e.AutoIndent == nil || *e.AutoIndent
}

func DefaultEditorSettings() EditorSettings {
	return EditorSettings{
		TabSize:                 4,
		InsertSpaces:            true,
		DiffMode:                DiffModeSplit,
		DiffContext:             DiffContextChanges,
		LineNumbers:             true,
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

type ExplorerSettings struct {
	ShowHidden     bool `json:"showHidden"`
	ShowGitIgnored bool `json:"showGitIgnored"`
}

type GitSettings struct {
	FileView string `json:"fileView"`
}

func DefaultGitSettings() GitSettings {
	return GitSettings{FileView: GitFileViewTree}
}

type SidebarSettings struct {
	PanelOrder []string `json:"panelOrder,omitempty"`
}

func DefaultExplorerSettings() ExplorerSettings {
	return ExplorerSettings{
		ShowHidden:     true,
		ShowGitIgnored: true,
	}
}

type PluginSettings struct {
	Enabled *bool `json:"enabled,omitempty"`
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

type Settings struct {
	Version   int    `json:"version"`
	Theme     string `json:"theme,omitempty"`
	DebugMode bool   `json:"debugMode,omitempty"`
	// These sections must NOT use omitzero: their defaults are non-zero, so an
	// all-false/all-zero section would be omitted on save and silently revert to
	// the defaults on the next load.
	Editor       EditorSettings       `json:"editor"`
	Search       SearchSettings       `json:"search"`
	Explorer     ExplorerSettings     `json:"explorer"`
	Git          GitSettings          `json:"git"`
	Terminal     TerminalSettings     `json:"terminal"`
	LSP          LSPSettings          `json:"lsp"`
	Autocomplete AutocompleteSettings `json:"autocomplete"`
	Markdown     MarkdownSettings     `json:"markdown"`
	Sidebar      SidebarSettings      `json:"sidebar,omitzero"`
	// Plugins is safe: its only field is a tri-state *bool where nil means the
	// default, so the zero value and "unset" mean the same thing.
	Plugins    PluginSettings    `json:"plugins,omitzero"`
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
	"version": true, "theme": true, "debugMode": true, "editor": true,
	"search": true, "explorer": true, "git": true, "terminal": true, "lsp": true,
	"autocomplete": true, "markdown": true, "plugins": true, "formatters": true,
	"sidebar": true,
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
		Editor:       DefaultEditorSettings(),
		Search:       DefaultSearchSettings(),
		Explorer:     DefaultExplorerSettings(),
		Git:          DefaultGitSettings(),
		Terminal:     DefaultTerminalSettings(),
		LSP:          DefaultLSPSettings(),
		Autocomplete: DefaultAutocompleteSettings(),
		Markdown:     DefaultMarkdownSettings(),
	}
}

func (s Settings) FormatterForExt(ext string) string {
	if s.Formatters == nil {
		return ""
	}
	return s.Formatters[ext]
}

func normalizeSettings(s *Settings) {
	if !slices.Contains(GutterStyles, s.Editor.GutterStyle) {
		s.Editor.GutterStyle = "compact"
	}
	if !slices.Contains(BorderStyles, s.Editor.BorderStyle) {
		s.Editor.BorderStyle = "default"
	}
	if !slices.Contains(DiffModes, s.Editor.DiffMode) {
		s.Editor.DiffMode = DiffModeSplit
	}
	if !slices.Contains(DiffContexts, s.Editor.DiffContext) {
		s.Editor.DiffContext = DiffContextChanges
	}
	if !slices.Contains(GitFileViews, s.Git.FileView) {
		s.Git.FileView = GitFileViewTree
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
