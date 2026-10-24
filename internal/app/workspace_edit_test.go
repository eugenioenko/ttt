package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/lsp"
)

func workspaceEditTestApp(t *testing.T, files map[string]string) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(func() {
		a.CloseTasks()
		a.EditorGroup.CloseAllTabs()
	})
	return a, dir
}

func decodeWorkspaceEdit(t *testing.T, raw string) *lsp.WorkspaceEdit {
	t.Helper()
	var edit lsp.WorkspaceEdit
	if err := json.Unmarshal([]byte(raw), &edit); err != nil {
		t.Fatal(err)
	}
	return &edit
}

func renameEdit(uri string, line, start, end int, newText string) string {
	return fmt.Sprintf(`{"textDocument":{"uri":%q,"version":1},"edits":[
		{"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}},"newText":%q,"annotationId":"a"}]}`,
		uri, line, start, line, end, newText)
}

func bufferText(t *testing.T, a *App, path string) string {
	t.Helper()
	a.EditorGroup.OpenFile(path)
	buf := a.EditorGroup.ActiveBuffer()
	if buf == nil {
		t.Fatalf("no buffer for %s", path)
	}
	return strings.Join(buf.Lines, "\n")
}

func TestRenameAppliesDocumentChanges(t *testing.T) {
	a, dir := workspaceEditTestApp(t, map[string]string{
		"main.go": "package main\n\nfunc main() { greet() }\n",
		"util.go": "package main\n\nfunc greet() {}\n",
	})
	mainPath, utilPath := filepath.Join(dir, "main.go"), filepath.Join(dir, "util.go")
	a.EditorGroup.OpenFile(mainPath)

	a.ApplyWorkspaceEdit(decodeWorkspaceEdit(t, `{"documentChanges":[`+
		renameEdit(FileURI(mainPath), 2, 14, 19, "hello")+","+
		renameEdit(FileURI(utilPath), 2, 5, 10, "hello")+`]}`))

	if got := a.EditorGroup.ActiveFilePath(); got != mainPath {
		t.Errorf("active file = %s, want %s", got, mainPath)
	}
	if got := bufferText(t, a, mainPath); !strings.Contains(got, "func main() { hello() }") {
		t.Errorf("main.go = %q, want greet renamed", got)
	}
	if got := bufferText(t, a, utilPath); !strings.Contains(got, "func hello() {}") {
		t.Errorf("util.go = %q, want greet renamed", got)
	}
}

func TestCodeActionAppliesDocumentChanges(t *testing.T) {
	a, dir := workspaceEditTestApp(t, map[string]string{
		"main.go": "package main\n\nimport \"os\"\n\nfunc main() {}\n",
	})
	path := filepath.Join(dir, "main.go")
	a.EditorGroup.OpenFile(path)

	a.applyCodeActionEdit(path, decodeWorkspaceEdit(t, `{"documentChanges":[{"textDocument":{"uri":`+
		fmt.Sprintf("%q", FileURI(path))+`,"version":1},"edits":[
		{"range":{"start":{"line":1,"character":0},"end":{"line":3,"character":0}},"newText":""}]}]}`))

	if got := bufferText(t, a, path); got != "package main\n\nfunc main() {}\n" {
		t.Errorf("main.go = %q, want the import removed", got)
	}
}

func TestWorkspaceEditWithFileOperationChangesNothing(t *testing.T) {
	const content = "package main\n\nfunc greet() {}\n"
	a, dir := workspaceEditTestApp(t, map[string]string{"util.go": content})
	path := filepath.Join(dir, "util.go")
	a.EditorGroup.OpenFile(path)
	edit := decodeWorkspaceEdit(t, `{"documentChanges":[`+renameEdit(FileURI(path), 2, 5, 10, "hello")+
		`,{"kind":"rename","oldUri":`+fmt.Sprintf("%q", FileURI(path))+`,"newUri":"file:///elsewhere.go"}]}`)

	a.ApplyWorkspaceEdit(edit)
	a.applyCodeActionEdit(path, edit)

	if got := bufferText(t, a, path); got != content {
		t.Errorf("util.go = %q, want it unchanged", got)
	}
	if !strings.Contains(a.Status.Notification, "does not support") {
		t.Errorf("notification = %q, want the unsupported file operation reported", a.Status.Notification)
	}
}

func TestSuccessiveDocumentChangesApplyInOrder(t *testing.T) {
	a, dir := workspaceEditTestApp(t, map[string]string{"main.go": "foo bar\n"})
	path := filepath.Join(dir, "main.go")
	a.EditorGroup.OpenFile(path)
	uri := FileURI(path)

	// The second edit's range is in the text the first one leaves: "Xfoo bar".
	a.ApplyWorkspaceEdit(decodeWorkspaceEdit(t, `{"documentChanges":[`+
		renameEdit(uri, 0, 0, 0, "X")+","+renameEdit(uri, 0, 5, 8, "baz")+`]}`))

	if got := bufferText(t, a, path); got != "Xfoo baz\n" {
		t.Errorf("main.go = %q, want \"Xfoo baz\\n\"", got)
	}
	if !strings.Contains(a.Status.Notification, "1 file") {
		t.Errorf("notification = %q, want one file counted", a.Status.Notification)
	}

	// Multiple successive edits to the same file undo in a single step (#876).
	a.EditorGroup.Undo()
	if got := bufferText(t, a, path); got != "foo bar\n" {
		t.Errorf("after undo = %q, want \"foo bar\\n\"", got)
	}
}

func TestInterleavedDocumentChangesSingleUndoPerFile(t *testing.T) {
	a, dir := workspaceEditTestApp(t, map[string]string{
		"a.go": "foo\n",
		"b.go": "bar\n",
	})
	aPath, bPath := filepath.Join(dir, "a.go"), filepath.Join(dir, "b.go")
	aUri, bUri := FileURI(aPath), FileURI(bPath)

	// Interleaved batches: A, B, A
	a.ApplyWorkspaceEdit(decodeWorkspaceEdit(t, `{"documentChanges":[`+
		renameEdit(aUri, 0, 0, 3, "step1")+","+
		renameEdit(bUri, 0, 0, 3, "step2")+","+
		renameEdit(aUri, 0, 0, 5, "step3")+`]}`))

	if got := bufferText(t, a, aPath); got != "step3\n" {
		t.Errorf("a.go = %q, want step3", got)
	}
	if got := bufferText(t, a, bPath); got != "step2\n" {
		t.Errorf("b.go = %q, want step2", got)
	}

	// Single undo in a.go reverts all changes made to a.go across interleaved batches
	a.EditorGroup.OpenFile(aPath)
	a.EditorGroup.Undo()
	if got := bufferText(t, a, aPath); got != "foo\n" {
		t.Errorf("a.go after single undo = %q, want \"foo\\n\"", got)
	}

	// Single undo in b.go reverts all changes made to b.go
	a.EditorGroup.OpenFile(bPath)
	a.EditorGroup.Undo()
	if got := bufferText(t, a, bPath); got != "bar\n" {
		t.Errorf("b.go after single undo = %q, want \"bar\\n\"", got)
	}
}

func TestPickCodeActionEditSkipsFileOperations(t *testing.T) {
	fileOp := decodeWorkspaceEdit(t, `{"documentChanges":[{"kind":"create","uri":"file:///new.go"}]}`)
	applicable := decodeWorkspaceEdit(t, `{"documentChanges":[`+renameEdit("file:///a.go", 0, 0, 1, "x")+`]}`)
	empty := decodeWorkspaceEdit(t, `{}`)

	if got := pickCodeActionEdit([]lsp.CodeAction{{Edit: empty}, {Edit: fileOp}, {Edit: applicable}}); got != applicable {
		t.Errorf("picked %+v, want the action ttt can apply", got)
	}
	if got := pickCodeActionEdit([]lsp.CodeAction{{Edit: nil}, {Edit: fileOp}}); got != fileOp {
		t.Errorf("picked %+v, want the file operation so it is reported", got)
	}
	if got := pickCodeActionEdit([]lsp.CodeAction{{Edit: empty}}); got != nil {
		t.Errorf("picked %+v, want nothing", got)
	}
}
