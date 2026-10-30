package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
)

func TestTerminalPaddingClickFocusesWithoutSelecting(t *testing.T) {
	for name, position := range map[string]ui.SplitPosition{"bottom": ui.SplitBottom, "right": ui.SplitRight} {
		t.Run(name, func(t *testing.T) {
			h := newTestHarness(t, 100, 30)
			defer h.stop()
			h.app.ContentSplit.Position = position
			h.app.Settings.Terminal.Shell = "/bin/cat"
			h.exec("terminal.toggle")
			if len(h.app.Terminals) != 1 {
				t.Fatal("terminal did not open")
			}
			tab := h.app.Terminals[0]
			defer h.app.CloseTerminal(tab.ID)
			r := tab.Widget.GetRect()
			for _, x := range []int{r.X - 1, r.X + r.W} {
				h.app.FocusEditor()
				h.redraw()
				h.click(x, r.Y)
				if h.app.Root.Focused != h.app.TerminalPanel || !tab.Widget.WantsRawKeys() {
					t.Fatalf("padding click at (%d,%d) did not focus terminal", x, r.Y)
				}
				if tab.Widget.HasSelection() {
					t.Fatal("padding click started a selection")
				}
			}
		})
	}
}
