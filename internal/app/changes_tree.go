package app

import (
	"slices"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/widgets"
)

const commitFolderPrefix = "folder:history:"

type workNodeKind string

const (
	workNodeRoot     workNodeKind = "root"
	workNodeSection  workNodeKind = "section"
	workNodeFolder   workNodeKind = "folder"
	workNodeFile     workNodeKind = "file"
	workNodePRRoot   workNodeKind = "pr-root"
	workNodePRFolder workNodeKind = "pr-folder"
	workNodePRFile   workNodeKind = "pr-file"
)

func workingNodeID(kind workNodeKind, dir, path string, staged bool) string {
	stage := "unstaged"
	if staged {
		stage = "staged"
	}
	return strings.Join([]string{"working", string(kind), stage, dir, path}, "\x00")
}

type filePathDir struct {
	name  string
	path  string
	dirs  map[string]*filePathDir
	files []git.FileStatus
}

func newFilePathDir(name, path string) *filePathDir {
	return &filePathDir{name: name, path: path, dirs: make(map[string]*filePathDir)}
}

// compactFileTree turns Git's slash-separated paths into directory nodes.
// Consecutive directories with no files or siblings render as one row, which
// preserves the hierarchy without spending most of a short sidebar on folders.
func compactFileTree(scope string, files []git.FileStatus, makeLeaf func(git.FileStatus) *widgets.TreeNode, expanded map[string]bool) []*widgets.TreeNode {
	return compactFileTreeWithFolderID(files, makeLeaf, func(path string) string {
		return "folder:" + scope + ":" + path
	}, expanded)
}

func compactFileTreeWithFolderID(files []git.FileStatus, makeLeaf func(git.FileStatus) *widgets.TreeNode, folderID func(string) string, expanded map[string]bool) []*widgets.TreeNode {
	root := newFilePathDir("", "")
	for _, file := range files {
		parts := strings.Split(file.Path, "/")
		if len(parts) == 1 {
			root.files = append(root.files, file)
			continue
		}
		dir := root
		for _, part := range parts[:len(parts)-1] {
			childPath := part
			if dir.path != "" {
				childPath = dir.path + "/" + part
			}
			child := dir.dirs[part]
			if child == nil {
				child = newFilePathDir(part, childPath)
				dir.dirs[part] = child
			}
			dir = child
		}
		dir.files = append(dir.files, file)
	}

	return compactPathChildren(root, makeLeaf, folderID, expanded)
}

func compactPathChildren(dir *filePathDir, makeLeaf func(git.FileStatus) *widgets.TreeNode, folderID func(string) string, expanded map[string]bool) []*widgets.TreeNode {
	dirNames := make([]string, 0, len(dir.dirs))
	for name := range dir.dirs {
		dirNames = append(dirNames, name)
	}
	slices.Sort(dirNames)

	nodes := make([]*widgets.TreeNode, 0, len(dirNames)+len(dir.files))
	for _, name := range dirNames {
		start := dir.dirs[name]
		current := start
		labels := []string{current.name}
		for len(current.files) == 0 && len(current.dirs) == 1 {
			for _, child := range current.dirs {
				current = child
				labels = append(labels, current.name)
			}
		}

		id := folderID(start.path)
		isExpanded := true
		if saved, ok := expanded[id]; ok {
			isExpanded = saved
		}
		nodes = append(nodes, &widgets.TreeNode{
			ID:         id,
			Label:      strings.Join(labels, "/"),
			Expandable: true,
			Expanded:   isExpanded,
			Children:   compactPathChildren(current, makeLeaf, folderID, expanded),
		})
	}

	slices.SortFunc(dir.files, func(a, b git.FileStatus) int {
		return strings.Compare(a.Path, b.Path)
	})
	for _, file := range dir.files {
		leaf := makeLeaf(file)
		leaf.Label = gitPathBase(file.Path)
		leaf.TruncateLeft = false
		nodes = append(nodes, leaf)
	}
	return nodes
}

func gitPathBase(path string) string {
	if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
		return path[slash+1:]
	}
	return path
}

func isCommitFolderNode(node *widgets.TreeNode) bool {
	return node != nil && strings.HasPrefix(node.ID, commitFolderPrefix)
}

func (cp *ChangesPanel) selectedViewTargetID(tree *widgets.TreeWidget) string {
	if tree == nil || tree.Selected() == nil {
		return ""
	}
	node := tree.Selected()
	ref, working := cp.workNodes[node.ID]
	if isCommitFolderNode(node) || working && (ref.Kind == workNodeFolder || ref.Kind == workNodePRFolder) {
		if leaf := cp.firstFileDescendant(node.Children); leaf != nil {
			return leaf.ID
		}
	}
	return node.ID
}

func (cp *ChangesPanel) firstFileDescendant(nodes []*widgets.TreeNode) *widgets.TreeNode {
	for _, node := range nodes {
		if _, ok := cp.workFiles[node.ID]; ok {
			return node
		}
		if _, ok := cp.logFiles[node.ID]; ok {
			return node
		}
		if leaf := cp.firstFileDescendant(node.Children); leaf != nil {
			return leaf
		}
	}
	return nil
}

// revealTreeSelection restores by stable identity and opens only the ancestors
// needed to make that identity visible. It deliberately never falls back to a
// numeric row, since Tree and List have different visible row counts.
func revealTreeSelection(tree *widgets.TreeWidget, id string) bool {
	if tree == nil || id == "" || !expandPathToID(tree.Config.Items, id) {
		return false
	}
	tree.SetItems(tree.Config.Items)
	tree.SelectByID(id)
	return tree.Selected() != nil && tree.Selected().ID == id
}

func expandPathToID(nodes []*widgets.TreeNode, id string) bool {
	for _, node := range nodes {
		if node.ID == id {
			return true
		}
		if expandPathToID(node.Children, id) {
			node.Expanded = true
			return true
		}
	}
	return false
}

func (cp *ChangesPanel) FileView() string { return cp.fileView }

func (cp *ChangesPanel) SetFileView(view string) {
	if view != config.GitFileViewTree {
		view = config.GitFileViewList
	}
	if cp.fileView == view {
		return
	}

	workingSelection := cp.selectedViewTargetID(cp.Tree)
	logSelection := cp.selectedViewTargetID(cp.CommitLog)
	cp.saveExpanded()
	cp.saveCommitLogState()
	cp.fileView = view
	cp.buildTree()
	cp.rebuildCommitFileNodes()
	revealTreeSelection(cp.Tree, workingSelection)
	revealTreeSelection(cp.CommitLog, logSelection)
}

func (cp *ChangesPanel) rebuildCommitFileNodes() {
	cp.logFiles = make(map[string]commitFileRef)
	for _, node := range cp.CommitLog.Config.Items {
		commit, ok := cp.logCommits[node.ID]
		if !ok || len(node.Children) == 0 {
			continue
		}
		files, cached := cp.commitFiles[commit.Dir+"\x00"+commit.Ref]
		if cached {
			node.Children = cp.commitFileNodes(commit.Dir, commit.Ref, commit.Short, node.ID, files)
		}
	}
	cp.CommitLog.SetItems(cp.CommitLog.Config.Items)
}

func (cp *ChangesPanel) fileNodes(dir string, files []git.FileStatus, staged bool, group int, pr bool) []*widgets.TreeNode {
	fileKind := workNodeFile
	folderKind := workNodeFolder
	if pr {
		fileKind = workNodePRFile
		folderKind = workNodePRFolder
	}
	makeLeaf := func(file git.FileStatus) *widgets.TreeNode {
		return cp.fileNode(dir, file, staged, fileKind, group, pr)
	}
	if cp.fileView == config.GitFileViewList {
		nodes := make([]*widgets.TreeNode, 0, len(files))
		for _, file := range files {
			nodes = append(nodes, makeLeaf(file))
		}
		return nodes
	}
	return compactFileTreeWithFolderID(files, makeLeaf, func(path string) string {
		id := workingNodeID(folderKind, dir, path, staged)
		cp.workNodes[id] = workNodeRef{Dir: dir, Path: path, Staged: staged, Kind: folderKind, Group: group, PR: pr}
		return id
	}, cp.expanded)
}
