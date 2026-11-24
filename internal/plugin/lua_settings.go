package plugin

import (
	lua "github.com/yuin/gopher-lua"
)

func setupSettingsModule(L *lua.LState, p *Plugin) {
	loader := func(L *lua.LState) int {
		mod := L.NewTable()

		L.SetField(mod, "get", L.NewFunction(func(L *lua.LState) int {
			key := L.CheckString(1)
			if err := p.Granted.CheckSettingsKey(key); err != nil {
				L.ArgError(1, err.Error())
				return 0
			}
			if p.Settings == nil {
				L.Push(lua.LNil)
				return 1
			}
			val, ok := p.Settings.Get(key)
			if !ok {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(goToLua(L, val))
			return 1
		}))

		L.SetField(mod, "set_default", L.NewFunction(func(L *lua.LState) int {
			key := L.CheckString(1)
			if err := p.Granted.CheckSettingsKey(key); err != nil {
				L.ArgError(1, err.Error())
				return 0
			}
			if p.installDefault == nil {
				L.RaiseError("set_default requires on_install")
				return 0
			}
			if L.Get(2) == lua.LNil {
				L.ArgError(2, "expected a default value")
				return 0
			}
			if err := p.installDefault(key, luaToGo(L.Get(2))); err != nil {
				L.RaiseError("%s", err.Error())
			}
			return 0
		}))
		L.SetField(mod, "set", L.NewFunction(func(L *lua.LState) int {
			key := L.CheckString(1)
			value := L.Get(2)
			if err := p.Granted.CheckSettingsKey(key); err != nil {
				L.ArgError(1, err.Error())
				return 0
			}
			if p.Settings == nil {
				L.ArgError(1, "settings API not available")
				return 0
			}
			if err := p.Settings.Set(key, luaToGo(value)); err != nil {
				L.ArgError(2, err.Error())
				return 0
			}
			return 0
		}))

		L.Push(mod)
		return 1
	}
	L.PreloadModule("ttt.settings", loader)
}
