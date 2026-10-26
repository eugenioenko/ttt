package plugin

import (
	lua "github.com/yuin/gopher-lua"
)

func sysRequestExec(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		path := L.CheckString(1)
		callback := L.CheckFunction(2)
		if err := p.Granted.CheckExecPathRequest(path); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		respond := func(granted bool) {
			if err := p.CallLuaFunc(callback, lua.LBool(granted)); err != nil {
				p.logError("request_exec callback", err)
			}
		}
		if p.Granted.CheckExec(path) == nil {
			p.SafePostAsync(&PluginAsyncResult{Plugin: p, Callback: func() { respond(true) }})
			L.Push(lua.LTrue)
			return 1
		}
		if p.RequestExecPath == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("approval dialog not available"))
			return 2
		}
		if err := p.RequestExecPath(path, respond); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}
}
