package e2e

import (
	"path/filepath"
	"testing"
)

func TestTabMenuRevealInExplorerShowsSidebarAndSelectsFile(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("sidebar.search")
	h.app.HideSidebar()
	nested := filepath.Join(h.dir, "subdir", "nested.txt")
	h.app.EditorGroup.OpenFile(nested)
	h.redraw()

	tabRect := h.app.EditorGroup.TabBar.GetRect()
	h.app.EditorGroup.TabBar.OnTabRightClick(h.app.EditorGroup.ActiveTabIndex(), tabRect.X+1, tabRect.Y)
	menu := focusedContextMenu(t, h)

	copyIndex := -1
	for index, item := range menu.Items {
		if item.Command == "file.copyRelativePath" {
			copyIndex = index
			break
		}
	}
	if copyIndex < 0 || copyIndex+1 >= len(menu.Items) {
		t.Fatalf("tab menu has no item after Copy Relative Path: %+v", menu.Items)
	}
	reveal := menu.Items[copyIndex+1]
	if reveal.Label != "Reveal in Explorer" || reveal.Command != "explorer.revealActiveFile" {
		t.Fatalf("item after Copy Relative Path = %+v, want Reveal in Explorer", reveal)
	}

	menu.OnExec(reveal.Command)
	h.redraw()

	if !h.app.Sidebar.Visible {
		t.Error("sidebar stayed hidden")
	}
	if h.app.Sidebar.ActivePanel != "explorer" {
		t.Errorf("active panel = %q, want explorer", h.app.Sidebar.ActivePanel)
	}
	if selected := h.app.Explorer.Tree.Selected(); selected == nil || selected.ID != nested {
		t.Fatalf("selected %+v, want %s", selected, nested)
	}
}
