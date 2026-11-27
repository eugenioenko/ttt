package e2e

import (
	"path/filepath"
	"testing"
)

func TestExplorerOpenInTerminal(t *testing.T) {
	for name, tc := range map[string]struct{ node, wantDir string }{
		"folder":               {"subdir", "subdir"},
		"file opens in parent": {filepath.Join("subdir", "nested.txt"), "subdir"},
	} {
		t.Run(name, func(t *testing.T) {
			h := newTestHarness(t, 100, 30)
			defer h.stop()
			h.app.Settings.Terminal.Shell = "/bin/cat"
			h.exec("sidebar.explorer")
			h.app.Explorer.RevealPath(filepath.Join(h.dir, tc.node))
			for _, node := range h.app.Explorer.Tree.FlatList() {
				if node.ID == filepath.Join(h.dir, tc.node) {
					h.app.ExplorerContextNode = node
				}
			}
			if h.app.ExplorerContextNode == nil {
				t.Fatalf("node %s not in explorer", tc.node)
			}

			h.exec("explorer.openInTerminal")

			if len(h.app.Terminals) != 1 {
				t.Fatalf("expected 1 terminal, got %d", len(h.app.Terminals))
			}
			defer h.app.Terminals[0].Term.Close()
			if got, want := h.app.Terminals[0].Widget.WorkDir, filepath.Join(h.dir, tc.wantDir); got != want {
				t.Errorf("terminal cwd = %q, want %q", got, want)
			}
		})
	}
}
