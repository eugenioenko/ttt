package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestUpperCase(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "upper.txt")
	requireNoError(t, os.WriteFile(f, []byte("Hello World\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.exec("editor.selectAll")
	h.exec("editor.upperCase")
	h.redraw()

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "HELLO WORLD" {
		t.Errorf("expected 'HELLO WORLD', got %q", got)
	}
}

func TestLowerCase(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "lower.txt")
	requireNoError(t, os.WriteFile(f, []byte("Hello World\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.exec("editor.selectAll")
	h.exec("editor.lowerCase")
	h.redraw()

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "hello world" {
		t.Errorf("expected 'hello world', got %q", got)
	}
}

func TestTitleCase(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "title.txt")
	requireNoError(t, os.WriteFile(f, []byte("hello world\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.exec("editor.selectAll")
	h.exec("editor.titleCase")
	h.redraw()

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", got)
	}
}

func TestCaseTransformNoSelection(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "nosel.txt")
	requireNoError(t, os.WriteFile(f, []byte("Hello World\n"), 0644))
	h.openFile(f)
	h.redraw()

	// Run transforms without selection - should be no-op
	h.exec("editor.upperCase")
	h.exec("editor.lowerCase")
	h.exec("editor.titleCase")
	h.redraw()

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "Hello World" {
		t.Errorf("expected 'Hello World' (unchanged), got %q", got)
	}
}

func TestTitleCaseApostrophe(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "apostrophe.txt")
	requireNoError(t, os.WriteFile(f, []byte("don't stop\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.exec("editor.selectAll")
	h.exec("editor.titleCase")
	h.redraw()

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "Don't Stop" {
		t.Errorf("expected \"Don't Stop\", got %q", got)
	}
}

func TestTitleCaseHyphen(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "hyphen.txt")
	requireNoError(t, os.WriteFile(f, []byte("self-aware\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.exec("editor.selectAll")
	h.exec("editor.titleCase")
	h.redraw()

	got := h.app.EditorGroup.Editor.Buf.Lines[0]
	if got != "Self-Aware" {
		t.Errorf("expected \"Self-Aware\", got %q", got)
	}
}

func TestUpperCaseMultiLine(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "multiupper.txt")
	requireNoError(t, os.WriteFile(f, []byte("hello\nworld\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.exec("editor.selectAll")
	h.exec("editor.upperCase")
	h.redraw()

	if h.app.EditorGroup.Editor.Buf.Lines[0] != "HELLO" {
		t.Errorf("expected 'HELLO', got %q", h.app.EditorGroup.Editor.Buf.Lines[0])
	}
	if h.app.EditorGroup.Editor.Buf.Lines[1] != "WORLD" {
		t.Errorf("expected 'WORLD', got %q", h.app.EditorGroup.Editor.Buf.Lines[1])
	}
}

func TestUpperCaseMultiLineUndo(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "undo.txt")
	requireNoError(t, os.WriteFile(f, []byte("hello\nworld\nthird\nfourth\n"), 0644))
	h.openFile(f)
	h.redraw()

	ed := h.app.EditorGroup.Editor
	// Select lines 0-2 via shift+down x3
	for range 3 {
		h.pressKey(tcell.KeyDown, tcell.ModShift)
	}
	h.exec("editor.upperCase")
	h.redraw()

	if ed.Buf.Lines[0] != "HELLO" {
		t.Errorf("after transform: expected 'HELLO', got %q", ed.Buf.Lines[0])
	}
	if ed.Buf.Lines[3] != "fourth" {
		t.Errorf("after transform: expected 'fourth' untouched, got %q", ed.Buf.Lines[3])
	}

	h.exec("editor.undo")
	h.redraw()

	if ed.Buf.Lines[0] != "hello" {
		t.Errorf("after 1 undo: expected 'hello', got %q", ed.Buf.Lines[0])
	}
	if ed.Buf.Lines[1] != "world" {
		t.Errorf("after 1 undo: expected 'world', got %q", ed.Buf.Lines[1])
	}
	if ed.Buf.Lines[2] != "third" {
		t.Errorf("after 1 undo: expected 'third', got %q", ed.Buf.Lines[2])
	}
	if ed.Buf.Lines[3] != "fourth" {
		t.Errorf("after 1 undo: expected 'fourth', got %q", ed.Buf.Lines[3])
	}
}
