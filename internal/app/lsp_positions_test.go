package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func utf16Edit(line, start, end int, text string) lsp.TextEdit {
	return lsp.TextEdit{Range: lsp.Range{Start: lsp.Position{Line: line, Character: start}, End: lsp.Position{Line: line, Character: end}}, NewText: text}
}

func TestUTF16EditsUseOneSnapshotAndUndoStep(t *testing.T) {
	a := completionTestApp(t, "a.go", "😀😀 first é second 中\n")
	original := strings.Join(a.EditorGroup.ActiveBuffer().Lines, "\n")
	edits := []lsp.TextEdit{utf16Edit(0, 5, 10, "𐐀"), utf16Edit(0, 13, 19, "last")}
	before := append([]lsp.TextEdit(nil), edits...)
	a.ApplyTextEdits(edits)
	if got := a.EditorGroup.ActiveBuffer().Lines[0]; got != "😀😀 𐐀 é last 中" {
		t.Fatalf("edited line = %q", got)
	}
	if !reflect.DeepEqual(edits, before) {
		t.Fatal("wire edits were mutated")
	}
	a.EditorGroup.Undo()
	if got := strings.Join(a.EditorGroup.ActiveBuffer().Lines, "\n"); got != original {
		t.Fatalf("undo = %q, want %q", got, original)
	}
}

func TestUTF16CompletionAdditionalEditsShareOriginalCoordinates(t *testing.T) {
	a := completionTestApp(t, "a.go", "😀😀 pre fn tail\n")
	moveCursor(a, 0, 9)
	a.completion.lines = append([]string(nil), a.EditorGroup.ActiveBuffer().Lines...)
	items := []lsp.CompletionItem{{Label: "function", AdditionalTextEdits: []lsp.TextEdit{
		utf16Edit(0, 12, 16, "end"), utf16Edit(0, 5, 8, "prefix"),
	}}}
	item := LspToUICompletions(items, a.completion.lines)[0]
	edit := utf16Edit(0, 9, 11, "function")
	a.insertCompletion(item, &edit)
	if got := a.EditorGroup.ActiveBuffer().Lines[0]; got != "😀😀 prefix function end" {
		t.Fatalf("completion = %q", got)
	}
	if _, col := a.EditorGroup.ActiveCursor(); col != 18 {
		t.Fatalf("cursor = %d, want 18", col)
	}
	a.EditorGroup.Undo()
	if got := a.EditorGroup.ActiveBuffer().Lines[0]; got != "😀😀 pre fn tail" {
		t.Fatalf("completion undo = %q", got)
	}
}

func TestUTF16CompletionKeepsRequestSnapshotWhileTyping(t *testing.T) {
	a := completionTestApp(t, "a.go", "😀😀 st tail\n")
	a.completion.lines = []string{"😀😀 s tail", ""}
	a.completion.col = 4
	moveCursor(a, 0, 5)
	edit := utf16Edit(0, 5, 6, "string")
	item := LspToUICompletions([]lsp.CompletionItem{{Label: "string", AdditionalTextEdits: []lsp.TextEdit{utf16Edit(0, 7, 11, "end")}}}, a.completion.lines)[0]
	a.insertCompletion(item, &edit)
	if got := a.EditorGroup.ActiveBuffer().Lines[0]; got != "😀😀 string end" {
		t.Fatalf("completion after typing = %q", got)
	}
}

func TestUTF16CompletionNotifiesAfterPlacingCursor(t *testing.T) {
	a := completionTestApp(t, "a.go", "😀😀 fn\n")
	moveCursor(a, 0, 5)
	notified := false
	a.EditorGroup.Editor.OnChange = func() {
		notified = true
		if line, col := a.EditorGroup.ActiveCursor(); line != 0 || col != 8 {
			t.Fatalf("change callback cursor = %d:%d, want 0:8", line, col)
		}
	}
	edit := utf16Edit(0, 5, 7, "call(")
	a.insertCompletion(ui.CompletionItem{Label: "call"}, &edit)
	if !notified {
		t.Fatal("change callback was not called")
	}
}

func TestUTF16WorkspaceEditsUseUnsavedTargetBuffers(t *testing.T) {
	a := completionTestApp(t, "a.go", "disk a\n")
	first := a.EditorGroup.ActiveFilePath()
	second := filepath.Join(filepath.Dir(first), "b.go")
	third := filepath.Join(filepath.Dir(first), "c.go")
	for _, path := range []string{second, third} {
		if err := os.WriteFile(path, []byte("😀 target!\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a.EditorGroup.ActiveBuffer().SetLines([]string{"😀😀 target!"})
	a.EditorGroup.ActiveBuffer().Dirty = true
	a.EditorGroup.OpenFile(second)
	a.EditorGroup.ActiveBuffer().SetLines([]string{"中😀😀 target!"})
	a.EditorGroup.ActiveBuffer().Dirty = true
	a.EditorGroup.OpenFile(first)
	edit := &lsp.WorkspaceEdit{Changes: map[string][]lsp.TextEdit{
		FileURI(first):  {utf16Edit(0, 5, 11, "renamed")},
		FileURI(second): {utf16Edit(0, 6, 12, "renamed")},
		FileURI(third):  {utf16Edit(0, 3, 9, "renamed")},
	}}
	a.ApplyWorkspaceEdit(edit)
	for path, want := range map[string]string{first: "😀😀 renamed!", second: "中😀😀 renamed!", third: "😀 renamed!"} {
		if got := a.EditorGroup.BufferForPath(path).Lines[0]; got != want {
			t.Fatalf("%s = %q, want %q", path, got, want)
		}
		a.EditorGroup.OpenFile(path)
		a.EditorGroup.Undo()
		if strings.Contains(a.EditorGroup.ActiveBuffer().Lines[0], "renamed") {
			t.Fatal("workspace edit did not undo in one step")
		}
	}
}

func TestUTF16DiagnosticsReferencesAndSymbolsUseTargetText(t *testing.T) {
	a := completionTestApp(t, "a.go", "disk\n")
	path := a.EditorGroup.ActiveFilePath()
	a.EditorGroup.ActiveBuffer().SetLines([]string{"😀😀 target"})
	lines := a.lspLines(path)
	if got := a.lspColumn(path, 0, 3); got != 5 {
		t.Fatalf("request column = %d, want 5", got)
	}
	r := utf16Edit(0, 5, 11, "").Range
	diags := LspToUIDiagnostics([]lsp.Diagnostic{{Range: r}}, lines)
	if diags[0].StartCol != 3 || diags[0].EndCol != 9 {
		t.Fatalf("diagnostics = %+v", diags)
	}
	other := filepath.Join(filepath.Dir(path), "b.go")
	if err := os.WriteFile(other, []byte("中😀 target\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.ShowReferences([]lsp.Location{{URI: FileURI(path), Range: r}, {URI: FileURI(other), Range: utf16Edit(0, 4, 10, "").Range}})
	for _, item := range a.References.Items {
		if item.Col != 3 {
			t.Fatalf("reference = %+v", item)
		}
	}
	symbols := []lsp.DocumentSymbol{{Range: r, SelectionRange: r, Children: []lsp.DocumentSymbol{{Range: r, SelectionRange: r}}}}
	runeSymbols(lines, symbols)
	if symbols[0].SelectionRange.Start.Character != 3 || symbols[0].Children[0].Range.End.Character != 9 {
		t.Fatalf("symbols = %+v", symbols)
	}
	for _, label := range []json.RawMessage{json.RawMessage(`[5,11]`), json.RawMessage(`"target"`)} {
		sig := &lsp.SignatureHelp{Signatures: []lsp.SignatureInformation{{Label: "😀😀 target", Parameters: []lsp.ParameterInformation{{Label: label}}}}}
		result := LspToSignatureHelpResult(sig)
		if got := result.Label[result.ParamStart:result.ParamEnd]; got != "target" {
			t.Fatalf("signature parameter = %q", got)
		}
	}
}

func TestUTF16NavigationUsesUnsavedDestination(t *testing.T) {
	dir := t.TempDir()
	first, second := filepath.Join(dir, "a.go"), filepath.Join(dir, "b.go")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("disk\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := prepareListenTestApp(t)
	a.EditorGroup.OpenFile(second)
	a.EditorGroup.ActiveBuffer().SetLines([]string{"😀😀 target"})
	a.EditorGroup.ActiveBuffer().Dirty = true
	a.EditorGroup.OpenFile(first)
	startListenTestEventLoop(t, a)
	a.Screen.PostEvent(tcell.NewEventInterrupt(&LocationResult{Locations: []lsp.Location{{
		URI: FileURI(second), Range: utf16Edit(0, 5, 11, "").Range,
	}}}))
	if err := runExecMain(a, func() error {
		if path := a.EditorGroup.ActiveFilePath(); path != second {
			t.Fatalf("destination = %q, want %q", path, second)
		}
		if line, col := a.EditorGroup.ActiveCursor(); line != 0 || col != 3 {
			t.Fatalf("destination cursor = %d:%d, want 0:3", line, col)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
