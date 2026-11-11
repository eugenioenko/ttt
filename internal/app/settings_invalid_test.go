package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func TestInvalidSettingsAreReportedAndPreserved(t *testing.T) {
	for _, mode := range []string{"startup", "reload", "toggle", "syntax", "menu", "language", "form"} {
		t.Run(mode, func(t *testing.T) {
			previous := config.OverrideConfigDir
			config.OverrideConfigDir = t.TempDir()
			t.Cleanup(func() { config.OverrideConfigDir = previous })
			path := filepath.Join(config.OverrideConfigDir, "settings.json")
			invalid := `{"editor":{"tabSize":2,},"vim":{"enabled":true}}`
			if err := os.WriteFile(path, []byte(invalid), 0o644); err != nil {
				t.Fatal(err)
			}
			settings := config.DefaultSettings()
			settings.Editor.TabSize = 7
			var a *App
			if mode == "startup" {
				cfg := config.Load("")
				borders := BuildBorderSet(cfg.Theme.Borders)
				a = BuildAppFromConfig(&cfg, &borders, workspace.New(nil), nil)
			} else {
				a = buildTestApp(t, settings)
			}
			t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
			switch mode {
			case "reload":
				a.ReloadSettings()
				if a.Settings.Editor.TabSize != 7 {
					t.Error("failed reload replaced the current settings")
				}
			case "toggle":
				a.ToggleWordWrap()
			case "syntax":
				a.ToggleSyntaxHighlight()
			case "menu":
				a.ToggleMenuBar()
			case "language":
				a.Settings.Language.Locale = "de"
				a.SaveAndApplySettings()
			case "form":
				a.ShowSettings()
				a.ApplySettingsView()
				if strings.Contains(a.settingsView.status.Config.Text, "Settings applied") {
					t.Error("form reported successful persistence")
				}
			}
			if !strings.Contains(a.Status.Notification, path) || strings.Contains(a.Status.Notification, "Settings reloaded") {
				t.Errorf("missing settings error: %q", a.Status.Notification)
			}

			hint := ""
			if mode == "menu" {
				hint = a.menuBarRestoreHint()
			}
			if mode == "language" {
				hint = i18n.Text("status.languageRestartRequired")
			}
			if hint != "" {
				found := false
				for _, line := range a.Output.Lines {
					found = found || line.Message == hint
				}
				if !found {
					t.Errorf("missing guidance in Output: %q", hint)
				}
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != invalid {
				t.Errorf("settings file changed: %s, error %v", got, err)
			}
		})
	}
}

func TestFavoritesPreserveSettingsAfterReadError(t *testing.T) {
	if !isolatedSettingsExecutable(t) {
		return
	}
	previous := config.OverrideConfigDir
	config.OverrideConfigDir = ""
	t.Cleanup(func() { config.OverrideConfigDir = previous })
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	adjacent := filepath.Join(filepath.Dir(exe), "config")
	if err := os.Mkdir(adjacent, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(adjacent) })
	if err := os.Symlink("settings.json", filepath.Join(adjacent, "settings.json")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	homeConfig := filepath.Join(os.Getenv("HOME"), ".config", "ttt")
	if err := os.MkdirAll(homeConfig, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(homeConfig, "settings.json")
	original := `{"theme":"dracula","editor":{"tabSize":7},"vim":{"enabled":true}}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
	a.saveFavorites([]string{"~/project"}, "Saved favorite")
	if !strings.Contains(a.Status.Notification, "Failed to save favorites") {
		t.Errorf("missing read error: %q", a.Status.Notification)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != original {
		t.Errorf("home settings changed after adjacent read error: %s, error %v", got, err)
	}
}
