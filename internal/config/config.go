package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/eugenioenko/ttt/internal/config/themes"
)

type AppConfig struct {
	Keybindings []KeyBinding
	Settings    Settings
	Theme       ThemeConfig
}

func Load(settingsFile string) AppConfig {
	cfg := AppConfig{
		Keybindings: DefaultKeybindings(),
		Settings:    DefaultSettings(),
		Theme:       DefaultTheme(),
	}

	paths := configPaths()

	if data, err := readFirst(paths, "keybindings.json"); err == nil {
		if kb, err := LoadKeybindings(data); err == nil {
			cfg.Keybindings = MergeKeybindings(DefaultKeybindings(), kb)
		}
	}

	if settingsFile != "" {
		if data, err := os.ReadFile(settingsFile); err == nil {
			json.Unmarshal(data, &cfg.Settings)
		}
	} else if data, err := readFirst(paths, "settings.json"); err == nil {
		json.Unmarshal(data, &cfg.Settings)
	}

	if cfg.Settings.Theme != "" {
		themeFile := cfg.Settings.Theme + ".json"
		if data, err := readFirstTheme(paths, themeFile); err == nil {
			json.Unmarshal(data, &cfg.Theme)
		} else if data, err := themes.FS.ReadFile(themeFile); err == nil {
			json.Unmarshal(data, &cfg.Theme)
		}
	}

	normalizeSettings(&cfg.Settings)

	cfg.Theme.ResolveColors()

	return cfg
}

var OverrideConfigDir string

func configPaths() []string {
	if OverrideConfigDir != "" {
		return []string{OverrideConfigDir}
	}

	var paths []string

	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "config"))
	}

	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "ttt"))
	}

	return paths
}

func ListThemes() []string {
	seen := make(map[string]bool)
	var names []string

	for _, dir := range configPaths() {
		themesDir := filepath.Join(dir, "themes")
		entries, err := os.ReadDir(themesDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if name := themeNameFromFile(e.Name()); name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	entries, err := themes.FS.ReadDir(".")
	if err == nil {
		for _, e := range entries {
			if name := themeNameFromFile(e.Name()); name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	sort.Strings(names)
	return names
}

func LoadTheme(name string) (ThemeConfig, error) {
	theme := DefaultTheme()
	if name == "" {
		theme.ResolveColors()
		return theme, nil
	}
	themeFile := name + ".json"

	if data, err := readFirstTheme(configPaths(), themeFile); err == nil {
		if err := json.Unmarshal(data, &theme); err != nil {
			return theme, err
		}
		theme.ResolveColors()
		return theme, nil
	}

	data, err := themes.FS.ReadFile(themeFile)
	if err != nil {
		return theme, err
	}
	if err := json.Unmarshal(data, &theme); err != nil {
		return theme, err
	}
	theme.ResolveColors()
	return theme, nil
}

func themeNameFromFile(filename string) string {
	if strings.HasSuffix(filename, ".json") {
		return strings.TrimSuffix(filename, ".json")
	}
	return ""
}

func ConfigFilePath(filename string) string {
	paths := configPaths()
	for _, dir := range paths {
		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	if OverrideConfigDir != "" {
		os.MkdirAll(OverrideConfigDir, 0755)
		return filepath.Join(OverrideConfigDir, filename)
	}
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, ".config", "ttt")
		os.MkdirAll(dir, 0755)
		return filepath.Join(dir, filename)
	}
	return filepath.Join(".config", filename)
}

func EnsureConfigFile(path, defaultContent string) {
	if _, err := os.Stat(path); err == nil {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte(defaultContent), 0644)
}

func readFirst(dirs []string, filename string) ([]byte, error) {
	for _, dir := range dirs {
		data, err := os.ReadFile(filepath.Join(dir, filename))
		if err == nil {
			return data, nil
		}
	}
	return nil, os.ErrNotExist
}

func readFirstTheme(dirs []string, filename string) ([]byte, error) {
	for _, dir := range dirs {
		data, err := os.ReadFile(filepath.Join(dir, "themes", filename))
		if err == nil {
			return data, nil
		}
	}
	return nil, os.ErrNotExist
}
