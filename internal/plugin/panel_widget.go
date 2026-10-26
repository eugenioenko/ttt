package plugin

import (
	"github.com/eugenioenko/ttt/internal/core/clipboard"
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
	height     int
	// autofocus asks the next render to focus the autofocus input, when
	// FocusAutofocusInput ran before the input existed.
	autofocus bool
	// OnClose runs once, when the editor tab holding the panel closes.
	OnClose func()
	sel     panelSelection
	fileKey string
}

func NewPluginPanelWidget(p *Plugin, renderFunc, eventFunc *lua.LFunction) *PluginPanelWidget {
	return &PluginPanelWidget{
		plugin:     p,
		renderFunc: renderFunc,
		eventFunc:  eventFunc,
		state:      NewWidgetState(),
	}
}

func (pw *PluginPanelWidget) Height() int { return pw.height }
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
		pw.sel.snapshot(surface)
		pw.sel.highlight(surface)
		return
	}

	if proxy.UsedWidgets() {
		root := pw.state.Reconcile(proxy.Descriptors(), pw.plugin)
		if pw.autofocus {
			pw.autofocus = false
			if pw.focused {
				pw.focusInput()
			}
		}
		r := pw.GetRect()
		root.SetRect(widgets.Rect{X: r.X, Y: r.Y, W: w, H: h})
		root.Render(surface)
	}

	pw.sel.snapshot(surface)
	pw.sel.highlight(surface)
}

func (pw *PluginPanelWidget) HandleEvent(ev tcell.Event) widgets.EventResult {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		mx, my := e.Position()
		owns := pw.state != nil && pw.state.root != nil && ownsDragAt(pw.state.root, mx, my)
		if result, ok := pw.sel.track(e, pw.GetRect(), owns); ok {
			return result
		}
		result := pw.dispatch(ev)
		if result == widgets.EventCaptured {
			pw.sel.blocked = true
		}
		return result
	case *tcell.EventKey:
		if isCopyKey(e) && pw.CopyTextSelection() {
			return widgets.EventConsumed
		}
		if e.Key() == tcell.KeyEscape && pw.sel.clear() {
			return widgets.EventConsumed
		}
	}
	return pw.dispatch(ev)
}

// CopyTextSelection copies the selected text and reports whether there was any.
// A focused input with its own selection wins, so the copy goes to it.
func (pw *PluginPanelWidget) CopyTextSelection() bool {
	if !pw.sel.active {
		return false
	}
	if inp := pw.FocusedInput(); inp != nil && inp.HasSelection() {
		return false
	}
	clipboard.Set(pw.sel.text())
	pw.sel.clear()
	return true
}

// SelectAllText selects the focused input's text, or else the panel's.
func (pw *PluginPanelWidget) SelectAllText() {
	if inp := pw.FocusedInput(); inp != nil {
		inp.SelectAll()
		return
	}
	pw.sel.selectAll()
}

func (pw *PluginPanelWidget) CancelPointerCapture() bool { return pw.sel.release() }

func (pw *PluginPanelWidget) dispatch(ev tcell.Event) widgets.EventResult {
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

// FocusedInput returns the input focused inside the panel, or nil, so pastes
// reach it.
func (pw *PluginPanelWidget) FocusedInput() *widgets.InputWidget {
	if pw.state == nil || pw.state.focus == nil {
		return nil
	}
	inp, _ := pw.state.focus.Focused().(*widgets.InputWidget)
	return inp
}

func (pw *PluginPanelWidget) Focusable() bool { return true }
func (pw *PluginPanelWidget) SetFocused(focused bool) {
	pw.focused = focused
	if !focused {
		pw.autofocus = false
		pw.sel.release()
	}
	if pw.state != nil && pw.state.focus != nil {
		pw.state.focus.SetActive(focused)
	}
}

// FocusAutofocusInput moves the panel's focus to its first input with
// autofocus set. Without one it tries once more on the next render, which
// shows an input the same plugin callback has just added.
func (pw *PluginPanelWidget) FocusAutofocusInput() {
	if !pw.focusInput() {
		pw.autofocus = true
	}
}

func (pw *PluginPanelWidget) focusInput() bool {
	for _, item := range pw.state.focus.Items() {
		if iw, ok := item.(*widgets.InputWidget); ok && iw.Config.Autofocus {
			return pw.state.focus.Focus(iw)
		}
	}
	return false
}
func (pw *PluginPanelWidget) IsFocused() bool { return pw.focused }

func (pw *PluginPanelWidget) PluginName() string { return pw.plugin.Name }

// PluginTabKey is the tab key of a panel a file handler opened; the editor
// group keys the file's tab with it so ttt.close_tab and ttt.focus_tab find it.
func (pw *PluginPanelWidget) PluginTabKey() string { return pw.fileKey }

func (pw *PluginPanelWidget) Close() {
	if pw.state != nil && pw.state.root != nil {
		closeImages(pw.state.root)
	}
	if onClose := pw.OnClose; onClose != nil {
		pw.OnClose = nil
		onClose()
	}
}
