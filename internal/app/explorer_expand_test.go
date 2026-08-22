package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func TestExplorerExpandAllReachesEveryVisibleDescendantAndCollapseAllClosesTree(t *testing.T) {
	rootPath := t.TempDir()
	for _, dir := range []string{
		"deep/one/two",
		"visible/inner",
		"visible/.hidden",
		"visible/.ignored",
	} {
		if err := os.MkdirAll(filepath.Join(rootPath, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(rootPath, ".gitignore"), []byte("visible/.ignored/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTreeGit(t, rootPath, "init", "-q")
	for _, path := range []string{
		"deep/one/two/deep.txt",
		"visible/inner/inner.txt",
		"visible/.hidden/hidden.txt",
		"visible/.ignored/ignored.txt",
	} {
		if err := os.WriteFile(filepath.Join(rootPath, filepath.FromSlash(path)), []byte(path), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), rootPath)
	root := explorer.Tree.Config.Items[0]

	explorer.ExpandAll()
	gitMetadata := nodeWithLabel(root.Children, ".git")
	if gitMetadata == nil || gitMetadata.Expanded || len(gitMetadata.Children) != 0 {
		t.Fatalf("bulk expansion traversed .git metadata: %+v", gitMetadata)
	}
	for _, label := range []string{"deep", "one", "two", "deep.txt", "visible", "inner", "inner.txt", ".hidden", "hidden.txt", ".ignored", "ignored.txt"} {
		node := nodeWithLabel(root.Children, label)
		if node == nil {
			t.Errorf("one expand did not materialize %q", label)
		} else if node.Expandable && !node.Expanded {
			t.Errorf("one expand left %q collapsed", label)
		}
	}
	explorer.CollapseAll()
	for _, node := range []*widgets.TreeNode{root, nodeWithLabel(root.Children, "deep"), nodeWithLabel(root.Children, "visible")} {
		if node != nil && node.Expanded {
			t.Errorf("collapse left %q open", node.Label)
		}
	}
}

func TestExplorerExpandAllLeavesGitMetadataManual(t *testing.T) {
	rootPath := t.TempDir()
	runTreeGit(t, rootPath, "init", "-q")
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), rootPath)
	explorer.ExpandAll()
	gitMetadata := nodeWithLabel(explorer.Tree.Config.Items, ".git")
	if gitMetadata == nil || gitMetadata.Expanded || len(gitMetadata.Children) != 0 {
		t.Fatalf("bulk expansion traversed .git metadata: %+v", gitMetadata)
	}
	explorer.Tree.SelectByID(gitMetadata.ID)
	explorer.Tree.ActivateSelected()
	if !gitMetadata.Expanded || nodeWithLabel(gitMetadata.Children, "HEAD") == nil {
		t.Fatalf("manual expansion no longer opens .git metadata: %+v", gitMetadata)
	}
}

func TestExplorerExpandAllStopsAtSymlinkCycles(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootPath, "deep", "one"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rootPath, filepath.Join(rootPath, "deep", "one", "loop")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), rootPath)
	explorer.ExpandAll()
	loop := nodeWithLabel(explorer.Tree.Config.Items, "loop")
	if loop == nil || loop.Expanded {
		t.Fatalf("cycle symlink was bulk-expanded: %+v", loop)
	}
	if len(loop.Children) != 0 {
		t.Fatalf("cycle recursively materialized children: %+v", loop.Children)
	}
}

func TestExplorerExpandAllDoesNotTraverseExternalDirectorySymlink(t *testing.T) {
	rootPath := t.TempDir()
	externalPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(externalPath, "large", "external", "tree"), 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(rootPath, "external-link")
	if err := os.Symlink(externalPath, linkPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), rootPath)
	explorer.ExpandAll()
	link := nodeWithLabel(explorer.Tree.Config.Items, "external-link")
	if link == nil || link.Expanded || len(link.Children) != 0 {
		t.Fatalf("bulk expansion traversed external symlink: %+v", link)
	}

	explorer.Tree.SelectByID(link.ID)
	explorer.Tree.ActivateSelected()
	if !link.Expanded || nodeWithLabel(link.Children, "large") == nil {
		t.Fatalf("manual expansion no longer opens directory symlinks: %+v", link)
	}
}
