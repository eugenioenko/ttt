package plugin

import "testing"

func TestControlledInputValueUpdatesWithoutChangeCallback(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer cleanup()
	if err := p.State.DoString(`
local ttt = require("ttt")
url = "example/first"
changes = 0
ttt.register({sidebar = {title = "Repository", render = function(panel)
  panel:input({value = url, on_change = function(text) changes = changes + 1; url = text end})
end}})
`); err != nil {
		t.Fatal(err)
	}
	render := func() WidgetDesc {
		t.Helper()
		proxy := NewPanelProxy(newMockSurface(40, 10), p)
		if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
			t.Fatal(err)
		}
		return proxy.Descriptors()[0]
	}
	input := createInputWidget(render(), p)
	if input.Text() != "example/first" {
		t.Fatal("initial input value was not applied")
	}
	input.SetText("example/typed")
	if p.State.GetGlobal("changes").String() != "1" {
		t.Fatal("typing did not notify Lua")
	}
	input.SetFocused(true)
	if err := p.State.DoString(`url = "example/saved"`); err != nil {
		t.Fatal(err)
	}
	updateWidget(input, render(), p)
	if input.Text() != "example/saved" || !input.IsFocused() || p.State.GetGlobal("changes").String() != "1" {
		t.Fatal("controlled update notified Lua or lost focus")
	}
	if err := p.State.DoString(`url = ""`); err != nil {
		t.Fatal(err)
	}
	updateWidget(input, render(), p)
	if input.Text() != "" {
		t.Fatal("empty controlled value did not clear the input")
	}
	input.SetText("keep this")
	updateWidget(input, WidgetDesc{Kind: WidgetInput}, p)
	if input.Text() != "keep this" {
		t.Fatal("omitted value cleared an uncontrolled input")
	}
}

func TestControlledInputIgnoresUnsupportedValueTypes(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer cleanup()
	if err := p.State.DoString(`
local ttt=require("ttt")
value=true
ttt.register({sidebar={title="Input",render=function(panel) panel:input({value=value}) end}})
`); err != nil {
		t.Fatal(err)
	}
	input := createInputWidget(WidgetDesc{}, p)
	input.SetText("preserve this")
	for _, value := range []string{"true", "{}", "function() end", "nil", "42", "\"\""} {
		if err := p.State.DoString("value=" + value); err != nil {
			t.Fatal(err)
		}
		proxy := NewPanelProxy(newMockSurface(40, 10), p)
		if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
			t.Fatal(err)
		}
		updateWidget(input, proxy.Descriptors()[0], p)
		want := "preserve this"
		if value == "42" {
			want = "42"
		} else if value == "\"\"" {
			want = ""
		}
		if input.Text() != want {
			t.Fatalf("value=%s: text=%q, want %q", value, input.Text(), want)
		}
	}
}
