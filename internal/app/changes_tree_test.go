package app

import (
	"strings"
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

func TestChangesTreeCompactsPathsAndKeepsFileIdentity(t *testing.T) {
	cp := NewChangesPanel("/repo")
	cp.groups = []changesGroup{{
		Dir:  "/repo",
		Name: "repo",
		Unstaged: []git.FileStatus{
			{Status: "M", Path: "README.md"},
			{Status: "M", Path: "docs-web/src/content/docs/themes.md"},
			{Status: "M", Path: "docs-web/src/content/docs/settings.md"},
			{Status: "M", Path: "internal/app/changes_panel.go"},
			{Status: "M", Path: "internal/ui/diff_widget.go"},
		},
	}}
	cp.buildTree()

	section := cp.Tree.Config.Items[0]
	if nodeWithLabel(section.Children, "docs-web/src/content/docs") == nil {
		t.Fatalf("single-child directory chain was not compacted: %+v", section.Children)
	}
	internal := nodeWithLabel(section.Children, "internal")
	if internal == nil || nodeWithLabel(internal.Children, "app") == nil || nodeWithLabel(internal.Children, "ui") == nil {
		t.Fatalf("branching directories lost their hierarchy: %+v", internal)
	}
	leaf := nodeWithLabel(section.Children, "changes_panel.go")
	if leaf == nil {
		t.Fatal("tree did not render the file basename")
	}
	dir, status, staged, ok := cp.parseFileNode(leaf)
	if !ok || dir != "/repo" || status.Path != "internal/app/changes_panel.go" || staged {
		t.Fatalf("tree leaf lost its canonical file identity: dir=%q status=%+v staged=%v ok=%v", dir, status, staged, ok)
	}
}

func TestGitFileViewSwitchPreservesSelectionAndRevealsAncestors(t *testing.T) {
	cp := NewChangesPanel("/repo")
	cp.groups = []changesGroup{{
		Dir:      "/repo",
		Name:     "repo",
		Unstaged: []git.FileStatus{{Status: "M", Path: "deep/nested/file.go"}},
	}}
	cp.buildTree()

	section := cp.Tree.Config.Items[0]
	folder := nodeWithLabel(section.Children, "deep/nested")
	if folder == nil {
		t.Fatal("missing compact folder")
	}
	folder.Expanded = false
	cp.Tree.SetItems(cp.Tree.Config.Items)

	cp.SetFileView(config.GitFileViewList)
	fileID := "file:/repo:deep/nested/file.go:false"
	if !revealTreeSelection(cp.Tree, fileID) {
		t.Fatal("flat view did not contain the nested file")
	}
	if got := cp.Tree.Selected(); got == nil || got.Label != "deep/nested/file.go" {
		t.Fatalf("flat view label = %+v, want full path", got)
	}

	cp.SetFileView(config.GitFileViewTree)
	if got := cp.Tree.Selected(); got == nil || got.ID != fileID || got.Label != "file.go" {
		t.Fatalf("tree switch did not preserve the selected file: %+v", got)
	}
	folder = nodeWithLabel(cp.Tree.Config.Items, "deep/nested")
	if folder == nil || !folder.Expanded {
		t.Fatal("selected file's collapsed ancestor was not revealed")
	}
}

func TestCommitFilesUseTreeAndFolderLabelsToggle(t *testing.T) {
	cp := NewChangesPanel("/repo")
	const ref = "0123456789012345678901234567890123456789"
	commitID := "commit:" + ref
	files := []git.FileStatus{
		{Status: "M", Path: "pkg/history/one.go"},
		{Status: "A", Path: "pkg/history/two.go"},
	}
	cp.commitFiles["/repo\x00"+ref] = files
	cp.logCommits[commitID] = commitFileRef{Dir: "/repo", Ref: ref, Short: ref[:7]}
	commit := &widgets.TreeNode{
		ID:       commitID,
		Label:    "nested files",
		Expanded: true,
		Children: cp.commitFileNodes("/repo", ref, ref[:7], commitID, files),
	}
	cp.CommitLog.SetItems([]*widgets.TreeNode{commit})

	folder := nodeWithLabel(commit.Children, "pkg/history")
	if folder == nil || !strings.HasPrefix(folder.ID, commitFolderPrefix) {
		t.Fatalf("commit files were not grouped under a history folder: %+v", commit.Children)
	}
	if !revealTreeSelection(cp.CommitLog, folder.ID) {
		t.Fatal("could not select commit folder")
	}
	cp.CommitLog.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if folder.Expanded {
		t.Fatal("activating a commit folder label did not collapse it")
	}

	cp.SetFileView(config.GitFileViewList)
	oneID := "cfile:" + commitID + ":pkg/history/one.go"
	leaf := nodeWithID(cp.CommitLog.Config.Items, oneID)
	if leaf == nil || leaf.Label != "pkg/history/one.go" {
		t.Fatalf("list view did not restore full commit paths: %+v", leaf)
	}
	if got := cp.CommitLog.Selected(); got == nil || got.ID != oneID {
		t.Fatalf("folder selection did not move deterministically to its first file: %+v", got)
	}

	cp.SetFileView(config.GitFileViewTree)
	if got := cp.CommitLog.Selected(); got == nil || got.ID != oneID || got.Label != "one.go" {
		t.Fatalf("commit file selection did not survive the switch back to tree: %+v", got)
	}
}

func TestCompactFileTreeTreatsColonAsPathContent(t *testing.T) {
	nodes := compactFileTree("test", []git.FileStatus{{Status: "M", Path: "dir:one/file:name.go"}}, func(file git.FileStatus) *widgets.TreeNode {
		return &widgets.TreeNode{ID: file.Path, Label: file.Path}
	}, nil)
	if len(nodes) != 1 || nodes[0].Label != "dir:one" || len(nodes[0].Children) != 1 || nodes[0].Children[0].Label != "file:name.go" {
		t.Fatalf("colon-containing path was split as identity syntax: %+v", nodes)
	}
}

func TestWorkingFolderSelectionKeepsItsRepositoryContext(t *testing.T) {
	cp := NewChangesPanel("/one", "/two")
	cp.groups = []changesGroup{
		{Dir: "/one", Name: "one", Unstaged: []git.FileStatus{{Status: "M", Path: "one.go"}}},
		{Dir: "/two", Name: "two", Unstaged: []git.FileStatus{{Status: "M", Path: "nested/two.go"}}},
	}
	cp.multiRoot = true
	cp.buildTree()

	folder := nodeWithLabel(cp.Tree.Config.Items[1].Children, "nested")
	if folder == nil || !revealTreeSelection(cp.Tree, folder.ID) {
		t.Fatal("could not select the second repository's folder")
	}
	if got := cp.selectedGroupDir(); got != "/two" {
		t.Errorf("folder selection resolved repo %q, want /two", got)
	}
	if gi := cp.groupIndexFromNode(folder); gi != -1 {
		t.Errorf("folder unexpectedly gained repo-wide bulk actions through group index %d", gi)
	}
	cp.buildTree()
	if got := cp.Tree.Selected(); got == nil || got.ID != folder.ID {
		t.Fatalf("status rebuild did not preserve the selected folder: %+v", got)
	}
}
