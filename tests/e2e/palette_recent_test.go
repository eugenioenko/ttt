package e2e

import (
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestCommandPaletteListsRecentCommandsFirst(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	openPalette := func() *ui.SelectDialogWidget {
		t.Helper()
		h.pressCtrl(tcell.KeyCtrlP)
		palette, ok := h.app.Root.TopOverlayWidget().(*ui.SelectDialogWidget)
		if !ok {
			t.Fatalf("Ctrl+P opened %T, want command palette", h.app.Root.TopOverlayWidget())
		}
		return palette
	}
	runFromPalette := func(id string) {
		t.Helper()
		palette := openPalette()
		palette.Selected = slices.IndexFunc(palette.Items, func(item ui.PaletteItem) bool { return item.ID == id })
		if palette.Selected < 0 {
			t.Fatalf("command %s is not in the palette", id)
		}
		h.pressKey(tcell.KeyEnter, tcell.ModNone)
	}

	palette := openPalette()
	alphabetical := make([]string, len(palette.Items))
	for i, item := range palette.Items {
		alphabetical[i] = item.ID
	}
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	h.exec("theme.switch")
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	if got := config.LoadState().RecentCommands; len(got) != 0 {
		t.Fatalf("recent commands after a run outside the palette = %v, want none", got)
	}

	// Toggling the sidebar saves the whole in-memory state, which must keep the
	// recent commands already written to disk.
	for _, id := range []string{"editor.lineEnding", "theme.switch", "editor.lineEnding", "sidebar.toggle"} {
		runFromPalette(id)
		h.pressKey(tcell.KeyEscape, tcell.ModNone)
	}
	want := []string{"sidebar.toggle", "editor.lineEnding", "theme.switch"}
	state := config.LoadState()
	if !slices.Equal(state.RecentCommands, want) {
		t.Fatalf("saved recent commands = %v, want %v", state.RecentCommands, want)
	}
	if !state.SidebarHidden {
		t.Fatal("sidebar.toggle from the palette did not save the sidebar layout")
	}

	palette = openPalette()
	if len(palette.Items) != len(alphabetical) {
		t.Fatalf("palette items = %d, want %d", len(palette.Items), len(alphabetical))
	}
	rest := slices.DeleteFunc(slices.Clone(alphabetical), func(id string) bool { return slices.Contains(want, id) })
	for i, id := range append(slices.Clone(want), rest...) {
		if palette.Items[i].ID != id {
			t.Fatalf("palette item %d = %s, want %s", i, palette.Items[i].ID, id)
		}
	}

	for _, r := range "line ending" {
		h.pressRune(r)
	}
	if palette.Items[0].ID != "editor.lineEnding" {
		t.Fatalf("query result = %s, want the best fuzzy match editor.lineEnding", palette.Items[0].ID)
	}
}
