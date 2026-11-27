package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestSaveAllKeepsEditorOpen(t *testing.T) {
	h := newTestHarness(t, 100, 28)
	defer h.stop()
	alpha := filepath.Join(h.dir, "alpha.txt")
	beta := filepath.Join(h.dir, "beta.txt")
	h.openFile(alpha)
	h.typeText("alpha edit")
	h.exec("editor.split")
	h.openFile(beta)
	h.typeText("beta edit")
	h.exec("file.new")
	h.typeText("scratch")

	h.exec("file.saveAll")
	h.assertContains("Save As")
	scratch := filepath.Join(h.dir, "scratch.txt")
	h.typeText(scratch)
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	for path, want := range map[string]string{alpha: "alpha edit", beta: "beta edit", scratch: "scratch"} {
		if got := readFile(t, path); !strings.Contains(got, want) {
			t.Errorf("%s = %q, want it to contain %q", filepath.Base(path), got, want)
		}
	}
	if !*h.app.Running || h.app.Root.HasOverlay() {
		t.Fatal("Save All must leave the editor open with no dialog")
	}
	for _, group := range h.app.EditorSplit.Groups() {
		if group.AnyDirty() {
			t.Fatal("Save All left a modified buffer")
		}
	}
}

func TestSaveAllCancelKeepsEarlierSaves(t *testing.T) {
	h := newTestHarness(t, 100, 28)
	defer h.stop()
	alpha := filepath.Join(h.dir, "alpha.txt")
	h.openFile(alpha)
	h.typeText("alpha edit")
	h.exec("file.new")
	h.typeText("scratch")

	h.exec("file.saveAll")
	h.assertContains("Save As")
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	if got := readFile(t, alpha); !strings.Contains(got, "alpha edit") {
		t.Fatalf("alpha.txt = %q, want the earlier save to be kept", got)
	}
	if !*h.app.Running || h.app.Root.HasOverlay() || !h.app.EditorGroup.IsDirty() {
		t.Fatal("cancelling must stop the sequence and keep the unnamed buffer modified")
	}
}

func TestSaveAllFromFileMenu(t *testing.T) {
	h := newTestHarness(t, 100, 28)
	defer h.stop()
	alpha := filepath.Join(h.dir, "alpha.txt")
	h.openFile(alpha)
	h.typeText("alpha edit")

	h.click(h.app.MenuBar.ItemAnchorX(0), h.app.MenuBar.GetRect().Y)
	menu := focusedContextMenu(t, h)
	index := -1
	for i, item := range menu.Items {
		if item.Command == "file.saveAll" {
			index = i
			break
		}
	}
	if index < 1 || menu.Items[index-1].Command != "file.saveAs" {
		t.Fatalf("Save All must sit directly below Save As, got index %d", index)
	}
	r := menu.GetRect()
	h.click(r.X+2, r.Y+1+index)

	if got := readFile(t, alpha); !strings.Contains(got, "alpha edit") {
		t.Fatalf("alpha.txt = %q, want it saved from the menu", got)
	}
}

func TestSaveAllAcrossSplitsWithSearchBarOpen(t *testing.T) {
	for _, bar := range []string{"search.find", "search.replace"} {
		for _, action := range []string{"save all", "save all and quit"} {
			t.Run(bar+"/"+action, func(t *testing.T) {
				h := newTestHarness(t, 100, 28)
				defer h.stop()
				h.app.Settings.Editor.SaveAllOnQuit = true
				alpha := filepath.Join(h.dir, "alpha.txt")
				beta := filepath.Join(h.dir, "beta.txt")
				h.openFile(alpha)
				h.typeText("alpha edit")
				h.exec("editor.split")
				h.openFile(beta)
				h.typeText("beta edit")
				h.exec(bar)

				if action == "save all" {
					h.exec("file.saveAll")
				} else {
					h.exec("editor.quit")
					h.pressRune('s')
				}

				for path, want := range map[string]string{alpha: "alpha edit", beta: "beta edit"} {
					if got := readFile(t, path); !strings.Contains(got, want) {
						t.Errorf("%s = %q, want it to contain %q", filepath.Base(path), got, want)
					}
				}
				if h.app.Root.HasOverlay() {
					t.Error("search bar should close when saving moves to another split")
				}
				if want := action == "save all"; *h.app.Running != want {
					t.Errorf("running = %v, want %v", *h.app.Running, want)
				}
			})
		}
	}
}
