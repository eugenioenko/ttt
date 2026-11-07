package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

func TestLuaHStackAlign(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	err := p.State.DoString(`
		local ttt = require("ttt")
		align = "right"
		ttt.register({ sidebar = { title = "T", render = function(panel)
			panel:hstack({ height = 1, align = align, render = function(h) h:button({ label = "OK" }) end })
		end } })
	`)
	if err != nil {
		t.Fatal(err)
	}
	ws := NewWidgetState()
	render := func() *widgets.HStackWidget {
		proxy := NewPanelProxy(newMockSurface(80, 20), p)
		if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
			t.Fatal(err)
		}
		return ws.Reconcile(proxy.Descriptors(), p).Children[0].(*widgets.HStackWidget)
	}
	if hs := render(); hs.Align != "right" {
		t.Fatalf("align = %q, want right", hs.Align)
	}
	p.State.SetGlobal("align", lua.LString("center"))
	if hs := render(); hs.Align != "center" {
		t.Fatalf("reconciled align = %q, want center", hs.Align)
	}
}
