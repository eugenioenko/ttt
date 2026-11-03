package e2e

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/view"
	"github.com/gdamore/tcell/v3"
)

// editorText returns the active buffer's contents joined by newlines.
func editorText(h *testHarness) string {
	return strings.Join(h.app.EditorGroup.Editor.Buf.Lines, "\n")
}

func TestExternalChangeReloadsCleanBuffer(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := filepath.Join(h.dir, "reload.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.redraw()

	if !strings.Contains(editorText(h), "original") {
		t.Fatalf("expected editor to show original content, got %q", editorText(h))
	}

	modifyOnDisk(t, path, "updated externally\n")
	// Simulate the watcher firing for this path.
	h.app.HandleFileChanged(path)
	h.redraw()

	if !strings.Contains(editorText(h), "updated externally") {
		t.Errorf("expected clean buffer to auto-reload, got %q", editorText(h))
	}
	if got := h.app.Status.Notification; got != "" {
		t.Errorf("expected silent reload, got notification %q", got)
	}
}

func TestExternalChangeKeepsDirtyBuffer(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := filepath.Join(h.dir, "dirty.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.app.Root.SetFocus(h.app.EditorGroup)
	h.redraw()

	// Make an unsaved edit so the buffer is dirty.
	h.pressRune('Z')
	if !h.app.EditorGroup.IsDirtyPath(path) {
		t.Fatalf("expected buffer to be dirty after typing")
	}
	dirtyBefore := editorText(h)

	modifyOnDisk(t, path, "updated externally\n")
	h.app.HandleFileChanged(path)
	h.redraw()

	// A dirty buffer must NOT be reloaded — the user's edits survive.
	if editorText(h) != dirtyBefore {
		t.Errorf("dirty buffer was overwritten by reload: %q", editorText(h))
	}
	if strings.Contains(editorText(h), "updated externally") {
		t.Errorf("dirty buffer should not have picked up disk content")
	}
}

func TestExternalChangeClampsCursor(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := filepath.Join(h.dir, "shrink.txt")
	if err := os.WriteFile(path, []byte("line1\nline2\nline3\nline4\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.app.Root.SetFocus(h.app.EditorGroup)
	h.redraw()

	// Move the cursor down to a line that will not exist after the reload.
	for i := 0; i < 3; i++ {
		h.pressKey(tcell.KeyDown, tcell.ModNone)
	}

	modifyOnDisk(t, path, "only one line\n")
	h.app.HandleFileChanged(path)
	h.redraw()

	line, _ := h.app.EditorGroup.ActiveCursor()
	if line >= len(h.app.EditorGroup.Editor.Buf.Lines) {
		t.Errorf("cursor line %d out of bounds after reload (buffer has %d lines)",
			line, len(h.app.EditorGroup.Editor.Buf.Lines))
	}
}

func TestExternalDeletionWarnsAndPreservesBuffer(t *testing.T) {
	for _, name := range []string{"clean", "dirty"} {
		t.Run(name, func(t *testing.T) {
			h := newTestHarness(t, 80, 24)
			defer h.stop()
			path := filepath.Join(h.dir, "deleted.txt")
			if err := os.WriteFile(path, []byte("original\n"), 0644); err != nil {
				t.Fatal(err)
			}
			h.app.EditorGroup.OpenFile(path)
			h.app.Root.SetFocus(h.app.EditorGroup)
			if name == "dirty" {
				h.pressRune('Z')
			}
			buf := h.app.EditorGroup.ActiveBuffer()
			before, dirty := editorText(h), buf.Dirty
			line, col := h.app.EditorGroup.ActiveCursor()
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}

			h.app.HandleFileChanged(path)
			h.redraw()

			h.assertContains("deleted.txt was deleted on disk")
			if h.app.Status.NotifyLevel != view.NotifyWarning {
				t.Fatalf("expected deletion warning, got level %v", h.app.Status.NotifyLevel)
			}
			if h.app.EditorGroup.ActiveBuffer() != buf || editorText(h) != before || buf.Dirty != dirty {
				t.Fatal("deletion changed the buffer or its dirty state")
			}
			if gotLine, gotCol := h.app.EditorGroup.ActiveCursor(); gotLine != line || gotCol != col {
				t.Fatalf("deletion moved cursor to %d:%d, want %d:%d", gotLine, gotCol, line, col)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("deleted file should remain absent, got %v", err)
			}
			if dirty {
				h.exec("editor.undo")
				if editorText(h) != "original\n" || buf.Dirty {
					t.Fatal("deletion did not preserve undo history and the save point")
				}
				h.exec("editor.redo")
				if editorText(h) != before || !buf.Dirty {
					t.Fatal("deletion did not preserve redo history")
				}
			}

			h.exec("file.save")
			if h.app.Root.HasOverlay() {
				t.Fatal("saving a deleted file should recreate it without a conflict dialog")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != before || buf.Dirty {
				t.Fatalf("save did not recreate the buffer contents: %q, dirty = %v", data, buf.Dirty)
			}
		})
	}
}

func TestExternalChangePermissionErrorIsNotDeletion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod does not deny stat on Windows")
	}
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	dir := filepath.Join(h.dir, "restricted")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.app.Root.SetFocus(h.app.EditorGroup)
	h.pressRune('Z')
	buf := h.app.EditorGroup.ActiveBuffer()
	if !buf.Dirty {
		t.Fatal("expected an unsaved edit before the file check")
	}
	before := editorText(h)
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsPermission(err) {
		t.Skipf("directory permissions do not prevent stat: %v", err)
	}

	h.app.HandleFileChanged(path)

	if got := h.app.Status.Notification; !strings.Contains(got, "Failed to check file.txt:") || strings.Contains(got, "deleted") {
		t.Fatalf("expected a file check error instead of a deletion warning, got %q", got)
	}
	if h.app.Status.NotifyLevel != view.NotifyWarning {
		t.Fatalf("expected file check warning, got level %v", h.app.Status.NotifyLevel)
	}
	if h.app.EditorGroup.ActiveBuffer() != buf || editorText(h) != before || !buf.Dirty {
		t.Fatal("failed file check changed the buffer")
	}
}
