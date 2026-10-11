package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestSelectAllCollapsesMultiCursors(t *testing.T) {
	h := newTestHarness(t, 80, 30)
	defer h.stop()

	f := filepath.Join(h.dir, "words.txt")
	os.WriteFile(f, []byte("foo bar\nbaz foo\nend foo"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	editor := h.app.EditorGroup.Editor
	editor.Cursor.Line, editor.Cursor.Col = 0, 1
	h.exec("multicursor.selectAll")
	if editor.Multi == nil || len(editor.Multi.Cursors) != 3 {
		t.Fatalf("fixture: want 3 cursors on foo, got %+v", editor.Multi)
	}

	h.exec("editor.selectAll")
	if editor.Multi != nil && editor.Multi.IsMulti() {
		t.Fatalf("select all kept %d cursors", len(editor.Multi.Cursors))
	}

	h.pressRune('x')
	h.pressKey(tcell.KeyTab, tcell.ModNone)
	if got := strings.Join(editor.Buf.Lines, "\n"); strings.TrimSpace(got) != "x" || strings.Contains(got, "\n") {
		t.Fatalf("typing over select all should replace the whole file, got %q", got)
	}
}
