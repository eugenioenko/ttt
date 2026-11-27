package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/gdamore/tcell/v3"
)

func TestFileChangedStatErrorWarnsAndPreservesBuffer(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{"permission", os.ErrPermission},
		{"other", errors.New("stat failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := buildTestApp(t, config.DefaultSettings())
			t.Cleanup(a.CloseTasks)
			t.Cleanup(a.EditorGroup.CloseAllTabs)
			sim := term.NewSimScreen()
			if err := sim.Init(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(sim.Fini)
			a.Screen = term.NewTcellScreenFrom(sim)
			path := filepath.Join(t.TempDir(), "watched.txt")
			if err := os.WriteFile(path, []byte("original\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := a.EditorGroup.OpenFile(path); err != nil {
				t.Fatal(err)
			}
			a.EditorGroup.Editor.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "Z", 0))
			buf := a.EditorGroup.ActiveBuffer()
			before := strings.Join(buf.Lines, "\n")
			if !buf.Dirty {
				t.Fatal("expected an unsaved edit before the file check")
			}
			statErr := &os.PathError{Op: "stat", Path: path, Err: test.err}

			a.handleFileChanged(path, func(got string) (os.FileInfo, error) {
				if got != path {
					t.Fatalf("stat path = %q, want %q", got, path)
				}
				return nil, statErr
			})

			if got, want := a.Status.Notification, "Failed to check watched.txt: "+statErr.Error(); got != want {
				t.Fatalf("notification = %q, want %q", got, want)
			}
			if a.Status.NotifyLevel != view.NotifyWarning {
				t.Fatalf("notification level = %v, want warning", a.Status.NotifyLevel)
			}
			if a.EditorGroup.ActiveBuffer() != buf || strings.Join(buf.Lines, "\n") != before || !buf.Dirty {
				t.Fatal("failed file check changed the buffer")
			}
			a.EditorGroup.Undo()
			if strings.Join(buf.Lines, "\n") != "original\n" || buf.Dirty {
				t.Fatal("failed file check changed undo history or the save point")
			}
		})
	}
}
