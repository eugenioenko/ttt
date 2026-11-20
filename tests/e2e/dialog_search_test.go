package e2e

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestDialogCommandsOpenOverFindAndReplace(t *testing.T) {
	for _, search := range []string{"search.find", "search.replace"} {
		for _, command := range []string{
			"options.tabBarStyle",
			"options.editorSelectionStyle",
			"options.terminalSelectionStyle",
			"language.select",
			"editor.changeLanguage",
			"apps.openCurrentFileWith",
			"apps.add",
			"apps.addOpenWith",
			"debug.keyboardTester",
		} {
			t.Run(search+"/"+command, func(t *testing.T) {
				h := newTestHarness(t, 120, 40)
				defer h.stop()
				h.exec("file.new")
				h.exec(search)
				searchBar := h.app.Root.TopOverlayWidget()
				h.exec(command)
				if len(h.app.Root.Overlays) != 2 || !h.app.Root.HasModalOverlay() {
					t.Fatal("dialog should open above the search bar")
				}
				dialog := h.app.Root.TopOverlayWidget()
				h.exec(command)
				if len(h.app.Root.Overlays) != 2 || h.app.Root.TopOverlayWidget() != dialog {
					t.Fatal("repeated command should not open another dialog")
				}
				h.pressKey(tcell.KeyEscape, tcell.ModNone)
				if len(h.app.Root.Overlays) != 1 || h.app.Root.TopOverlayWidget() != searchBar || h.app.Root.HasModalOverlay() {
					t.Fatal("dismissing the dialog should preserve the search bar")
				}
			})
		}
	}
}
