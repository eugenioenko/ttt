package e2e

import (
	"path/filepath"
	"testing"
)

func TestLSPRestartWithoutServerWarns(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()

	h.exec("lsp.restartServer")
	h.assertContains("No language server for the active file")

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.redraw()
	h.exec("lsp.restartServer")
	h.assertContains("No language server for the active file")
	h.assertNotContains("Restarting")
}
