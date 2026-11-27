package widgets

import (
	"slices"
	"strings"
	"testing"
)

func TestTreeFilterRevealsLoadedMatchesWithoutExpanding(t *testing.T) {
	file := &TreeNode{ID: "a/src/main.go", Label: "main.go"}
	folder := &TreeNode{ID: "a/src", Label: "src", Expandable: true, Children: []*TreeNode{file, {Label: "notes.txt"}}}
	root := &TreeNode{ID: "a", Label: "a", Expandable: true, Children: []*TreeNode{folder, {Label: "unloaded", Expandable: true}}}
	other := &TreeNode{ID: "b", Label: "b", Expandable: true, Children: []*TreeNode{{ID: "b/MAIN.go", Label: "MAIN.go"}}}
	expansions := 0
	tree := NewTreeWidget(TreeConfig{Items: []*TreeNode{root, other}, OnExpand: func(*TreeNode) { expansions++ }})
	tree.SetFilter(func(n *TreeNode) bool { return strings.Contains(strings.ToLower(n.Label), "main") })
	var ids []string
	for _, n := range tree.FlatList() {
		ids = append(ids, n.ID)
	}
	if !slices.Equal(ids, []string{"a", "a/src", "a/src/main.go", "b", "b/MAIN.go"}) {
		t.Fatalf("filtered IDs = %v", ids)
	}
	if root.Expanded || folder.Expanded || other.Expanded || expansions != 0 {
		t.Fatal("filter changed expansion state or loaded folders")
	}
	tree.SelectByID(file.ID)
	tree.SetItems(tree.Config.Items)
	if tree.ItemCount() != 5 {
		t.Fatal("model refresh lost filter")
	}
	tree.AppendItem(&TreeNode{ID: "added", Label: "main.txt"})
	if tree.ItemCount() != 6 {
		t.Fatal("append did not apply filter")
	}
	tree.SetFilter(func(*TreeNode) bool { return false })
	if tree.Selected() != nil || tree.ItemCount() != 0 {
		t.Fatal("no-match filter retained rows")
	}
	tree.SetFilter(nil)
	if tree.ItemCount() != 3 || root.Expanded || folder.Expanded {
		t.Fatal("clearing filter did not restore collapsed tree")
	}
}
