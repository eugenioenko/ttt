package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBookmarkCommandsToggleAndNavigate(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	path := filepath.Join(h.dir, "marks.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.redraw()
	e := h.app.EditorGroup.Editor
	e.Cursor.Line = 1

	h.exec("bookmark.toggle")
	if _, ok := e.GetBookmark(1); !ok {
		t.Fatal("bookmark.toggle did not mark the cursor line")
	}

	e.Cursor.Line = 0
	h.exec("bookmark.next")
	if e.Cursor.Line != 1 {
		t.Fatalf("bookmark.next moved to line %d, want 1", e.Cursor.Line)
	}

	e.Cursor.Line = 3
	h.exec("bookmark.next")
	if e.Cursor.Line != 1 {
		t.Fatalf("bookmark.next from the end moved to line %d, want it to wrap to 1", e.Cursor.Line)
	}

	e.Cursor.Line = 0
	h.exec("bookmark.previous")
	if e.Cursor.Line != 1 {
		t.Fatalf("bookmark.previous from the start moved to line %d, want it to wrap to 1", e.Cursor.Line)
	}

	h.exec("bookmark.toggle")
	if _, ok := e.GetBookmark(1); ok {
		t.Fatal("bookmark.toggle did not remove the bookmark on the cursor line")
	}
}
