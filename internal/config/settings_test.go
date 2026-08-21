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
	if s.Editor.DiffMode != DiffModeSplit {
		t.Fatalf("expected DiffMode %q, got %q", DiffModeSplit, s.Editor.DiffMode)
	}
	if s.Editor.DiffWordWrap {
		t.Fatal("expected DiffWordWrap false")
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

func TestNormalizeSettingsDiffMode(t *testing.T) {
	for _, mode := range DiffModes {
		s := DefaultSettings()
		s.Editor.DiffMode = mode
		normalizeSettings(&s)
		if s.Editor.DiffMode != mode {
			t.Errorf("valid diffMode %q normalized to %q", mode, s.Editor.DiffMode)
		}
	}

	s := DefaultSettings()
	s.Editor.DiffMode = "sideways"
	normalizeSettings(&s)
	if s.Editor.DiffMode != DiffModeSplit {
		t.Errorf("invalid diffMode normalized to %q, want %q", s.Editor.DiffMode, DiffModeSplit)
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
	if !e.IsShowTrailingNewlineEnabled() {
		t.Error("expected ShowTrailingNewline true by default (nil)")
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
		t.Error("expected Servers map to be initialized")
	}
	if len(ls.Servers) != 0 {
		t.Errorf("expected empty Servers map, got %d entries", len(ls.Servers))
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

// TestSettingsPreservesPluginKeys verifies that top-level keys outside the core
// schema (e.g. plugin-namespaced "vim") survive a load/marshal roundtrip via the
// Extra catch-all. Without this, ttt.settings.get/set could never read or persist
// plugin settings such as vim.enabled.
func TestSettingsPreservesPluginKeys(t *testing.T) {
	raw := []byte(`{"version":1,"vim":{"enabled":false,"clipboard":true}}`)

	var s Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Extra["vim"] == nil {
		t.Fatal("expected vim key captured in Extra")
	}

	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	vim, ok := m["vim"].(map[string]any)
	if !ok {
		t.Fatalf("vim key not preserved through marshal, got: %s", out)
	}
	if vim["enabled"] != false {
		t.Errorf("expected vim.enabled=false, got %v", vim["enabled"])
	}
	if vim["clipboard"] != true {
		t.Errorf("expected vim.clipboard=true, got %v", vim["clipboard"])
	}
}

// TestSettingsEmptyExtraByteIdentical guards that with no plugin keys present the
// custom MarshalJSON produces exactly the plain struct encoding (field ordering
// preserved), which the settings-roundtrip relies on.
func TestSettingsEmptyExtraByteIdentical(t *testing.T) {
	s := DefaultSettings()
	s.Theme = "default-dark"
	got, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	type alias Settings
	want, err := json.Marshal(alias(s))
	if err != nil {
		t.Fatalf("marshal alias: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("empty-Extra MarshalJSON diverged from struct encoding:\n got: %s\nwant: %s", got, want)
	}
}
