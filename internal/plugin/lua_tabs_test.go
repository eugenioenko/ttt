package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

func TestLuaTabs(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	err := p.State.DoString(`
		local ttt = require("ttt")
		active, picked = "keys", {}
		ttt.register({ sidebar = { title = "T", render = function(panel)
			panel:tabs({
				items = {{ id = "details", label = "Details" }, { id = "keys", label = "Keys" }},
				active = active,
				on_change = function(id) picked[#picked + 1] = id end,
			})
		end } })
	`)
	if err != nil {
		t.Fatal(err)
	}
	ws := NewWidgetState()
	render := func() *widgets.TabsWidget {
		proxy := NewPanelProxy(newMockSurface(80, 20), p)
		if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
			t.Fatal(err)
		}
		return ws.Reconcile(proxy.Descriptors(), p).Children[0].(*widgets.TabsWidget)
	}

	tabs := render()
	if tabs.ActiveID() != "keys" || tabs.Config.Items[0].Label != "Details" {
		t.Fatalf("active = %q, items = %+v", tabs.ActiveID(), tabs.Config.Items)
	}
	tabs.Config.OnTabClick(1)
	tabs.Config.OnTabClick(0)
	picked := p.State.GetGlobal("picked").(*lua.LTable)
	if picked.Len() != 1 || picked.RawGetInt(1).String() != "details" {
		t.Fatalf("on_change should get only the newly picked tab, got %d calls", picked.Len())
	}

	p.State.SetGlobal("active", lua.LNil)
	if again := render(); again != tabs || again.ActiveID() != "details" {
		t.Fatalf("without active the first tab is active, got %q", again.ActiveID())
	}
}
