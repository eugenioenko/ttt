package plugin

import (
	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

type PluginDialogWidget struct {
	*widgets.DialogWidget
	Panel  *PluginPanelWidget
	height int
}

func (d *PluginDialogWidget) Render(surface widgets.Surface) {
	_, h := surface.Size()
	d.Panel.height = max(1, min(d.height, h-6))
	d.DialogWidget.Render(surface)
}

func setupDialogFunctions(L *lua.LState, mod *lua.LTable, p *Plugin) {
	L.SetField(mod, "open_dialog", L.NewFunction(func(L *lua.LState) int {
		cfg := L.CheckTable(1)
		render, ok := L.GetField(cfg, "render").(*lua.LFunction)
		if !ok {
			L.ArgError(1, "render function required")
			return 0
		}
		width, height := 70, 12
		if n, ok := L.GetField(cfg, "width").(lua.LNumber); ok {
			width = int(n)
		}
		if n, ok := L.GetField(cfg, "height").(lua.LNumber); ok {
			height = int(n)
		}
		if width < 8 || height < 1 {
			L.ArgError(1, "width must be at least 8 and height must be positive")
			return 0
		}
		event, _ := L.GetField(cfg, "on_event").(*lua.LFunction)
		panel := NewPluginPanelWidget(p, render, event)
		dialog := widgets.NewDialogWidget(width)
		if title := L.GetField(cfg, "title"); title != lua.LNil {
			dialog.Title = title.String()
		}
		dialog.SetContent(panel)
		var onClose func()
		if fn, ok := L.GetField(cfg, "on_close").(*lua.LFunction); ok {
			onClose = func() { p.CallLuaFunc(fn) }
		}
		shown := p.OpenDialog != nil && p.OpenDialog(&PluginDialogWidget{DialogWidget: dialog, Panel: panel, height: height}, onClose)
		L.Push(lua.LBool(shown))
		return 1
	}))
	L.SetField(mod, "close_dialog", L.NewFunction(func(L *lua.LState) int {
		if p.CloseDialog != nil {
			p.CloseDialog()
		}
		return 0
	}))
}
