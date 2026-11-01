package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func forEachCompactMode(t *testing.T, run func(t *testing.T, compact bool)) {
	for _, compact := range []bool{true, false} {
		t.Run(fmt.Sprintf("compactFolders=%v", compact), func(t *testing.T) {
			run(t, compact)
		})
	}
}

func compactSettings(compact bool) config.ExplorerSettings {
	settings := config.DefaultExplorerSettings()
	settings.CompactFolders = compact
	return settings
}

func expandExplorerNode(t *testing.T, explorer *NavigationPanel, node *widgets.TreeNode) {
	t.Helper()
	explorer.Tree.SelectByID(node.ID)
	explorer.Tree.ActivateSelected()
	if !node.Expanded {
		t.Fatalf("activating %q did not expand it", node.Label)
	}
}

func childLabels(node *widgets.TreeNode) []string {
	var labels []string
	for _, child := range node.Children {
		labels = append(labels, child.Label)
	}
	return labels
}

func TestExplorerCompactsSingleChildFolderChainOnExpand(t *testing.T) {
	rootPath := t.TempDir()
	writeRevealFixture(t, rootPath, "src/main/java/App.java", "src/main/java/util/Strings.java", "README.md")
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), config.IconsNone, rootPath)
	root := explorer.Tree.Config.Items[0]

	src := nodeWithLabel(root.Children, "src")
	if src == nil {
		t.Fatalf("root children = %v, want collapsed src row", childLabels(root))
	}
	expandExplorerNode(t, explorer, src)

	deepest := filepath.Join(rootPath, "src", "main", "java")
	if src.Label != "src/main/java" || src.ID != deepest {
		t.Fatalf("compact row = %q (%s), want src/main/java (%s)", src.Label, src.ID, deepest)
	}
	if got, want := childLabels(src), []string{"util", "App.java"}; !slices.Equal(got, want) {
		t.Fatalf("compact row children = %v, want %v", got, want)
	}
	if got, want := childLabels(root), []string{"src/main/java", "README.md"}; !slices.Equal(got, want) {
		t.Fatalf("root children = %v, want %v", got, want)
	}
}

func TestExplorerDoesNotCompactRootsOrFoldersWithSiblings(t *testing.T) {
	rootPath := t.TempDir()
	writeRevealFixture(t, rootPath, "only/a/one.txt", "only/b/two.txt")
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), config.IconsNone, rootPath)
	root := explorer.Tree.Config.Items[0]
	if root.ID != rootPath || len(root.Children) != 1 || root.Children[0].Label != "only" {
		t.Fatalf("root was compacted: %q children %v", root.Label, childLabels(root))
	}
	only := root.Children[0]
	expandExplorerNode(t, explorer, only)
	if only.Label != "only" {
		t.Fatalf("folder with two subfolders compacted to %q", only.Label)
	}
}

func TestExplorerCompactFolderSurvivesReloadAndSplitsWhenChainChanges(t *testing.T) {
	rootPath := t.TempDir()
	writeRevealFixture(t, rootPath, "a/b/c/file.txt")
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), config.IconsNone, rootPath)
	expandExplorerNode(t, explorer, explorer.Tree.Config.Items[0].Children[0])

	explorer.Reload()
	row := explorer.Tree.Config.Items[0].Children[0]
	if row.Label != "a/b/c" || !row.Expanded || nodeWithLabel(row.Children, "file.txt") == nil {
		t.Fatalf("reload lost compact row: %q expanded=%v children %v", row.Label, row.Expanded, childLabels(row))
	}

	writeRevealFixture(t, rootPath, "a/b/note.txt")
	explorer.Reload()
	row = explorer.Tree.Config.Items[0].Children[0]
	if row.Label != "a/b" || row.ID != filepath.Join(rootPath, "a", "b") {
		t.Fatalf("file in chain did not split the row: %q (%s)", row.Label, row.ID)
	}
	if got, want := childLabels(row), []string{"c", "note.txt"}; !slices.Equal(got, want) {
		t.Fatalf("split row children = %v, want %v", got, want)
	}
}

func TestExplorerWatchesEveryFolderInExpandedCompactRow(t *testing.T) {
	rootPath := t.TempDir()
	writeRevealFixture(t, rootPath, "a/b/c/file.txt")
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), config.IconsNone, rootPath)
	expandExplorerNode(t, explorer, explorer.Tree.Config.Items[0].Children[0])

	dirs := explorer.WatchedDirs()
	for _, want := range []string{rootPath, filepath.Join(rootPath, "a"), filepath.Join(rootPath, "a", "b"), filepath.Join(rootPath, "a", "b", "c")} {
		if !slices.Contains(dirs, want) {
			t.Errorf("WatchedDirs() = %v, missing %s", dirs, want)
		}
	}
}

func TestExplorerCompactStopsAtDirectorySymlink(t *testing.T) {
	rootPath := t.TempDir()
	externalPath := t.TempDir()
	writeRevealFixture(t, externalPath, "inner/file.txt")
	if err := os.MkdirAll(filepath.Join(rootPath, "links"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalPath, filepath.Join(rootPath, "links", "target")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), config.IconsNone, rootPath)
	links := nodeWithLabel(explorer.Tree.Config.Items[0].Children, "links")
	expandExplorerNode(t, explorer, links)

	if links.Label != "links" || links.ID != filepath.Join(rootPath, "links") {
		t.Fatalf("compaction crossed the symlink: %q (%s)", links.Label, links.ID)
	}
	if got, want := childLabels(links), []string{"target"}; !slices.Equal(got, want) {
		t.Fatalf("links children = %v, want %v", got, want)
	}
}

func TestExplorerCompactFoldersSettingOff(t *testing.T) {
	rootPath := t.TempDir()
	writeRevealFixture(t, rootPath, "a/b/c/file.txt")
	explorer := NewNavigationPanel(compactSettings(false), config.IconsNone, rootPath)
	a := explorer.Tree.Config.Items[0].Children[0]
	expandExplorerNode(t, explorer, a)
	if a.Label != "a" || len(a.Children) != 1 || a.Children[0].Label != "b" {
		t.Fatalf("compaction ran with the setting off: %q children %v", a.Label, childLabels(a))
	}
}

func TestExplorerCompactRowStaysOpenWhenNewFolderSplitsIt(t *testing.T) {
	rootPath := t.TempDir()
	writeRevealFixture(t, rootPath, "src/main/java/App.java")
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), config.IconsNone, rootPath)
	expandExplorerNode(t, explorer, explorer.Tree.Config.Items[0].Children[0])

	if err := os.Mkdir(filepath.Join(rootPath, "src", "main", "kotlin"), 0o755); err != nil {
		t.Fatal(err)
	}
	explorer.Reload()

	row := explorer.Tree.Config.Items[0].Children[0]
	if row.Label != "src/main" || !row.Expanded {
		t.Fatalf("split row = %q expanded=%v, want open src/main", row.Label, row.Expanded)
	}
	java := nodeWithLabel(row.Children, "java")
	if java == nil || !java.Expanded || nodeWithLabel(java.Children, "App.java") == nil {
		t.Fatalf("previously open folder closed after split: %+v", java)
	}
	if kotlin := nodeWithLabel(row.Children, "kotlin"); kotlin == nil {
		t.Fatalf("new folder missing: %v", childLabels(row))
	}
}

func TestExplorerFolderStaysOpenWhenNewFolderMakesItCompact(t *testing.T) {
	rootPath := t.TempDir()
	writeRevealFixture(t, rootPath, "README.md")
	if err := os.MkdirAll(filepath.Join(rootPath, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), config.IconsNone, rootPath)
	expandExplorerNode(t, explorer, nodeWithLabel(explorer.Tree.Config.Items[0].Children, "empty"))

	if err := os.Mkdir(filepath.Join(rootPath, "empty", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	explorer.Reload()

	row := nodeWithLabel(explorer.Tree.Config.Items[0].Children, "empty/sub")
	if row == nil || !row.Expanded {
		t.Fatalf("folder closed after it became compact: %v", childLabels(explorer.Tree.Config.Items[0]))
	}
}

func TestExplorerCompactRowStaysOpenWhenChainGrows(t *testing.T) {
	rootPath := t.TempDir()
	writeRevealFixture(t, rootPath, "a/b/file.txt", "a/b/c/inner.txt")
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), config.IconsNone, rootPath)
	expandExplorerNode(t, explorer, explorer.Tree.Config.Items[0].Children[0])

	if err := os.Remove(filepath.Join(rootPath, "a", "b", "file.txt")); err != nil {
		t.Fatal(err)
	}
	explorer.Reload()

	row := explorer.Tree.Config.Items[0].Children[0]
	if row.Label != "a/b/c" || !row.Expanded || nodeWithLabel(row.Children, "inner.txt") == nil {
		t.Fatalf("grown row = %q expanded=%v children %v", row.Label, row.Expanded, childLabels(row))
	}
}

func TestExplorerReloadKeepsFoldersOpenAndSelectionWhenFolderIsAdded(t *testing.T) {
	forEachCompactMode(t, func(t *testing.T, compact bool) {
		rootPath := t.TempDir()
		writeRevealFixture(t, rootPath, "src/main/java/App.java")
		explorer := NewNavigationPanel(compactSettings(compact), config.IconsNone, rootPath)
		app := filepath.Join(rootPath, "src", "main", "java", "App.java")
		if !explorer.RevealPath(app) {
			t.Fatal("RevealPath failed")
		}

		if err := os.Mkdir(filepath.Join(rootPath, "src", "main", "kotlin"), 0o755); err != nil {
			t.Fatal(err)
		}
		explorer.Reload()

		open := []string{"src", "main", "java"}
		if compact {
			open = []string{"src/main", "java"}
		}
		root := explorer.Tree.Config.Items[0]
		for _, label := range open {
			if node := nodeWithLabel(root.Children, label); node == nil || !node.Expanded {
				t.Errorf("%q not open after reload: %+v", label, node)
			}
		}
		if kotlin := nodeWithLabel(root.Children, "kotlin"); kotlin == nil || kotlin.Expanded {
			t.Errorf("new folder = %+v, want a closed kotlin row", kotlin)
		}
		if selected := explorer.Tree.Selected(); selected == nil || selected.ID != app {
			t.Fatalf("selected %+v after reload, want %s", selected, app)
		}
	})
}

func TestExplorerReloadKeepsSelectedFolderWhenItBecomesCompact(t *testing.T) {
	forEachCompactMode(t, func(t *testing.T, compact bool) {
		rootPath := t.TempDir()
		writeRevealFixture(t, rootPath, "zeta.txt")
		if err := os.Mkdir(filepath.Join(rootPath, "empty"), 0o755); err != nil {
			t.Fatal(err)
		}
		explorer := NewNavigationPanel(compactSettings(compact), config.IconsNone, rootPath)
		expandExplorerNode(t, explorer, nodeWithLabel(explorer.Tree.Config.Items[0].Children, "empty"))

		for _, dir := range []string{"aaa", filepath.Join("empty", "sub")} {
			if err := os.Mkdir(filepath.Join(rootPath, dir), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		explorer.Reload()

		want := "empty"
		if compact {
			want = "empty/sub"
		}
		if selected := explorer.Tree.Selected(); selected == nil || selected.Label != want {
			t.Fatalf("selected %+v after reload, want the %q row", selected, want)
		}
	})
}
