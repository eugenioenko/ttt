package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestKeyValueMutedKeysDrawsValuesInItemStyle(t *testing.T) {
	p := &Plugin{Name: "test", Granted: PermissionSet{PanelSidebar: true}}
	p.State = NewSandbox()
	defer p.State.Close()
	setupTTTModule(p.State, p)
	err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.register({
			sidebar = {
				title = "Test",
				render = function(panel)
					panel:keyvalue({ muted_keys = true, { key = "Name", value = "acme" } })
				end,
			},
		})
	`)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	pw := NewPluginPanelWidget(p, p.RenderFunc, p.EventFunc)
	surface := newMockSurface(30, 2)
	pw.Render(surface)

	styles := map[rune]term.Style{}
	for _, text := range surface.texts {
		styles[[]rune(text.text)[0]] = text.style
	}
	if styles['N'] != term.StyleMuted {
		t.Errorf("key style = %v, want muted", styles['N'])
	}
	if got, ok := styles['a']; !ok || got != term.StylePaletteItem {
		t.Errorf("value style = %v (drawn %v), want the item style", got, ok)
	}
}
