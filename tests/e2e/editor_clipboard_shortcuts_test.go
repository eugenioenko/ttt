package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/gdamore/tcell/v3"
)

func TestEditorClipboardShortcuts(t *testing.T) {
	clipboard.DisableSystem()
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := filepath.Join(h.dir, "clipboard.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.app.Root.SetFocus(h.app.EditorGroup)
	e := h.app.EditorGroup.Editor
	e.Selection.Start(0, 0)
	e.Cursor.Col = 5
	h.redraw()

	h.pressKey(tcell.KeyInsert, tcell.ModCtrl)
	if got := clipboard.Get(); got != "hello" {
		t.Fatalf("Ctrl+Insert copied %q, want %q", got, "hello")
	}

	e.Selection.Clear()
	e.Cursor.Col = 5
	clipboard.Set(" world")
	h.pressKey(tcell.KeyInsert, tcell.ModShift)
	if got := e.Buf.Lines[0]; got != "hello world" {
		t.Fatalf("Shift+Insert produced %q, want %q", got, "hello world")
	}

	e.Selection.Start(0, 0)
	e.Cursor.Col = 5
	h.pressCtrl(tcell.KeyCtrlC)
	e.Selection.Clear()
	e.Cursor.Col = len([]rune(e.Buf.Lines[0]))
	h.pressCtrl(tcell.KeyCtrlV)
	if got := e.Buf.Lines[0]; got != "hello worldhello" {
		t.Fatalf("Ctrl+V produced %q, want %q", got, "hello worldhello")
	}
}
