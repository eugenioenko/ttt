package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()
	if s.Editor.TabSize != 4 {
		t.Fatalf("expected TabSize 4, got %d", s.Editor.TabSize)
	}
	if !s.Editor.InsertSpaces {
		t.Fatal("expected InsertSpaces true")
	}
}

func TestSettingsPartialJSON(t *testing.T) {
	s := DefaultSettings()
	json.Unmarshal([]byte(`{"editor": {"tabSize": 2}}`), &s)

	if s.Editor.TabSize != 2 {
		t.Fatalf("expected TabSize 2, got %d", s.Editor.TabSize)
	}
	if !s.Editor.InsertSpaces {
		t.Fatal("InsertSpaces should still be true (not in JSON)")
	}
}

func TestSettingsEmptyJSON(t *testing.T) {
	s := DefaultSettings()
	json.Unmarshal([]byte(`{}`), &s)

	if s.Editor.TabSize != 4 {
		t.Fatalf("expected TabSize 4, got %d", s.Editor.TabSize)
	}
}

func TestNormalizeSettingsValidGutterStyles(t *testing.T) {
	for _, style := range []string{"minimal", "compact", "extended"} {
		s := DefaultSettings()
		s.Editor.GutterStyle = style
		normalizeSettings(&s)
		if s.Editor.GutterStyle != style {
			t.Errorf("expected GutterStyle %q to be preserved, got %q", style, s.Editor.GutterStyle)
		}
	}
}

func TestNormalizeSettingsInvalidGutterStyle(t *testing.T) {
	s := DefaultSettings()
	s.Editor.GutterStyle = "invalid"
	normalizeSettings(&s)
	if s.Editor.GutterStyle != "compact" {
		t.Errorf("expected invalid GutterStyle to become 'compact', got %q", s.Editor.GutterStyle)
	}
}

func TestNormalizeSettingsEmptyGutterStyle(t *testing.T) {
	s := DefaultSettings()
	s.Editor.GutterStyle = ""
	normalizeSettings(&s)
	if s.Editor.GutterStyle != "compact" {
		t.Errorf("expected empty GutterStyle to become 'compact', got %q", s.Editor.GutterStyle)
	}
}

func TestLSPSettingsIsEnabled(t *testing.T) {
	// nil Enabled means default (true)
	s := LSPSettings{}
	if !s.IsEnabled() {
		t.Error("expected IsEnabled() true when Enabled is nil")
	}

	// explicit true
	enabled := true
	s.Enabled = &enabled
	if !s.IsEnabled() {
		t.Error("expected IsEnabled() true when Enabled is true")
	}

	// explicit false
	disabled := false
	s.Enabled = &disabled
	if s.IsEnabled() {
		t.Error("expected IsEnabled() false when Enabled is false")
	}
}

func TestLSPSettingsShouldNotifyAvailability(t *testing.T) {
	// nil means default (true)
	s := LSPSettings{}
	if !s.ShouldNotifyAvailability() {
		t.Error("expected ShouldNotifyAvailability() true when nil")
	}

	yes := true
	s.NotifyAvailability = &yes
	if !s.ShouldNotifyAvailability() {
		t.Error("expected ShouldNotifyAvailability() true when explicitly true")
	}

	no := false
	s.NotifyAvailability = &no
	if s.ShouldNotifyAvailability() {
		t.Error("expected ShouldNotifyAvailability() false when explicitly false")
	}
}

func TestEditorSettingsIsGitGutterEnabled(t *testing.T) {
	// nil means default (true)
	e := EditorSettings{}
	if !e.IsGitGutterEnabled() {
		t.Error("expected IsGitGutterEnabled() true when nil")
	}

	yes := true
	e.GitGutter = &yes
	if !e.IsGitGutterEnabled() {
		t.Error("expected IsGitGutterEnabled() true when explicitly true")
	}

	no := false
	e.GitGutter = &no
	if e.IsGitGutterEnabled() {
		t.Error("expected IsGitGutterEnabled() false when explicitly false")
	}
}

func TestDefaultEditorSettings(t *testing.T) {
	e := DefaultEditorSettings()
	if e.TabSize != 4 {
		t.Errorf("expected TabSize 4, got %d", e.TabSize)
	}
	if !e.InsertSpaces {
		t.Error("expected InsertSpaces true")
	}
	if !e.LineNumbers {
		t.Error("expected LineNumbers true")
	}
	if !e.InsertFinalNewline {
		t.Error("expected InsertFinalNewline true")
	}
	if e.GutterStyle != "compact" {
		t.Errorf("expected GutterStyle 'compact', got %q", e.GutterStyle)
	}
	if e.BracketPairColorization {
		t.Error("expected BracketPairColorization false by default")
	}
}

func TestDefaultTerminalSettings(t *testing.T) {
	ts := DefaultTerminalSettings()
	if ts.Scrollback != 1000 {
		t.Errorf("expected Scrollback 1000, got %d", ts.Scrollback)
	}
	if ts.Shell != "" {
		t.Errorf("expected Shell to be empty by default, got %q", ts.Shell)
	}
}

func TestDefaultAutocompleteSettings(t *testing.T) {
	ac := DefaultAutocompleteSettings()
	if !ac.Enabled {
		t.Error("expected Enabled true")
	}
	if !ac.AutoSuggest {
		t.Error("expected AutoSuggest true")
	}
	if ac.Debounce != 150 {
		t.Errorf("expected Debounce 150, got %d", ac.Debounce)
	}
	if !ac.SignatureHelp {
		t.Error("expected SignatureHelp true")
	}
}

func TestDefaultSearchSettings(t *testing.T) {
	ss := DefaultSearchSettings()
	if ss.Debounce != 350 {
		t.Errorf("expected Debounce 350, got %d", ss.Debounce)
	}
}

func TestDefaultExplorerSettings(t *testing.T) {
	es := DefaultExplorerSettings()
	if !es.ShowHidden {
		t.Error("expected ShowHidden true")
	}
	if !es.ShowGitIgnored {
		t.Error("expected ShowGitIgnored true")
	}
}

func TestDefaultLSPSettings(t *testing.T) {
	ls := DefaultLSPSettings()
	if ls.HoverDelay != 500 {
		t.Errorf("expected HoverDelay 500, got %d", ls.HoverDelay)
	}
	if ls.Servers == nil {
		t.Error("expected Servers to be loaded from embedded JSON")
	}
}

func TestReferenceSettingsMatchesDefaults(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	refPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "config", "settings.json")
	refData, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("failed to read config/settings.json: %v", err)
	}

	s := DefaultSettings()
	s.Theme = "default-dark"
	generated, _ := json.MarshalIndent(s, "", "  ")
	generated = append(generated, '\n')

	if string(generated) != string(refData) {
		t.Errorf("config/settings.json is out of date with DefaultSettings().\n"+
			"Regenerate it by running DefaultSettings() with theme='default-dark' and saving the output.\n"+
			"Got %d bytes, want %d bytes", len(refData), len(generated))
	}
}
