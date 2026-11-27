package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestSearchInputFocusWithFindOverlay(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyCtrlF, tcell.KeyCtrlR} {
		t.Run(tcell.KeyNames[key], func(t *testing.T) {
			h := newTestHarness(t, 120, 30)
			defer h.stop()
			h.exec("file.new")
			h.exec("output.show")
			h.app.Output.AddLine(ui.OutputLine{Message: "output focus target"})
			h.exec("sidebar.search")
			h.app.Search.Debounce.DelayMs = 60_000
			t.Cleanup(h.app.Search.Debounce.Stop)
			h.app.FocusEditor()
			h.pressCtrl(key)
			bar := h.app.Root.TopOverlayWidget()
			localInput := bar.(ui.InputHolder).FocusedInput()
			h.typeText("local")

			r := h.app.Search.Input.GetRect()
			h.click(r.X+2, r.Y)
			h.typeText("global")
			if !h.app.Search.Input.IsFocused() {
				t.Fatal("global search input did not receive widget focus")
			}
			pasteText(h, pasteKeys(" pasted"))
			if got := h.app.Search.Input.Text(); got != "global pasted" {
				t.Fatalf("global search input = %q, want global pasted (focus %T)", got, h.app.Root.Focused)
			}
			if got := localInput.Text(); got != "local" {
				t.Fatalf("local search input changed to %q", got)
			}
			if got := editorText(h); got != "" {
				t.Fatalf("global search input leaked into editor: %q", got)
			}

			er := h.app.EditorGroup.Editor.GetRect()
			h.click(er.X+6, er.Y+2)
			if h.app.Search.Input.IsFocused() {
				t.Fatal("global search input stayed focused after clicking the editor")
			}
			h.pressRune('x')
			if got := editorText(h); got != "x" {
				t.Fatalf("editor input after clicking back = %q, want x", got)
			}
			outputRect := h.app.Output.GetRect()
			h.click(outputRect.X+4, screenRowOf(t, h, "output focus target"))
			if h.app.Root.Focused != h.app.BottomPanel.ActiveWidget() {
				t.Fatalf("focus after clicking output = %T, want bottom panel", h.app.Root.Focused)
			}
			if h.app.Root.TopOverlayWidget() != bar {
				t.Fatal("switching input focus closed the find overlay")
			}
		})
	}
}
