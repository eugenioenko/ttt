package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func (h *testHarness) saveAsNewFile(text, path string) {
	h.t.Helper()
	h.exec("file.new")
	h.typeText(text)
	h.exec("file.saveAs")
	h.redraw()
	h.typeText(path)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	h.redraw()
}

func TestSaveAsExistingFileAsksBeforeReplacing(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := filepath.Join(h.dir, "precious.txt")
	if err := os.WriteFile(path, []byte("precious\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.saveAsNewFile("scratch", path)

	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected a confirmation dialog, got %d overlays", len(h.app.Root.Overlays))
	}
	h.assertContains("already exists")
	if got := readFile(t, path); got != "precious\n" {
		t.Fatalf("file replaced before confirming: %q", got)
	}

	// Cancel is the default selection.
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if got := readFile(t, path); got != "precious\n" {
		t.Fatalf("Cancel replaced the file: %q", got)
	}
	if !h.app.EditorGroup.IsActiveVirtual() {
		t.Fatal("Cancel should leave the tab unsaved")
	}
}

func TestSaveAsExistingFileReplace(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := filepath.Join(h.dir, "precious.txt")
	if err := os.WriteFile(path, []byte("precious\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.saveAsNewFile("scratch", path)

	h.pressKey(tcell.KeyRight, tcell.ModNone)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if len(h.app.Root.Overlays) != 0 {
		t.Fatalf("expected dialog dismissed after Replace, got %d overlays", len(h.app.Root.Overlays))
	}
	if got := readFile(t, path); strings.TrimSpace(got) != "scratch" {
		t.Fatalf("expected the file replaced, got %q", got)
	}
	if h.app.EditorGroup.ActiveFilePath() != path {
		t.Fatalf("tab path = %q, want %q", h.app.EditorGroup.ActiveFilePath(), path)
	}
}

func TestSaveAsSameFileDoesNotAsk(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := filepath.Join(h.dir, "own.txt")
	if err := os.WriteFile(path, []byte("own\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.redraw()
	h.exec("file.saveAs")
	h.redraw()
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	h.redraw()

	if len(h.app.Root.Overlays) != 0 {
		t.Fatalf("expected no dialog when saving to the file's own path, got %d overlays", len(h.app.Root.Overlays))
	}
}

func TestSaveAsNewPathDoesNotAsk(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := filepath.Join(h.dir, "fresh.txt")
	h.saveAsNewFile("fresh", path)

	if len(h.app.Root.Overlays) != 0 {
		t.Fatalf("expected no dialog for a new path, got %d overlays", len(h.app.Root.Overlays))
	}
	if got := readFile(t, path); strings.TrimSpace(got) != "fresh" {
		t.Fatalf("expected the new file saved, got %q", got)
	}
}
