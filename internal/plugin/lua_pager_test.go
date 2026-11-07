package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

func TestLuaPager(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	err := p.State.DoString(`
		local ttt = require("ttt")
		offset, paged = 20, nil
		ttt.register({ sidebar = { title = "T", render = function(panel)
			panel:pager({ offset = offset, limit = 20, total = 85, on_change = function(o) paged = o end })
		end } })
	`)
	if err != nil {
		t.Fatal(err)
	}
	ws := NewWidgetState()
	render := func() *widgets.PagerWidget {
		proxy := NewPanelProxy(newMockSurface(80, 20), p)
		if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
			t.Fatal(err)
		}
		return ws.Reconcile(proxy.Descriptors(), p).Children[0].(*widgets.PagerWidget)
	}

	pager := render()
	if pager.Config.Offset != 20 || pager.Config.Limit != 20 || pager.Config.Total != 85 {
		t.Fatalf("pager config = %+v", pager.Config)
	}
	pager.Config.OnChange(40)
	if v := p.State.GetGlobal("paged"); v != lua.LNumber(40) {
		t.Fatalf("on_change got %v", v)
	}

	p.State.SetGlobal("offset", lua.LNumber(60))
	if again := render(); again != pager || again.Config.Offset != 60 {
		t.Fatalf("reconcile should reuse the pager and take the new offset, got %+v", again.Config)
	}
}
