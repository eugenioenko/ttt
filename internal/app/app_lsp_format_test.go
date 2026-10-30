package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/lsp"
)

// formattingEdits replaces the first word of the line, whatever file it lands
// in, so a wrongly applied result is visible.
func formattingEdits() []lsp.TextEdit {
	return []lsp.TextEdit{{
		Range: lsp.Range{
			Start: lsp.Position{Line: 0, Character: 0},
			End:   lsp.Position{Line: 0, Character: 7},
		},
		NewText: "BROKEN",
	}}
}

func formatTestApp(t *testing.T) (*App, string, string) {
	t.Helper()
	dir := t.TempDir()
	first := filepath.Join(dir, "first.go")
	second := filepath.Join(dir, "second.go")
	for path, content := range map[string]string{
		first:  "package first\n",
		second: "package second\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := buildTestApp(t, config.DefaultSettings())
	a.EditorGroup.OpenFile(first)
	t.Cleanup(func() {
		a.CloseTasks()
		a.EditorGroup.CloseAllTabs()
	})
	return a, first, second
}

// A formatting result that arrives for another file must not be applied to
// the file the user is looking at (#876).
func TestFormattingResultForAnotherFileIsDropped(t *testing.T) {
	a, first, second := formatTestApp(t)
	buf := a.EditorGroup.Editor.Buf
	version := buf.Version()

	// The user switches to the other file while the server formats.
	a.EditorGroup.OpenFile(second)

	a.applyFormattingResult(&FormattingResult{
		Path:    first,
		Version: version,
		Buffer:  buf,
		Edits:   formattingEdits(),
	})

	if got := strings.Join(a.EditorGroup.Editor.Buf.Lines, "\n"); got != "package second\n" {
		t.Errorf("the other file was edited: %q", got)
	}
}

// A formatting result for a buffer that changed while the server was
// formatting must not be applied either (#876).
func TestFormattingResultForAChangedBufferIsDropped(t *testing.T) {
	a, first, _ := formatTestApp(t)
	buf := a.EditorGroup.Editor.Buf
	version := buf.Version()

	// The user types while the server formats.
	a.EditorGroup.Editor.ExecCommand(&undo.InsertStringCommand{Line: 0, Col: 13, Text: "X"})

	a.applyFormattingResult(&FormattingResult{
		Path:    first,
		Version: version,
		Buffer:  buf,
		Edits:   formattingEdits(),
	})

	if got := strings.Join(a.EditorGroup.Editor.Buf.Lines, "\n"); got != "package firstX\n" {
		t.Errorf("the changed buffer was edited: %q", got)
	}
}

// Closing and reopening the same path creates a new buffer whose version
// counter can match the old result; its identity must still reject the result
// (#1084).
func TestFormattingResultForAReopenedBufferIsDropped(t *testing.T) {
	a, first, _ := formatTestApp(t)
	old := a.EditorGroup.Editor.Buf
	version := old.Version()

	a.EditorGroup.CloseTab()
	if err := os.WriteFile(first, []byte("package reopened\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.EditorGroup.OpenFile(first)
	if a.EditorGroup.Editor.Buf == old {
		t.Fatal("reopening the path reused the old buffer")
	}
	if got := a.EditorGroup.Editor.Buf.Version(); got != version {
		t.Fatalf("reopened buffer version = %d, want the old %d for this regression", got, version)
	}

	a.applyFormattingResult(&FormattingResult{
		Path:    first,
		Version: version,
		Buffer:  old,
		Edits:   formattingEdits(),
	})

	if got := strings.Join(a.EditorGroup.Editor.Buf.Lines, "\n"); got != "package reopened\n" {
		t.Errorf("the reopened buffer was edited: %q", got)
	}
}

// A formatting result for the file that is open and unchanged is applied.
func TestFormattingResultForTheOpenFileIsApplied(t *testing.T) {
	a, first, _ := formatTestApp(t)
	buf := a.EditorGroup.Editor.Buf
	version := buf.Version()

	a.applyFormattingResult(&FormattingResult{
		Path:    first,
		Version: version,
		Buffer:  buf,
		Edits:   formattingEdits(),
	})

	if got := strings.Join(a.EditorGroup.Editor.Buf.Lines, "\n"); got != "BROKEN first\n" {
		t.Errorf("the formatting was not applied: %q", got)
	}
}

// Multiple formatting edits are applied as a single undo step (#876).
func TestFormattingEditsAreSingleUndoStep(t *testing.T) {
	a, first, _ := formatTestApp(t)
	buf := a.EditorGroup.Editor.Buf
	version := buf.Version()

	edits := []lsp.TextEdit{
		{
			Range: lsp.Range{
				Start: lsp.Position{Line: 0, Character: 0},
				End:   lsp.Position{Line: 0, Character: 7},
			},
			NewText: "package",
		},
		{
			Range: lsp.Range{
				Start: lsp.Position{Line: 0, Character: 8},
				End:   lsp.Position{Line: 0, Character: 13},
			},
			NewText: "formatted",
		},
	}

	a.applyFormattingResult(&FormattingResult{
		Path:    first,
		Version: version,
		Buffer:  buf,
		Edits:   edits,
	})

	if got := strings.Join(a.EditorGroup.Editor.Buf.Lines, "\n"); got != "package formatted\n" {
		t.Fatalf("edits not applied: %q", got)
	}

	a.EditorGroup.Editor.Undo.Undo(buf)
	if got := strings.Join(a.EditorGroup.Editor.Buf.Lines, "\n"); got != "package first\n" {
		t.Errorf("expected original content after single undo, got: %q", got)
	}
}
