package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
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

func TestCanceledQueuedLSPApplyEditIsNotApplied(t *testing.T) {
	a := newListenTestApp(t)
	path := filepath.Join(t.TempDir(), "main.go")
	const original = "original"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runExecMain(a, func() error { return a.EditorGroup.OpenFile(path) }); err != nil {
		t.Fatal(err)
	}
	before := strings.Join(a.EditorGroup.BufferForPath(path).Lines, "\n")

	release, blockerDone := holdExecEventLoop(t, a)
	request := &LSPApplyEditRequest{
		Edit: lsp.WorkspaceEdit{Changes: map[string][]lsp.TextEdit{
			FileURI(path): {{
				Range:   lsp.Range{Start: lsp.Position{}, End: lsp.Position{Character: len(original)}},
				NewText: "changed",
			}},
		}},
		Result:    make(chan lsp.ApplyWorkspaceEditResult, 1),
		Lifecycle: newExecRequestLifecycle(),
	}
	if err := a.Screen.PostEvent(tcell.NewEventInterrupt(request)); err != nil {
		t.Fatal(err)
	}
	if !request.Lifecycle.cancel() {
		t.Fatal("queued request was claimed before cancellation")
	}

	close(release)
	if err := <-blockerDone; err != nil {
		t.Fatal(err)
	}
	if err := runExecMain(a, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(a.EditorGroup.BufferForPath(path).Lines, "\n"); got != before {
		t.Fatalf("buffer = %q, want canceled edit to leave it unchanged (%q)", got, before)
	}
}

func TestLSPApplyEditReportsFailureWhenAnyBatchFails(t *testing.T) {
	a := newListenTestApp(t)
	path := filepath.Join(t.TempDir(), "main.go")
	const original = "original\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runExecMain(a, func() error { return a.EditorGroup.OpenFile(path) }); err != nil {
		t.Fatal(err)
	}

	edit := decodeWorkspaceEdit(t, `{"documentChanges":[`+
		renameEdit(FileURI(path), 0, 0, 8, "first")+","+
		renameEdit(FileURI(path), 0, 0, 5, "done")+","+
		renameEdit(FileURI(t.TempDir()), 0, 0, 0, "ignored")+`]}`)

	var filesCount int
	var allApplied bool
	if err := runExecMain(a, func() error {
		batches, ok := a.workspaceTextEdits(edit)
		if !ok {
			return fmt.Errorf("workspace text edits were rejected")
		}
		filesCount, allApplied = a.applyWorkspaceEditBatches(batches, false)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if filesCount != 1 {
		t.Fatalf("distinct file count = %d, want 1 for repeated batches on one file", filesCount)
	}
	if allApplied {
		t.Fatal("allApplied = true, want false when one batch cannot open its file")
	}

	request := &LSPApplyEditRequest{
		Edit:      *edit,
		Result:    make(chan lsp.ApplyWorkspaceEditResult, 1),
		Lifecycle: newExecRequestLifecycle(),
	}
	if err := a.Screen.PostEvent(tcell.NewEventInterrupt(request)); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-request.Result:
		if result.Applied {
			t.Fatal("LSP apply-edit result reports Applied=true when a batch failed")
		}
		if result.FailureReason == "" {
			t.Fatal("LSP apply-edit failure has no reason")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for LSP apply-edit result")
	}
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
	var lspDiags []lsp.Diagnostic
	if err := json.Unmarshal([]byte(`[{
		"range":{"start":{"line":0,"character":3},"end":{"line":0,"character":10}},
		"severity":1,"source":"compiler","message":"undefined: missing","code":"E042"
	},{
		"range":{"start":{"line":0,"character":9},"end":{"line":0,"character":10}},
		"severity":2,"source":"compiler","message":"outside selection","code":42
	}]`), &lspDiags); err != nil {
		t.Fatal(err)
	}
	storedDiags := LspToUIDiagnostics(lspDiags, a.EditorGroup.ActiveBuffer().Lines)
	if storedDiags[1].Code != float64(42) {
		t.Fatalf("stored numeric diagnostic code = %#v, want 42", storedDiags[1].Code)
	}
	a.EditorGroup.SetDiagnosticsSource(lspDiagnosticsSource("go"), path, storedDiags)
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
	if got[0].Code != "E042" {
		t.Fatalf("diagnostic code = %#v, want the server-provided code", got[0].Code)
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
		Path: mainPath, Buffer: buf, Version: buf.Version(), OpenBuffers: a.snapshotCodeActionBuffers(),
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

func TestInteractiveCodeActionEditFailsWhenAnyBatchCannotApply(t *testing.T) {
	a, dir := workspaceEditTestApp(t, map[string]string{
		"main.go": "call()\n",
	})
	mainPath := filepath.Join(dir, "main.go")
	a.EditorGroup.OpenFile(mainPath)
	edit := &lsp.WorkspaceEdit{DocumentChanges: []lsp.DocumentChange{
		{
			TextDocument: lsp.VersionedTextDocumentIdentifier{URI: FileURI(mainPath)},
			Edits:        []lsp.TextEdit{utf16Edit(0, 0, 4, "run")},
		},
		{
			TextDocument: lsp.VersionedTextDocumentIdentifier{URI: FileURI(t.TempDir())},
			Edits:        []lsp.TextEdit{utf16Edit(0, 0, 0, "ignored")},
		},
	}}

	if _, ok := a.applyInteractiveCodeActionEdit(mainPath, edit, a.snapshotCodeActionBuffers()); ok {
		t.Fatal("partially applied code action reported full success")
	}
	if got := strings.Join(a.EditorGroup.BufferForPath(mainPath).Lines, "\n"); got != "run()\n" {
		t.Fatalf("main.go = %q, want first batch to have applied before the failure", got)
	}
}

func TestCodeActionRejectsEditWhenAffectedOpenBufferWasReplaced(t *testing.T) {
	a, dir := workspaceEditTestApp(t, map[string]string{
		"main.go": "call()\n",
		"util.go": "func old()\n",
	})
	mainPath, utilPath := filepath.Join(dir, "main.go"), filepath.Join(dir, "util.go")
	a.EditorGroup.OpenFile(mainPath)
	a.EditorGroup.CommitActiveTab()
	a.EditorGroup.OpenFile(utilPath)
	a.EditorGroup.CommitActiveTab()
	a.EditorGroup.OpenFile(mainPath)
	mainBuffer := a.EditorGroup.BufferForPath(mainPath)
	utilBuffer := a.EditorGroup.BufferForPath(utilPath)
	snapshots := a.snapshotCodeActionBuffers()
	edit := &lsp.WorkspaceEdit{Changes: map[string][]lsp.TextEdit{
		FileURI(mainPath): {utf16Edit(0, 0, 4, "run")},
		FileURI(utilPath): {utf16Edit(0, 5, 8, "new")},
	}}

	a.EditorGroup.SwitchToTabByPath(utilPath)
	a.EditorGroup.CloseTab()
	a.EditorGroup.OpenFile(utilPath)
	replacement := a.EditorGroup.BufferForPath(utilPath)
	if replacement == utilBuffer {
		t.Fatal("reopened utility file retained the original buffer identity")
	}
	if replacement.Version() != snapshots[utilPath].Version {
		t.Fatalf("replacement version = %d, want captured version %d to isolate identity check",
			replacement.Version(), snapshots[utilPath].Version)
	}
	a.EditorGroup.SwitchToTabByPath(mainPath)

	result := &CodeActionsResult{
		Path: mainPath, Buffer: mainBuffer, Version: mainBuffer.Version(), OpenBuffers: snapshots,
		Actions: []lsp.CodeAction{{Title: "Rename helper", Edit: edit}},
	}
	if a.EditorGroup.ActiveFilePath() != result.Path || a.editorBuffer() != result.Buffer || result.Buffer.Version() != result.Version {
		t.Fatalf("preconditions failed: path=%q/%q buffer=%p/%p version=%d/%d",
			a.EditorGroup.ActiveFilePath(), result.Path, a.editorBuffer(), result.Buffer, result.Buffer.Version(), result.Version)
	}
	a.HandleCodeActionsResult(result)
	overlay, ok := a.Root.TopOverlayWidget().(*ui.WidgetAdapter)
	if !ok {
		t.Fatal("code action picker did not open")
	}
	dialog := overlay.W.(*widgets.DialogWidget)
	selectWidget := dialog.Content.(*widgets.SelectWidget)
	selectWidget.Config.OnSelect("0")

	if got := strings.Join(mainBuffer.Lines, "\n"); got != "call()\n" {
		t.Fatalf("main.go = %q, want action rejected before any batch applies", got)
	}
	if got := strings.Join(replacement.Lines, "\n"); got != "func old()\n" {
		t.Fatalf("replacement util.go = %q, want unchanged text", got)
	}
}

func TestCodeActionRejectsEditWhenAffectedOpenBufferChanged(t *testing.T) {
	a, dir := workspaceEditTestApp(t, map[string]string{
		"main.go": "call()\n",
		"util.go": "func old()\n",
	})
	mainPath, utilPath := filepath.Join(dir, "main.go"), filepath.Join(dir, "util.go")
	a.EditorGroup.OpenFile(mainPath)
	a.EditorGroup.CommitActiveTab()
	a.EditorGroup.OpenFile(utilPath)
	a.EditorGroup.CommitActiveTab()
	a.EditorGroup.OpenFile(mainPath)
	edit := &lsp.WorkspaceEdit{Changes: map[string][]lsp.TextEdit{
		FileURI(mainPath): {utf16Edit(0, 0, 4, "run")},
		FileURI(utilPath): {utf16Edit(0, 5, 8, "new")},
	}}
	snapshots := a.snapshotCodeActionBuffers()
	a.EditorGroup.BufferForPath(utilPath).SetLines([]string{"func changed()"})

	if _, ok := a.applyInteractiveCodeActionEdit(mainPath, edit, snapshots); ok {
		t.Fatal("changed affected buffer was accepted")
	}
	if got := strings.Join(a.EditorGroup.BufferForPath(mainPath).Lines, "\n"); got != "call()\n" {
		t.Fatalf("main.go = %q, want stale action rejected before any batch applies", got)
	}
}

func TestWorkspaceEditViewerBatchIsNotCountedAsApplied(t *testing.T) {
	a, dir := workspaceEditTestApp(t, map[string]string{
		"main.go": "call()\n",
	})
	mainPath := filepath.Join(dir, "main.go")
	viewerPath := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(viewerPath, []byte{0, 1, 2}, 0o644); err != nil {
		t.Fatal(err)
	}
	a.EditorGroup.OpenFile(mainPath)

	filesCount, allApplied := a.applyWorkspaceEditBatches([]lsp.FileEdits{
		{URI: FileURI(mainPath), Edits: []lsp.TextEdit{utf16Edit(0, 0, 4, "run")}},
		{URI: FileURI(viewerPath), Edits: []lsp.TextEdit{utf16Edit(0, 0, 0, "ignored")}},
	}, false)

	if filesCount != 1 {
		t.Fatalf("distinct applied file count = %d, want 1", filesCount)
	}
	if allApplied {
		t.Fatal("viewer batch was skipped but workspace edit reported complete success")
	}
	if got := strings.Join(a.EditorGroup.BufferForPath(mainPath).Lines, "\n"); got != "run()\n" {
		t.Fatalf("main.go = %q, want successful text batch to apply", got)
	}
	if a.EditorGroup.ActiveFilePath() != mainPath {
		t.Fatalf("active file = %q, want original file restored", a.EditorGroup.ActiveFilePath())
	}
}

func TestWorkspaceEditCannotApplyToPluginSnapshot(t *testing.T) {
	a, dir := workspaceEditTestApp(t, map[string]string{
		"main.go":   "call()\n",
		"plugin.go": "source file\n",
	})
	mainPath := filepath.Join(dir, "main.go")
	pluginPath := filepath.Join(dir, "plugin.go")
	a.EditorGroup.OpenFile(mainPath)
	a.EditorGroup.OpenPluginTab(pluginPath, "Plugin snapshot", ui.NewCommitDetailWidget(dir, "ref", "abcdef0", false))
	activePath := a.EditorGroup.ActiveFilePath()

	filesCount, allApplied := a.applyWorkspaceEditBatches([]lsp.FileEdits{{
		URI:   FileURI(pluginPath),
		Edits: []lsp.TextEdit{utf16Edit(0, 0, 6, "edited")},
	}}, false)

	if filesCount != 0 || allApplied {
		t.Fatalf("workspace edit result = (%d, %v), want no applied files and incomplete result", filesCount, allApplied)
	}
	if a.EditorGroup.ActiveFilePath() != activePath {
		t.Fatalf("active file = %q, want original active path restored", a.EditorGroup.ActiveFilePath())
	}
	if a.EditorGroup.BufferForPath(pluginPath) != nil {
		t.Fatal("workspace edit opened a text buffer over the plugin snapshot")
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

type recordingCodeActionClient struct{ commands []lsp.Command }

func (c *recordingCodeActionClient) ExecuteCommand(command string, arguments []json.RawMessage) error {
	c.commands = append(c.commands, lsp.Command{Command: command, Arguments: arguments})
	return nil
}

func TestCodeActionsResultRunsCommandWithEmptyEdit(t *testing.T) {
	a, dir := workspaceEditTestApp(t, map[string]string{
		"main.go": "call()\n",
	})
	sim := term.NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sim.Fini)
	a.Screen = term.NewTcellScreenFrom(sim)
	client := &recordingCodeActionClient{}
	var gotServer, gotWorkDir string
	a.lookupCodeActionClient = func(server, workDir string) (codeActionCommandClient, error) {
		gotServer, gotWorkDir = server, workDir
		return client, nil
	}

	mainPath := filepath.Join(dir, "main.go")
	a.EditorGroup.OpenFile(mainPath)
	mainBuffer := a.EditorGroup.BufferForPath(mainPath)
	snapshots := a.snapshotCodeActionBuffers()

	wantCommand := lsp.Command{
		Command:   "custom.action",
		Arguments: []json.RawMessage{json.RawMessage(`{"uri":"file:///main.go","line":3}`), json.RawMessage(`"selected"`)},
	}
	wantWorkDir := a.lspWorkDir(mainPath)
	result := &CodeActionsResult{
		Path: mainPath, Buffer: mainBuffer, Version: mainBuffer.Version(), OpenBuffers: snapshots,
		Server: "test-server",
		Actions: []lsp.CodeAction{
			{Title: "Other command", Command: &lsp.Command{Command: "other.action"}},
			{Title: "Empty edit with command", Edit: &lsp.WorkspaceEdit{}, Command: &wantCommand},
		},
	}

	a.HandleCodeActionsResult(result)
	overlay, ok := a.Root.TopOverlayWidget().(*ui.WidgetAdapter)
	if !ok {
		t.Fatal("code action picker did not open")
	}
	dialog := overlay.W.(*widgets.DialogWidget)
	selectWidget := dialog.Content.(*widgets.SelectWidget)
	synctest.Test(t, func(t *testing.T) {
		selectWidget.Config.OnSelect("1")
		synctest.Wait()

		if a.Status.Notification == i18n.Text("status.codeActionApplied") {
			t.Fatalf("notification = %q, want empty edit not to notify codeActionApplied", a.Status.Notification)
		}

		select {
		case ev := <-sim.EventQ():
			interrupt, ok := ev.(*tcell.EventInterrupt)
			if !ok {
				t.Fatalf("expected EventInterrupt, got %T", ev)
			}
			cmdResult, ok := interrupt.Data().(*CodeActionCommandResult)
			if !ok {
				t.Fatalf("expected CodeActionCommandResult, got %T", interrupt.Data())
			}
			if cmdResult.Err != nil {
				t.Fatalf("command failed: %v", cmdResult.Err)
			}
			if !cmdResult.Notify {
				t.Fatal("cmdResult.Notify = false, want true for empty edit")
			}
		default:
			t.Fatal("command execution did not post a result")
		}
		if gotServer != "test-server" || gotWorkDir != wantWorkDir {
			t.Fatalf("client lookup = (%q, %q), want (%q, %q)", gotServer, gotWorkDir, "test-server", wantWorkDir)
		}
		if !reflect.DeepEqual(client.commands, []lsp.Command{wantCommand}) {
			t.Fatalf("executed commands = %+v, want %+v", client.commands, wantCommand)
		}
	})
}

func TestCodeActionsResultPreventsCommandOnUnsupportedEdit(t *testing.T) {
	a, dir := workspaceEditTestApp(t, map[string]string{
		"main.go": "call()\n",
	})
	sim := term.NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sim.Fini)
	a.Screen = term.NewTcellScreenFrom(sim)
	client := &recordingCodeActionClient{}
	lookups := 0
	a.lookupCodeActionClient = func(string, string) (codeActionCommandClient, error) {
		lookups++
		return client, nil
	}

	mainPath := filepath.Join(dir, "main.go")
	a.EditorGroup.OpenFile(mainPath)
	mainBuffer := a.EditorGroup.BufferForPath(mainPath)
	snapshots := a.snapshotCodeActionBuffers()

	unsupportedEdit := decodeWorkspaceEdit(t, `{"documentChanges":[{"kind":"create","uri":"file:///new.go"}]}`)

	result := &CodeActionsResult{
		Path: mainPath, Buffer: mainBuffer, Version: mainBuffer.Version(), OpenBuffers: snapshots,
		Server: "test-server",
		Actions: []lsp.CodeAction{{
			Title:   "Unsupported edit with command",
			Edit:    unsupportedEdit,
			Command: &lsp.Command{Command: "custom.action"},
		}},
	}

	a.HandleCodeActionsResult(result)
	overlay, ok := a.Root.TopOverlayWidget().(*ui.WidgetAdapter)
	if !ok {
		t.Fatal("code action picker did not open")
	}
	dialog := overlay.W.(*widgets.DialogWidget)
	selectWidget := dialog.Content.(*widgets.SelectWidget)
	synctest.Test(t, func(t *testing.T) {
		selectWidget.Config.OnSelect("0")
		synctest.Wait()

		if a.Status.Notification == i18n.Text("status.codeActionApplied") {
			t.Fatalf("notification = %q, want unsupported edit rejected", a.Status.Notification)
		}
		if a.Status.Notification != i18n.Text("status.unsupportedWorkspaceEdit") {
			t.Fatalf("notification = %q, want unsupportedWorkspaceEdit warning", a.Status.Notification)
		}

		select {
		case ev := <-sim.EventQ():
			t.Fatalf("unexpected event from command execution: %+v", ev)
		default:
		}
		if lookups != 0 || len(client.commands) != 0 {
			t.Fatalf("unsupported edit dispatched a command: lookups=%d, commands=%+v", lookups, client.commands)
		}
	})
}
