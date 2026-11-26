package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/jjtest"
	"github.com/gdamore/tcell/v3"
)

func TestJJChangesUseWorkingCopyWording(t *testing.T) {
	h := newTestHarness(t, 140, 32)
	defer h.stop()
	initializeHarnessJJRepository(t, h.dir)
	jjtest.Write(t, h.dir, "file.txt", "changed\n")
	h.app.Repository.Start()
	h.exec("sidebar.changes")
	h.app.Changes.Tree.SetSelectedIndex(1)
	h.exec("changes.discard")
	h.assertContains("jj undo")
	h.assertNotContains("irreversible")
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	h.exec("changes.viewAll")
	h.assertContains("file.txt · working copy")
	h.assertNotContains("unstaged")
}
