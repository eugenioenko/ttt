package plugin

import (
	lua "github.com/yuin/gopher-lua"
)

func setupSystemModule(L *lua.LState, p *Plugin) {
	loader := func(L *lua.LState) int {
		mod := L.NewTable()

		hasExec := len(p.Granted.SystemExec) > 0
		hasEnv := p.Granted.Check("system.env") == nil

		if hasExec {
			L.SetField(mod, "exec", L.NewFunction(sysExec(p)))
			L.SetField(mod, "exec_async", L.NewFunction(sysExecAsync(p)))
			L.SetField(mod, "request_exec", L.NewFunction(sysRequestExec(p)))
		}

		if hasEnv {
			L.SetField(mod, "env", L.NewFunction(sysEnv(p)))
		}

		L.Push(mod)
		return 1
	}

	L.PreloadModule("ttt.system", loader)
}

// parseExecArgs reads the optional argv array and options table that follow the
// binary name, starting at stack index start. The only option is stdin, whose
// contents are written to the child's standard input.
func parseExecArgs(L *lua.LState, start int) (args []string, stdin string) {
	if argsTbl, ok := L.Get(start).(*lua.LTable); ok {
		argsTbl.ForEach(func(_, v lua.LValue) {
			args = append(args, v.String())
		})
	}
	if optsTbl, ok := L.Get(start + 1).(*lua.LTable); ok {
		if s, ok := L.GetField(optsTbl, "stdin").(lua.LString); ok {
			stdin = string(s)
		}
	}
	return args, stdin
}

func sysExec(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.System == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("system API not available"))
			return 2
		}
		binary := L.CheckString(1)
		if err := p.Granted.CheckExec(binary); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}

		args, stdin := parseExecArgs(L, 2)

		tbl := L.NewTable()
		stdout, stderr, exitCode, err := p.System.Exec(binary, args, stdin)
		if err != nil {
			L.SetField(tbl, "stdout", lua.LString(""))
			L.SetField(tbl, "stderr", lua.LString(err.Error()))
			L.SetField(tbl, "exit_code", lua.LNumber(-1))
		} else {
			L.SetField(tbl, "stdout", lua.LString(stdout))
			L.SetField(tbl, "stderr", lua.LString(stderr))
			L.SetField(tbl, "exit_code", lua.LNumber(exitCode))
		}
		L.Push(tbl)
		return 1
	}
}

func sysExecAsync(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.System == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("system API not available"))
			return 2
		}
		binary := L.CheckString(1)
		if err := p.Granted.CheckExec(binary); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}

		var args []string
		var stdin string
		var callback *lua.LFunction
		if fn, ok := L.Get(2).(*lua.LFunction); ok {
			callback = fn
		} else {
			args, stdin = parseExecArgs(L, 2)
			if fn, ok := L.Get(3).(*lua.LFunction); ok {
				callback = fn
			} else {
				callback = L.CheckFunction(4)
			}
		}

		go func() {
			stdout, stderr, exitCode, err := p.System.Exec(binary, args, stdin)
			resultFn := func() {
				if p.State == nil {
					return
				}
				tbl := p.State.NewTable()
				if err != nil {
					p.State.SetField(tbl, "stdout", lua.LString(""))
					p.State.SetField(tbl, "stderr", lua.LString(err.Error()))
					p.State.SetField(tbl, "exit_code", lua.LNumber(-1))
				} else {
					p.State.SetField(tbl, "stdout", lua.LString(stdout))
					p.State.SetField(tbl, "stderr", lua.LString(stderr))
					p.State.SetField(tbl, "exit_code", lua.LNumber(exitCode))
				}
				if callErr := p.CallLuaFunc(callback, tbl); callErr != nil {
					p.logError("async exec callback", callErr)
				}
			}
			p.SafePostAsync(&PluginAsyncResult{Plugin: p, Callback: resultFn})
		}()

		return 0
	}
}

func sysEnv(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.System == nil {
			L.Push(lua.LString(""))
			return 1
		}
		name := L.CheckString(1)
		L.Push(lua.LString(p.System.Env(name)))
		return 1
	}
}
