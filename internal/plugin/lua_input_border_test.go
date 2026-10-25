package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

func TestLuaInputBorderTitleAndPassword(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	err := p.State.DoString(`
		local ttt = require("ttt")
		bordered = true
		ttt.register({ sidebar = { title = "T", render = function(panel)
			panel:input({ border = bordered, title = "Secret", password = true })
		end } })
	`)
	if err != nil {
		t.Fatal(err)
	}
	ws := NewWidgetState()
	render := func() *widgets.InputWidget {
		proxy := NewPanelProxy(newMockSurface(80, 20), p)
		if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
			t.Fatal(err)
		}
		return ws.Reconcile(proxy.Descriptors(), p).Children[0].(*widgets.InputWidget)
	}
	inp := render()
	if !inp.Config.Bordered || inp.Config.Title != "Secret" || !inp.Config.Mask || inp.Height() != 3 {
		t.Fatalf("config = %+v, height %d", inp.Config, inp.Height())
	}
	p.State.SetGlobal("bordered", lua.LFalse)
	if inp = render(); inp.Config.Bordered || inp.Config.Prefix != widgets.DefaultPrefix {
		t.Fatalf("dropping the border should restore the default prefix: %+v", inp.Config)
	}
}
