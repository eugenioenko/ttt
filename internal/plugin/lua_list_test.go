package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

func TestLuaTreeAndListBadgeRightAndInset(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	err := p.State.DoString(`
		local ttt = require("ttt")
		inset = 1
		ttt.register({ sidebar = { title = "T", render = function(panel)
			local items = {{ id = "a", label = "Namespaces", badge = "71" }}
			panel:tree({ items = items, badge_right = true, inset = inset })
			panel:list({ items = items, badge_right = true, inset = inset })
		end } })
	`)
	if err != nil {
		t.Fatal(err)
	}
	ws := NewWidgetState()
	render := func() []widgets.Widget {
		proxy := NewPanelProxy(newMockSurface(80, 20), p)
		if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
			t.Fatal(err)
		}
		return ws.Reconcile(proxy.Descriptors(), p).Children
	}
	for i, w := range render() {
		if tw := w.(*widgets.TreeWidget); !tw.Config.BadgeRight || tw.Config.Inset != 1 {
			t.Errorf("widget %d: badge_right %v inset %d", i, tw.Config.BadgeRight, tw.Config.Inset)
		}
	}
	p.State.SetGlobal("inset", lua.LNumber(2))
	for i, w := range render() {
		if tw := w.(*widgets.TreeWidget); tw.Config.Inset != 2 {
			t.Errorf("widget %d: reconciled inset %d, want 2", i, tw.Config.Inset)
		}
	}
}
