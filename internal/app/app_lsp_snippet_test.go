package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestInsertCompletionExpandsSnippetTextEdit(t *testing.T) {
	a := completionTestApp(t, "a.html", "<div class\n")
	editor := a.EditorGroup.Editor
	editor.Cursor.Line = 0
	editor.Cursor.Col = 10

	a.insertCompletion(ui.CompletionItem{
		Label:            "class",
		InsertTextFormat: lsp.InsertTextFormatSnippet,
	}, &lsp.TextEdit{
		Range: lsp.Range{
			Start: lsp.Position{Line: 0, Character: 5},
			End:   lsp.Position{Line: 0, Character: 10},
		},
		NewText: `class="$1"`,
	})

	if got := editor.Buf.Lines[0]; got != `<div class=""` {
		t.Fatalf("buffer = %q, want %q", got, `<div class=""`)
	}
	if editor.Cursor.Line != 0 || editor.Cursor.Col != 12 {
		t.Fatalf("cursor = %d:%d, want 0:12", editor.Cursor.Line, editor.Cursor.Col)
	}
	if editor.Selection.Active {
		t.Fatal("an empty stop must not select text")
	}
}

func TestInsertCompletionExpandsSnippetInsertText(t *testing.T) {
	a := completionTestApp(t, "a.css", "\n")
	editor := a.EditorGroup.Editor

	a.insertCompletion(ui.CompletionItem{
		Label:            "align-items",
		InsertText:       "align-items: $0;",
		InsertTextFormat: lsp.InsertTextFormatSnippet,
	}, nil)

	if got := editor.Buf.Lines[0]; got != "align-items: ;" {
		t.Fatalf("buffer = %q, want %q", got, "align-items: ;")
	}
	if editor.Cursor.Line != 0 || editor.Cursor.Col != 13 {
		t.Fatalf("cursor = %d:%d, want 0:13", editor.Cursor.Line, editor.Cursor.Col)
	}
}

func TestInsertCompletionSelectsSnippetDefault(t *testing.T) {
	a := completionTestApp(t, "a.html", "\n")
	editor := a.EditorGroup.Editor

	a.insertCompletion(ui.CompletionItem{
		Label:            "div",
		InsertText:       "${1:div}",
		InsertTextFormat: lsp.InsertTextFormatSnippet,
	}, nil)

	if got := editor.Buf.Lines[0]; got != "div" {
		t.Fatalf("buffer = %q, want %q", got, "div")
	}
	if !editor.Selection.Active {
		t.Fatal("default text must be selected")
	}
	if editor.Selection.Anchor.Line != 0 || editor.Selection.Anchor.Col != 0 {
		t.Fatalf("anchor = %d:%d, want 0:0", editor.Selection.Anchor.Line, editor.Selection.Anchor.Col)
	}
	if editor.Cursor.Line != 0 || editor.Cursor.Col != 3 {
		t.Fatalf("cursor = %d:%d, want 0:3", editor.Cursor.Line, editor.Cursor.Col)
	}
}

func TestInsertCompletionSnippetWithNewlines(t *testing.T) {
	a := completionTestApp(t, "a.html", "\n")
	editor := a.EditorGroup.Editor

	a.insertCompletion(ui.CompletionItem{
		Label:            "div",
		InsertText:       "<div>\n  $1\n</div>",
		InsertTextFormat: lsp.InsertTextFormatSnippet,
	}, nil)

	want := []string{"<div>", "  ", "</div>"}
	if len(editor.Buf.Lines) != len(want) {
		t.Fatalf("lines = %#v, want %#v", editor.Buf.Lines, want)
	}
	for i := range want {
		if editor.Buf.Lines[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, editor.Buf.Lines[i], want[i])
		}
	}
	if editor.Cursor.Line != 1 || editor.Cursor.Col != 2 {
		t.Fatalf("cursor = %d:%d, want 1:2", editor.Cursor.Line, editor.Cursor.Col)
	}
}

func TestInsertCompletionKeepsPlainTextDollar(t *testing.T) {
	a := completionTestApp(t, "a.scss", "\n")
	editor := a.EditorGroup.Editor

	a.insertCompletion(ui.CompletionItem{
		Label:            "primary",
		InsertText:       "$primary",
		InsertTextFormat: lsp.InsertTextFormatPlainText,
	}, nil)

	if got := editor.Buf.Lines[0]; got != "$primary" {
		t.Fatalf("buffer = %q, want %q", got, "$primary")
	}
	if editor.Cursor.Col != 8 {
		t.Fatalf("cursor col = %d, want 8", editor.Cursor.Col)
	}
}
