package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func TestWelcomeTabListsStartActions(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	h.exec("help.welcome")
	h.redraw()
	for _, label := range []string{"Welcome", "Open Folder…", "New File"} {
		h.assertContains(label)
	}
}

func TestEmptyExplorerRunsItsActions(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	var ran []string
	h.app.Explorer.OnAction = func(id string) { ran = append(ran, id) }
	h.app.Explorer.SetRoots(nil)
	h.exec("sidebar.explorer")
	h.redraw()
	h.assertContains("No folder open")

	h.app.Explorer.Tree.SelectByID("command:workspace.openFolder")
	h.app.Explorer.Tree.ActivateSelected()
	if len(ran) != 1 || ran[0] != "workspace.openFolder" {
		t.Fatalf("actions run = %v, want [workspace.openFolder]", ran)
	}
}

func TestWelcomeRowRunsOnClick(t *testing.T) {
	h := newTestHarness(t, 100, 40)
	defer h.stop()

	h.exec("help.welcome")
	h.redraw()
	for y := 0; y < 40; y++ {
		row := h.screenRow(y)
		if x := displayColumnOf(row, "Settings"); x >= 0 && !strings.Contains(row, "Welcome") {
			h.click(x, y)
			h.redraw()
			// The form's buttons show on every settings tab.
			h.assertContains("Cancel")
			h.assertContains("Apply")
			return
		}
	}
	t.Fatalf("Settings row not found:\n%s", h.screenText())
}

func TestClosingTheWorkspaceShowsWelcome(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	h.exec("workspace.close")
	h.redraw()
	if n := len(h.app.Workspace.Paths()); n != 0 {
		t.Fatalf("%d folders left after Close Workspace", n)
	}
	h.assertContains("Welcome")
	h.assertNotContains("untitled")
	if h.app.Sidebar.Visible {
		t.Error("sidebar still shown next to the welcome page")
	}
}

func TestRemovingTheLastFolderShowsWelcome(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	for _, p := range h.app.Workspace.Paths() {
		h.app.FileOpRemoveRoot(p)
	}
	h.redraw()
	if n := len(h.app.Workspace.Paths()); n != 0 {
		t.Fatalf("%d folders left after removing every root", n)
	}
	h.assertContains("Welcome")
}

// With no folder open the welcome page is the editor's empty state: it gives
// way to anything opened and comes back once that closes.
func TestWelcomeIsTheEmptyState(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	h.exec("workspace.close")
	h.exec("file.new")
	h.assertNotContains("Welcome")

	h.exec("tab.close")
	h.assertContains("Welcome")

	h.exec("tab.close")
	h.assertContains("Welcome")
}

// Favorites that exist are listed with their path and open on click; missing
// ones are left out.
func TestWelcomeFavoriteOpensFolder(t *testing.T) {
	h := newTestHarness(t, 100, 40)
	defer h.stop()

	fav := filepath.Join(t.TempDir(), "fav-project")
	if err := os.Mkdir(fav, 0o755); err != nil {
		t.Fatal(err)
	}
	h.app.Settings.Welcome.Favorites = []string{fav, filepath.Join(fav, "missing-project")}
	h.exec("workspace.close")
	h.redraw()
	h.assertNotContains("missing-project")
	for y := 0; y < 40; y++ {
		row := h.screenRow(y)
		if x := displayColumnOf(row, "fav-project"); x >= 0 {
			h.click(x, y)
			h.redraw()
			if paths := h.app.Workspace.Paths(); len(paths) != 1 || paths[0] != fav {
				t.Fatalf("workspace = %v, want [%s]", paths, fav)
			}
			return
		}
	}
	t.Fatalf("favorite row not found:\n%s", h.screenText())
}

// Adding the open folder saves it and lists it on the welcome page; removing
// it brings back the hint.
func TestWelcomeFavoriteCommands(t *testing.T) {
	h := newTestHarness(t, 100, 40)
	defer h.stop()

	root := h.app.Workspace.Paths()[0]
	h.exec("welcome.addFavorite")
	if favs := h.app.Settings.Welcome.Favorites; len(favs) != 1 {
		t.Fatalf("favorites = %v, want the open folder", favs)
	}
	h.exec("workspace.close")
	h.redraw()
	h.assertContains("Favorites")
	h.assertContains(filepath.Base(root))
	h.assertNotContains("Right-click a folder")

	h.app.ExplorerContextNode = &widgets.TreeNode{ID: root}
	h.exec("welcome.removeFavorite")
	if favs := h.app.Settings.Welcome.Favorites; len(favs) != 0 {
		t.Fatalf("favorites = %v after removing, want none", favs)
	}
	h.redraw()
	h.assertContains("Right-click a folder")
}

// Loose files opened without a folder keep the old behaviour: closing them
// all leaves an untitled tab, not the welcome page.
func TestClosingLooseFilesLeavesUntitled(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	for _, p := range h.app.Workspace.Paths() {
		h.app.Workspace.RemoveFolder(p)
	}
	h.exec("file.new")
	h.exec("tab.closeAll")
	h.redraw()
	h.assertNotContains("Welcome")
	h.assertContains("untitled")
}

// The copy button on a favorite copies its path instead of opening it.
func TestWelcomeFavoriteCopyButton(t *testing.T) {
	clipboard.DisableSystem()
	h := newTestHarness(t, 100, 40)
	defer h.stop()

	fav := filepath.Join(t.TempDir(), "fav-project")
	if err := os.Mkdir(fav, 0o755); err != nil {
		t.Fatal(err)
	}
	h.app.Settings.Welcome.Favorites = []string{fav}
	h.exec("workspace.close")
	h.redraw()
	for y := 0; y < 40; y++ {
		row := h.screenRow(y)
		if strings.Contains(row, "fav-project") {
			h.click(displayColumnOf(row, "⧉"), y)
			h.redraw()
			if n := len(h.app.Workspace.Paths()); n != 0 {
				t.Fatalf("copy button opened the folder (%d open)", n)
			}
			if got := clipboard.Get(); got != fav {
				t.Fatalf("clipboard = %q, want %q", got, fav)
			}
			return
		}
	}
	t.Fatalf("favorite row not found:\n%s", h.screenText())
}

// A long favorites list scrolls inside its section; the title stays.
func TestWelcomeLongFavoritesScroll(t *testing.T) {
	h := newTestHarness(t, 90, 40)
	defer h.stop()

	var favs []string
	for i := 1; i <= 30; i++ {
		dir := filepath.Join(t.TempDir(), fmt.Sprintf("proj-%02d", i))
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		favs = append(favs, dir)
	}
	h.app.Settings.Welcome.Favorites = favs
	h.exec("workspace.close")
	h.redraw()
	h.assertContains("Terminal Text Tool")
	h.assertContains("Favorites  1–")
	h.assertContains("of 30")
	h.assertNotContains("proj-30")

	h.pressKey(tcell.KeyEnd, tcell.ModNone)
	h.redraw()
	h.assertContains("–30 of 30")
	h.assertContains("proj-30")
	h.assertNotContains("proj-01")
}

// Closing the workspace with files open leaves them alone; the welcome page
// shows once they are closed.
func TestCloseWorkspaceKeepsOpenFiles(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	dir := t.TempDir()
	first, file := filepath.Join(dir, "first.txt"), filepath.Join(dir, "keep.txt")
	for _, p := range []string{first, file} {
		if err := os.WriteFile(p, []byte("content of "+filepath.Base(p)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Opening a file twice pins its tab, so the second one does not replace it.
	for _, p := range []string{first, first, file, file} {
		h.app.EditorGroup.OpenFile(p)
	}
	if n := h.app.EditorGroup.TabCount(); n < 2 {
		t.Fatalf("%d tabs open, want both files", n)
	}
	// The active tab is not the last one, where a transient welcome tab would
	// have shifted the selection.
	h.app.EditorGroup.SwitchToTabByPath(first)
	h.exec("workspace.close")
	h.redraw()
	h.assertNotContains("Welcome")
	if got := h.app.EditorGroup.ActiveFilePath(); got != first {
		t.Fatalf("active tab = %q after Close Workspace, want %q", got, first)
	}

	h.exec("tab.closeAll")
	h.redraw()
	h.assertContains("Welcome")
}

// A welcome action runs on the press; the rest of the click must not pull the
// focus back out of the dialog it opened, or typing goes nowhere.
func TestWelcomeClickLeavesFocusInDialog(t *testing.T) {
	h := newTestHarness(t, 100, 40)
	defer h.stop()

	h.exec("help.welcome")
	h.redraw()
	for y := 0; y < 40; y++ {
		row := h.screenRow(y)
		if x := displayColumnOf(row, "Open Workspace…"); x >= 0 {
			h.click(x, y)
			for _, r := range "demo.ttt" {
				h.pressRune(r)
			}
			h.redraw()
			h.assertContains("❯ demo.ttt")
			return
		}
	}
	t.Fatalf("Open Workspace row not found:\n%s", h.screenText())
}
