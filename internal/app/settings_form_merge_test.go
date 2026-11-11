package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestSettingsFormPreservesConcurrentChanges(t *testing.T) {
	previous := config.OverrideConfigDir
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = previous })
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
	a.ShowSettings()
	v := a.settingsView
	lineNumbers := !a.Settings.Editor.LineNumbers
	v.working.Editor.LineNumbers = lineNumbers

	for _, size := range []int{7, 3} {
		live := *a.Settings
		if size == 3 {
			lineNumbers = !lineNumbers
			live.Editor.LineNumbers = lineNumbers
		}
		live.Theme = "dracula"
		live.Editor.WordWrap = true
		live.Editor.TabSize = size
		live.Terminal.Shell = "/bin/bash"
		a.ApplySettings(live)
		a.ApplySettingsView()

		for _, got := range []config.Settings{*a.Settings, config.LoadSettings()} {
			if got.Theme != "dracula" || !got.Editor.WordWrap || got.Editor.TabSize != size || got.Terminal.Shell != "/bin/bash" {
				t.Errorf("Apply reverted concurrent changes: theme=%q wrap=%v tabSize=%d shell=%q", got.Theme, got.Editor.WordWrap, got.Editor.TabSize, got.Terminal.Shell)
			}
			if got.Editor.LineNumbers != lineNumbers {
				t.Error("Apply lost the form edit")
			}
		}
	}
}

func TestSettingsFormFailedSaveRetainsDraftForRetry(t *testing.T) {
	previous := config.OverrideConfigDir
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = previous })
	path := filepath.Join(config.OverrideConfigDir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"editor":`), 0o644); err != nil {
		t.Fatal(err)
	}
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
	a.ShowSettings()
	want := !a.Settings.Editor.LineNumbers
	a.settingsView.working.Editor.LineNumbers = want
	a.ApplySettingsView()
	live := *a.Settings
	live.Editor.LineNumbers = !want
	live.Theme = "dracula"
	a.ApplySettings(live)
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	a.ApplySettingsView()
	got := config.LoadSettings()
	if got.Editor.LineNumbers != want || got.Theme != "dracula" {
		t.Fatalf("retry lost draft or concurrent change: lineNumbers=%v theme=%q", got.Editor.LineNumbers, got.Theme)
	}
}

func TestSettingsFormEditedFieldWinsConflict(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
	a.ShowSettings()
	v := a.settingsView
	v.working.Editor.TabSize = 7
	v.working.Theme = "dracula"
	live := *a.Settings
	live.Editor.TabSize = 2
	live.Theme = "monokai"
	v.commitTo(&live)
	if live.Editor.TabSize != 7 || live.Theme != "dracula" {
		t.Fatalf("explicit form edits did not win: tabSize=%d theme=%q", live.Editor.TabSize, live.Theme)
	}
}
