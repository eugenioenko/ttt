package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
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
