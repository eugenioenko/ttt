package e2e

import (
	"context"
	"testing"

	"github.com/eugenioenko/ttt/internal/update"
	"github.com/gdamore/tcell/v3"
)

func TestHelpDialogsOpenOverFindAndReplace(t *testing.T) {
	for _, search := range []string{"search.find", "search.replace"} {
		for _, tc := range []struct {
			command, text string
			index         int
		}{
			{command: "update.check", text: "Check for Updates", index: 0},
			{command: "about", text: "Thank you!", index: 1},
		} {
			t.Run(search+"/"+tc.command, func(t *testing.T) {
				h := updateHarness(t, update.Binary)
				h.app.Updates = fakeUpdater{latest: func(context.Context) (update.Release, error) {
					return update.Release{Tag: "v1.0.0"}, nil
				}}
				h.exec("file.new")
				h.exec(search)
				searchBar := h.app.Root.TopOverlayWidget()
				h.exec("menu.help")
				for range tc.index {
					h.pressKey(tcell.KeyDown, tcell.ModNone)
				}
				h.pressKey(tcell.KeyEnter, tcell.ModNone)
				h.assertContains(tc.text)
				if len(h.app.Root.Overlays) != 2 || !h.app.Root.HasModalOverlay() {
					t.Fatal("Help dialog should open above the search bar")
				}
				dialog := h.app.Root.TopOverlayWidget()
				h.exec(tc.command)
				if len(h.app.Root.Overlays) != 2 || h.app.Root.TopOverlayWidget() != dialog {
					t.Fatal("repeated command should not open another dialog")
				}
				h.pressKey(tcell.KeyEscape, tcell.ModNone)
				if len(h.app.Root.Overlays) != 1 || h.app.Root.TopOverlayWidget() != searchBar || h.app.Root.HasModalOverlay() {
					t.Fatal("dismissing the Help dialog should preserve the search bar")
				}
			})
		}
	}
}

func TestExplorerHelpDialog(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("explorer.help")

	h.assertContains("Explorer Shortcuts")
	h.assertContains("Open file or toggle folder")
	h.assertContains("Collapse folder")
	h.assertContains("Expand folder")

	// Dismiss with Escape
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	h.assertNotContains("Explorer Shortcuts")
}

func TestSearchHelpDialog(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("search.help")

	h.assertContains("Search Shortcuts")
	h.assertContains("Toggle case sensitivity")
	h.assertContains("Toggle regex mode")
	h.assertContains("Next input field")

	// Dismiss with Enter
	h.pressKey(tcell.KeyEnter, tcell.ModNone)

	h.assertNotContains("Search Shortcuts")
}

func TestChangesHelpDialog(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	h.exec("changes.help")

	h.assertContains("Changes Shortcuts")
	h.assertContains("Toggle stage/unstage file")
	h.assertContains("Stage all files")
	h.assertContains("Discard selected file")
	h.assertContains("Open compact diff")

	// Dismiss with Escape
	h.pressKey(tcell.KeyEscape, tcell.ModNone)

	h.assertNotContains("Changes Shortcuts")
}
