package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestExplorerExpandAllLoadsProgressivelyAndCollapseAllClosesTree(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootPath, "src", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootPath, "src", "nested", "file.go"), []byte("package nested\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), rootPath)
	root := explorer.Tree.Config.Items[0]
	src := nodeWithLabel(root.Children, "src")
	if src == nil || len(src.Children) != 0 {
		t.Fatalf("test setup src = %+v", src)
	}

	explorer.ExpandAll()
	nested := nodeWithLabel(src.Children, "nested")
	if !root.Expanded || !src.Expanded || nested == nil || nested.Expanded {
		t.Fatalf("first expand should load only represented level: root=%v src=%v nested=%+v", root.Expanded, src.Expanded, nested)
	}
	explorer.ExpandAll()
	if !nested.Expanded || nodeWithLabel(nested.Children, "file.go") == nil {
		t.Fatal("second expand did not load nested level")
	}
	explorer.CollapseAll()
	if root.Expanded || src.Expanded || nested.Expanded {
		t.Fatalf("collapse left folders open: root=%v src=%v nested=%v", root.Expanded, src.Expanded, nested.Expanded)
	}
}
