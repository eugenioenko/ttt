package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/plugin"
)

func TestPluginThemeLifecycle(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	a := buildTestApp(t, config.DefaultSettings())
	p := &plugin.Plugin{Name: "theme-pack", Dir: t.TempDir(), Manifest: plugin.Manifest{
		Themes: []plugin.Theme{{Path: "dark.json"}, {Path: "light.json", Name: "day"}},
	}}
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(p.Dir, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("dark.json", `{"default":{"bg":"#123456"}}`)
	write("light.json", `{"default":{"bg":"#ffffff"}}`)
	a.installPluginThemes(p)
	for _, name := range []string{"dark", "day"} {
		if !slices.Contains(config.ListThemes(), "plugins/theme-pack/"+name) {
			t.Fatalf("missing theme %s: %v", name, config.ListThemes())
		}
	}
	a.Settings.Theme = "plugins/theme-pack/dark"
	write("dark.json", `{"default":{"bg":"#654321"}}`)
	a.installPluginThemes(p)
	loaded, err := config.LoadTheme(a.Settings.Theme)
	if err != nil || loaded.Default.Bg != "#654321" {
		t.Fatalf("updated theme = %+v, %v", loaded.Default, err)
	}
	if a.Palette.Bg != BuildTerminalPalette(loaded).Bg {
		t.Fatal("active theme palette was not refreshed")
	}
	write("light.json", `invalid`)
	a.installPluginThemes(p)
	if _, err := config.LoadTheme("plugins/theme-pack/day"); err != nil {
		t.Fatal("invalid update removed an installed theme:", err)
	}
	p.Manifest.Themes = p.Manifest.Themes[:1]
	a.Settings.Theme = "plugins/theme-pack/day"
	a.installPluginThemes(p)
	if slices.Contains(config.ListThemes(), "plugins/theme-pack/day") {
		t.Fatal("removed contribution remains in picker")
	}
	if a.Settings.Theme != "" || config.LoadSettings().Theme != "" {
		t.Fatal("removing the active contribution did not save the default selection")
	}
	a.Settings.Theme = "plugins/theme-pack/dark"
	other := &plugin.Plugin{Name: "another", Dir: p.Dir, Manifest: p.Manifest}
	a.installPluginThemes(other)
	manual := filepath.Join(config.OverrideConfigDir, "themes", "dark.json")
	os.WriteFile(manual, []byte(`{}`), 0644)
	a.uninstallPluginThemes(p.Name)
	if a.Settings.Theme != "" || config.LoadSettings().Theme != "" {
		t.Fatal("uninstall did not save the default theme selection")
	}
	if a.Palette.Bg != BuildTerminalPalette(config.DefaultTheme()).Bg {
		t.Fatal("uninstall did not apply default colors")
	}
	if slices.Contains(config.ListThemes(), "plugins/theme-pack/dark") {
		t.Fatal("uninstalled theme remains in picker")
	}
	if !slices.Contains(config.ListThemes(), "plugins/another/dark") {
		t.Fatal("uninstall removed another plugin's theme")
	}
	if _, err := os.Stat(manual); err != nil {
		t.Fatal("uninstall removed manual theme:", err)
	}
}

func TestPluginThemeDestinationIsolation(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	for _, name := range []string{".", "..", "../outside", "a/b", ""} {
		if err := replacePluginThemes(name, nil); err == nil {
			t.Errorf("accepted invalid plugin name %q", name)
		}
	}
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "keep.json")
	os.WriteFile(sentinel, []byte(`{}`), 0644)
	if err := os.Symlink(outside, filepath.Join(config.OverrideConfigDir, "themes")); err != nil {
		t.Skip(err)
	}
	if err := replacePluginThemes("pack", map[string][]byte{"dark.json": []byte(`{}`)}); err == nil {
		t.Fatal("accepted symlinked destination")
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatal("modified external theme directory:", err)
	}
}
