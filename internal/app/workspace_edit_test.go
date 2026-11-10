package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
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

func TestCurrentCodeActionRangeUsesSelectionAndUTF16(t *testing.T) {
	a := completionTestApp(t, "main.go", "😀abc\nnext\n")
	path := a.EditorGroup.ActiveFilePath()
	moveCursor(a, 0, 3)

	if got := a.currentCodeActionRange(path); got.Start != (lsp.Position{Line: 0, Character: 4}) ||
		got.End != (lsp.Position{Line: 0, Character: 4}) {
		t.Fatalf("cursor range = %+v, want zero-width UTF-16 position 0:4", got)
	}

	a.EditorGroup.Editor.Selection.Start(0, 1)
	if got := a.currentCodeActionRange(path); got.Start != (lsp.Position{Line: 0, Character: 2}) ||
		got.End != (lsp.Position{Line: 0, Character: 4}) {
		t.Fatalf("selection range = %+v, want UTF-16 range 0:2-0:4", got)
	}
}

func TestCodeActionsCommandIsAvailableInPaletteAndContextMenu(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(func() {
		a.CloseTasks()
		a.EditorGroup.CloseAllTabs()
	})
	a.Reg = command.NewRegistry()
	RegisterCommands(a)
	initMenuText()
	cmd, ok := a.Reg.Get("editor.codeActions")
	if !ok || cmd.Handler == nil {
		t.Fatal("editor.codeActions is not registered for the command palette")
	}

	for _, item := range editorContextMenu {
		if item.Command == "editor.codeActions" {
			return
		}
	}
	t.Fatal("editor context menu does not include code actions")
}

func TestApplicableCodeActionsFiltersUnsupportedActions(t *testing.T) {
	edit := &lsp.WorkspaceEdit{Changes: map[string][]lsp.TextEdit{
		"file:///a.go": {utf16Edit(0, 0, 1, "x")},
	}}
	actions := applicableCodeActions([]lsp.CodeAction{
		{Title: "Command-only"},
		{Title: "Empty edit", Edit: &lsp.WorkspaceEdit{}},
		{Title: "Replace", Edit: edit},
		{Title: "Gopls refactor", Command: &lsp.Command{Command: "gopls.apply_fix"}},
		{Title: "Command with empty edit", Edit: &lsp.WorkspaceEdit{}, Command: &lsp.Command{Command: "gopls.apply_fix"}},
	})
	if len(actions) != 3 || actions[0].Title != "Replace" ||
		actions[1].Title != "Gopls refactor" || actions[2].Title != "Command with empty edit" {
		t.Fatalf("applicable actions = %+v, want direct-edit and command actions", actions)
	}
}

func TestCodeActionDiagnosticsUseLSPSourcesAndUTF16(t *testing.T) {
	a := completionTestApp(t, "main.go", "😀 missing\n")
	path := a.EditorGroup.ActiveFilePath()
	a.EditorGroup.SetDiagnosticsSource(lspDiagnosticsSource("go"), path, []ui.Diagnostic{{
		StartLine: 0, StartCol: 2, EndLine: 0, EndCol: 9,
		Severity: ui.DiagError, Source: "compiler", Message: "undefined: missing",
	}, {
		StartLine: 0, StartCol: 8, EndLine: 0, EndCol: 9,
		Severity: ui.DiagWarning, Source: "compiler", Message: "outside selection",
	}})
	a.EditorGroup.SetDiagnosticsSource("plugin:test", path, []ui.Diagnostic{{
		StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 1,
		Severity: ui.DiagWarning, Source: "plugin", Message: "not for gopls",
	}})

	got := a.codeActionDiagnostics(path, "go", lsp.Range{
		Start: lsp.Position{Line: 0, Character: 4},
		End:   lsp.Position{Line: 0, Character: 6},
	})
	if len(got) != 1 {
		t.Fatalf("diagnostics = %+v, want one gopls diagnostic", got)
	}
	if got[0].Range.Start != (lsp.Position{Line: 0, Character: 3}) ||
		got[0].Range.End != (lsp.Position{Line: 0, Character: 10}) {
		t.Fatalf("diagnostic range = %+v, want UTF-16 range 0:3-0:10", got[0].Range)
	}
	if got[0].Source != "compiler" || got[0].Message != "undefined: missing" {
		t.Fatalf("diagnostic = %+v, want source and message preserved", got[0])
	}
}

func TestLSPRangesIntersectAtCursorAndSelection(t *testing.T) {
	tests := []struct {
		name string
		a    lsp.Range
		b    lsp.Range
		want bool
	}{
		{"cursor inside", lsp.Range{Start: lsp.Position{Line: 1, Character: 2}, End: lsp.Position{Line: 1, Character: 5}}, lsp.Range{Start: lsp.Position{Line: 1, Character: 3}, End: lsp.Position{Line: 1, Character: 3}}, true},
		{"cursor at end", lsp.Range{Start: lsp.Position{Line: 1, Character: 2}, End: lsp.Position{Line: 1, Character: 5}}, lsp.Range{Start: lsp.Position{Line: 1, Character: 5}, End: lsp.Position{Line: 1, Character: 5}}, false},
		{"selection overlaps", lsp.Range{Start: lsp.Position{Line: 1, Character: 2}, End: lsp.Position{Line: 1, Character: 5}}, lsp.Range{Start: lsp.Position{Line: 1, Character: 4}, End: lsp.Position{Line: 1, Character: 7}}, true},
		{"selection touches end", lsp.Range{Start: lsp.Position{Line: 1, Character: 2}, End: lsp.Position{Line: 1, Character: 5}}, lsp.Range{Start: lsp.Position{Line: 1, Character: 5}, End: lsp.Position{Line: 1, Character: 7}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lspRangesIntersect(tt.a, tt.b); got != tt.want {
				t.Fatalf("lspRangesIntersect(%+v, %+v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCodeActionPickerAppliesWorkspaceEdits(t *testing.T) {
	a, dir := workspaceEditTestApp(t, map[string]string{
		"main.go": "call()\n",
		"util.go": "func old()\n",
	})
	mainPath, utilPath := filepath.Join(dir, "main.go"), filepath.Join(dir, "util.go")
	a.EditorGroup.OpenFile(mainPath)
	buf := a.EditorGroup.ActiveBuffer()
	edit := &lsp.WorkspaceEdit{Changes: map[string][]lsp.TextEdit{
		FileURI(mainPath): {utf16Edit(0, 0, 4, "run")},
		FileURI(utilPath): {utf16Edit(0, 5, 8, "new")},
	}}

	a.HandleCodeActionsResult(&CodeActionsResult{
		Path: mainPath, Buffer: buf, Version: buf.Version(),
		Actions: []lsp.CodeAction{{Title: "Rename helper", Edit: edit}},
	})

	overlay, ok := a.Root.TopOverlayWidget().(*ui.WidgetAdapter)
	if !ok {
		t.Fatal("code action picker did not open")
	}
	dialog, ok := overlay.W.(*widgets.DialogWidget)
	if !ok {
		t.Fatalf("picker overlay = %T, want dialog adapter", overlay.W)
	}
	selectWidget, ok := dialog.Content.(*widgets.SelectWidget)
	if !ok {
		t.Fatalf("picker content = %T, want select widget", dialog.Content)
	}
	if len(selectWidget.Config.Items) != 1 || selectWidget.Config.Items[0].Label != "Rename helper" {
		t.Fatalf("picker items = %+v, want the returned action", selectWidget.Config.Items)
	}

	selectWidget.Config.OnSelect("0")
	if got := a.EditorGroup.ActiveFilePath(); got != mainPath {
		t.Fatalf("active file = %q, want %q", got, mainPath)
	}
	if got := strings.Join(a.EditorGroup.BufferForPath(mainPath).Lines, "\n"); got != "run()\n" {
		t.Errorf("main.go = %q, want action edit applied", got)
	}
	if got := strings.Join(a.EditorGroup.BufferForPath(utilPath).Lines, "\n"); got != "func new()\n" {
		t.Errorf("util.go = %q, want workspace action edit applied", got)
	}
	if got := a.Status.Notification; got != "Code action applied" {
		t.Errorf("notification = %q, want code action confirmation", got)
	}
}

func TestCodeActionsResultDropsChangedBuffer(t *testing.T) {
	a := completionTestApp(t, "main.go", "before\n")
	buf := a.EditorGroup.ActiveBuffer()
	buf.SetLines([]string{"after"})

	a.HandleCodeActionsResult(&CodeActionsResult{
		Path: a.EditorGroup.ActiveFilePath(), Buffer: buf, Version: buf.Version() - 1,
		Actions: []lsp.CodeAction{{Title: "Replace", Edit: &lsp.WorkspaceEdit{Changes: map[string][]lsp.TextEdit{
			FileURI(a.EditorGroup.ActiveFilePath()): {utf16Edit(0, 0, 5, "changed")},
		}}}},
	})
	if a.Root.HasOverlay() {
		t.Fatal("stale code action result opened a picker")
	}
	if got := buf.Lines[0]; got != "after" {
		t.Fatalf("buffer = %q, want stale result not applied", got)
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
