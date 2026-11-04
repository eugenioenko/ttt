package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestAddCursorShortcutsAndTabState(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	path := filepath.Join(h.dir, "cursors.txt")
	const original = "alpha\nbravo\ncharlie\ndelta\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.app.FocusEditor()
	h.redraw()
	h.pressKey(tcell.KeyDown, 0)
	h.pressKey(tcell.KeyRight, 0)
	h.pressKey(tcell.KeyUp, tcell.ModAlt|tcell.ModShift)
	h.pressKey(tcell.KeyDown, tcell.ModAlt|tcell.ModShift)
	if got := h.app.EditorGroup.MultiCursorCount(); got != 3 {
		t.Fatalf("cursor count = %d, want 3", got)
	}
	h.app.EditorGroup.NewFile()
	h.redraw()
	h.app.EditorGroup.PrevTab()
	h.redraw()
	if got := h.app.EditorGroup.MultiCursorCount(); got != 3 {
		t.Fatalf("cursor count after switching tabs = %d, want 3", got)
	}
	h.exec("multicursor.undoCursor")
	if got := h.app.EditorGroup.MultiCursorCount(); got != 2 {
		t.Fatalf("cursor count after undo cursor = %d, want 2", got)
	}
	h.pressKey(tcell.KeyEscape, 0)
	if h.app.EditorGroup.IsMultiCursorActive() {
		t.Fatal("Escape should collapse to a single cursor")
	}
	h.exec("multicursor.addCursorAbove")
	h.exec("multicursor.addCursorBelow")
	h.pressRune('X')
	e := h.app.EditorGroup.Editor
	if got := strings.Join(e.Buf.Lines, "\n"); got != "aXlpha\nbXravo\ncXharlie\ndelta\n" {
		t.Fatalf("buffer after typing = %q", got)
	}
	h.exec("editor.undo")
	if got := strings.Join(e.Buf.Lines, "\n"); got != original {
		t.Fatalf("buffer after undo = %q", got)
	}
}
