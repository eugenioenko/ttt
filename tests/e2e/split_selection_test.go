package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitSelectionToLines(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "split.txt")
	requireNoError(t, os.WriteFile(f, []byte("line1\nline2\nline3\nline4\n"), 0644))
	h.openFile(f)
	h.redraw()

	editor := h.app.EditorGroup.Editor
	if editor == nil {
		t.Fatal("editor is nil after opening file")
	}

	// Select lines 0-2 (first 3 lines): anchor at (0,0), cursor at (3,0)
	editor.Cursor.Line = 3
	editor.Cursor.Col = 0
	editor.Selection.Start(0, 0)
	h.redraw()

	// Execute the split selection command
	h.exec("editor.splitSelectionToLines")

	// Should have created multi-cursors (one per line)
	if editor.Multi == nil {
		t.Fatal("expected multi-cursor to be active after split")
	}
	if len(editor.Multi.Cursors) != 3 {
		t.Errorf("expected 3 cursors, got %d", len(editor.Multi.Cursors))
	}
	// Selection should be cleared
	if editor.Selection.Active {
		t.Error("expected selection to be cleared after split")
	}
}

func TestSplitSelectionToLines_NoSelection(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "nosplit.txt")
	requireNoError(t, os.WriteFile(f, []byte("line1\nline2\nline3\n"), 0644))
	h.openFile(f)
	h.redraw()

	editor := h.app.EditorGroup.Editor

	// No selection active — should be a no-op
	h.exec("editor.splitSelectionToLines")

	if editor.Multi != nil && editor.Multi.IsMulti() {
		t.Error("expected no multi-cursor when no selection is active")
	}
}

func TestSplitSelectionToLines_SingleLine(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "single.txt")
	requireNoError(t, os.WriteFile(f, []byte("hello world\nline2\n"), 0644))
	h.openFile(f)
	h.redraw()

	editor := h.app.EditorGroup.Editor

	// Select within a single line
	editor.Cursor.Line = 0
	editor.Cursor.Col = 5
	editor.Selection.Start(0, 0)
	h.redraw()

	h.exec("editor.splitSelectionToLines")

	if editor.Multi != nil && editor.Multi.IsMulti() {
		t.Error("expected no multi-cursor for single-line selection")
	}
}

func TestSplitSelectionToLines_TypeAtAllCursors(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "typesplit.txt")
	requireNoError(t, os.WriteFile(f, []byte("aaa\nbbb\nccc\nddd\n"), 0644))
	h.openFile(f)
	h.redraw()

	editor := h.app.EditorGroup.Editor

	// Select from start of line 0 to start of line 3 (covers lines 0,1,2)
	editor.Cursor.Line = 3
	editor.Cursor.Col = 0
	editor.Selection.Start(0, 0)
	h.redraw()

	h.exec("editor.splitSelectionToLines")

	// Verify 3 cursors at end of each line
	if editor.Multi == nil {
		t.Fatal("expected multi-cursor to be active")
	}
	if len(editor.Multi.Cursors) != 3 {
		t.Fatalf("expected 3 cursors, got %d", len(editor.Multi.Cursors))
	}

	// Type a character — it should appear on all 3 cursor lines
	h.pressRune('X')

	// Check buffer content
	if editor.Buf.Lines[0] != "aaaX" {
		t.Errorf("expected line 0 to be 'aaaX', got %q", editor.Buf.Lines[0])
	}
	if editor.Buf.Lines[1] != "bbbX" {
		t.Errorf("expected line 1 to be 'bbbX', got %q", editor.Buf.Lines[1])
	}
	if editor.Buf.Lines[2] != "cccX" {
		t.Errorf("expected line 2 to be 'cccX', got %q", editor.Buf.Lines[2])
	}
	// Line 3 should be untouched
	if editor.Buf.Lines[3] != "ddd" {
		t.Errorf("expected line 3 to remain 'ddd', got %q", editor.Buf.Lines[3])
	}
}
