package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func TestCustomSettingsWritesStayInActiveFile(t *testing.T) {
	for _, mode := range []string{"toggle", "form", "plugin", "favorites", "invalid", "invalid-default", "missing", "open"} {
		t.Run(mode, func(t *testing.T) {
			previous := config.OverrideConfigDir
			config.OverrideConfigDir = t.TempDir()
			t.Cleanup(func() { config.OverrideConfigDir = previous })
			defaultPath := filepath.Join(config.OverrideConfigDir, "settings.json")
			original := `{"editor":{"tabSize":6},"defaultOnly":true}`
			if mode == "invalid-default" {
				original = `{"editor":`
			}
			if err := os.WriteFile(defaultPath, []byte(original), 0o644); err != nil {
				t.Fatal(err)
			}
			custom := filepath.Join(t.TempDir(), "custom.json")
			contents := `{"editor":{"tabSize":7},"customOnly":true}`
			if err := os.WriteFile(custom, []byte(contents), 0o644); err != nil {
				t.Fatal(err)
			}
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			relative, err := filepath.Rel(cwd, custom)
			if err != nil {
				t.Fatal(err)
			}
			cfg := config.Load(relative)
			borders := BuildBorderSet(cfg.Theme.Borders)
			a := BuildAppFromConfig(&cfg, &borders, workspace.New(nil), nil)
			t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
			t.Chdir(t.TempDir())
			switch mode {
			case "form":
				a.ShowSettings()
				a.settingsView.working.Editor.WordWrap = true
				a.ApplySettingsView()
			case "plugin":
				if err := NewPluginSettingsAPI(a).Set("editor.wordWrap", true); err != nil {
					t.Fatal(err)
				}
			case "favorites":
				if err := os.WriteFile(custom, []byte(`{"editor":{"tabSize":3},"customOnly":true}`), 0o644); err != nil {
					t.Fatal(err)
				}
				a.saveFavorites([]string{"/favorite"}, "saved")
			case "open":
				a.Reg = command.NewRegistry()
				registerPaletteCommands(a)
				a.Reg.Execute("settings.open")
				if got := a.EditorGroup.ActiveFilePath(); !samePath(got, custom) {
					t.Errorf("opened %q, want %q", got, custom)
				}
			case "invalid":
				contents = `{"editor":`
				if err := os.WriteFile(custom, []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
				a.ToggleWordWrap()
				if !strings.Contains(a.Status.Notification, custom) {
					t.Errorf("missing custom-path error: %s", a.Status.Notification)
				}
			case "missing":
				if err := os.Remove(custom); err != nil {
					t.Fatal(err)
				}
				a.ToggleWordWrap()
			default:
				a.ToggleWordWrap()
			}
			data, err := os.ReadFile(defaultPath)
			if err != nil || string(data) != original {
				t.Errorf("default file changed: %s, %v", data, err)
			}
			if mode == "invalid" {
				data, err = os.ReadFile(custom)
				if err != nil || string(data) != contents {
					t.Errorf("invalid custom file overwritten: %s, %v", data, err)
				}
				return
			}
			got, err := config.ReadSettingsFile(custom)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "favorites" {
				if got.Editor.TabSize != 3 || len(got.Welcome.Favorites) != 1 {
					t.Errorf("favorites lost disk settings: %+v", got.Welcome)
				}
			} else if mode != "open" && !got.Editor.WordWrap {
				t.Error("custom file was not updated")
			}
			if string(got.Extra["customOnly"]) != "true" {
				t.Error("custom extension settings lost")
			}
		})
	}
}

func TestPluginDefaultUsesExplicitSavedKeys(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.settingsFile = filepath.Join(t.TempDir(), "settings.json")
	api := NewPluginSettingsAPI(a)
	saved, err := api.SavedSettings()
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := saved.Get("editor.tabSize"); exists {
		t.Fatal("built-in default counted as a user setting")
	}
	if err = saved.Set("editor.tabSize", 7); err != nil {
		t.Fatal(err)
	}
	if a.Settings.Editor.TabSize != 7 {
		t.Fatal("did not apply installed default")
	}
	saved, err = api.SavedSettings()
	if err != nil {
		t.Fatal(err)
	}
	if err = saved.Set("editor.tabSize", nil); err != nil {
		t.Fatal(err)
	}
	if a.Settings.Editor.TabSize != config.DefaultSettings().Editor.TabSize {
		t.Fatal("did not restore built-in tab size")
	}
	saved, err = api.SavedSettings()
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := saved.Get("editor.tabSize"); exists {
		t.Fatal("deleted key still present on disk")
	}
}

func TestPluginNestedDefaultsOnlyRemoveObjectsCreatedByInstall(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.settingsFile = filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(a.settingsFile, []byte(`{"prefs":{"empty":{},"cmd":"custom"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	plug := filepath.Join(dir, "defaults")
	if err := os.Mkdir(plug, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"defaults","version":"1","entry":"main.ttt.lua","permissions":{"settings":true,"settings_keys":["prefs.*"]}}`
	hook := `local settings=require("ttt.settings")
	require("ttt").on_install(function()
	  settings.set_default("prefs.cmd.path","tool")
	  settings.set_default("prefs.empty.path","tool")
	  settings.set_default("prefs.owned",{enabled=true})
	  settings.set_default("prefs.owned.extra.path","tool")
	end)`
	for name, text := range map[string]string{"plugin.ttt.json": manifest, "main.ttt.lua": hook} {
		if err := os.WriteFile(filepath.Join(plug, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m := plugin.NewManager(dir, filepath.Join(t.TempDir(), "registry.json"))
	defer m.Shutdown()
	m.SetSettingsAPI(NewPluginSettingsAPI(a))
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatal(pending)
	}
	if err := m.ApproveAndLoad(pending[0]); err != nil {
		t.Fatal(err)
	}
	saved, err := NewPluginSettingsAPI(a).SavedSettings()
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"prefs.empty.path": "tool", "prefs.owned.extra.path": "tool", "prefs.cmd": "custom"} {
		if got, ok := saved.Get(key); !ok || got != want {
			t.Fatalf("%s = %#v, want %q", key, got, want)
		}
	}
	if err := m.RunUninstallHook("defaults"); err != nil {
		t.Fatal(err)
	}
	obj, err := config.ReadSettingsObject(a.settingsFile)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"prefs": map[string]any{"empty": map[string]any{}, "cmd": "custom"}}
	if !reflect.DeepEqual(obj, want) {
		t.Fatalf("remaining user settings = %#v, want %#v", obj, want)
	}
}
