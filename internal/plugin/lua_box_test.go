package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

func TestLuaBoxTitleAndWidth(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	err := p.State.DoString(`
		local ttt = require("ttt")
		title = "Name"
		ttt.register({ sidebar = { title = "T", render = function(panel)
			panel:box({ border = true, title = title, width = 20, render = function(b) b:label("x") end })
		end } })
	`)
	if err != nil {
		t.Fatal(err)
	}
	ws := NewWidgetState()
	render := func() *widgets.BoxWidget {
		proxy := NewPanelProxy(newMockSurface(80, 20), p)
		if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
			t.Fatal(err)
		}
		return ws.Reconcile(proxy.Descriptors(), p).Children[0].(*widgets.BoxWidget)
	}
	if box := render(); box.Title != "Name" || box.Width() != 20 {
		t.Fatalf("title %q, width %d", box.Title, box.Width())
	}
	p.State.SetGlobal("title", lua.LString("Value"))
	if box := render(); box.Title != "Value" {
		t.Fatalf("reconciled title = %q", box.Title)
	}
}
