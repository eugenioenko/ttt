package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

func TestLuaButtonStyle(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	err := p.State.DoString(`
		local ttt = require("ttt")
		style = nil
		ttt.register({ sidebar = { title = "T", render = function(panel)
			panel:button({ label = "Save", style = style })
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

	if got := render().Config.Variant; got != widgets.ButtonFilled {
		t.Fatalf("default variant = %d, want filled", got)
	}
	for name, want := range buttonVariants {
		p.State.SetGlobal("style", lua.LString(name))
		if got := render().Config.Variant; got != want {
			t.Errorf("style %q: variant %d, want %d", name, got, want)
		}
	}
	p.State.SetGlobal("style", lua.LString("shiny"))
	if got := render().Config.Variant; got != widgets.ButtonFilled {
		t.Errorf("an unknown style should fall back to filled, got %d", got)
	}
}

func TestLuaButtonSpansReconcile(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	if err := p.State.DoString(`
		local ttt = require("ttt")
		color = "success"
		ttt.register({sidebar = {title = "T", render = function(panel)
			panel:button({label = "CI", spans = {{text = "✓1", style = color}}})
		end}})
	`); err != nil {
		t.Fatal(err)
	}
	ws := NewWidgetState()
	for _, tc := range []struct {
		name  string
		style term.Style
	}{{"success", term.StyleSuccess}, {"danger", term.StyleDanger}} {
		p.State.SetGlobal("color", lua.LString(tc.name))
		proxy := NewPanelProxy(newMockSurface(80, 20), p)
		if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
			t.Fatal(err)
		}
		button := ws.Reconcile(proxy.Descriptors(), p).Children[0].(*widgets.ButtonWidget)
		if len(button.Config.Spans) != 1 || button.Config.Spans[0].Text != "✓1" || button.Config.Spans[0].Style != tc.style {
			t.Fatalf("spans not refreshed: %+v", button.Config.Spans)
		}
	}
}
