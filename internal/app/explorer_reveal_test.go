package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func writeRevealFixture(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, file := range files {
		path := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(file), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExplorerRevealPathExpandsAncestorsAndSelectsFile(t *testing.T) {
	rootPath := t.TempDir()
	writeRevealFixture(t, rootPath, "src/app/deep/main.go", "docs/readme.md")
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), config.IconsNone, rootPath)
	root := explorer.Tree.Config.Items[0]

	target := filepath.Join(rootPath, "src", "app", "deep", "main.go")
	if !explorer.RevealPath(target) {
		t.Fatal("RevealPath returned false for a file inside the root")
	}
	for _, label := range []string{"src", "app", "deep"} {
		if node := nodeWithLabel(root.Children, label); node == nil || !node.Expanded {
			t.Errorf("ancestor %q not expanded: %+v", label, node)
		}
	}
	if docs := nodeWithLabel(root.Children, "docs"); docs == nil || docs.Expanded {
		t.Errorf("unrelated folder changed: %+v", docs)
	}
	if selected := explorer.Tree.Selected(); selected == nil || selected.ID != target {
		t.Fatalf("selected %+v, want %s", selected, target)
	}
}

func TestExplorerRevealPathFindsFileCreatedInCollapsedFolder(t *testing.T) {
	rootPath := t.TempDir()
	writeRevealFixture(t, rootPath, "pkg/old.go")
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), config.IconsNone, rootPath)
	pkg := nodeWithLabel(explorer.Tree.Config.Items[0].Children, "pkg")
	explorer.Tree.SelectByID(pkg.ID)
	explorer.Tree.ActivateSelected()
	explorer.Tree.ActivateSelected()
	if pkg.Expanded || nodeWithLabel(pkg.Children, "old.go") == nil {
		t.Fatalf("fixture expected pkg collapsed with loaded children: %+v", pkg)
	}

	writeRevealFixture(t, rootPath, "pkg/new.go")
	target := filepath.Join(rootPath, "pkg", "new.go")
	if !explorer.RevealPath(target) {
		t.Fatal("RevealPath missed a file created after the folder was loaded")
	}
	if selected := explorer.Tree.Selected(); selected == nil || selected.ID != target {
		t.Fatalf("selected %+v, want %s", selected, target)
	}
}

func TestExplorerRevealPathLeavesTreeUntouchedForUnreachablePaths(t *testing.T) {
	rootPath := t.TempDir()
	writeRevealFixture(t, rootPath, "src/main.go", ".secret/key.txt")
	settings := config.DefaultExplorerSettings()
	settings.ShowHidden = false
	explorer := NewNavigationPanel(settings, config.IconsNone, rootPath)
	before := explorer.Tree.ItemCount()
	selectedBefore := explorer.Tree.SelectedIndex()

	for _, path := range []string{
		filepath.Join(t.TempDir(), "elsewhere.go"),
		filepath.Join(rootPath, ".secret", "key.txt"),
		filepath.Join(rootPath, "src", "missing.go"),
		rootPath,
	} {
		if explorer.RevealPath(path) {
			t.Errorf("RevealPath(%s) = true, want false", path)
		}
	}
	if src := nodeWithLabel(explorer.Tree.Config.Items[0].Children, "src"); src == nil || src.Expanded {
		t.Errorf("failed reveal expanded src: %+v", src)
	}
	if explorer.Tree.ItemCount() != before || explorer.Tree.SelectedIndex() != selectedBefore {
		t.Errorf("failed reveal changed the tree: items %d->%d, selected %d->%d",
			before, explorer.Tree.ItemCount(), selectedBefore, explorer.Tree.SelectedIndex())
	}
}

func TestExplorerRevealPathExpandsTheMatchingRootInMultiRootWorkspace(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	writeRevealFixture(t, first, "a.txt")
	writeRevealFixture(t, second, "lib/b.txt")
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), config.IconsNone, first, second)
	firstRoot, secondRoot := explorer.Tree.Config.Items[0], explorer.Tree.Config.Items[1]

	target := filepath.Join(second, "lib", "b.txt")
	if !explorer.RevealPath(target) {
		t.Fatal("RevealPath returned false for a file in the second root")
	}
	if firstRoot.Expanded {
		t.Error("reveal expanded the other root")
	}
	if !secondRoot.Expanded {
		t.Error("reveal left the containing root collapsed")
	}
	if selected := explorer.Tree.Selected(); selected == nil || selected.ID != target {
		t.Fatalf("selected %+v, want %s", selected, target)
	}
}
