package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func openReadOnlyFile(t *testing.T, h *testHarness) string {
	t.Helper()
	path := filepath.Join(h.dir, "locked.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0444); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.redraw()
	return path
}

func TestReadOnlyFileShowsIndicator(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	openReadOnlyFile(t, h)
	h.assertContains("locked.txt (readonly)")

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.redraw()
	h.assertNotContains("alpha.txt (readonly)")
}

func TestReadOnlyFileSavePromptsAndCancelKeepsFile(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := openReadOnlyFile(t, h)
	h.pressRune('x')
	h.exec("file.save")
	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected a confirmation dialog when saving a read-only file, got %d overlays", len(h.app.Root.Overlays))
	}
	h.assertContains("is read-only")

	h.pressRune('c')
	if len(h.app.Root.Overlays) != 0 {
		t.Fatalf("expected dialog dismissed after Cancel, got %d overlays", len(h.app.Root.Overlays))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original\n" {
		t.Errorf("expected the read-only file to be untouched after Cancel, got %q", string(data))
	}
}

func TestReadOnlyFileSaveOverwriteKeepsPermissions(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := openReadOnlyFile(t, h)
	h.pressRune('x')
	h.exec("file.save")
	h.pressRune('o')
	if len(h.app.Root.Overlays) != 0 {
		t.Fatalf("expected dialog dismissed after Overwrite, got %d overlays", len(h.app.Root.Overlays))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "xoriginal\n" {
		t.Errorf("expected the explicit overwrite to save the buffer, got %q", string(data))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0222 != 0 {
		t.Errorf("expected the file to stay read-only, got mode %o", info.Mode().Perm())
	}
}
