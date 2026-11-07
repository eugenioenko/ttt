package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestFileDeleteTaskDeliveryAndShutdown(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	for _, shutdown := range []bool{false, true} {
		t.Run(map[bool]string{false: "delivery", true: "shutdown"}[shutdown], func(t *testing.T) {
			a := buildTestApp(t, config.DefaultSettings())
			t.Cleanup(a.CloseTasks)
			baselineTasks := len(a.tasks.tasks)
			posted := make(chan func(), 1)
			a.tasks.post = func(fn func()) error { posted <- fn; return nil }
			contexts := make(chan context.Context, 1)
			reloaded := false
			a.confirmFileDelete("note.txt", func() { reloaded = true }, func(ctx context.Context, _ string) error {
				contexts <- ctx
				return nil
			})
			a.Root.TopOverlayWidget().(*ui.WidgetAdapter).Inner().(*widgets.DialogWidget).Buttons[1].Handler()
			var apply func()
			select {
			case apply = <-posted:
			case <-time.After(time.Second):
				t.Fatal("delete task did not post completion")
			}
			if reloaded || a.Root.HasOverlay() {
				t.Fatal("worker applied UI changes or left the confirmation open")
			}
			ctx := <-contexts
			if shutdown {
				a.CloseTasks()
				if ctx.Err() != context.Canceled {
					t.Fatal("shutdown did not cancel the delete task")
				}
			}
			apply()
			remaining := baselineTasks
			if shutdown {
				remaining = 0
			}
			if reloaded == shutdown || len(a.tasks.tasks) != remaining {
				t.Fatalf("reload=%v, remaining tasks=%d", reloaded, len(a.tasks.tasks))
			}
		})
	}
}

func TestPermanentDeleteFinishesBeforeQuit(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(a.CloseTasks)
	baselineTasks := len(a.tasks.tasks)
	posted := make(chan func(), 1)
	a.tasks.post = func(fn func()) error { posted <- fn; return nil }
	dir := filepath.Join(t.TempDir(), "folder")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "child.txt"), []byte("delete me"), 0o600); err != nil {
		t.Fatal(err)
	}
	finished := false
	a.startFileDelete(dir, false, func() { finished = true }, nil)
	if !finished || len(a.tasks.tasks) != baselineTasks {
		t.Fatal("permanent deletion returned with unfinished work")
	}
	running := true
	a.Running = &running
	a.Quit()
	if running {
		t.Fatal("quit did not complete after permanent deletion")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("quit left directory contents behind: %v", err)
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
			a.confirmFileDelete(path, func() { reloaded = true }, func(_ context.Context, got string) error {
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
