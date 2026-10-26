package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
	lua "github.com/yuin/gopher-lua"
)

func TestLuaButtonDisabled(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	err := p.State.DoString(`
		local ttt = require("ttt")
		busy, clicks = true, 0
		ttt.register({ sidebar = { title = "T", render = function(panel)
			panel:button({ label = "Save", disabled = busy, on_click = function() clicks = clicks + 1 end })
		end } })
	`)
	if err != nil {
		t.Fatal(err)
	}
	ws := NewWidgetState()
	render := func() *widgets.ButtonWidget {
		proxy := NewPanelProxy(newMockSurface(80, 20), p)
		if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
			t.Fatal(err)
		}
		return ws.Reconcile(proxy.Descriptors(), p).Children[0].(*widgets.ButtonWidget)
	}
	enter := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)

	btn := render()
	btn.SetFocused(true)
	btn.HandleEvent(enter)
	if v := p.State.GetGlobal("clicks"); v != lua.LNumber(0) {
		t.Fatalf("disabled button ran on_click: clicks = %v", v)
	}
	p.State.SetGlobal("busy", lua.LFalse)
	btn = render()
	btn.SetFocused(true)
	btn.HandleEvent(enter)
	if v := p.State.GetGlobal("clicks"); v != lua.LNumber(1) {
		t.Fatalf("re-enabled button: clicks = %v", v)
	}
}
