package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func treeNodeWithLabel(nodes []*widgets.TreeNode, label string) *widgets.TreeNode {
	for _, node := range nodes {
		if node.Label == label {
			return node
		}
		if found := treeNodeWithLabel(node.Children, label); found != nil {
			return found
		}
	}
	return nil
}

func menuCommandWithLabel(items []ui.ContextMenuItem, label string) string {
	for _, item := range items {
		if item.Label == label {
			return item.Command
		}
		if command := menuCommandWithLabel(item.Submenu, label); command != "" {
			return command
		}
	}
	return ""
}

func TestChangesAndExplorerThreeDotMenusExposeBulkActions(t *testing.T) {
	h := newTestHarness(t, 100, 28)
	defer h.stop()

	h.exec("sidebar.changes")
	h.app.ShowSidebarMoreMenu(4, 3)
	h.redraw()
	h.assertContains("Git Files")
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	h.pressKey(tcell.KeyRight, tcell.ModNone)
	h.assertContains("Expand All")
	h.assertContains("Collapse All")
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	h.exec("sidebar.explorer")
	h.app.ShowSidebarMoreMenu(4, 3)
	h.redraw()
	h.assertContains("Expand All")
	h.assertContains("Collapse All")
}

func TestChangesRightClickAndThreeDotMenusHaveGitFileParity(t *testing.T) {
	h := newTestHarness(t, 100, 28)
	defer h.stop()
	panel := h.app.BuildChangesPanelMenu()
	context := h.app.BuildChangesContextMenu()
	for _, command := range []string{"options.useGitFileTree", "options.useGitFileList", "changes.expandAllWorkingTree", "changes.collapseAllWorkingTree"} {
		panelItem, panelOK := findMenuCommand(panel, command)
		contextItem, contextOK := findMenuCommand(context, command)
		if !panelOK || !contextOK || panelItem.Checked != contextItem.Checked {
			t.Errorf("menu parity for %s: panel=%+v/%v context=%+v/%v", command, panelItem, panelOK, contextItem, contextOK)
		}
	}

	h.exec("sidebar.changes")
	h.app.ShowChangesContextMenu(5, 5)
	h.redraw()
	if _, ok := h.app.Root.Focused.(*ui.ContextMenuWidget); !ok {
		t.Fatalf("right-click menu focus = %T", h.app.Root.Focused)
	}
	h.assertContains("Git Files")
}

func TestChangesMenusExpandChangesWithoutExpandingActiveCommitDetail(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	runHistoryGit(t, h.dir, "init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(h.dir, "deep", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.dir, "deep", "nested", "file.go"), []byte("package nested\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	previousScreen := h.app.Changes.Screen
	h.app.Changes.Screen = nil
	h.app.Changes.Refresh()
	h.app.Changes.Screen = previousScreen
	h.app.Changes.SetFileView("tree")
	h.exec("sidebar.changes")
	detail := ui.NewCommitDetailWidget(h.dir, "ref", "ref", false)
	detail.SetDetail("detail", []ui.CommitDetailFile{{Path: "detail.txt", Error: "DETAIL SENTINEL"}}, "")
	detail.CollapseAllFiles()
	h.app.EditorGroup.OpenPluginTab("routing-detail", "Commit detail", detail)

	menus := [][]ui.ContextMenuItem{h.app.BuildChangesPanelMenu(), h.app.BuildChangesContextMenu()}
	for index, menu := range menus {
		h.app.Changes.CollapseAll()
		detail.CollapseAllFiles()
		command := menuCommandWithLabel(menu, "Expand All")
		if command == "" {
			t.Fatalf("menu %d has no Expand All command", index)
		}
		h.exec(command)
		changes := h.app.Changes.Tree.Config.Items[0]
		folder := treeNodeWithLabel(changes.Children, "deep/nested")
		if !changes.Expanded || folder == nil || !folder.Expanded {
			t.Fatalf("menu %d did not expand Changes: changes=%v folder=%+v", index, changes.Expanded, folder)
		}
		h.assertNotContains("DETAIL SENTINEL")
	}
}
