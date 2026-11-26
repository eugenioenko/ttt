package e2e

import (
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestEditorContextMenuTargetsClickedGroup(t *testing.T) {
	for _, split := range []string{"editor.split", "editor.splitDown"} {
		t.Run(split, func(t *testing.T) {
			h := newTestHarness(t, 140, 38)
			defer h.stop()
			first := h.app.EditorGroup
			h.typeText("first group")
			h.exec(split)
			second := h.app.EditorGroup
			h.typeText("second group")
			h.app.Root.SetFocus(first)
			h.redraw()

			r := second.ContentRect()
			h.rightClick(r.X+8, r.Y+3)
			menu, ok := h.app.Root.TopOverlayWidget().(*ui.ContextMenuWidget)
			if !ok {
				t.Fatal("right-clicking the editor did not open a context menu")
			}
			menu.Selected = slices.IndexFunc(menu.Items, func(item ui.ContextMenuItem) bool {
				return item.Command == "editor.selectAll"
			})
			if menu.Selected < 0 {
				t.Fatal("editor context menu did not contain Select All")
			}
			h.pressKey(tcell.KeyEnter, tcell.ModNone)

			if got := second.SelectedText(); got != "second group" {
				t.Errorf("selection in the clicked group = %q, want second group", got)
			}
			if got := first.SelectedText(); got != "" {
				t.Errorf("selection in the previously active group = %q, want empty", got)
			}
			if h.app.Root.Focused != second {
				t.Error("focus did not return to the clicked editor")
			}
		})
	}
}
