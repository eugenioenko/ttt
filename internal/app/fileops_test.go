package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func TestContainingFolder(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := containingFolder(file); got != dir {
		t.Errorf("file: got %q, want %q", got, dir)
	}
	if got := containingFolder(dir); got != dir {
		t.Errorf("dir: got %q, want %q", got, dir)
	}

	// Non-existent path falls back to filepath.Dir.
	missing := filepath.Join(dir, "sub", "gone.txt")
	if got := containingFolder(missing); got != filepath.Dir(missing) {
		t.Errorf("missing: got %q, want %q", got, filepath.Dir(missing))
	}
}

func TestFileDeleteConfirmation(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	for _, tc := range []struct {
		name                   string
		useTrash, cancel, fail bool
	}{
		{name: "trash", useTrash: true},
		{name: "permanent"},
		{name: "trash failure", useTrash: true, fail: true},
		{name: "cancel trash", useTrash: true, cancel: true},
		{name: "cancel permanent", cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := config.DefaultSettings()
			settings.Explorer.UseTrash = tc.useTrash
			a := buildTestApp(t, settings)
			t.Cleanup(a.CloseTasks)
			path := filepath.Join(t.TempDir(), "note.txt")
			if err := os.WriteFile(path, []byte("recover me"), 0o600); err != nil {
				t.Fatal(err)
			}
			moved := filepath.Join(t.TempDir(), "note.txt")
			called, reloaded := false, false
			a.confirmFileDelete(path, func() { reloaded = true }, func(got string) error {
				called = true
				if got != path {
					t.Fatalf("trash target = %q, want %q", got, path)
				}
				if tc.fail {
					return os.ErrPermission
				}
				return os.Rename(got, moved)
			})
			dialog := a.Root.TopOverlayWidget().(*ui.WidgetAdapter).Inner().(*widgets.DialogWidget)
			want := "Permanently delete note.txt? This cannot be undone."
			if tc.useTrash {
				want = "Move note.txt to Trash?"
			}
			if got := dialog.Content.(*widgets.ParagraphWidget).Text; got != want {
				t.Fatalf("confirmation = %q, want %q", got, want)
			}
			// The approved operation must keep matching the displayed confirmation.
			a.Settings.Explorer.UseTrash = !tc.useTrash
			button := 1
			if tc.cancel {
				button = 0
			}
			dialog.Buttons[button].Handler()
			if a.Root.HasOverlay() || called != (tc.useTrash && !tc.cancel) || reloaded != (!tc.cancel && !tc.fail) {
				t.Fatalf("overlay=%v, trash called=%v, reloaded=%v", a.Root.HasOverlay(), called, reloaded)
			}
			if tc.cancel || tc.fail {
				if data, err := os.ReadFile(path); err != nil || string(data) != "recover me" {
					t.Fatalf("source changed after cancellation/failure: %q, %v", data, err)
				}
			} else if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("source still exists: %v", err)
			} else if tc.useTrash {
				if data, err := os.ReadFile(moved); err != nil || string(data) != "recover me" {
					t.Fatalf("trash lost file: %q, %v", data, err)
				}
			}
		})
	}
}
