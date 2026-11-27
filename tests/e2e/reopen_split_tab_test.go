package e2e

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestReopenClosedTabAfterItsSplitCloses(t *testing.T) {
	h := newTestHarness(t, 140, 32)
	defer h.stop()

	left := h.app.EditorGroup
	alpha, beta, gamma := filepath.Join(h.dir, "alpha.txt"), filepath.Join(h.dir, "beta.txt"), filepath.Join(h.dir, "gamma.txt")
	h.openFile(alpha)
	h.openFile(gamma)
	h.exec("tab.close")

	h.exec("editor.split")
	h.openFile(beta)
	h.pressKey(tcell.KeyRight, tcell.ModNone)
	h.exec("tab.close")

	if groups := h.app.EditorSplit.Groups(); len(groups) != 1 || groups[0] != left {
		t.Fatal("closing the last tab did not remove its split")
	}
	h.exec("tab.reopenClosed")
	if got := left.ActiveFilePath(); got != beta {
		t.Errorf("first reopened tab = %q, want %q", got, beta)
	}
	if line, col := left.ActiveCursor(); line != 0 || col != 1 {
		t.Errorf("cursor in reopened tab = %d:%d, want 0:1", line, col)
	}
	h.exec("tab.reopenClosed")
	if got := left.ActiveFilePath(); got != gamma {
		t.Errorf("second reopened tab = %q, want %q", got, gamma)
	}
}
