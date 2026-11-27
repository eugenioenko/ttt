package e2e

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func TestInputActionPressRoutesThroughOwners(t *testing.T) {
	for _, command := range []string{"search.find", "search.replace", "sidebar.search"} {
		t.Run(command, func(t *testing.T) {
			h := newTestHarness(t, 120, 30)
			defer h.stop()
			h.exec("file.new")
			h.exec(command)
			var input *widgets.InputWidget
			var options *ui.SearchOptions
			switch bar := h.app.Root.TopOverlayWidget().(type) {
			case *ui.FindBarWidget:
				input, options = bar.Input, &bar.Options
			case *ui.ReplaceBarWidget:
				input, options = bar.SearchInput, &bar.Options
			default:
				input, options = h.app.Search.Input, &h.app.Search.Options
			}
			hit := input.ActionHits[0]
			for _, x := range []int{hit.X, hit.X + 1} {
				h.app.Root.HandleEvent(tcell.NewEventMouse(x, hit.Y, tcell.Button1, tcell.ModNone))
				h.redraw()
			}
			if !options.CaseSensitive || !h.app.Root.PointerCaptureActive() {
				t.Fatal("held action did not keep one toggle and capture the press")
			}
			h.app.Root.HandleEvent(tcell.NewEventMouse(0, 29, tcell.ButtonNone, tcell.ModNone))
			if h.app.Root.PointerCaptureActive() {
				t.Fatal("release outside input kept the action captured")
			}
			h.click(hit.X, hit.Y)
			if options.CaseSensitive || editorText(h) != "" {
				t.Fatal("next action press failed or changed the editor")
			}
		})
	}
}
