package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBookmarkToggle(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "marks.txt")
	os.WriteFile(f, []byte("line1\nline2\nline3\nline4\nline5"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	// Toggle bookmark on line 0
	h.exec("bookmark.toggle")
	if !h.app.EditorGroup.Editor.Bookmarks[0] {
		t.Fatal("expected bookmark on line 0")
	}

	// Verify gutter shows bookmark indicator
	h.redraw()
	screen := h.screenText()
	if !strings.Contains(screen, "●") {
		t.Errorf("expected bookmark indicator in gutter, got:\n%s", screen)
	}

	// Toggle again to remove
	h.exec("bookmark.toggle")
	if h.app.EditorGroup.Editor.Bookmarks[0] {
		t.Fatal("expected bookmark on line 0 to be removed")
	}
}

func TestBookmarkNextPrev(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "nav.txt")
	os.WriteFile(f, []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	// Set bookmarks on lines 2 and 5 (0-indexed)
	h.app.EditorGroup.Editor.Cursor.Line = 2
	h.exec("bookmark.toggle")
	h.app.EditorGroup.Editor.Cursor.Line = 5
	h.exec("bookmark.toggle")

	// Go to line 0
	h.app.EditorGroup.Editor.Cursor.Line = 0
	h.app.EditorGroup.Editor.Cursor.Col = 0
	h.redraw()

	// Next bookmark should go to line 2
	h.exec("bookmark.next")
	if h.app.EditorGroup.Editor.Cursor.Line != 2 {
		t.Errorf("expected cursor on line 2, got %d", h.app.EditorGroup.Editor.Cursor.Line)
	}

	// Next bookmark should go to line 5
	h.exec("bookmark.next")
	if h.app.EditorGroup.Editor.Cursor.Line != 5 {
		t.Errorf("expected cursor on line 5, got %d", h.app.EditorGroup.Editor.Cursor.Line)
	}

	// Next bookmark should wrap to line 2
	h.exec("bookmark.next")
	if h.app.EditorGroup.Editor.Cursor.Line != 2 {
		t.Errorf("expected cursor to wrap to line 2, got %d", h.app.EditorGroup.Editor.Cursor.Line)
	}

	// Prev bookmark should go to line 5 (wrap backward)
	h.exec("bookmark.prev")
	if h.app.EditorGroup.Editor.Cursor.Line != 5 {
		t.Errorf("expected cursor on line 5 after prev, got %d", h.app.EditorGroup.Editor.Cursor.Line)
	}

	// Prev bookmark should go to line 2
	h.exec("bookmark.prev")
	if h.app.EditorGroup.Editor.Cursor.Line != 2 {
		t.Errorf("expected cursor on line 2 after prev, got %d", h.app.EditorGroup.Editor.Cursor.Line)
	}
}

func TestBookmarkClearAll(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "clear.txt")
	os.WriteFile(f, []byte("line1\nline2\nline3"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	// Set bookmarks on lines 0 and 2
	h.exec("bookmark.toggle")
	h.app.EditorGroup.Editor.Cursor.Line = 2
	h.exec("bookmark.toggle")

	if !h.app.EditorGroup.Editor.HasBookmarks() {
		t.Fatal("expected bookmarks to exist")
	}

	// Clear all
	h.exec("bookmark.clearAll")

	if h.app.EditorGroup.Editor.HasBookmarks() {
		t.Fatal("expected all bookmarks to be cleared")
	}
}

func TestBookmarkPerTab(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f1 := filepath.Join(h.dir, "tab1.txt")
	f2 := filepath.Join(h.dir, "tab2.txt")
	os.WriteFile(f1, []byte("file1-line1\nfile1-line2"), 0644)
	os.WriteFile(f2, []byte("file2-line1\nfile2-line2"), 0644)

	h.app.EditorGroup.OpenFile(f1)
	h.app.EditorGroup.PinActiveTab()
	h.redraw()
	h.exec("bookmark.toggle") // bookmark line 0 in tab1

	h.app.EditorGroup.OpenFile(f2)
	h.redraw()

	// Tab2 should have no bookmarks
	if h.app.EditorGroup.Editor.HasBookmarks() {
		t.Fatal("tab2 should have no bookmarks")
	}

	// Switch back to tab1
	h.app.EditorGroup.SwitchTab(0)
	h.redraw()

	// Tab1 should still have its bookmark
	if !h.app.EditorGroup.Editor.Bookmarks[0] {
		t.Fatal("tab1 should still have bookmark on line 0")
	}
}

func TestBookmarkNextNoBookmarks(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "empty.txt")
	os.WriteFile(f, []byte("no bookmarks"), 0644)
	h.app.EditorGroup.OpenFile(f)
	h.redraw()

	// Should be a no-op, not crash
	h.exec("bookmark.next")
	h.exec("bookmark.prev")
}
