package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/undo"
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

func TestSaveAsRefusesDirtyOpenTarget(t *testing.T) {
	for _, when := range []string{"before", "during"} {
		t.Run(when+" confirmation", func(t *testing.T) {
			h := newTestHarness(t, 80, 24)
			defer h.stop()
			h.app.EditorGroup.OnError = h.app.StatusError
			source, target := filepath.Join(h.dir, "alpha.txt"), filepath.Join(h.dir, "beta.txt")
			if err := h.app.EditorGroup.OpenFile(target); err != nil {
				t.Fatal(err)
			}
			h.app.EditorGroup.CommitActiveTab()
			if when == "before" {
				h.pressRune('X')
			}
			targetBuf, targetUndo := h.app.EditorGroup.ActiveBuffer(), h.app.EditorGroup.Editor.Undo
			if err := h.app.EditorGroup.OpenFile(source); err != nil {
				t.Fatal(err)
			}
			h.exec("file.saveAs")
			h.pressKey(tcell.KeyCtrlA, tcell.ModCtrl)
			h.typeText(target)
			h.pressKey(tcell.KeyEnter, tcell.ModNone)
			if when == "during" {
				h.assertContains("already exists")
				edit := &undo.InsertRuneCommand{Line: 0, Col: 0, Rune: 'X'}
				edit.Apply(targetBuf)
				targetUndo.Push(edit)
				h.pressKey(tcell.KeyRight, tcell.ModNone)
				h.pressKey(tcell.KeyEnter, tcell.ModNone)
			}
			h.assertContains("already open")
			if h.app.Root.HasOverlay() {
				t.Fatal("SaveAs left a confirmation for a target with unsaved changes")
			}
			if h.app.EditorGroup.ActiveFilePath() != source || len(h.app.EditorGroup.OpenFilePaths()) != 2 {
				t.Fatal("SaveAs changed the source document's identity")
			}
			if got := readFile(t, target); got != "b" {
				t.Fatalf("SaveAs overwrote the open target: %q", got)
			}
			if targetBuf.Lines[0] != "Xb" || !targetBuf.Dirty {
				t.Fatal("SaveAs discarded the target's unsaved edits")
			}
		})
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

func TestSaveAsDanglingSymlinkAsksBeforeReplacing(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	link := filepath.Join(h.dir, "link.txt")
	if err := os.Symlink(filepath.Join(h.dir, "missing.txt"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	h.saveAsNewFile("scratch", link)

	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected a confirmation dialog, got %d overlays", len(h.app.Root.Overlays))
	}
	h.assertContains("already exists")
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("Cancel should leave the symlink in place, got %v, %v", info, err)
	}
}

func TestSaveAsHardLinkToCurrentFileAsks(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	path := filepath.Join(h.dir, "own.txt")
	if err := os.WriteFile(path, []byte("own\n"), 0644); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(h.dir, "alias.txt")
	if err := os.Link(path, alias); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.redraw()
	h.exec("file.saveAs")
	h.redraw()
	for range len(path) {
		h.pressKey(tcell.KeyBackspace, tcell.ModNone)
	}
	h.typeText(alias)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	h.redraw()

	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected a confirmation dialog for a hard link to the open file, got %d overlays", len(h.app.Root.Overlays))
	}
	h.assertContains("alias.txt already exists")
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	a, errA := os.Stat(path)
	b, errB := os.Stat(alias)
	if errA != nil || errB != nil || !os.SameFile(a, b) {
		t.Fatal("Cancel should leave the hard link intact")
	}
	if h.app.EditorGroup.ActiveFilePath() != path {
		t.Fatalf("tab path = %q, want %q", h.app.EditorGroup.ActiveFilePath(), path)
	}
}

// openOtherFolder starts the harness from one directory and then switches the
// workspace to another, the way File > Open Folder does without a chdir.
func openOtherFolder(t *testing.T) (h *testHarness, launch, folder string) {
	t.Helper()
	launch = t.TempDir()
	t.Chdir(launch)
	h = newTestHarness(t, 80, 24)
	h.app.Workspace.Folders = nil
	h.app.Workspace.AddFolder(t.TempDir())
	return h, launch, h.app.Workspace.Primary()
}

func TestSaveAsRelativeNameUsesWorkspaceFolder(t *testing.T) {
	h, launch, folder := openOtherFolder(t)
	defer h.stop()

	h.saveAsNewFile("scratch", "note.txt")

	if _, err := os.Stat(filepath.Join(launch, "note.txt")); err == nil {
		t.Fatal("file was saved in the launch directory")
	}
	want := filepath.Join(folder, "note.txt")
	if got := readFile(t, want); strings.TrimSpace(got) != "scratch" {
		t.Fatalf("expected the file saved in the workspace folder, got %q", got)
	}
	if h.app.EditorGroup.ActiveFilePath() != want {
		t.Fatalf("tab path = %q, want %q", h.app.EditorGroup.ActiveFilePath(), want)
	}
}

func TestSaveAsRelativeNameAsksBeforeReplacingInWorkspaceFolder(t *testing.T) {
	h, _, folder := openOtherFolder(t)
	defer h.stop()

	path := filepath.Join(folder, "precious.txt")
	if err := os.WriteFile(path, []byte("precious\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.saveAsNewFile("scratch", "precious.txt")

	if len(h.app.Root.Overlays) != 1 {
		t.Fatalf("expected a confirmation dialog, got %d overlays", len(h.app.Root.Overlays))
	}
	h.assertContains("already exists")
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if got := readFile(t, path); got != "precious\n" {
		t.Fatalf("Cancel replaced the file: %q", got)
	}
}
