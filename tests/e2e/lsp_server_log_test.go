package e2e

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestLSPNotifyWithoutServerSkipsBufferCopy(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()

	h.app.LspManager = lsp.NewManager(&config.LSPSettings{Servers: map[string]config.LSPServerConfig{
		"fake": {Command: []string{"ttt-no-such-binary"}, Languages: map[string]string{".go": "go"}},
	}})
	path := filepath.Join(h.dir, "big.log")
	line := strings.Repeat("x", 255) + "\n"
	if err := os.WriteFile(path, []byte(strings.Repeat(line, 32*1024)), 0o644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(path)
	h.redraw()

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	h.app.EnsureLSPOpen(path)
	<-h.app.NotifyLSPChange(path, "", h.app.EditorGroup.Editor.Buf.Lines)
	runtime.ReadMemStats(&after)

	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 1024*1024 {
		t.Errorf("notifying LSP of an 8 MiB file with no server allocated %d bytes", allocated)
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
