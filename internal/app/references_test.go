package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/lsp"
)

func TestShowReferencesReadsFromBufferForUnsavedChanges(t *testing.T) {
	h := newGitGutterTestHarness(t)
	a := h.a

	// file.txt on disk contains "one\ntwo\n". Insert a line at the top of
	// the buffer without saving so the disk and buffer diverge.
	a.EditorGroup.Editor.ExecCommand(&undo.InsertLineCommand{Idx: 0, Text: "INSERTED"})

	path := a.EditorGroup.ActiveFilePath()
	a.ShowReferences([]lsp.Location{{
		URI:   FileURI(path),
		Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 0}},
	}})

	if len(a.References.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(a.References.Items))
	}
	if got := a.References.Items[0].Text; got != "INSERTED" {
		t.Errorf("line text = %q, want %q (should come from buffer)", got, "INSERTED")
	}
}

func TestShowReferencesFallsBackToDiskWhenFileNotOpen(t *testing.T) {
	h := newGitGutterTestHarness(t)
	a := h.a

	dir := t.TempDir()
	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(other, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a.ShowReferences([]lsp.Location{{
		URI:   FileURI(other),
		Range: lsp.Range{Start: lsp.Position{Line: 1, Character: 0}},
	}})

	if len(a.References.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(a.References.Items))
	}
	if got := a.References.Items[0].Text; got != "world" {
		t.Errorf("line text = %q, want %q (should come from disk)", got, "world")
	}
}
