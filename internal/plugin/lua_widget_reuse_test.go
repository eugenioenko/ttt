package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

func TestReusedStacksTakeTheNewLayout(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	err := p.State.DoString(`
		local ttt = require("ttt")
		page = 1
		ttt.register({ sidebar = { title = "T", render = function(panel)
			if page == 1 then
				panel:hstack({ height = 1, gap = 2, render = function(h) h:label("a") end })
				panel:box({ height = 3, render = function(b) b:label("b") end })
			else
				panel:hstack({ render = function(h) h:label("a") end })
				panel:box({ render = function(b) b:label("b") end })
			end
		end } })
	`)
	if err != nil {
		t.Fatal(err)
	}
	ws := NewWidgetState()
	render := func() *widgets.VStackWidget {
		proxy := NewPanelProxy(newMockSurface(80, 20), p)
		if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
			t.Fatal(err)
		}
		return ws.Reconcile(proxy.Descriptors(), p)
	}
	render()
	p.State.SetGlobal("page", lua.LNumber(2))
	root := render()
	hs := root.Children[0].(*widgets.HStackWidget)
	if hs.FixedHeight != 0 || hs.Gap != 0 {
		t.Fatalf("reused hstack kept height %d gap %d", hs.FixedHeight, hs.Gap)
	}
	if box := root.Children[1].(*widgets.BoxWidget); box.FixedHeight != 0 {
		t.Fatalf("reused box kept height %d", box.FixedHeight)
	}
}

func TestLuaButtonKeepsDefaultPadding(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.register({ sidebar = { title = "T", render = function(panel)
			panel:button({ label = "New", margin_top = 1 })
			panel:button({ label = "Flat", padding_left = 0 })
		end } })
	`)
	if err != nil {
		t.Fatal(err)
	}
	proxy := NewPanelProxy(newMockSurface(80, 20), p)
	if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
		t.Fatal(err)
	}
	root := NewWidgetState().Reconcile(proxy.Descriptors(), p)
	padded := root.Children[0].(*widgets.ButtonWidget)
	if padded.Box.PaddingLeft != 1 || padded.Box.PaddingRight != 1 || padded.Box.MarginTop != 1 {
		t.Errorf("default button box = %+v, want 1 cell padding and the margin", padded.Box)
	}
	if flat := root.Children[1].(*widgets.ButtonWidget); flat.Box.PaddingLeft != 0 || flat.Box.PaddingRight != 0 {
		t.Errorf("explicit padding_left should win: %+v", flat.Box)
	}
}
