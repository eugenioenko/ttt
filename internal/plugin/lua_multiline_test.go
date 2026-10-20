package plugin

import "testing"

func TestPluginMultilineInputKeepsTextAcrossRedraws(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer cleanup()
	if err := p.State.DoString(`
local ttt = require("ttt")
ttt.register({sidebar = {title = "Review", render = function(panel)
  panel:input({multiline = true, min_rows = 4, max_rows = 6})
end}})
`); err != nil {
		t.Fatal(err)
	}
	proxy := NewPanelProxy(newMockSurface(40, 10), p)
	if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
		t.Fatal(err)
	}
	desc := proxy.Descriptors()[0]
	input := createInputWidget(desc, p)
	input.SetText("draft\nmessage")
	input.SetFocused(true)
	updateWidget(input, desc, p)
	if !input.Config.Multiline || !input.Config.Wrap || input.HeightAtWidth(40) != 4 || input.Text() != "draft\nmessage" || !input.IsFocused() {
		t.Fatal("multiline options or draft state were lost during reconciliation")
	}
}
