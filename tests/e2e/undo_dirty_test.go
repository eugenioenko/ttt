package e2e

import (
	"os"
	"path/filepath"
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
