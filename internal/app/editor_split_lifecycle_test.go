package app

import (
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestSplitGroupOwnsAndReleasesFoldTasks(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(a.CloseTasks)
	count := len(a.tasks.tasks)
	a.SplitEditor()
	if a.EditorGroup.FoldTask == nil || a.EditorGroup.FoldDelay == nil || len(a.tasks.tasks) != count+2 {
		t.Fatal("split group did not register its own fold tasks")
	}
	a.CloseEditorSplit()
	if len(a.tasks.tasks) != count {
		t.Fatal("closing the group leaked its fold tasks")
	}
}

func TestSplitRetainsInactiveDocumentSnapshotsAndDiagnostics(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(a.CloseTasks)
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	owner := a.EditorGroup
	if err := owner.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	owner.SetDiagnosticsSource("plugin:test", path, []ui.Diagnostic{{Message: "existing"}})
	original := owner.ActiveBuffer()
	a.SplitEditor()
	peer := a.EditorGroup
	if got := a.snapshotCodeActionBuffers()[path]; got.Buffer != original || got.Version != original.Version() {
		t.Fatal("inactive document missing from code-action snapshot")
	}
	if err := peer.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	if got := peer.DiagnosticsByPath()[path]; len(got) != 1 || got[0].Message != "existing" {
		t.Fatal("new split lost existing diagnostics")
	}
	a.DocVersions[path] = 3
	key := pullKey{path: path, version: 3, epoch: a.lspEpochs["test"]}
	if !a.setPulledDiagnostics("test", key, pulledDiagnostics{replace: true, diags: []lsp.Diagnostic{{Message: "pulled"}}}) {
		t.Fatal("diagnostics not applied")
	}
	for _, group := range a.editorGroups() {
		if len(group.DiagnosticsByPath()[path]) != 2 {
			t.Fatal("diagnostics missing from a document view")
		}
	}
	a.ClearPluginSources("test")
	for _, group := range a.editorGroups() {
		if got := group.DiagnosticsByPath()[path]; len(got) != 1 || got[0].Message != "pulled" {
			t.Fatal("plugin diagnostics remained in a document view")
		}
	}
}
