package plugin

import (
	lua "github.com/yuin/gopher-lua"
)

func setupStorageModule(L *lua.LState, p *Plugin) {
	loader := func(L *lua.LState) int {
		mod := L.NewTable()

		if p.Granted.Check("storage") == nil {
			L.SetField(mod, "get", L.NewFunction(storageGet(p)))
			L.SetField(mod, "set", L.NewFunction(storageSet(p)))
			L.SetField(mod, "remove", L.NewFunction(storageRemove(p)))
			L.SetField(mod, "keys", L.NewFunction(storageKeys(p)))
		}

		L.Push(mod)
		return 1
	}

	L.PreloadModule("ttt.storage", loader)
}

func (p *Plugin) storage(L *lua.LState) *pluginStore {
	if p.store == nil {
		path, err := storagePath(p.StorageDir, p.Name)
		if err != nil {
			L.RaiseError("%s", err.Error())
		}
		p.store = &pluginStore{path: path}
	}
	return p.store
}

func storageGet(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		key := L.CheckString(1)
		v, ok, err := p.storage(L).get(key)
		if err != nil {
			L.RaiseError("%s", err.Error())
		}
		if !ok {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(goToLua(L, v))
		return 1
	}
}

func storageSet(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		key := L.CheckString(1)
		value := L.CheckAny(2)
		switch value.Type() {
		case lua.LTNil, lua.LTBool, lua.LTNumber, lua.LTString, lua.LTTable:
		default:
			L.ArgError(2, "storage values must be nil, a boolean, number, string, or table, got "+value.Type().String())
		}
		if err := p.storage(L).set(key, luaToGo(value)); err != nil {
			L.RaiseError("%s", err.Error())
		}
		return 0
	}
}

func storageRemove(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		key := L.CheckString(1)
		if err := p.storage(L).set(key, nil); err != nil {
			L.RaiseError("%s", err.Error())
		}
		return 0
	}
}

func storageKeys(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		keys, err := p.storage(L).keys()
		if err != nil {
			L.RaiseError("%s", err.Error())
		}
		tbl := L.NewTable()
		for _, k := range keys {
			tbl.Append(lua.LString(k))
		}
		L.Push(tbl)
		return 1
	}
}
