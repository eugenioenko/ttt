package plugin

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
	lua "github.com/yuin/gopher-lua"
)

type PluginPanelWidget struct {
	widgets.BaseWidget
	plugin     *Plugin
	renderFunc *lua.LFunction
	eventFunc  *lua.LFunction
	state      *WidgetState
	focused    bool
}

func NewPluginPanelWidget(p *Plugin, renderFunc, eventFunc *lua.LFunction) *PluginPanelWidget {
	return &PluginPanelWidget{
		plugin:     p,
		renderFunc: renderFunc,
		eventFunc:  eventFunc,
		state:      NewWidgetState(),
	}
}

func (pw *PluginPanelWidget) Height() int { return 0 }
func (pw *PluginPanelWidget) Width() int  { return 0 }

func (pw *PluginPanelWidget) Render(surface widgets.Surface) {
	w, h := surface.Size()
	if w <= 0 || h <= 0 {
		return
	}

	surface.Fill(term.Cell{Ch: ' ', Style: term.StyleDefault})

	proxy := NewPanelProxy(surface, pw.plugin)
	if err := pw.plugin.CallRenderWith(pw.renderFunc, proxy); err != nil {
		msg := "Plugin error: " + err.Error()
		surface.DrawText(0, 0, msg, w, term.StyleDanger)
		return
	}

	if proxy.UsedWidgets() {
		root := pw.state.Reconcile(proxy.Descriptors(), pw.plugin)
		r := pw.GetRect()
		root.SetRect(widgets.Rect{X: r.X, Y: r.Y, W: w, H: h})
		root.Render(surface)
	}
}

func (pw *PluginPanelWidget) HandleEvent(ev tcell.Event) widgets.EventResult {
	if pw.state != nil && pw.state.focus != nil {
		if result := pw.state.focus.HandleEvent(ev); result != widgets.EventIgnored {
			return result
		}
	}

	if pw.state != nil && pw.state.root != nil {
		if result := pw.state.root.HandleEvent(ev); result != widgets.EventIgnored {
			return result
		}
	}

	if pw.eventFunc != nil && pw.plugin.State != nil {
		tbl := eventToLua(pw.plugin.State, ev)
		if tbl != nil {
			pw.plugin.CallLuaFunc(pw.eventFunc, tbl)
		}
	}

	return widgets.EventIgnored
}

func (pw *PluginPanelWidget) CursorPosition() (int, int, bool) {
	if pw.state != nil && pw.state.focus != nil {
		if fw := pw.state.focus.Focused(); fw != nil {
			if cp, ok := fw.(widgets.CursorPositioner); ok {
				return cp.CursorPosition()
			}
		}
	}
	return 0, 0, false
}

func (pw *PluginPanelWidget) Focusable() bool { return true }
func (pw *PluginPanelWidget) SetFocused(focused bool) {
	pw.focused = focused
	if pw.state != nil && pw.state.focus != nil {
		pw.state.focus.SetActive(focused)
	}
}
func (pw *PluginPanelWidget) IsFocused() bool { return pw.focused }

func (pw *PluginPanelWidget) PluginName() string { return pw.plugin.Name }

func (pw *PluginPanelWidget) Close() {
	if pw.state != nil && pw.state.root != nil {
		closeImages(pw.state.root)
	}
}
