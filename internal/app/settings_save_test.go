package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func TestSettingsSaveRequiresExplicitReload(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(fmt.Sprint(custom), func(t *testing.T) {
			previous := config.OverrideConfigDir
			config.OverrideConfigDir = t.TempDir()
			t.Cleanup(func() { config.OverrideConfigDir = previous })
			path := filepath.Join(config.OverrideConfigDir, "settings.json")
			selected := ""
			if custom {
				path = filepath.Join(t.TempDir(), "custom.json")
				selected = path
			}
			if err := os.WriteFile(path, []byte(`{"editor":{"tabSize":7}}`), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg := config.Load(selected)
			borders := BuildBorderSet(cfg.Theme.Borders)
			a := BuildAppFromConfig(&cfg, &borders, workspace.New(nil), nil)
			t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
			if err := a.EditorGroup.OpenFile(path); err != nil {
				t.Fatal(err)
			}
			editor := a.EditorGroup.Editor
			editor.ExecCommand(&undo.ReplaceLinesCommand{
				OldLines: append([]string(nil), editor.Buf.Lines...),
				NewLines: []string{`{"editor":{"tabSize":2}}`},
			})
			editor.FlushOnChange()
			a.SaveFile()
			if a.Settings.Editor.TabSize != 7 {
				t.Errorf("file save reloaded settings: tabSize=%d", a.Settings.Editor.TabSize)
			}
			disk, err := config.ReadSettingsFile(path)
			if err != nil || disk.Editor.TabSize != 2 {
				t.Fatalf("file was not saved: tabSize=%d, error=%v", disk.Editor.TabSize, err)
			}
			a.ReloadSettings()
			if a.Settings.Editor.TabSize != 2 {
				t.Errorf("explicit reload did not read selected file: tabSize=%d", a.Settings.Editor.TabSize)
			}
		})
	}
}
