package app

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestKeybindingChangesOnlyApplyAfterSave(t *testing.T) {
	for _, action := range []string{"edit", "reset", "clear"} {
		t.Run(action, func(t *testing.T) {
			previous := config.OverrideConfigDir
			config.OverrideConfigDir = t.TempDir()
			t.Cleanup(func() { config.OverrideConfigDir = previous })
			a := buildTestApp(t, config.DefaultSettings())
			t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
			a.Reg = command.NewRegistry()
			a.Keybindings = config.DefaultKeybindings()
			a.Reg.Register(command.Command{ID: "file.save", Title: "Save"})
			if action == "reset" {
				for i := range a.Keybindings {
					if a.Keybindings[i].Command == "file.save" {
						a.Keybindings[i].Key = "ctrl+alt+s"
					}
				}
			}
			a.RebindKeys()
			before := slices.Clone(a.Keybindings)
			beforeCommand, _ := a.Reg.Get("file.save")
			if err := config.SaveKeybindings(before); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(config.OverrideConfigDir, "keybindings.json")
			saved, readErr := os.ReadFile(path)
			if readErr != nil && !os.IsNotExist(readErr) {
				t.Fatal(readErr)
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
			blocker := filepath.Join(path, "keep")
			if err := os.WriteFile(blocker, []byte("keep"), 0o644); err != nil {
				t.Fatal(err)
			}

			a.ShowKeybindings()
			w := a.Root.TopOverlayWidget().(*ui.KeybindingsWidget)
			change := func() {
				switch action {
				case "edit":
					w.OnEdit("file.save", "ctrl+alt+s")
				case "reset":
					w.OnReset("file.save")
				case "clear":
					w.OnClear("file.save")
				}
			}
			change()
			if !reflect.DeepEqual(a.Keybindings, before) {
				t.Error("failed save changed active bindings")
			}
			gotCommand, _ := a.Reg.Get("file.save")
			if gotCommand.Shortcut != beforeCommand.Shortcut || w.GetShortcut("file.save") != beforeCommand.Shortcut {
				t.Error("failed save changed displayed or registered shortcut")
			}
			if !strings.Contains(a.Status.Notification, "Could not save keyboard shortcuts:") {
				t.Errorf("missing save error: %q", a.Status.Notification)
			}
			if data, err := os.ReadFile(blocker); err != nil || string(data) != "keep" {
				t.Fatal("failed save changed destination")
			}

			if err := os.Remove(blocker); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if readErr == nil {
				if err := os.WriteFile(path, saved, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			change()
			want := ""
			if action == "edit" {
				want = "ctrl+alt+s"
			}
			if action == "reset" {
				want = "ctrl+s"
			}
			wantDisplay := FormatKeyBinding(want, a.Root.ControlModifiers)
			gotCommand, _ = a.Reg.Get("file.save")
			if gotCommand.Shortcut != wantDisplay || w.GetShortcut("file.save") != wantDisplay {
				t.Fatalf("successful retry: registered=%q displayed=%q want=%q", gotCommand.Shortcut, w.GetShortcut("file.save"), wantDisplay)
			}
			loaded := config.Load("")
			got := ""
			for _, kb := range loaded.Keybindings {
				if kb.Command == "file.save" {
					got = kb.Key
				}
			}
			if got != want {
				t.Errorf("reloaded binding = %q, want %q", got, want)
			}
		})
	}
}
