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

func TestToggleCursorAltClick(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	path := filepath.Join(h.dir, "toggle_cursors.txt")
	const original = "alpha\nbravo\ncharlie\ndelta\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.app.FocusEditor()
	h.redraw()

	e := h.app.EditorGroup.Editor
	r := e.GetRect()
	gutterW := e.GutterWidth()

	// Initial state: 1 cursor at (0, 0)
	if count := h.app.EditorGroup.MultiCursorCount(); count != 1 {
		t.Fatalf("initial cursor count = %d, want 1", count)
	}

	// Alt+Click line 1 ('bravo') at col 2
	line1X := r.X + gutterW + 2
	line1Y := r.Y + 1
	h.app.Root.HandleEvent(tcell.NewEventMouse(line1X, line1Y, tcell.Button1, tcell.ModAlt))
	h.app.Root.HandleEvent(tcell.NewEventMouse(line1X, line1Y, tcell.ButtonNone, 0))
	if count := h.app.EditorGroup.MultiCursorCount(); count != 2 {
		t.Fatalf("cursor count after Alt+Click line 1 = %d, want 2", count)
	}
	if e.Multi == nil || len(e.Multi.Cursors) != 2 || e.Multi.Cursors[1].Line != 1 || e.Multi.Cursors[1].Col != 2 {
		t.Fatalf("cursors after Alt+Click line 1 = %+v, want (1, 2)", e.Multi.Cursors)
	}

	// Alt+Click line 2 ('charlie') at col 3
	line2X := r.X + gutterW + 3
	line2Y := r.Y + 2
	h.app.Root.HandleEvent(tcell.NewEventMouse(line2X, line2Y, tcell.Button1, tcell.ModAlt))
	h.app.Root.HandleEvent(tcell.NewEventMouse(line2X, line2Y, tcell.ButtonNone, 0))
	if count := h.app.EditorGroup.MultiCursorCount(); count != 3 {
		t.Fatalf("cursor count after Alt+Click line 2 = %d, want 3", count)
	}
	if e.Multi == nil || len(e.Multi.Cursors) != 3 || e.Multi.Cursors[2].Line != 2 || e.Multi.Cursors[2].Col != 3 {
		t.Fatalf("cursors after Alt+Click line 2 = %+v, want (2, 3)", e.Multi.Cursors)
	}

	// Alt+Click line 1 at col 2 again -> toggles off that cursor!
	h.app.Root.HandleEvent(tcell.NewEventMouse(line1X, line1Y, tcell.Button1, tcell.ModAlt))
	h.app.Root.HandleEvent(tcell.NewEventMouse(line1X, line1Y, tcell.ButtonNone, 0))
	if count := h.app.EditorGroup.MultiCursorCount(); count != 2 {
		t.Fatalf("cursor count after Alt+Click toggle off line 1 = %d, want 2", count)
	}

	// Alt+Click line 2 at col 3 again -> toggles off line 2 cursor, leaving only the initial cursor!
	h.app.Root.HandleEvent(tcell.NewEventMouse(line2X, line2Y, tcell.Button1, tcell.ModAlt))
	h.app.Root.HandleEvent(tcell.NewEventMouse(line2X, line2Y, tcell.ButtonNone, 0))
	if count := h.app.EditorGroup.MultiCursorCount(); count != 1 {
		t.Fatalf("cursor count after toggling to last cursor = %d, want 1", count)
	}
	if h.app.EditorGroup.IsMultiCursorActive() {
		t.Fatal("expected multi cursor to be inactive after removing down to 1 cursor")
	}

	// Alt+Click the sole remaining cursor -> must leave it in place!
	line0X := r.X + gutterW
	line0Y := r.Y
	h.app.Root.HandleEvent(tcell.NewEventMouse(line0X, line0Y, tcell.Button1, tcell.ModAlt))
	h.app.Root.HandleEvent(tcell.NewEventMouse(line0X, line0Y, tcell.ButtonNone, 0))
	if count := h.app.EditorGroup.MultiCursorCount(); count != 1 {
		t.Fatalf("cursor count after Alt+Click sole cursor = %d, want 1", count)
	}
	if e.Cursor.Line != 0 || e.Cursor.Col != 0 {
		t.Fatalf("cursor position = (%d, %d), want (0, 0)", e.Cursor.Line, e.Cursor.Col)
	}
}
