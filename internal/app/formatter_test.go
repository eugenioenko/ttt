package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

func TestUnchangedFormatterDoesNotEmitAnotherEdit(t *testing.T) {
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("test formatter requires cat")
	}
	for _, trailing := range []bool{false, true} {
		for _, ending := range []string{"\n", "\n\n"} {
			t.Run(fmt.Sprintf("trailing=%v/newlines=%d", trailing, len(ending)), func(t *testing.T) {
				settings := config.DefaultSettings()
				settings.Editor.ShowTrailingNewline = &trailing
				a := buildTestApp(t, settings)
				t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
				path := filepath.Join(t.TempDir(), "README.md")
				if err := os.WriteFile(path, []byte("# README\n\nUnchanged paragraph."+ending), 0o644); err != nil {
					t.Fatal(err)
				}
				a.EditorGroup.OpenFile(path)
				buf := a.EditorGroup.ActiveBuffer()
				version := buf.Version()
				changes := 0
				a.EditorGroup.Editor.OnChange = func() { changes++ }
				if err := a.applyExternalFormatter("cat", path); err != nil {
					t.Fatal(err)
				}
				if buf.Version() != version || buf.Dirty || changes != 0 {
					t.Fatalf("unchanged formatter emitted an edit: version %d -> %d, dirty=%v, events=%d", version, buf.Version(), buf.Dirty, changes)
				}
			})
		}
	}
}

func TestFormatterPreservesRealFinalNewlineChanges(t *testing.T) {
	if _, err := exec.LookPath("printf"); err != nil {
		t.Skip("test formatter requires printf")
	}
	for _, add := range []bool{false, true} {
		t.Run(fmt.Sprintf("add=%v", add), func(t *testing.T) {
			settings := config.DefaultSettings()
			trailing := false
			settings.Editor.ShowTrailingNewline = &trailing
			settings.Editor.InsertFinalNewline = false
			a := buildTestApp(t, settings)
			t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
			path := filepath.Join(t.TempDir(), "README.md")
			if err := os.WriteFile(path, []byte("foo"), 0o644); err != nil {
				t.Fatal(err)
			}
			a.EditorGroup.OpenFile(path)
			formatter, want := "printf foo", "foo"
			if add {
				formatter, want = "printf foo\\n", "foo\n"
			} else {
				a.EditorGroup.Editor.ExecCommand(&undo.SplitLineCommand{Line: 0, Col: 3})
			}
			if err := a.applyExternalFormatter(formatter, path); err != nil {
				t.Fatal(err)
			}
			if !a.EditorGroup.Save() {
				t.Fatal("save failed")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != want {
				t.Fatalf("formatter newline change was lost: got %q, want %q, error %v", got, want, err)
			}
		})
	}
}
