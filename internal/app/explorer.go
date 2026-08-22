package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

type NavigationPanel struct {
	Tree     *widgets.TreeWidget
	Adapter  *ui.WidgetAdapter
	Settings config.ExplorerSettings
	Roots    []string

	OnOpenFile   func(path string)
	OnRightClick func(node *widgets.TreeNode, sx, sy int)
	OnRootMenu   func(node *widgets.TreeNode, sx, sy int)
}

func NewNavigationPanel(settings config.ExplorerSettings, paths ...string) *NavigationPanel {
	n := &NavigationPanel{
		Settings: settings,
		Roots:    paths,
	}

	items := make([]*widgets.TreeNode, len(paths))
	multiRoot := len(paths) > 1
	for i, p := range paths {
		root := &widgets.TreeNode{
			ID:         p,
			Label:      filepath.Base(p),
			Expanded:   !multiRoot,
			Expandable: true,
		}
		items[i] = root
	}

	tree := widgets.NewTreeWidget(widgets.TreeConfig{
		Items: items,
		OnExpand: func(node *widgets.TreeNode) {
			n.loadChildren(node)
		},
		OnCommand: func(cmd string, node *widgets.TreeNode) {
			if cmd == "activate" && n.OnOpenFile != nil {
				n.OnOpenFile(node.ID)
			}
		},
		OnKey: func(ev *tcell.EventKey, _ *widgets.TreeNode) bool {
			switch term.KeyRune(ev) {
			case 'r', 'R':
				n.Reload()
				return true
			}
			return false
		},
		OnMenu: func(_ []widgets.MenuEntry, node *widgets.TreeNode, sx, sy int) {
			if n.isRoot(node) {
				if n.OnRootMenu != nil {
					n.OnRootMenu(node, sx, sy)
				}
			} else if n.OnRightClick != nil {
				n.OnRightClick(node, sx, sy)
			}
		},
	})
	n.Tree = tree

	for _, root := range items {
		if root.Expanded {
			n.loadChildren(root)
		}
	}
	tree.SetItems(items)

	n.Adapter = ui.NewWidgetAdapter(tree)
	return n
}

func (n *NavigationPanel) isRoot(node *widgets.TreeNode) bool {
	return slices.Contains(n.Roots, node.ID)
}

func (n *NavigationPanel) SetActiveFile(path string) {
	n.Tree.SetActiveID(path)
}

func (n *NavigationPanel) Reload() {
	n.Tree.Reload()
}

func (n *NavigationPanel) ExpandAll() {
	selectedID := ""
	if selected := n.Tree.Selected(); selected != nil {
		selectedID = selected.ID
	}
	var expand func(*widgets.TreeNode, map[string]bool)
	expand = func(node *widgets.TreeNode, ancestors map[string]bool) {
		if node == nil || !node.Expandable {
			return
		}
		info, err := os.Lstat(node.ID)
		if err == nil && (info.Mode()&os.ModeSymlink != 0 || info.IsDir() && filepath.Base(node.ID) == ".git") {
			return
		}
		node.Expanded = true
		canonical, err := filepath.EvalSymlinks(node.ID)
		if err != nil {
			canonical = filepath.Clean(node.ID)
		}
		if ancestors[canonical] {
			return
		}
		if len(node.Children) == 0 {
			n.loadChildren(node)
		}
		ancestors[canonical] = true
		for _, child := range node.Children {
			expand(child, ancestors)
		}
		delete(ancestors, canonical)
	}
	for _, root := range n.Tree.Config.Items {
		expand(root, make(map[string]bool))
	}
	n.Tree.SetItems(n.Tree.Config.Items)
	if selectedID != "" {
		n.Tree.SelectByID(selectedID)
	}
}

func (n *NavigationPanel) CollapseAll() {
	n.Tree.CollapseAll()
}

func (n *NavigationPanel) SetRoots(paths []string) {
	expanded := map[string]bool{}
	n.Tree.CollectExpanded(expanded)

	n.Roots = paths
	multiRoot := len(paths) > 1
	items := make([]*widgets.TreeNode, len(paths))
	for i, p := range paths {
		wasExpanded := expanded[p]
		root := &widgets.TreeNode{
			ID:         p,
			Label:      filepath.Base(p),
			Expanded:   wasExpanded || !multiRoot,
			Expandable: true,
		}
		if root.Expanded {
			n.loadChildren(root)
		}
		items[i] = root
	}
	n.Tree.SetItems(items)
	n.Tree.RestoreExpanded(expanded)
}

func (n *NavigationPanel) loadChildren(node *widgets.TreeNode) {
	entries := ui.LoadDirEntries(node.ID, n.Settings)
	node.Children = nil
	for _, de := range entries {
		child := &widgets.TreeNode{
			ID:         de.Path,
			Label:      de.Name,
			Expandable: de.IsDir,
			Muted:      de.GitIgnored || strings.HasPrefix(de.Name, "."),
		}
		node.Children = append(node.Children, child)
	}
}
