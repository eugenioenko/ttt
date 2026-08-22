package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func findTreeNode(nodes []*widgets.TreeNode, match func(*widgets.TreeNode) bool) *widgets.TreeNode {
	for _, node := range nodes {
		if match(node) {
			return node
		}
		if found := findTreeNode(node.Children, match); found != nil {
			return found
		}
	}
	return nil
}

func nodeWithLabel(nodes []*widgets.TreeNode, label string) *widgets.TreeNode {
	return findTreeNode(nodes, func(node *widgets.TreeNode) bool { return node.Label == label })
}

func nodeWithID(nodes []*widgets.TreeNode, id string) *widgets.TreeNode {
	return findTreeNode(nodes, func(node *widgets.TreeNode) bool { return node.ID == id })
}

func TestCompactFileTreeHandlesEmptySingleDeepAndDuplicateBasenames(t *testing.T) {
	makeLeaf := func(file git.FileStatus) *widgets.TreeNode {
		return &widgets.TreeNode{ID: file.Path, Label: file.Path}
	}
	if nodes := compactFileTree("test", nil, makeLeaf, nil); len(nodes) != 0 {
		t.Fatalf("empty tree = %+v", nodes)
	}

	files := []git.FileStatus{
		{Path: "README.md"},
		{Path: "one/shared.go"},
		{Path: "two/deep/nested/shared.go"},
	}
	nodes := compactFileTree("test", files, makeLeaf, nil)
	if nodeWithLabel(nodes, "README.md") == nil || nodeWithLabel(nodes, "two/deep/nested") == nil {
		t.Fatalf("tree lost root or compact deep path: %+v", nodes)
	}
	one := nodeWithID(nodes, "one/shared.go")
	two := nodeWithID(nodes, "two/deep/nested/shared.go")
	if one == nil || two == nil || one == two || one.Label != "shared.go" || two.Label != "shared.go" {
		t.Fatalf("duplicate basenames lost distinct identities: one=%+v two=%+v", one, two)
	}
}

func TestGitFileViewSwitchPreservesFileSelection(t *testing.T) {
	cp := NewChangesPanel("/repo")
	cp.groups = []changesGroup{{
		Dir:      "/repo",
		Name:     "repo",
		Unstaged: []git.FileStatus{{Status: "M", Path: "deep/nested/file.go"}},
	}}
	cp.SetFileView(config.GitFileViewTree)
	fileID := "file:/repo:deep/nested/file.go:false"
	if !revealTreeSelection(cp.Tree, fileID) {
		t.Fatal("tree did not contain nested file")
	}

	cp.SetFileView(config.GitFileViewList)
	if got := cp.Tree.Selected(); got == nil || got.ID != fileID || got.Label != "deep/nested/file.go" {
		t.Fatalf("list switch did not preserve file: %+v", got)
	}
	cp.SetFileView(config.GitFileViewTree)
	if got := cp.Tree.Selected(); got == nil || got.ID != fileID || got.Label != "file.go" {
		t.Fatalf("tree switch did not preserve file: %+v", got)
	}
}

func TestWorkingTreeSelectionAndFolderStateSurviveStageMove(t *testing.T) {
	cp := NewChangesPanel("/repo")
	cp.SetFileView(config.GitFileViewTree)
	file := git.FileStatus{Status: "M", Path: "src/nested/file.go"}
	cp.groups = []changesGroup{{Dir: "/repo", Name: "repo", Unstaged: []git.FileStatus{file}}}
	cp.buildTree()
	folder := nodeWithLabel(cp.Tree.Config.Items, "src/nested")
	leaf := nodeWithLabel(cp.Tree.Config.Items, "file.go")
	if folder == nil || leaf == nil || !revealTreeSelection(cp.Tree, leaf.ID) {
		t.Fatal("test setup did not build working tree")
	}
	file.Staged = true
	cp.groups = []changesGroup{{Dir: "/repo", Name: "repo", Staged: []git.FileStatus{file}}}
	cp.buildTree()
	folder = nodeWithLabel(cp.Tree.Config.Items, "src/nested")
	if got := cp.Tree.Selected(); got == nil || got.Label != "file.go" {
		t.Fatalf("file selection did not survive staged-group move: %+v", got)
	}
	if folder == nil || !folder.Expanded {
		t.Fatalf("selected file was not revealed after staged-group move: %+v", folder)
	}

	folder.Expanded = false
	cp.Tree.SetItems(cp.Tree.Config.Items)
	cp.Tree.SelectByID(cp.Tree.Config.Items[0].ID)
	cp.saveExpanded()
	file.Staged = false
	cp.groups = []changesGroup{{Dir: "/repo", Name: "repo", Unstaged: []git.FileStatus{file}}}
	cp.buildTree()
	folder = nodeWithLabel(cp.Tree.Config.Items, "src/nested")
	if folder == nil || folder.Expanded {
		t.Fatalf("folder state did not survive staged-group move: %+v", folder)
	}
}

func TestFolderNodesNeverResolveAsFilesOrExecuteFileActions(t *testing.T) {
	cp := NewChangesPanel("/repo")
	cp.SetFileView(config.GitFileViewTree)
	cp.groups = []changesGroup{{Dir: "/repo", Name: "repo", Unstaged: []git.FileStatus{{Status: "R", Path: "src/file:name.go", OldPath: "old/file:name.go"}}}}
	cp.buildTree()
	folder := nodeWithLabel(cp.Tree.Config.Items, "src")
	if folder == nil {
		t.Fatal("missing folder")
	}
	if _, _, _, ok := cp.parseFileNode(folder); ok {
		t.Fatal("folder resolved as a file")
	}
	leaf := nodeWithLabel(cp.Tree.Config.Items, "file:name.go")
	_, status, _, ok := cp.parseFileNode(leaf)
	if !ok || status.Path != "src/file:name.go" || status.OldPath != "old/file:name.go" {
		t.Fatalf("rename/colon path identity was not preserved: status=%+v ok=%v", status, ok)
	}
	opened := false
	cp.OnOpenDiff = func(string, git.FileStatus, bool) { opened = true }
	cp.handleCommand("activate", folder)
	if opened {
		t.Fatal("folder activation executed a file action")
	}
}

func TestCommitFileTreeFoldersToggleAndBulkStatePersists(t *testing.T) {
	cp := NewChangesPanel("/repo")
	cp.SetFileView(config.GitFileViewTree)
	const ref = "0123456789012345678901234567890123456789"
	commitID := "commit:" + ref
	files := []git.FileStatus{{Status: "M", Path: "pkg/history/one.go"}, {Status: "A", Path: "pkg/history/two.go"}}
	cp.commitFiles["/repo\x00"+ref] = files
	cp.logCommits[commitID] = commitFileRef{Dir: "/repo", Ref: ref, Short: ref[:7]}
	commit := &widgets.TreeNode{ID: commitID, Label: "files", Expanded: true, Children: cp.commitFileNodes("/repo", ref, ref[:7], commitID, files)}
	cp.CommitLog.SetItems([]*widgets.TreeNode{commit})
	folder := nodeWithLabel(commit.Children, "pkg/history")
	if folder == nil || !revealTreeSelection(cp.CommitLog, folder.ID) {
		t.Fatal("missing commit folder")
	}
	cp.CommitLog.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if folder.Expanded {
		t.Fatal("folder label activation did not collapse")
	}
	cp.ExpandAll()
	if !folder.Expanded || !commit.Expanded {
		t.Fatal("expand all did not open loaded commit folder")
	}
	cp.CollapseAll()
	if folder.Expanded || !commit.Expanded {
		t.Fatal("collapse all changed commit-row disclosure or left folder open")
	}
}
