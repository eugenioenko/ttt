package plugin

import (
	"fmt"
	"slices"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

func checkSettingsWrite(ps PermissionSet, key string, install bool) error {
	if err := ps.CheckSettingsKey(key); err != nil {
		return err
	}
	parts := strings.SplitN(key, ".", 3)
	if strings.EqualFold(parts[0], "debug") && (len(parts) == 1 || strings.EqualFold(parts[1], "adapters")) {
		name, ok := strings.CutPrefix(key, "debug.adapters.")
		if !install || !ok || name == "" || strings.ContainsAny(name, ".*") || !slices.Contains(ps.SettingsKeys, key) {
			return fmt.Errorf("debug adapters require an exact settings key and set_default during install")
		}
	}
	return nil
}

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
			if err := checkSettingsWrite(p.Granted, key, true); err != nil {
				L.ArgError(1, err.Error())
				return 0
			}
			if p.installDefault == nil {
				L.RaiseError("set_default requires on_install")
				return 0
			}
			value := luaToGo(L.Get(2))
			if value == nil {
				L.ArgError(2, "expected a default value")
				return 0
			}
			if err := p.installDefault(key, value); err != nil {
				L.RaiseError("%s", err.Error())
			}
			return 0
		}))
		L.SetField(mod, "set", L.NewFunction(func(L *lua.LState) int {
			key := L.CheckString(1)
			value := L.Get(2)
			if err := checkSettingsWrite(p.Granted, key, false); err != nil {
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
