package app

import (
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func TestExplorerCompactUnicodeSegmentHitTesting(t *testing.T) {
	root := t.TempDir()
	writeRevealFixture(t, root, "e\u0301te\u0301/日本/file.txt")
	explorer := NewNavigationPanel(compactSettings(true), config.IconsNone, root)
	explorer.ExpandAll()
	node := explorer.Tree.Config.Items[0].Children[0]
	cells := make([][]term.Cell, 4)
	for y := range cells {
		cells[y] = make([]term.Cell, 40)
	}
	explorer.Tree.SetRect(widgets.Rect{W: 40, H: 4})
	explorer.Tree.Render(ui.NewRenderSurface(cells, ui.Rect{W: 40, H: 4}))
	for _, tc := range []struct {
		x    int
		path string
	}{
		{0, filepath.Join(root, "e\u0301te\u0301")},
		{3, filepath.Join(root, "e\u0301te\u0301")},
		{4, filepath.Join(root, "e\u0301te\u0301", "日本")},
		{7, filepath.Join(root, "e\u0301te\u0301", "日本")},
	} {
		if got := explorer.compactSegmentAt(node, node.LabelX()+tc.x); got.ID != tc.path {
			t.Errorf("column %d resolved to %q, want %q", tc.x, got.ID, tc.path)
		}
	}
}
