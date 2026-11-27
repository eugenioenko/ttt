package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestExplorerSearchSlashFocusNavigationAndEscape(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.exec("sidebar.explorer")
	initialCount := h.app.Explorer.Tree.ItemCount()
	h.pressRune('/')
	input := h.app.Explorer.Search.Input
	if !input.IsFocused() || input.Text() != "" {
		t.Fatal("slash did not show and focus empty search input")
	}
	h.typeText(".TXT")
	h.assertContains("❯")
	h.assertContains("─")
	if input.Text() != ".TXT" {
		t.Fatalf("typing did not go to search: %q", input.Text())
	}
	tree := h.app.Explorer.Tree
	if tree.Selected().Label != "alpha.txt" {
		t.Fatalf("first match = %q", tree.Selected().Label)
	}
	for _, n := range tree.FlatList() {
		if n.Label == "subdir" {
			t.Fatal("unmatched folder is visible")
		}
	}
	h.pressKey(tcell.KeyDown, tcell.ModNone)
	if tree.Selected().Label != "beta.txt" {
		t.Fatalf("Down selected %q", tree.Selected().Label)
	}
	h.pressKey(tcell.KeyUp, tcell.ModNone)
	if tree.Selected().Label != "alpha.txt" {
		t.Fatal("Up did not return to first match")
	}
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if h.app.EditorGroup.ActiveFilePath() != filepath.Join(h.dir, "alpha.txt") {
		t.Fatal("Enter did not open selected match")
	}
	h.exec("sidebar.explorer")
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	if input.IsFocused() || input.Text() != "" || tree.ItemCount() != initialCount {
		t.Fatal("Escape did not clear and hide search")
	}
	if strings.Contains(h.screenText(), "❯") {
		t.Fatal("search input still rendered after Escape")
	}
}

func TestExplorerSearchMoreMenuClearAndRefocus(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	h.exec("sidebar.explorer")
	h.app.ShowSidebarMoreMenu(3, 3)
	h.redraw()
	menu, ok := h.app.Root.TopOverlayWidget().(*ui.ContextMenuWidget)
	if !ok {
		t.Fatal("More menu did not open")
	}
	found := false
	for i, item := range menu.Items {
		if item.Label == "Search in Folders" && item.Command == "explorer.find" {
			menu.Selected = i
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Search in Folders missing from More menu")
	}
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	input := h.app.Explorer.Search.Input
	if !input.IsFocused() {
		t.Fatal("More menu command did not focus input")
	}
	h.typeText("missing")
	h.assertContains("No results")
	h.click(input.GetRect().X+3, input.GetRect().Y+2)
	h.pressRune('/')
	if !input.IsFocused() || input.Text() != "missing" {
		t.Fatal("slash did not refocus existing search")
	}
	if len(input.ActionHits) != 1 {
		t.Fatal("clear action not rendered")
	}
	hit := input.ActionHits[0]
	h.click(hit.X, hit.Y)
	if input.IsFocused() || input.Text() != "" {
		t.Fatal("X did not clear and hide input")
	}
	h.pressRune('/')
	h.typeText("alpha")
	if input.Text() != "alpha" {
		t.Fatal("slash did not reopen focused input after X")
	}
	tree := h.app.Explorer.Tree
	rect := tree.GetRect()
	h.click(rect.X+5, rect.Y+tree.SelectedIndex()-tree.ScrollTop())
	if h.app.EditorGroup.ActiveFilePath() != filepath.Join(h.dir, "alpha.txt") {
		t.Fatal("click did not open filtered file below the search header")
	}
}
