package plugin

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
)

func newChatPanel(t *testing.T) *PluginPanelWidget {
	t.Helper()
	p := &Plugin{Name: "chat", Granted: PermissionSet{PanelBottom: true}}
	p.State = NewSandbox()
	t.Cleanup(p.State.Close)
	setupTTTModule(p.State, p)
	err := p.State.DoString(`
		require("ttt").register({
			bottom = {
				title = "Chat",
				render = function(panel)
					panel:markdown({ text = "hello", follow = true })
					panel:input({ placeholder = "say", autofocus = true })
				end,
			},
		})
	`)
	if err != nil {
		t.Fatal(err)
	}
	return NewPluginPanelWidget(p, p.BottomRenderFunc, nil)
}

func TestFocusAutofocusInputFindsTheInput(t *testing.T) {
	pw := newChatPanel(t)
	pw.Render(newMockSurface(40, 10))
	pw.SetFocused(true)
	if _, ok := pw.state.focus.Focused().(*widgets.InputWidget); ok {
		t.Fatal("gaining focus alone moved it to the autofocus input")
	}
	pw.FocusAutofocusInput()
	if _, ok := pw.state.focus.Focused().(*widgets.InputWidget); !ok {
		t.Fatalf("focused %T, want the autofocus input", pw.state.focus.Focused())
	}
	if sv, ok := pw.state.items[0].(*widgets.ScrollViewWidget); !ok || !sv.Follow {
		t.Fatal("markdown follow was not applied")
	}
}

func TestFocusAutofocusInputWaitsForFirstRender(t *testing.T) {
	pw := newChatPanel(t)
	pw.SetFocused(true)
	pw.FocusAutofocusInput()
	pw.Render(newMockSurface(40, 10))
	if _, ok := pw.state.focus.Focused().(*widgets.InputWidget); !ok {
		t.Fatalf("focused %T, want the autofocus input", pw.state.focus.Focused())
	}
}

func TestShowAndFocusPanel(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{PanelBottom: true})
	defer cleanup()
	type call struct {
		area  string
		focus bool
	}
	var calls []call
	p.ShowPanel = func(area string, focus bool) (bool, error) {
		calls = append(calls, call{area, focus})
		return !focus, nil
	}
	err := p.State.DoString(`
		local ttt = require("ttt")
		shown = ttt.show_panel("bottom")
		focused = ttt.focus_panel("bottom")
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != (call{"bottom", false}) || calls[1] != (call{"bottom", true}) {
		t.Fatalf("calls = %v", calls)
	}
	if p.State.GetGlobal("shown").String() != "true" || p.State.GetGlobal("focused").String() != "false" {
		t.Fatal("show_panel and focus_panel did not return ShowPanel's result")
	}
}

func TestShowPanelRejectsBadAreaAndMissingPermission(t *testing.T) {
	for script, want := range map[string]string{
		`require("ttt").show_panel("left")`:     `area must be "bottom" or "sidebar"`,
		`require("ttt").focus_panel("sidebar")`: "panel.sidebar permission not granted",
	} {
		p, cleanup := newTestPluginBase(PermissionSet{PanelBottom: true})
		p.ShowPanel = func(string, bool) (bool, error) { t.Fatal("ShowPanel called"); return false, nil }
		err := p.State.DoString(script)
		cleanup()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", script, err, want)
		}
	}
}
