package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func TestInvalidSettingsAreReportedAndPreserved(t *testing.T) {
	for _, mode := range []string{"startup", "reload", "toggle", "form"} {
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
			case "form":
				a.ShowSettings()
				a.ApplySettingsView()
				if strings.Contains(a.settingsView.status.Config.Text, "Settings applied") {
					t.Error("form reported successful persistence")
				}
			}
			if !strings.Contains(a.Status.Notification, "settings") || strings.Contains(a.Status.Notification, "Settings reloaded") {
				t.Errorf("missing settings error: %q", a.Status.Notification)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != invalid {
				t.Errorf("settings file changed: %s, error %v", got, err)
			}
		})
	}
}
