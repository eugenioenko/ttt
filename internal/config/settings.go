package config

import (
	_ "embed"
	"encoding/json"
	"os"
)

//go:embed lsp_servers.json
var lspServersJSON []byte

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
	var servers map[string]LSPServerConfig
	json.Unmarshal(lspServersJSON, &servers)
	return LSPSettings{
		HoverDelay: 500,
		Servers:    servers,
	}
}

type EditorSettings struct {
	TabSize                 int    `json:"tabSize"`
	InsertSpaces            bool   `json:"insertSpaces"`
	WordWrap                bool   `json:"wordWrap"`
	LineNumbers             bool   `json:"lineNumbers"`
	CursorStyle             string `json:"cursorStyle,omitempty"`
	FormatOnSave            bool   `json:"formatOnSave"`
	InsertFinalNewline      bool   `json:"insertFinalNewline"`
	TrimTrailingWhitespace  bool   `json:"trimTrailingWhitespace"`
	FocusOnOpen             bool   `json:"focusOnOpen"`
	GitGutter               *bool  `json:"gitGutter,omitempty"`
	GutterStyle             string `json:"gutterStyle,omitempty"`
	BracketPairColorization bool   `json:"bracketPairColorization"`
}

// IsGitGutterEnabled returns whether git gutter indicators are enabled.
// Defaults to true when the setting is not explicitly set.
func (e EditorSettings) IsGitGutterEnabled() bool {
	return e.GitGutter == nil || *e.GitGutter
}

func DefaultEditorSettings() EditorSettings {
	return EditorSettings{
		TabSize:                 4,
		InsertSpaces:            true,
		LineNumbers:             true,
		InsertFinalNewline:      true,
		GutterStyle:             "compact",
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

func DefaultExplorerSettings() ExplorerSettings {
	return ExplorerSettings{
		ShowHidden:     true,
		ShowGitIgnored: true,
	}
}

type Settings struct {
	Version      int                  `json:"version"`
	Theme        string               `json:"theme,omitempty"`
	DebugMode    bool                 `json:"debugMode,omitempty"`
	Editor       EditorSettings       `json:"editor,omitzero"`
	Search       SearchSettings       `json:"search,omitzero"`
	Explorer     ExplorerSettings     `json:"explorer,omitzero"`
	Terminal     TerminalSettings     `json:"terminal,omitzero"`
	LSP          LSPSettings          `json:"lsp,omitzero"`
	Autocomplete AutocompleteSettings `json:"autocomplete,omitzero"`
}

func DefaultSettings() Settings {
	return Settings{
		Version:      1,
		Editor:       DefaultEditorSettings(),
		Search:       DefaultSearchSettings(),
		Explorer:     DefaultExplorerSettings(),
		Terminal:     DefaultTerminalSettings(),
		LSP:          DefaultLSPSettings(),
		Autocomplete: DefaultAutocompleteSettings(),
	}
}

func normalizeSettings(s *Settings) {
	switch s.Editor.GutterStyle {
	case "minimal", "compact", "extended":
	default:
		s.Editor.GutterStyle = "compact"
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
