package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

func TestEmptyFormatterOutputPreservesEdits(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("test formatter requires true")
	}
	for _, mode := range []string{"manual", "save"} {
		t.Run(mode, func(t *testing.T) {
			settings := config.DefaultSettings()
			settings.Formatters = map[string]string{"js": "true"}
			settings.Editor.FormatOnSave = mode == "save"
			disabled := false
			settings.LSP.Enabled = &disabled
			a := buildTestApp(t, settings)
			t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
			path := filepath.Join(t.TempDir(), "t.js")
			original := "banana\napple\n"
			if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := a.EditorGroup.OpenFile(path); err != nil {
				t.Fatal(err)
			}
			editor := a.EditorGroup.Editor
			editor.ExecCommand(&undo.InsertStringCommand{Line: 0, Col: 0, Text: "zz"})
			editor.FlushOnChange()
			version := editor.Buf.Version()
			wantDisk := original
			if mode == "save" {
				a.SaveFile()
				wantDisk = "zz" + original
			} else {
				a.RunExternalFormatter()
			}
			if got := strings.Join(editor.Buf.Lines, "\n"); got != "zz"+original {
				t.Errorf("buffer = %q, want %q", got, "zz"+original)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != wantDisk {
				t.Errorf("saved file = %q, error = %v, want %q", data, err, wantDisk)
			}
			if editor.Buf.Version() != version || editor.Buf.Dirty != (mode == "manual") {
				t.Errorf("rejected output changed buffer state: version %d -> %d, dirty=%v", version, editor.Buf.Version(), editor.Buf.Dirty)
			}
			if !strings.Contains(a.Status.Notification, "no output") {
				t.Errorf("missing formatter warning: %q", a.Status.Notification)
			}
			a.EditorGroup.Undo()
			if got := strings.Join(editor.Buf.Lines, "\n"); got != original {
				t.Errorf("undo = %q, want the user's edit undone to %q", got, original)
			}
		})
	}
}

func TestEmptyFormatterOutputForEmptyDocument(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("test formatter requires true")
	}
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
	a.EditorGroup.NewFile()
	buf := a.EditorGroup.ActiveBuffer()
	version := buf.Version()
	if err := a.applyExternalFormatter("true", "empty.txt"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(buf.Lines, []string{""}) || buf.Version() != version || buf.Dirty {
		t.Fatalf("empty document changed: lines=%q, version=%d, dirty=%v", buf.Lines, buf.Version(), buf.Dirty)
	}
}

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

func TestFormatterAppendedNewlineHonorsSavePolicy(t *testing.T) {
	if _, err := exec.LookPath("awk"); err != nil {
		t.Skip("test formatter requires awk")
	}
	for _, show := range []bool{false, true} {
		for _, insert := range []bool{false, true} {
			t.Run(fmt.Sprintf("show=%v/insert=%v", show, insert), func(t *testing.T) {
				settings := config.DefaultSettings()
				settings.Editor.ShowTrailingNewline = &show
				settings.Editor.InsertFinalNewline = insert
				a := buildTestApp(t, settings)
				t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
				path := filepath.Join(t.TempDir(), "README.md")
				text := "# README\n\nText.\n"
				if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
				a.EditorGroup.OpenFile(path)
				buf := a.EditorGroup.ActiveBuffer()
				lines, version := slices.Clone(buf.Lines), buf.Version()
				changes := 0
				a.EditorGroup.Editor.OnChange = func() { changes++ }
				if err := a.applyExternalFormatter("awk 1", path); err != nil {
					t.Fatal(err)
				}
				wantEdit := !show && !insert
				wantChanges := 0
				if wantEdit {
					wantChanges = 1
				}
				if (buf.Version() != version) != wantEdit || buf.Dirty != wantEdit || changes != wantChanges {
					t.Fatalf("newline-only formatting: lines=%q, versionChanged=%v, dirty=%v, changes=%d; wantEdit=%v", buf.Lines, buf.Version() != version, buf.Dirty, changes, wantEdit)
				}
				if !wantEdit && !slices.Equal(buf.Lines, lines) {
					t.Fatalf("formatter inserted a visible trailing line: %q -> %q", lines, buf.Lines)
				}
				version = buf.Version()
				if err := a.applyExternalFormatter("awk 1", path); err != nil {
					t.Fatal(err)
				}
				if buf.Version() != version {
					t.Fatal("repeated formatting emitted another edit")
				}
			})
		}
	}
}

func TestFormatterPreservesAdditionalFinalNewlines(t *testing.T) {
	if _, err := exec.LookPath("printf"); err != nil {
		t.Skip("test formatter requires printf")
	}
	for _, newlines := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("newlines=%d", newlines), func(t *testing.T) {
			settings := config.DefaultSettings()
			show := false
			settings.Editor.ShowTrailingNewline = &show
			settings.Editor.InsertFinalNewline = true
			a := buildTestApp(t, settings)
			t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
			path := filepath.Join(t.TempDir(), "README.md")
			if err := os.WriteFile(path, []byte("foo\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			a.EditorGroup.OpenFile(path)
			formatter := "printf bar" + strings.Repeat("\\n", newlines)
			if err := a.applyExternalFormatter(formatter, path); err != nil {
				t.Fatal(err)
			}
			if !a.EditorGroup.Save() {
				t.Fatal("save failed")
			}
			want := "bar" + strings.Repeat("\n", newlines)
			got, err := os.ReadFile(path)
			if err != nil || string(got) != want {
				t.Fatalf("formatter final newlines were lost: got %q, want %q, error %v", got, want, err)
			}
			buf := a.EditorGroup.ActiveBuffer()
			version := buf.Version()
			changes := 0
			a.EditorGroup.Editor.OnChange = func() { changes++ }
			if err := a.applyExternalFormatter(formatter, path); err != nil {
				t.Fatal(err)
			}
			if buf.Version() != version || buf.Dirty || changes != 0 {
				t.Fatalf("formatting after save emitted another edit: lines=%q, dirty=%v, changes=%d", buf.Lines, buf.Dirty, changes)
			}
		})
	}
}
