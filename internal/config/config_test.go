package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadNoFiles(t *testing.T) {
	withTempConfigDir(t)
	cfg := Load("")
	if len(cfg.Keybindings) == 0 {
		t.Fatal("expected default keybindings")
	}
	if cfg.Settings.Editor.TabSize == 0 {
		t.Fatal("expected non-zero default TabSize")
	}
	if cfg.Settings.Theme != "default-dark" {
		t.Fatalf("theme = %q, want default-dark", cfg.Settings.Theme)
	}
	want, err := LoadTheme("default-dark")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Theme, want) {
		t.Fatal("expected embedded default-dark theme")
	}
}

func TestLoadDefaultThemeOverride(t *testing.T) {
	dir := withTempConfigDir(t)
	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themesDir, "default-dark.json"), []byte(`{"default":{"bg":"#123456"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Load("").Theme.Default.Bg; got != "#123456" {
		t.Fatalf("default theme background = %q, want local override", got)
	}
}

func TestLoadThemeSetting(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
		want string
	}{
		{"omitted", `{}`, "default-dark"},
		{"explicit", `{"theme":"default-light"}`, "default-light"},
		{"empty", `{"theme":""}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := withTempConfigDir(t)
			if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(tc.data), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg := Load("")
			if cfg.Settings.Theme != tc.want {
				t.Fatalf("theme = %q, want %q", cfg.Settings.Theme, tc.want)
			}
			if err := SaveSettings(cfg.Settings); err != nil {
				t.Fatal(err)
			}
			if got := Load(""); got.Settings.Theme != tc.want || !reflect.DeepEqual(got.Theme, cfg.Theme) {
				t.Fatal("theme changed after saving and reloading settings")
			}
		})
	}
}

func TestReadFirst(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.json"), []byte(`{"ok":true}`), 0644)

	data, err := readFirst([]string{"/nonexistent", dir}, "test.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("unexpected data: %s", data)
	}
}

func TestReadFirstPriority(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, "test.json"), []byte(`"first"`), 0644)
	os.WriteFile(filepath.Join(dir2, "test.json"), []byte(`"second"`), 0644)

	data, err := readFirst([]string{dir1, dir2}, "test.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `"first"` {
		t.Fatalf("expected first dir to win, got: %s", data)
	}
}

func TestReadFirstMissing(t *testing.T) {
	_, err := readFirst([]string{"/nonexistent"}, "test.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
