package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
)

// modifyOnDisk rewrites a file's contents and pushes its mtime forward so the
// editor's recorded disk state is guaranteed to look stale, regardless of
// filesystem timestamp resolution.
func modifyOnDisk(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
}

func TestSaveConflictPromptsBeforeOverwrite(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := filepath.Join(h.dir, "conflict.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.openFile(path)
	h.redraw()

	// Saving with no external change must not prompt.
	h.exec("file.save")
	if len(h.app.Root.Overlays) != 0 {
		t.Fatalf("expected no dialog when file is unchanged on disk, got %d overlays", len(h.app.Root.Overlays))
	}

	// Now the file changes underneath us; saving must prompt first.
	modifyOnDisk(t, path, "changed by another tool\n")
	h.exec("file.save")
	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected a confirmation dialog after external change, got %d overlays", len(h.app.Root.Overlays))
	}
	h.assertContains("modified on disk")
}

func TestSaveConflictOverwrite(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := filepath.Join(h.dir, "conflict.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.openFile(path)
	h.redraw()

	modifyOnDisk(t, path, "changed by another tool\n")
	h.exec("file.save")
	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected confirmation dialog, got %d overlays", len(h.app.Root.Overlays))
	}

	// Overwrite is the default selection; Enter activates it.
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if len(h.app.Root.Overlays) != 0 {
		t.Fatalf("expected dialog dismissed after Overwrite, got %d overlays", len(h.app.Root.Overlays))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original\n" {
		t.Errorf("expected our buffer to overwrite the file, got %q", string(data))
	}
}

func TestSaveConflictCancelKeepsDiskVersion(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := filepath.Join(h.dir, "conflict.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.openFile(path)
	h.redraw()

	modifyOnDisk(t, path, "changed by another tool\n")
	h.exec("file.save")
	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected confirmation dialog, got %d overlays", len(h.app.Root.Overlays))
	}

	// Cancel via its first-letter shortcut; the disk version must survive.
	h.pressRune('c')
	if len(h.app.Root.Overlays) != 0 {
		t.Fatalf("expected dialog dismissed after Cancel, got %d overlays", len(h.app.Root.Overlays))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "changed by another tool\n" {
		t.Errorf("expected the disk version to be kept after Cancel, got %q", string(data))
	}
}
