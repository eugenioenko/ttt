package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

func TestSavingSettingsAppliesBeforeNextSettingsWrite(t *testing.T) {
	for _, mode := range []string{"save", "save-as", "relative", "symlink", "form", "unrelated"} {
		t.Run(mode, func(t *testing.T) {
			a, path := settingsSaveApp(t)
			savedPath := path
			switch mode {
			case "relative":
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				savedPath, err = filepath.Rel(cwd, path)
				if err != nil {
					t.Fatal(err)
				}
			case "symlink":
				savedPath = filepath.Join(t.TempDir(), "linked.json")
				if err := os.Symlink(path, savedPath); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			case "unrelated":
				savedPath = filepath.Join(t.TempDir(), "settings.json")
				if err := os.WriteFile(savedPath, []byte("{}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			contents := `{"theme":"dracula","editor":{"tabSize":2},"formatters":{"go":"gofmt"},"vim":{"enabled":true}}`
			if mode == "save-as" {
				a.NewFileFromText(contents)
				a.saveFileAs(savedPath, nil)
			} else {
				if err := a.EditorGroup.OpenFile(savedPath); err != nil {
					t.Fatal(err)
				}
				replaceSettingsBuffer(a, contents)
				a.SaveFile()
			}
			if mode == "unrelated" {
				if a.Settings.Theme != "" || a.Settings.Editor.TabSize != 7 {
					t.Fatal("saving an unrelated settings.json applied its settings")
				}
				return
			}
			if a.Settings.Theme != "dracula" || a.Settings.Editor.TabSize != 2 || a.EditorGroup.Editor.TabSize != 2 || a.Settings.Formatters["go"] != "gofmt" {
				t.Errorf("saved settings were not applied: theme=%q tabSize=%d editorTabSize=%d formatters=%v", a.Settings.Theme, a.Settings.Editor.TabSize, a.EditorGroup.Editor.TabSize, a.Settings.Formatters)
			}
			if mode == "form" {
				a.ShowSettings()
				a.ApplySettingsView()
			} else {
				a.ToggleLineNumbers()
			}
			disk, err := config.ReadSettings()
			if err != nil {
				t.Fatal(err)
			}
			var vim map[string]bool
			if err := json.Unmarshal(disk.Extra["vim"], &vim); err != nil || !vim["enabled"] || disk.Theme != "dracula" || disk.Formatters["go"] != "gofmt" || disk.Editor.TabSize != 2 {
				t.Errorf("next settings write lost saved values: theme=%q tabSize=%d formatters=%v vim=%s", disk.Theme, disk.Editor.TabSize, disk.Formatters, disk.Extra["vim"])
			}
		})
	}
}

func TestSavingInvalidSettingsPreservesSessionAndCanRecover(t *testing.T) {
	a, path := settingsSaveApp(t)
	if err := a.EditorGroup.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	invalid := `{"editor":{"tabSize":2,},"vim":{"enabled":true}}`
	replaceSettingsBuffer(a, invalid)
	a.SaveFile()
	if a.EditorGroup.ActiveBuffer().Dirty {
		t.Fatal("raw JSON save failed")
	}
	if a.Settings.Editor.TabSize != 7 || !strings.Contains(a.Status.Notification, "Could not load settings:") {
		t.Errorf("invalid save changed settings or hid its error: tabSize=%d notification=%q", a.Settings.Editor.TabSize, a.Status.Notification)
	}
	a.ToggleLineNumbers()
	data, err := os.ReadFile(path)
	if err != nil || strings.TrimSpace(string(data)) != invalid {
		t.Fatalf("invalid JSON overwritten: %q, error %v", data, err)
	}
	replaceSettingsBuffer(a, `{"editor":{"tabSize":3},"vim":{"enabled":true}}`)
	a.SaveFile()
	if a.Settings.Editor.TabSize != 3 {
		t.Errorf("repair did not apply: tabSize=%d", a.Settings.Editor.TabSize)
	}
}

func TestFailedSettingsSaveDoesNotApplyEdits(t *testing.T) {
	a, path := settingsSaveApp(t)
	if err := a.EditorGroup.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	replaceSettingsBuffer(a, `{"editor":{"tabSize":2}}`)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	a.doSaveFile(nil)
	if !a.EditorGroup.ActiveBuffer().Dirty || a.Settings.Editor.TabSize != 7 {
		t.Errorf("failed save applied settings: dirty=%v tabSize=%d", a.EditorGroup.ActiveBuffer().Dirty, a.Settings.Editor.TabSize)
	}
}

func settingsSaveApp(t *testing.T) (*App, string) {
	t.Helper()
	previous := config.OverrideConfigDir
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = previous })
	path := filepath.Join(config.OverrideConfigDir, "settings.json")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := config.DefaultSettings()
	settings.Editor.TabSize = 7
	a := buildTestApp(t, settings)
	t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
	return a, path
}

func replaceSettingsBuffer(a *App, text string) {
	editor := a.EditorGroup.Editor
	editor.ExecCommand(&undo.ReplaceLinesCommand{
		OldLines: append([]string(nil), editor.Buf.Lines...),
		NewLines: []string{text},
	})
	editor.FlushOnChange()
}
