package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

type NavigationPanel struct {
	Tree     *widgets.TreeWidget
	Adapter  *ui.WidgetAdapter
	Settings config.ExplorerSettings
	Icons    string
	Roots    []string

	gitStyles     map[string]term.Style
	compactStarts map[string]string
	restoring     map[string]bool

	OnOpenFile   func(path string)
	OnAction     func(commandID string)
	OnRightClick func(node *widgets.TreeNode, sx, sy int)
	OnRootMenu   func(node *widgets.TreeNode, sx, sy int)
}

func NewNavigationPanel(settings config.ExplorerSettings, icons string, paths ...string) *NavigationPanel {
	n := &NavigationPanel{
		Settings:      settings,
		Icons:         icons,
		Roots:         paths,
		compactStarts: make(map[string]string),
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
			if cmd != "activate" {
				return
			}
			if id, ok := strings.CutPrefix(node.ID, explorerActionPrefix); ok {
				if n.OnAction != nil {
					n.OnAction(id)
				}
				return
			}
			if n.OnOpenFile != nil && node.ID != "" {
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
			if len(n.Roots) == 0 {
				return
			}
			if n.isRoot(node) {
				if n.OnRootMenu != nil {
					n.OnRootMenu(node, sx, sy)
				}
			} else if n.OnRightClick != nil {
				n.OnRightClick(n.compactSegmentAt(node, sx), sx, sy)
			}
		},
	})
	n.Tree = tree

	for _, root := range items {
		if root.Expanded {
			n.loadChildren(root)
		}
	}
	if len(paths) == 0 {
		items = emptyExplorerNodes()
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

func (n *NavigationPanel) RevealPath(path string) bool {
	root := n.rootContaining(path)
	if root == nil {
		return false
	}
	rel, err := filepath.Rel(root.ID, path)
	if err != nil || rel == "." {
		return false
	}
	folders := []*widgets.TreeNode{root}
	node := root
	for node.ID != path {
		if !node.Expanded || len(node.Children) == 0 {
			n.loadChildren(node)
		}
		node = childContaining(node.Children, path)
		if node == nil {
			return false
		}
		folders = append(folders, node)
	}
	for _, folder := range folders[:len(folders)-1] {
		folder.Expanded = true
	}
	n.Tree.SetItems(n.Tree.Config.Items)
	return n.Tree.RevealID(path)
}

func (n *NavigationPanel) rootContaining(path string) *widgets.TreeNode {
	var best *widgets.TreeNode
	for _, root := range n.Tree.Config.Items {
		if !n.isRoot(root) || !strings.HasPrefix(path, root.ID+string(filepath.Separator)) {
			continue
		}
		if best == nil || len(root.ID) > len(best.ID) {
			best = root
		}
	}
	return best
}

func childContaining(nodes []*widgets.TreeNode, path string) *widgets.TreeNode {
	for _, node := range nodes {
		if node.ID == path || strings.HasPrefix(path, node.ID+string(filepath.Separator)) {
			return node
		}
	}
	return nil
}

func (n *NavigationPanel) Reload() {
	selectedID := ""
	if selected := n.Tree.Selected(); selected != nil {
		selectedID = selected.ID
	}
	expanded := n.collectExpanded()
	n.restoring = expanded
	defer func() { n.restoring = nil }()
	for _, root := range n.Tree.Config.Items {
		if root.Expanded {
			n.loadChildren(root)
		}
	}
	n.Tree.RestoreExpanded(expanded)
	n.selectRow(selectedID)
	n.Tree.SetSelectedIndex(n.Tree.SelectedIndex())
}

func (n *NavigationPanel) selectRow(id string) {
	if id == "" {
		return
	}
	n.Tree.SelectByID(id)
	if selected := n.Tree.Selected(); selected != nil && selected.ID == id {
		return
	}
	for rowID := range n.compactStarts {
		if slices.Contains(n.compactChain(rowID), id) {
			n.Tree.SelectByID(rowID)
			return
		}
	}
}

func (n *NavigationPanel) collectExpanded() map[string]bool {
	expanded := map[string]bool{}
	n.Tree.CollectExpanded(expanded)
	var chains []string
	for id := range expanded {
		if _, ok := n.compactStarts[id]; ok {
			chains = append(chains, id)
		}
	}
	for _, id := range chains {
		for _, dir := range n.compactChain(id) {
			expanded[dir] = true
		}
	}
	return expanded
}

func (n *NavigationPanel) compactChain(id string) []string {
	start, ok := n.compactStarts[id]
	if !ok {
		return []string{id}
	}
	chain := []string{id}
	for dir := id; dir != start && strings.HasPrefix(dir, start+string(filepath.Separator)); {
		dir = filepath.Dir(dir)
		chain = append(chain, dir)
	}
	return chain
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
	expanded := n.collectExpanded()
	n.restoring = expanded
	defer func() { n.restoring = nil }()

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
	if len(paths) == 0 {
		items = emptyExplorerNodes()
	}
	n.Tree.SetItems(items)
	n.Tree.RestoreExpanded(expanded)
}

const explorerActionPrefix = "command:"

func emptyExplorerNodes() []*widgets.TreeNode {
	return []*widgets.TreeNode{
		{Label: "No folder open", Muted: true},
		{ID: explorerActionPrefix + "workspace.openFolder", Label: "Open Folder…"},
		{ID: explorerActionPrefix + "workspace.clone", Label: "Clone Repository…"},
	}
}

// WatchedDirs returns every root plus every expanded folder: the directories
// whose contents are currently materialized in the tree.
func (n *NavigationPanel) WatchedDirs() []string {
	if n.Tree == nil {
		return nil
	}
	var dirs []string
	seen := make(map[string]bool)
	add := func(dir string) {
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	var walk func(node *widgets.TreeNode, parent string)
	walk = func(node *widgets.TreeNode, parent string) {
		if node == nil {
			return
		}
		isRoot := parent == ""
		if !isRoot && !node.Expanded {
			return
		}
		add(node.ID)
		if !isRoot {
			for dir := filepath.Dir(node.ID); dir != parent && strings.HasPrefix(dir, parent+string(filepath.Separator)); dir = filepath.Dir(dir) {
				add(dir)
			}
		}
		for _, child := range node.Children {
			if child.Expandable {
				walk(child, node.ID)
			}
		}
	}
	for _, root := range n.Tree.Config.Items {
		walk(root, "")
	}
	return dirs
}

func (n *NavigationPanel) loadChildren(node *widgets.TreeNode) {
	n.loadEntries(node)
	if !n.Settings.CompactFolders {
		return
	}
	if !n.isRoot(node) {
		n.compact(node)
	}
	var starts map[string]bool
	for _, child := range node.Children {
		if !child.Expandable || len(n.compactStarts) == 0 {
			continue
		}
		if starts == nil {
			starts = make(map[string]bool, len(n.compactStarts))
			for _, start := range n.compactStarts {
				starts[start] = true
			}
		}
		if starts[child.ID] {
			n.loadEntries(child)
			n.compact(child)
		}
	}
}

func (n *NavigationPanel) compact(node *widgets.TreeNode) {
	if isSymlink(node.ID) {
		return
	}
	start, ok := n.compactStarts[node.ID]
	if !ok {
		start = node.ID
	}
	for id, s := range n.compactStarts {
		if s == start {
			delete(n.compactStarts, id)
		}
	}
	for len(node.Children) == 1 && node.Children[0].Expandable && !isSymlink(node.Children[0].ID) {
		child := node.Children[0]
		node.ID = child.ID
		node.Label += "/" + child.Label
		node.LabelStyle = child.LabelStyle
		n.loadEntries(node)
	}
	if node.ID != start {
		n.compactStarts[node.ID] = start
	}
	if n.restoring != nil {
		for _, dir := range n.compactChain(node.ID) {
			if n.restoring[dir] {
				n.restoring[node.ID] = true
				break
			}
		}
	}
}

func (n *NavigationPanel) compactSegmentAt(node *widgets.TreeNode, x int) *widgets.TreeNode {
	if _, compacted := n.compactStarts[node.ID]; !compacted || node.LabelX() < 0 || x < node.LabelX() {
		return node
	}
	segments := strings.Split(node.Label, "/")
	end := node.LabelX()
	for i, segment := range segments[:len(segments)-1] {
		end += textwidth.String(segment) + 1
		if x < end {
			path := node.ID
			for range len(segments) - 1 - i {
				path = filepath.Dir(path)
			}
			return &widgets.TreeNode{ID: path, Label: segment, Expandable: true}
		}
	}
	return node
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

func (n *NavigationPanel) loadEntries(node *widgets.TreeNode) {
	entries := ui.LoadDirEntries(node.ID, n.Settings)
	node.Children = nil
	for _, de := range entries {
		child := &widgets.TreeNode{
			ID:         de.Path,
			Label:      de.Name,
			Expandable: de.IsDir,
			Muted:      de.GitIgnored || strings.HasPrefix(de.Name, "."),
		}
		child.LabelStyle = n.gitStyleFor(child.ID)
		if n.Icons == config.IconsNerdFont && !de.IsDir {
			setFileIcon(child)
		}
		node.Children = append(node.Children, child)
	}
}

// ApplyGitStatus repaints already-loaded nodes in place instead of reloading
// from disk, since it runs on every status poll (every couple of seconds).
func (n *NavigationPanel) ApplyGitStatus(styles map[string]term.Style) {
	n.gitStyles = styles
	for _, root := range n.Tree.Config.Items {
		n.applyGitStyles(root)
	}
	n.Tree.SetItems(n.Tree.Config.Items)
}

func (n *NavigationPanel) applyGitStyles(node *widgets.TreeNode) {
	node.LabelStyle = n.gitStyleFor(node.ID)
	for _, child := range node.Children {
		n.applyGitStyles(child)
	}
}

func (n *NavigationPanel) gitStyleFor(path string) term.Style {
	if !n.Settings.GitStatusColors {
		return term.StyleDefault
	}
	if !n.Settings.DimStagedGitColors {
		return ui.GitDecorationLive(n.gitStyles[path])
	}
	return n.gitStyles[path]
}
