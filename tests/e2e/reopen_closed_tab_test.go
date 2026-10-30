package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func writeTabFiles(t *testing.T, dir string, names ...string) []string {
	t.Helper()
	var paths []string
	for _, n := range names {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("first line\nsecond line\n"), 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return paths
}

func TestReopenClosedTabRestoresFileAndCursor(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	p := writeTabFiles(t, h.dir, "a.txt", "b.txt")
	h.app.EditorGroup.OpenFile(p[0])
	h.app.EditorGroup.OpenFile(p[1])
	h.app.EditorGroup.GoToLineCol(2, 4)
	h.exec("tab.close")
	if got := h.app.EditorGroup.ActiveFilePath(); got == p[1] {
		t.Fatal("b.txt still active after closing it")
	}

	h.exec("tab.reopenClosed")

	if got := h.app.EditorGroup.ActiveFilePath(); got != p[1] {
		t.Fatalf("active = %q, want %q", got, p[1])
	}
	if c := h.app.EditorGroup.Editor.Cursor; c.Line != 1 || c.Col != 3 {
		t.Errorf("cursor = %d:%d, want 1:3", c.Line, c.Col)
	}
	tabs := h.app.EditorGroup.TabCount()
	c := writeTabFiles(t, h.dir, "c.txt")[0]
	h.app.EditorGroup.OpenFile(c)
	if got := h.app.EditorGroup.TabCount(); got != tabs+1 {
		t.Errorf("tabs = %d after opening another file, want %d: the reopened tab must not be a preview", got, tabs+1)
	}
}

func TestReopenClosedTabAfterCloseAllGoesNewestFirst(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	p := writeTabFiles(t, h.dir, "a.txt", "b.txt", "gone.txt")
	for _, f := range p {
		h.app.EditorGroup.OpenFile(f)
		h.app.EditorGroup.OpenFile(f) // a second open keeps the preview tab

	}
	h.exec("tab.closeAll")
	if err := os.Remove(p[2]); err != nil {
		t.Fatal(err)
	}

	h.exec("tab.reopenClosed")
	if got := h.app.EditorGroup.ActiveFilePath(); got != p[1] {
		t.Fatalf("first reopen = %q, want %q (the deleted file skipped)", got, p[1])
	}
	h.exec("tab.reopenClosed")
	if got := h.app.EditorGroup.ActiveFilePath(); got != p[0] {
		t.Fatalf("second reopen = %q, want %q", got, p[0])
	}
}

func TestCtrlShiftTReopensClosedTab(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	p := writeTabFiles(t, h.dir, "a.txt")
	h.app.EditorGroup.OpenFile(p[0])
	h.exec("tab.close")

	// Kitty-protocol terminals report Ctrl+Shift+T as the unshifted rune with
	// both modifiers.
	h.app.Root.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "t", tcell.ModCtrl|tcell.ModShift))
	h.redraw()

	if got := h.app.EditorGroup.ActiveFilePath(); got != p[0] {
		t.Fatalf("active = %q after Ctrl+Shift+T, want %q", got, p[0])
	}
}
