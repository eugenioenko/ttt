package plugin

import (
	"github.com/gdamore/tcell/v3"
	lua "github.com/yuin/gopher-lua"
)

// CommandLineOptions is what a plugin asked the command line to be. Grouped
// rather than passed positionally because the set grows: on_key was added for
// incremental search, and completion candidates are the obvious next one.
type CommandLineOptions struct {
	Prefix   string
	Text     string
	OnChange func(text string)
	OnSubmit func(text string)
	OnCancel func()

	// OnKey is offered every key before the input handles it, so a plugin can
	// drive the prompt live. Returning true consumes the key.
	OnKey func(ev *tcell.EventKey) bool
}

// newCommandLineModule builds the `ttt.command_line` table.
//
// Every entry is gated on the `keybindings` permission: opening the command
// line is a keyboard-focus grab, the same class of capability as claiming a
// key binding.
func newCommandLineModule(L *lua.LState, p *Plugin) *lua.LTable {
	mod := L.NewTable()

	L.SetField(mod, "show", L.NewFunction(func(L *lua.LState) int {
		if err := p.Granted.Check("keybindings"); err != nil {
			L.ArgError(1, "keybindings permission not granted")
			return 0
		}
		opts := L.OptTable(1, L.NewTable())

		prefix := ":"
		if v := L.GetField(opts, "prefix"); v != lua.LNil {
			prefix = v.String()
		}
		text := ""
		if v := L.GetField(opts, "text"); v != lua.LNil {
			text = v.String()
		}

		onChange := luaTextCallback(p, L.GetField(opts, "on_change"), "command_line.on_change")
		onSubmit := luaTextCallback(p, L.GetField(opts, "on_submit"), "command_line.on_submit")

		var onCancel func()
		if fn, ok := L.GetField(opts, "on_cancel").(*lua.LFunction); ok {
			onCancel = func() {
				if err := p.CallLuaFunc(fn); err != nil {
					p.logError("command_line.on_cancel", err)
				}
			}
		}

		var onKey func(*tcell.EventKey) bool
		if fn, ok := L.GetField(opts, "on_key").(*lua.LFunction); ok {
			onKey = func(ev *tcell.EventKey) bool {
				if p.State == nil {
					return false
				}
				// Same event shape as the key.press listener, so a plugin's
				// existing key normalisation works unchanged here.
				err := p.State.CallByParam(lua.P{
					Fn:      fn,
					NRet:    1,
					Protect: true,
				}, keyEventToLua(p.State, ev))
				if err != nil {
					p.logError("command_line.on_key", err)
					return false
				}
				ret := p.State.Get(-1)
				p.State.Pop(1)
				return lua.LVAsBool(ret)
			}
		}

		if p.ShowCommandLine != nil {
			p.ShowCommandLine(CommandLineOptions{
				Prefix:   prefix,
				Text:     text,
				OnChange: onChange,
				OnSubmit: onSubmit,
				OnCancel: onCancel,
				OnKey:    onKey,
			})
		}
		return 0
	}))

	L.SetField(mod, "hide", L.NewFunction(func(L *lua.LState) int {
		if err := p.Granted.Check("keybindings"); err != nil {
			L.ArgError(1, "keybindings permission not granted")
			return 0
		}
		if p.HideCommandLine != nil {
			p.HideCommandLine()
		}
		return 0
	}))

	L.SetField(mod, "set_text", L.NewFunction(func(L *lua.LState) int {
		if err := p.Granted.Check("keybindings"); err != nil {
			L.ArgError(1, "keybindings permission not granted")
			return 0
		}
		text := L.CheckString(1)
		if p.SetCommandLineText != nil {
			p.SetCommandLineText(text)
		}
		return 0
	}))

	L.SetField(mod, "active", L.NewFunction(func(L *lua.LState) int {
		if err := p.Granted.Check("keybindings"); err != nil {
			L.ArgError(1, "keybindings permission not granted")
			return 0
		}
		active := false
		if p.CommandLineActive != nil {
			active = p.CommandLineActive()
		}
		L.Push(lua.LBool(active))
		return 1
	}))

	return mod
}

// luaTextCallback wraps a Lua function taking a single string argument in a
// protected call, so a plugin error is logged instead of unwinding the editor.
func luaTextCallback(p *Plugin, v lua.LValue, context string) func(string) {
	fn, ok := v.(*lua.LFunction)
	if !ok {
		return nil
	}
	return func(text string) {
		if err := p.CallLuaFunc(fn, lua.LString(text)); err != nil {
			p.logError(context, err)
		}
	}
}
