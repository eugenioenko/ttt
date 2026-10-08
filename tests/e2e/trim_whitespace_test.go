package e2e

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestTrimTrailingWhitespaceWholeBuffer(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "trim.txt")
	os.WriteFile(f, []byte("a  \n\tb\t\nc \t \n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	h.exec("editor.trimTrailingWhitespace")

	want := []string{"a", "\tb", "c", ""}
	if got := h.app.EditorGroup.Editor.Buf.Lines; !slices.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestTrimTrailingWhitespaceSelection(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "trimsel.txt")
	os.WriteFile(f, []byte("one  \ntwo\t\nthree  \n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	ed := h.app.EditorGroup.Editor
	ed.Selection.Start(0, 0)
	ed.Cursor.Line = 1
	ed.Cursor.Col = 2
	ed.Selection.Active = true

	h.exec("editor.trimTrailingWhitespace")

	want := []string{"one", "two", "three  ", ""}
	if got := ed.Buf.Lines; !slices.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestTrimTrailingWhitespaceSingleUndo(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "trimundo.txt")
	content := "a  \n\tb\t\nc \t \n"
	os.WriteFile(f, []byte(content), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	original := slices.Clone(h.app.EditorGroup.Editor.Buf.Lines)

	h.exec("editor.trimTrailingWhitespace")
	h.exec("editor.undo")
	if got := h.app.EditorGroup.Editor.Buf.Lines; !slices.Equal(got, original) {
		t.Errorf("expected one undo to restore %q, got %q", original, got)
	}
}

func TestTrimTrailingWhitespaceKeepsCursorOnTrimmedLine(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "trimcursor.txt")
	os.WriteFile(f, []byte("c    \n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	h.app.EditorGroup.Editor.Cursor.Col = 5
	h.exec("editor.trimTrailingWhitespace")
	h.pressRune('x')

	if got := h.app.EditorGroup.Editor.Buf.Lines[0]; got != "cx" {
		t.Errorf("expected typing after the trim to append to the line, got %q", got)
	}
}

func TestTrimTrailingWhitespaceNoOp(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "trimclean.txt")
	os.WriteFile(f, []byte("clean\n\tindented\n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	h.exec("editor.trimTrailingWhitespace")

	ed := h.app.EditorGroup.Editor
	if want := []string{"clean", "\tindented", ""}; !slices.Equal(ed.Buf.Lines, want) {
		t.Errorf("expected %q, got %q", want, ed.Buf.Lines)
	}
	if ed.Buf.Dirty {
		t.Error("expected a buffer with no trailing whitespace to stay clean")
	}
	h.exec("editor.undo")
	if ed.Buf.Dirty {
		t.Error("expected no undo entry for a no-op trim")
	}
}

func TestTrimTrailingWhitespaceReadOnly(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	h.app.EditorGroup.OpenBufferReadOnly("readonly", "", []string{"a  ", "b\t", ""})
	h.redraw()

	h.exec("editor.trimTrailingWhitespace")

	want := []string{"a  ", "b\t", ""}
	if got := h.app.EditorGroup.Editor.Buf.Lines; !slices.Equal(got, want) {
		t.Errorf("expected read-only buffer to stay %q, got %q", want, got)
	}
}
