package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/view"
)

func TestConfigCreationFailureKeepsCurrentTab(t *testing.T) {
	for _, name := range []string{"settings", "keybindings"} {
		t.Run(name, func(t *testing.T) {
			previous := config.OverrideConfigDir
			config.OverrideConfigDir = t.TempDir()
			t.Cleanup(func() { config.OverrideConfigDir = previous })
			a := buildTestApp(t, config.DefaultSettings())
			t.Cleanup(func() { a.CloseTasks(); a.Repository.Close(); a.EditorGroup.CloseAllTabs() })
			a.Reg = command.NewRegistry()
			registerPaletteCommands(a)
			original := a.EditorGroup.ActiveBuffer()
			path := filepath.Join(config.OverrideConfigDir, name+".json")
			if err := os.Symlink(filepath.Join(config.OverrideConfigDir, "missing", name+".json"), path); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			open := func() {
				if name == "settings" {
					a.OpenSettingsFile()
				} else if !a.Reg.Execute("keybindings.open") {
					t.Fatal("keybindings command not registered")
				}
			}
			open()
			if a.EditorGroup.ActiveBuffer() != original || a.Status.NotifyLevel != view.NotifyError {
				t.Fatal("failed config creation opened a blank tab or lost its error")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			open()
			if a.EditorGroup.ActiveFilePath() != path {
				t.Fatal("successful retry did not open the config file")
			}
		})
	}
}
