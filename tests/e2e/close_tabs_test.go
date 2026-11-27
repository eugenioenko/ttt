package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCloseAllTabsNoDirty(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "clean.txt")
	requireNoError(t, os.WriteFile(f, []byte("hello\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.exec("tab.closeAll")
	h.redraw()

	name := h.app.EditorGroup.ActiveFileName()
	if name != "untitled" {
		t.Errorf("expected untitled after close all, got %q", name)
	}
}

func TestCloseAllTabsDirtyShowsDialog(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	f := filepath.Join(h.dir, "dirty.txt")
	requireNoError(t, os.WriteFile(f, []byte("hello\n"), 0644))
	h.openFile(f)
	h.redraw()

	h.pressRune('X')
	h.redraw()

	if !h.app.EditorGroup.IsDirty() {
		t.Fatal("expected dirty buffer")
	}

	h.exec("tab.closeAll")
	h.redraw()

	if !h.app.Root.HasOverlay() {
		t.Fatal("expected confirm dialog for dirty tabs")
	}
}

func TestCloseOtherTabsDirtyShowsDialog(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	a := filepath.Join(h.dir, "a.txt")
	b := filepath.Join(h.dir, "b.txt")
	requireNoError(t, os.WriteFile(a, []byte("aaa\n"), 0644))
	requireNoError(t, os.WriteFile(b, []byte("bbb\n"), 0644))

	h.openFile(a)
	h.openFile(b)
	h.redraw()

	// b is active (last opened). Dirty it.
	h.pressRune('X')
	h.redraw()

	// Switch to a so b becomes "other"
	h.openFile(a)
	h.redraw()

	active := h.app.EditorGroup.ActiveFileName()
	if active != "a.txt" {
		t.Fatalf("expected a.txt active, got %q", active)
	}

	h.exec("tab.closeOthers")
	h.redraw()

	if !h.app.Root.HasOverlay() {
		t.Fatal("expected confirm dialog for dirty other tabs")
	}
}

func TestCloseAllSavedKeepsDirty(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	clean := filepath.Join(h.dir, "clean.txt")
	dirty := filepath.Join(h.dir, "dirty.txt")
	requireNoError(t, os.WriteFile(clean, []byte("clean\n"), 0644))
	requireNoError(t, os.WriteFile(dirty, []byte("dirty\n"), 0644))

	h.openFile(clean)
	h.openFile(dirty)
	h.redraw()

	h.pressRune('X')
	h.redraw()

	h.exec("tab.closeAllSaved")
	h.redraw()

	name := h.app.EditorGroup.ActiveFileName()
	if name != "dirty.txt" {
		t.Errorf("expected dirty tab to remain, got %q", name)
	}
	if !h.app.EditorGroup.IsDirty() {
		t.Error("expected remaining tab to be dirty")
	}
}
