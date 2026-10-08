package e2e

import (
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/lsp"
)

func TestLSPShowServerLogWithoutServer(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()

	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.redraw()
	h.exec("lsp.showServerLog")
	h.assertContains("No language server for this file")
	if h.app.ContentSplit.ShowBottom {
		t.Error("Output panel opened for a file without a language server")
	}
}

func TestLSPShowServerLogNotInstalled(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()

	h.app.LspManager = lsp.NewManager(&config.LSPSettings{Servers: map[string]config.LSPServerConfig{
		"fake": {Command: []string{"ttt-no-such-binary"}, Languages: map[string]string{".txt": "plaintext"}},
	}})
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.redraw()
	h.exec("lsp.showServerLog")
	h.assertContains("fake: not installed")
	if !h.app.ContentSplit.ShowBottom {
		t.Error("Output panel not shown")
	}
	if got := h.app.BottomPanel.ActivePanel; got != "output" {
		t.Errorf("active bottom panel = %q, want output", got)
	}
}
