package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func explorerRootTestApp(t *testing.T, dir string) *App {
	t.Helper()
	cfg := config.AppConfig{
		Keybindings: config.DefaultKeybindings(),
		Settings:    config.DefaultSettings(),
		Theme:       config.DefaultTheme(),
	}
	borders := BuildBorderSet(cfg.Theme.Borders)
	a := BuildAppFromConfig(&cfg, &borders, workspace.New([]string{dir}), nil)
	sim := term.NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	a.Screen = term.NewTcellScreenFrom(sim)
	t.Cleanup(func() { sim.Fini() })
	t.Cleanup(func() { a.CloseTasks(); a.EditorGroup.CloseAllTabs() })
	a.Reg = command.NewRegistry()
	registerExplorerCommands(a)
	return a
}

// The right-click menu never offers delete for a workspace root, but the
// command palette and any keybinding reach explorer.delete directly.
func TestExplorerDeleteRefusesWorkspaceRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := explorerRootTestApp(t, dir)
	a.Explorer.Tree.SelectByID(dir)
	if selected := a.Explorer.Tree.Selected(); selected == nil || selected.ID != dir {
		t.Fatalf("workspace root is not selected: %+v", selected)
	}

	if !a.Reg.Execute("explorer.delete") {
		t.Fatal("explorer.delete is not registered")
	}
	if a.Root.HasOverlay() {
		t.Fatal("deleting the workspace root opened the delete confirmation")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("workspace root no longer exists: %v", err)
	}
}

func TestExplorerDeleteConfirmsForSelectionInsideWorkspaceRoot(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(keep, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := explorerRootTestApp(t, dir)
	a.ExplorerContextNode = &widgets.TreeNode{ID: keep}

	if !a.Reg.Execute("explorer.delete") {
		t.Fatal("explorer.delete is not registered")
	}
	if !a.Root.HasModalOverlay() {
		t.Fatal("deleting a file inside the workspace root did not open the delete confirmation")
	}
	a.DismissDialog()
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("file was removed before confirmation: %v", err)
	}
}
