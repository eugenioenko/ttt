package plugin

import (
	"context"
	"encoding/json"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func setupAutomationModule(L *lua.LState, p *Plugin) {
	L.PreloadModule("ttt.automation", func(L *lua.LState) int {
		mod := L.NewTable()
		L.SetField(mod, "run", L.NewFunction(func(L *lua.LState) int {
			action, args, callback := L.CheckString(1), L.CheckString(2), L.CheckFunction(3)
			for _, permission := range automationPermissions(action) {
				if err := p.Granted.Check(permission); err != nil {
					L.RaiseError("%s", err.Error())
					return 0
				}
			}
			run := p.RunAutomation
			if run == nil {
				L.RaiseError("automation API not available")
				return 0
			}
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			id := p.registerTimer(cancel)
			go func() {
				defer cancel()
				result, err := run(ctx, action, args)
				p.SafePostAsync(&PluginAsyncResult{Plugin: p, Callback: func() {
					p.ClearTimer(id)
					if p.State == nil {
						return
					}
					if err != nil {
						_ = p.CallLuaFunc(callback, lua.LNil, lua.LString(err.Error()))
						return
					}
					var value any
					_ = json.Unmarshal(result, &value)
					_ = p.CallLuaFunc(callback, goToLua(p.State, value), lua.LNil)
				}})
			}()
			L.Push(L.NewFunction(func(L *lua.LState) int { cancel(); return 0 }))
			return 1
		}))
		L.Push(mod)
		return 1
	})
}

// automationPermissions lists what an action needs. The debug dump includes
// buffer text and terminal history, so it needs their read permissions too.
func automationPermissions(action string) []string {
	switch action {
	case "session", "buffer":
		return []string{"editor.read"}
	case "screenshot", "wait-for":
		return []string{"screen.read"}
	case "debug":
		return []string{"screen.read", "editor.read", "terminal.read"}
	case "commands", "exec", "command", "panel", "copy", "open":
		return []string{"commands"}
	case "clipboard":
		return []string{"clipboard.read"}
	}
	return []string{"input"}
}
