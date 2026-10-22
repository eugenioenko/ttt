package app

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/lsp"
)

// A rename, a format or a code action is one change, so one undo has to
// revert all of it (#876).
func TestApplyTextEditsIsOneUndoStep(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	editor := a.EditorGroup.Editor
	editor.Buf.SetLines([]string{"hello world"})

	a.ApplyTextEdits([]lsp.TextEdit{
		{
			Range: lsp.Range{
				Start: lsp.Position{Line: 0, Character: 0},
				End:   lsp.Position{Line: 0, Character: 5},
			},
			NewText: "HI",
		},
		{
			Range: lsp.Range{
				Start: lsp.Position{Line: 0, Character: 6},
				End:   lsp.Position{Line: 0, Character: 11},
			},
			NewText: "EARTH",
		},
	})

	after := strings.Join(editor.Buf.Lines, "\n")
	if after != "HI EARTH" {
		t.Fatalf("buffer = %q, want %q", after, "HI EARTH")
	}

	a.EditorGroup.Undo()

	if got := strings.Join(editor.Buf.Lines, "\n"); got != "hello world" {
		t.Errorf("one undo left the buffer at %q, want %q", got, "hello world")
	}
}
