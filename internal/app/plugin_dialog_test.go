package app

import (
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
	"testing"
)

func TestPluginDialogInputFocusAndCleanup(t *testing.T) {
	a := prepareListenTestApp(t)
	p := &plugin.Plugin{Name: "dialog-test", Enabled: true}
	if err := p.InitFromSource(`
local ttt = require("ttt")
closed = 0
function show()
  return ttt.open_dialog({title = "Review", height = 8,
    on_close = function() closed = closed + 1 end,
    render = function(panel)
      panel:input({autofocus = true, wrap = true})
      panel:button({label = "Close", on_click = function() ttt.close_dialog() end})
    end})
end
`); err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	a.wirePluginDialogs(p)
	previous := a.Root.Focused
	if err := p.State.DoString(`assert(show()); assert(not show())`); err != nil {
		t.Fatal(err)
	}
	grid := make([][]term.Cell, 24)
	for y := range grid {
		grid[y] = make([]term.Cell, 80)
	}
	a.Root.Render(grid)
	if a.focusedWidgetInput() == nil {
		t.Fatal("dialog input did not receive focus")
	}
	a.PasteText("review\nmessage")
	if got := a.focusedWidgetInput().Text(); got != "review\nmessage" {
		t.Fatalf("pasted text = %q", got)
	}
	other := &plugin.Plugin{Name: "other"}
	a.wirePluginDialogs(other)
	other.CloseDialog()
	if !a.Root.HasOverlay() {
		t.Fatal("another plugin closed the dialog")
	}
	a.Root.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, "", 0))
	if a.Root.HasOverlay() || a.Root.Focused != previous || p.State.GetGlobal("closed").String() != "1" {
		t.Fatal("Escape did not close, notify, and restore focus")
	}
	if err := p.State.DoString(`assert(show())`); err != nil {
		t.Fatal(err)
	}
	a.RemovePluginPanels(p.Name)
	if a.Root.HasOverlay() || a.pluginDialog != nil || p.State.GetGlobal("closed").String() != "1" {
		t.Fatal("plugin teardown left a dialog or ran its close callback")
	}
}
