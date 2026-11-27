package app

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestLSPTextEditsPreserveSamePositionOrder(t *testing.T) {
	for _, tc := range []struct {
		name  string
		edits []lsp.TextEdit
		want  string
	}{
		{"inserts", []lsp.TextEdit{utf16Edit(0, 3, 3, "A"), utf16Edit(0, 3, 3, "B")}, "😀 ABxy tail\n"},
		{"inserts then replacement", []lsp.TextEdit{utf16Edit(0, 3, 3, "A"), utf16Edit(0, 3, 3, "B"), utf16Edit(0, 3, 5, "Q")}, "😀 ABQ tail\n"},
		{"multiline", []lsp.TextEdit{utf16Edit(0, 0, 0, "A\n"), utf16Edit(0, 0, 0, "B\n"), utf16Edit(0, 3, 5, "z")}, "A\nB\n😀 z tail\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := completionTestApp(t, "a.go", "😀 xy tail\n")
			a.ApplyTextEdits(tc.edits)
			if got := strings.Join(a.EditorGroup.ActiveBuffer().Lines, "\n"); got != tc.want {
				t.Fatalf("text = %q, want %q", got, tc.want)
			}
			a.EditorGroup.Undo()
			if got := strings.Join(a.EditorGroup.ActiveBuffer().Lines, "\n"); got != "😀 xy tail\n" {
				t.Fatalf("undo = %q", got)
			}
		})
	}
}

func TestCompletionEqualPositionEditsPreserveSnippetSelection(t *testing.T) {
	a := completionTestApp(t, "a.go", "😀 fn tail\n")
	moveCursor(a, 0, 4)
	a.completion.lines = append([]string(nil), a.EditorGroup.ActiveBuffer().Lines...)
	item := LspToUICompletions([]lsp.CompletionItem{{Label: "call", AdditionalTextEdits: []lsp.TextEdit{utf16Edit(0, 0, 0, "A\n"), utf16Edit(0, 0, 0, "B")}}}, a.completion.lines)[0]
	edit := utf16Edit(0, 3, 5, "call(${1:x})")
	item.InsertTextFormat = lsp.InsertTextFormatSnippet
	a.insertCompletion(item, &edit)
	e := a.EditorGroup.Editor
	if got := strings.Join(e.Buf.Lines, "\n"); got != "A\nB😀 call(x) tail\n" {
		t.Fatalf("completion = %q", got)
	}
	if e.Cursor.Line != 1 || e.Cursor.Col != 9 || !e.Selection.Active || e.Selection.Anchor.Line != 1 || e.Selection.Anchor.Col != 8 {
		t.Fatalf("cursor/selection = %+v/%+v", e.Cursor, e.Selection)
	}
	a.EditorGroup.Undo()
	if got := strings.Join(e.Buf.Lines, "\n"); got != "😀 fn tail\n" {
		t.Fatalf("undo = %q", got)
	}
}

func TestCompletionDiscardsRemovedTargetLine(t *testing.T) {
	a := completionTestApp(t, "a.go", "top\n😀 fn\n")
	e := a.EditorGroup.Editor
	a.completion.lines = append([]string(nil), e.Buf.Lines...)
	e.Buf.SetLines([]string{"keep"})
	moveCursor(a, 0, 4)
	defer func() {
		if p := recover(); p != nil {
			t.Errorf("accepting stale completion panicked: %v", p)
		}
	}()
	edit := utf16Edit(1, 3, 5, "call")
	a.insertCompletion(ui.CompletionItem{Label: "call", AdditionalEdits: []ui.AdditionalEdit{{NewText: "import x\n"}}}, &edit)
	if got := strings.Join(e.Buf.Lines, "\n"); got != "keep" || e.Cursor.Line != 0 || e.Cursor.Col != 4 {
		t.Fatalf("stale completion changed text/cursor: %q, %+v", got, e.Cursor)
	}
}

func TestWorkspaceEditExcludesFailedOpenFromCount(t *testing.T) {
	a := completionTestApp(t, "a.go", "😀 target\n")
	a.ApplyWorkspaceEdit(&lsp.WorkspaceEdit{Changes: map[string][]lsp.TextEdit{
		FileURI(t.TempDir()):                    {utf16Edit(0, 0, 0, "ignored")},
		FileURI(a.EditorGroup.ActiveFilePath()): {utf16Edit(0, 3, 9, "renamed")},
	}})
	if a.Status.Notification != "Renamed across 1 file(s)" {
		t.Fatalf("notification = %q", a.Status.Notification)
	}
	if got := a.EditorGroup.ActiveBuffer().Lines[0]; got != "😀 renamed" {
		t.Fatalf("renamed text = %q", got)
	}
}

func TestLSPColumnUsesOpenLineWithoutSnapshotAllocation(t *testing.T) {
	a := completionTestApp(t, "a.go", "disk\n")
	lines := make([]string, 20000)
	lines[15000] = "😀😀 value"
	a.EditorGroup.ActiveBuffer().SetLines(lines)
	path := a.EditorGroup.ActiveFilePath()
	if got := a.lspColumn(path, 15000, 3); got != 5 {
		t.Fatalf("UTF-16 column = %d, want 5", got)
	}
	if got := testing.AllocsPerRun(20, func() { a.lspColumn(path, 15000, 3) }); got != 0 {
		t.Fatalf("single-column conversion allocated %.0f times", got)
	}
	if a.lspColumn(path, -1, 1) != 0 || a.lspColumn(path, len(lines), 1) != 0 {
		t.Fatal("invalid line did not clamp to column zero")
	}
}
