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
			L.ArgError(1, "non-empty diff required")
			return 0
		}
		opts := ChangesOptions{Title: "Changes", Diff: string(diff)}
		if v := L.GetField(cfg, "title"); v != lua.LNil {
			opts.Title = v.String()
		}
		if v := L.GetField(cfg, "description"); v != lua.LNil {
			opts.Description = v.String()
		}
		opts.ID = opts.Title
		if v := L.GetField(cfg, "id"); v != lua.LNil {
			opts.ID = v.String()
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
