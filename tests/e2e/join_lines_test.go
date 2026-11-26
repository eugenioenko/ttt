package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJoinLines_Basic(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "join.txt")
	requireNoError(t, os.WriteFile(f, []byte("hello\n    world\nfoo\n"), 0644))
	h.openFile(f)
	h.redraw()

	// Cursor starts on line 0
	h.exec("editor.joinLines")

	line := h.app.EditorGroup.Editor.Buf.Lines[0]
	if line != "hello world" {
		t.Errorf("expected 'hello world', got %q", line)
	}
	// Cursor should be at the join point (col 5)
	if h.app.EditorGroup.Editor.Cursor.Col != 5 {
		t.Errorf("expected cursor col 5, got %d", h.app.EditorGroup.Editor.Cursor.Col)
	}
}

func TestJoinLines_LastLine(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "joinlast.txt")
	requireNoError(t, os.WriteFile(f, []byte("only line"), 0644))
	h.openFile(f)
	h.redraw()

	h.exec("editor.joinLines")

	// Should do nothing
	if len(h.app.EditorGroup.Editor.Buf.Lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(h.app.EditorGroup.Editor.Buf.Lines))
	}
	if h.app.EditorGroup.Editor.Buf.Lines[0] != "only line" {
		t.Errorf("expected 'only line', got %q", h.app.EditorGroup.Editor.Buf.Lines[0])
	}
}

func TestJoinLines_Undo(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "joinundo.txt")
	requireNoError(t, os.WriteFile(f, []byte("aaa\n    bbb\nccc\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.exec("editor.joinLines")

	if h.app.EditorGroup.Editor.Buf.Lines[0] != "aaa bbb" {
		t.Errorf("after join: expected 'aaa bbb', got %q", h.app.EditorGroup.Editor.Buf.Lines[0])
	}

	h.exec("editor.undo")

	if h.app.EditorGroup.Editor.Buf.Lines[0] != "aaa" {
		t.Errorf("after undo line 0: expected 'aaa', got %q", h.app.EditorGroup.Editor.Buf.Lines[0])
	}
	if h.app.EditorGroup.Editor.Buf.Lines[1] != "    bbb" {
		t.Errorf("after undo line 1: expected '    bbb', got %q", h.app.EditorGroup.Editor.Buf.Lines[1])
	}
}

func TestJoinLines_CurrentLineEndsWithSpace(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "joinspace.txt")
	requireNoError(t, os.WriteFile(f, []byte("hello \n    world\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.exec("editor.joinLines")

	line := h.app.EditorGroup.Editor.Buf.Lines[0]
	if line != "hello world" {
		t.Errorf("expected 'hello world', got %q", line)
	}
}

func TestJoinLines_NextLineEmpty(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "joinempty.txt")
	requireNoError(t, os.WriteFile(f, []byte("hello\n\nworld\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.exec("editor.joinLines")

	line := h.app.EditorGroup.Editor.Buf.Lines[0]
	if line != "hello" {
		t.Errorf("expected 'hello', got %q", line)
	}
	if len(h.app.EditorGroup.Editor.Buf.Lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(h.app.EditorGroup.Editor.Buf.Lines))
	}
}

func TestJoinLines_Selection(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "joinsel.txt")
	requireNoError(t, os.WriteFile(f, []byte("aaa\n  bbb\n  ccc\nddd\n"), 0644))
	h.openFile(f)
	h.redraw()

	// Select lines 0-2
	e := h.app.EditorGroup.Editor
	e.Cursor.Line = 0
	e.Cursor.Col = 0
	e.Selection.Active = true
	e.Selection.Anchor.Line = 2
	e.Selection.Anchor.Col = 3

	h.exec("editor.joinLines")

	line := h.app.EditorGroup.Editor.Buf.Lines[0]
	if line != "aaa bbb ccc" {
		t.Errorf("expected 'aaa bbb ccc', got %q", line)
	}
	if len(h.app.EditorGroup.Editor.Buf.Lines) != 3 {
		t.Errorf("expected 3 lines (joined + ddd + trailing), got %d", len(h.app.EditorGroup.Editor.Buf.Lines))
	}
}
