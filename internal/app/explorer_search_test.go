package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestExplorerSearchUsesLoadedNodesAcrossRoots(t *testing.T) {
	forEachCompactMode(t, func(t *testing.T, compact bool) {
		first, second := t.TempDir(), t.TempDir()
		for _, path := range []string{
			filepath.Join(first, "loaded", "needle.go"),
			filepath.Join(first, "loaded", "other.go"),
			filepath.Join(second, "needle.txt"),
			filepath.Join(second, "cold", "needle-unloaded.txt"),
		} {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		panel := NewNavigationPanel(compactSettings(compact), config.IconsNone, first, second)
		panel.ActivatePath(second)
		if !panel.RevealPath(filepath.Join(first, "loaded", "needle.go")) {
			t.Fatal("could not materialize first root")
		}
		root := panel.Tree.Config.Items[0]
		root.Expanded = false
		cold := nodeWithLabel(panel.Tree.Config.Items[1].Children, "cold")
		if cold == nil || len(cold.Children) != 0 {
			t.Fatal("cold folder was already loaded")
		}
		panel.Search.ShowSearch()
		panel.Search.Input.SetText("NEEDLE")
		var ids []string
		for _, node := range panel.Tree.FlatList() {
			ids = append(ids, node.ID)
		}
		want := []string{first, filepath.Join(first, "loaded"), filepath.Join(first, "loaded", "needle.go"), second, filepath.Join(second, "needle.txt")}
		if !slices.Equal(ids, want) {
			t.Fatalf("filtered IDs = %v, want %v", ids, want)
		}
		if len(cold.Children) != 0 || cold.Expanded || root.Expanded {
			t.Fatal("search loaded or expanded folders")
		}
		panel.ApplyGitStatus(nil)
		if panel.Tree.ItemCount() != len(want) {
			t.Fatal("git decorations reset filter")
		}
		panel.Search.HideSearch()
		if root.Expanded || panel.Tree.FlatList()[0] != root {
			t.Fatal("clear did not restore original expansion")
		}
	})
}
