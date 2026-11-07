package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
	lua "github.com/yuin/gopher-lua"
)

func TestLuaListOnSelectFiresWhenTheSelectedItemIsOpenedAgain(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	err := p.State.DoString(`
		local ttt = require("ttt")
		opened = 0
		ttt.register({ sidebar = { title = "T", render = function(panel)
			panel:list({ items = {{ id = "a", label = "Alpha" }}, on_select = function() opened = opened + 1 end })
		end } })
	`)
	if err != nil {
		t.Fatal(err)
	}
	proxy := NewPanelProxy(newMockSurface(40, 5), p)
	if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
		t.Fatal(err)
	}
	list := NewWidgetState().Reconcile(proxy.Descriptors(), p).Children[0].(*widgets.TreeWidget)
	list.SetFocused(true)
	list.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if v := p.State.GetGlobal("opened"); v != lua.LNumber(1) {
		t.Fatalf("Enter on the selected item opened %v times, want 1", v)
	}
}
