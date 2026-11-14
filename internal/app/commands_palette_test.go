package app

import (
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
)

var paletteHiddenExplorerCommands = []string{
	"explorer.delete",
	"explorer.rename",
	"explorer.open",
	"explorer.reveal",
	"explorer.copyAbsolutePath",
	"explorer.copyRelativePath",
}

var paletteVisibleExplorerCommands = []string{
	"explorer.openInTerminal",
	"explorer.newFile",
	"explorer.newFolder",
	"explorer.expandAll",
	"explorer.collapseAll",
	"explorer.refresh",
	"explorer.revealActiveFile",
}

func explorerCommandApp(t *testing.T) *App {
	t.Helper()
	a := buildTestApp(t, config.DefaultSettings())
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

func commandIDs(cmds []command.Command) []string {
	ids := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		ids = append(ids, cmd.ID)
	}
	return ids
}

func TestPaletteHiddenCommandsAreOutOfThePaletteButStillListedAndRunnable(t *testing.T) {
	a := explorerCommandApp(t)

	paletteIDs := commandIDs(paletteCommands(a.Reg))
	// Registry.List() feeds the keybindings editor and the plugin command list.
	listedIDs := commandIDs(a.Reg.List())

	if len(paletteIDs) == 0 {
		t.Fatal("palette list is empty")
	}
	for _, id := range paletteHiddenExplorerCommands {
		if slices.Contains(paletteIDs, id) {
			t.Errorf("%s is offered by the command palette", id)
		}
		if !slices.Contains(listedIDs, id) {
			t.Errorf("%s missing from Registry.List()", id)
		}
		if !a.Reg.Execute(id) {
			t.Errorf("%s is not executable by ID", id)
		}
	}
}

func TestVisibleExplorerCommandsStayInThePalette(t *testing.T) {
	a := explorerCommandApp(t)

	paletteIDs := commandIDs(paletteCommands(a.Reg))
	for _, id := range paletteVisibleExplorerCommands {
		if !slices.Contains(paletteIDs, id) {
			t.Errorf("%s should stay in the command palette", id)
		}
	}
}
