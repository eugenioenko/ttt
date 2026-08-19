package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/gdamore/tcell/v3"
)

func TestOpenDiff(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	oldLines := []string{"line one", "line two", "line three"}
	newLines := []string{"line one", "line TWO", "line three", "line four"}

	h.app.EditorGroup.OpenDiff("test.go", diff.FileDiff{}, oldLines, newLines, true)
	h.redraw()

	h.assertContains("test.go (diff)")
}

func TestOpenBufferReadOnly(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	lines := []string{"hello world", "second line", "third line"}
	h.app.EditorGroup.OpenBufferReadOnly("test (abc123)", "test.go", lines)
	h.redraw()

	h.assertContains("test (abc123)")
	h.assertContains("hello world")

	if !h.app.EditorGroup.IsActiveReadOnly() {
		t.Fatal("expected tab to be readonly")
	}

	// Typing should be blocked
	h.pressRune('x')
	h.redraw()
	buf := h.app.EditorGroup.ActiveBuffer()
	if buf == nil {
		t.Fatal("expected buffer")
	}
	if buf.Lines[0] != "hello world" {
		t.Fatalf("expected buffer unchanged, got %q", buf.Lines[0])
	}

	// Enter should be blocked
	h.pressKey(tcell.KeyEnter, 0)
	h.redraw()
	if len(buf.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(buf.Lines))
	}

	// Backspace should be blocked
	h.pressKey(tcell.KeyBackspace2, 0)
	h.redraw()
	if buf.Lines[0] != "hello world" {
		t.Fatalf("expected buffer unchanged after backspace, got %q", buf.Lines[0])
	}

	// Save should be a no-op
	saved := h.app.EditorGroup.Save()
	if saved {
		t.Fatal("expected save to return false for readonly tab")
	}
}

func TestOpenFileReadOnly(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "readonly.txt")
	os.WriteFile(f, []byte("original content\n"), 0644)

	h.app.EditorGroup.OpenFileReadOnly(f, "readonly.txt (v1)")
	h.redraw()

	h.assertContains("readonly.txt (v1)")

	if !h.app.EditorGroup.IsActiveReadOnly() {
		t.Fatal("expected tab to be readonly")
	}

	// Typing should be blocked
	h.pressRune('z')
	h.redraw()
	buf := h.app.EditorGroup.ActiveBuffer()
	if buf == nil {
		t.Fatal("expected buffer")
	}
	if buf.Lines[0] != "original content" {
		t.Fatalf("expected buffer unchanged, got %q", buf.Lines[0])
	}
}

func TestOpenBufferReadOnlyReuseTab(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.app.EditorGroup.OpenBufferReadOnly("test (v1)", "test.go", []string{"first"})
	h.redraw()

	h.app.EditorGroup.OpenBufferReadOnly("test (v1)", "test.go", []string{"updated"})
	h.redraw()

	buf := h.app.EditorGroup.ActiveBuffer()
	if buf == nil {
		t.Fatal("expected buffer")
	}
	if buf.Lines[0] != "updated" {
		t.Fatalf("expected buffer content updated on reopen, got %q", buf.Lines[0])
	}
}
