package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsLoadErrorsAndRecovery(t *testing.T) {
	dir := withTempConfigDir(t)
	path := filepath.Join(dir, "settings.json")
	if _, err := ReadSettings(); err != nil {
		t.Fatalf("missing settings should use defaults: %v", err)
	}
	for _, invalid := range []string{
		`{"theme":"dracula","editor":{"tabSize":2,}}`,
		`{"theme":"dracula","editor":{"tabSize":"two"}}`,
	} {
		if err := os.WriteFile(path, []byte(invalid), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, file := range []string{"", path} {
			cfg := Load(file)
			if cfg.SettingsError == nil {
				t.Errorf("Load(%q) ignored invalid settings", file)
			}
			if cfg.Settings.Theme != DefaultSettings().Theme || cfg.Settings.Editor.TabSize != DefaultSettings().Editor.TabSize {
				t.Errorf("failed load applied partial settings: %+v", cfg.Settings.Editor)
			}
		}
	}
	repaired := `{"theme":"dracula","editor":{"tabSize":2},"vim":{"enabled":true}}`
	if err := os.WriteFile(path, []byte(repaired), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := ReadSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.Editor.WordWrap = true
	if err := SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	got := LoadSettings()
	var vim map[string]bool
	if err := json.Unmarshal(got.Extra["vim"], &vim); err != nil || !vim["enabled"] || got.Theme != "dracula" || got.Editor.TabSize != 2 || !got.Editor.WordWrap {
		t.Fatalf("repair/reload/save lost settings: theme=%q tabSize=%d wrap=%v vim=%s", got.Theme, got.Editor.TabSize, got.Editor.WordWrap, got.Extra["vim"])
	}
	if err := os.WriteFile(path, []byte(`{"broken":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveSettings(s); err == nil {
		t.Error("save did not recheck a file corrupted after loading")
	}
}

func TestSaveSettingsPreservesInvalidFile(t *testing.T) {
	dir := withTempConfigDir(t)
	path := filepath.Join(dir, "settings.json")
	invalid := []byte(`{"theme":"dracula","editor":{"tabSize":2,},"vim":{"enabled":true}}`)
	if err := os.WriteFile(path, invalid, 0o644); err != nil {
		t.Fatal(err)
	}
	s := LoadSettings()
	s.Editor.WordWrap = true
	if err := SaveSettings(s); err == nil {
		t.Error("saving over malformed settings succeeded")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(invalid) {
		t.Fatalf("invalid settings overwritten: %s, error %v", got, err)
	}
}
