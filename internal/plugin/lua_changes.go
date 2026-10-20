package plugin

import (
	lua "github.com/yuin/gopher-lua"
	"strings"
)

type ChangesOptions struct{ ID, Title, Description, Diff string }

func setupChangesFunctions(L *lua.LState, mod *lua.LTable, p *Plugin) {
	L.SetField(mod, "open_changes", L.NewFunction(func(L *lua.LState) int {
		cfg := L.CheckTable(1)
		diff, ok := L.GetField(cfg, "diff").(lua.LString)
		if !ok || strings.TrimSpace(string(diff)) == "" {
			L.ArgError(1, "diff must be a non-empty string")
			return 0
		}
		opts := ChangesOptions{
			Title:       changesStringOption(L, cfg, "title", "Changes"),
			Description: changesStringOption(L, cfg, "description", ""),
			Diff:        string(diff),
		}
		opts.ID = changesStringOption(L, cfg, "id", opts.Title)
		if strings.TrimSpace(opts.ID) == "" {
			L.ArgError(1, "non-empty id or title required")
			return 0
		}
		if p.OpenChanges == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("changes view unavailable"))
			return 2
		}
		id, err := p.OpenChanges(opts)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(id))
		return 1
	}))
}

func changesStringOption(L *lua.LState, cfg *lua.LTable, name, fallback string) string {
	value := L.GetField(cfg, name)
	if value == lua.LNil {
		return fallback
	}
	text, ok := value.(lua.LString)
	if !ok {
		L.ArgError(1, name+" must be a string")
		return ""
	}
	return string(text)
}
