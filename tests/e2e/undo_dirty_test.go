package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
)

func TestUndoToClearsDirty(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "dirty.txt")
	os.WriteFile(f, []byte("hello\n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	h.pressRune('X')
	h.redraw()

	if !h.app.EditorGroup.IsDirty() {
		t.Fatal("expected dirty after typing")
	}

	h.exec("editor.undo")
	h.redraw()

	if h.app.EditorGroup.IsDirty() {
		t.Fatal("expected clean after undo to original state")
	}
}

func TestUndoToSavePoint(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "savepoint.txt")
	os.WriteFile(f, []byte("hello\n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	h.pressRune('A')
	h.redraw()

	h.app.EditorGroup.Save()
	h.redraw()

	if h.app.EditorGroup.IsDirty() {
		t.Fatal("expected clean after save")
	}

	h.pressRune('B')
	h.redraw()

	if !h.app.EditorGroup.IsDirty() {
		t.Fatal("expected dirty after typing post-save")
	}

	h.exec("editor.undo")
	h.redraw()

	if h.app.EditorGroup.IsDirty() {
		t.Fatal("expected clean after undo to save point")
	}
}

func TestPluginSetLineIdenticalTextIsUndoNoop(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "noop.txt")
	os.WriteFile(f, []byte("hello\n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	h.pressRune('X')
	h.redraw()

	api := app.NewPluginEditorAPI(h.app)
	api.SetLine(0, "Xhello")
	h.redraw()

	h.exec("editor.undo")
	h.redraw()

	buf := h.app.EditorGroup.ActiveBuffer()
	if got := buf.Lines[0]; got != "hello" {
		t.Fatalf("undo after no-op set_line should undo the typed edit, got %q", got)
	}
}

func TestPluginSetLineDifferentTextPushesUndo(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "change.txt")
	os.WriteFile(f, []byte("hello\n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	api := app.NewPluginEditorAPI(h.app)
	api.SetLine(0, "world")
	h.redraw()

	h.exec("editor.undo")
	h.redraw()

	buf := h.app.EditorGroup.ActiveBuffer()
	if got := buf.Lines[0]; got != "hello" {
		t.Fatalf("undo after set_line change should restore the original line, got %q", got)
	}
}

func TestPluginReplaceInvalidRangeIsNoop(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "invalid.txt")
	os.WriteFile(f, []byte("alpha\nbeta\ngamma\n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	api := app.NewPluginEditorAPI(h.app)
	api.Replace(0, 0, -1, 0, "x")
	api.Replace(0, -1, 0, 2, "x")
	api.Replace(0, 0, 0, -1, "x")
	api.Replace(2, 0, 0, 3, "x")
	api.Replace(0, 4, 0, 1, "x")
	api.Insert(0, -1, "x")
	h.redraw()

	buf := h.app.EditorGroup.ActiveBuffer()
	if got := strings.Join(buf.Lines, "\n"); got != "alpha\nbeta\ngamma\n" {
		t.Fatalf("invalid ranges should leave the buffer unchanged, got %q", got)
	}
	if buf.Dirty {
		t.Fatal("invalid ranges should not mark the buffer dirty")
	}
}

func TestPluginEditPastLineEndUndoes(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "pastend.txt")
	os.WriteFile(f, []byte("hello\n"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	api := app.NewPluginEditorAPI(h.app)
	buf := h.app.EditorGroup.ActiveBuffer()

	api.Insert(0, 99, "!")
	if got := buf.Lines[0]; got != "hello!" {
		t.Fatalf("insert past line end should append, got %q", got)
	}
	h.exec("editor.undo")
	if got := buf.Lines[0]; got != "hello" {
		t.Fatalf("undo of insert past line end got %q", got)
	}

	api.Replace(0, 99, 0, 99, "?")
	if got := buf.Lines[0]; got != "hello?" {
		t.Fatalf("replace past line end should append, got %q", got)
	}
	h.exec("editor.undo")
	if got := buf.Lines[0]; got != "hello" {
		t.Fatalf("undo of replace past line end got %q", got)
	}
}

func TestDeleteLineOnEmptyBufferStaysClean(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "empty.txt")
	os.WriteFile(f, []byte(""), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	h.exec("editor.deleteLine")
	h.redraw()

	if h.app.EditorGroup.IsDirty() {
		t.Fatal("deleting the only, empty line changes nothing and should not mark the buffer dirty")
	}
}
