package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestExplorerExpandAllLoadsOneNewLevelAndCollapseAllClosesEverything(t *testing.T) {
	rootPath := t.TempDir()
	nestedPath := filepath.Join(rootPath, "src", "nested")
	if err := os.MkdirAll(nestedPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedPath, "file.go"), []byte("package nested\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hiddenPath := filepath.Join(rootPath, ".hidden")
	if err := os.Mkdir(hiddenPath, 0o755); err != nil {
		t.Fatal(err)
	}

	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), rootPath)
	root := explorer.Tree.Config.Items[0]
	src := nodeWithLabel(root.Children, "src")
	if src == nil || len(src.Children) != 0 {
		t.Fatalf("test setup: src = %+v", src)
	}

	explorer.ExpandAll()
	nested := nodeWithLabel(src.Children, "nested")
	if !root.Expanded || !src.Expanded || nested == nil {
		t.Fatalf("first expand did not open represented folders: root=%v src=%v nested=%+v", root.Expanded, src.Expanded, nested)
	}
	if nested.Expanded || len(nested.Children) != 0 {
		t.Fatal("newly revealed nested folder should not recursively walk the filesystem in the same action")
	}
	hidden := nodeWithLabel(root.Children, ".hidden")
	if hidden == nil || hidden.Expanded {
		t.Fatalf("muted hidden folder should remain manually expandable: %+v", hidden)
	}

	explorer.ExpandAll()
	if !nested.Expanded || nodeWithLabel(nested.Children, "file.go") == nil {
		t.Fatal("second expand did not open the newly represented level")
	}
	explorer.CollapseAll()
	if root.Expanded || src.Expanded || nested.Expanded {
		t.Fatalf("collapse left folders open: root=%v src=%v nested=%v", root.Expanded, src.Expanded, nested.Expanded)
	}
}
