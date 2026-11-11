package plugin

import (
	lua "github.com/yuin/gopher-lua"
)

// setupTerminalModule exposes the terminal surface to Lua:
//
//	local term = require("ttt.terminal")
//	local id, err = term.spawn{ shell = "pi", args = {"--model", "…"}, cwd = ".", label = "pi", location = "editor" }
//	term.send(id, "review this\n")
//	local tail = term.read(id, 4000)   -- raw PTY bytes, ANSI included
//	term.focus(id)
//	term.close(id)
//	for _, t in ipairs(term.list()) do … end
//
// spawn/send/focus/close require `terminal.control`; read/list require
// `terminal.read`. Missing permissions omit the functions entirely (matching the
// system module), so `if term.spawn then … end` is the capability check.
// Errors follow the system module's convention: (nil, "message").
func setupTerminalModule(L *lua.LState, p *Plugin) {
	loader := func(L *lua.LState) int {
		mod := L.NewTable()
		if p.Granted.TerminalControl {
			L.SetField(mod, "spawn", L.NewFunction(termSpawn(p)))
			L.SetField(mod, "send", L.NewFunction(termSend(p)))
			L.SetField(mod, "focus", L.NewFunction(termFocus(p)))
			L.SetField(mod, "close", L.NewFunction(termClose(p)))
		}
		if p.Granted.TerminalRead {
			L.SetField(mod, "read", L.NewFunction(termRead(p)))
			L.SetField(mod, "screen", L.NewFunction(termScreen(p)))
			L.SetField(mod, "list", L.NewFunction(termList(p)))
		}
		if p.Granted.TerminalPalette {
			L.SetField(mod, "palette", L.NewFunction(termPalette(p)))
			L.SetField(mod, "request_palette", L.NewFunction(termRequestPalette(p)))
		}
		L.Push(mod)
		return 1
	}
	L.PreloadModule("ttt.terminal", loader)
}

func termRequestPalette(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Terminal == nil {
			return termErr(L, "terminal API not available")
		}
		p.Terminal.RequestTerminalPalette()
		L.Push(lua.LTrue)
		return 1
	}
}

func termPalette(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Terminal == nil {
			return termErr(L, "terminal API not available")
		}
		status, colors := p.Terminal.TerminalPalette()
		result := L.NewTable()
		L.SetField(result, "status", lua.LString(status))
		palette := L.NewTable()
		for code, value := range colors {
			L.SetField(palette, code, lua.LString(value))
		}
		L.SetField(result, "colors", palette)
		L.Push(result)
		return 1
	}
}

func termErr(L *lua.LState, msg string) int {
	L.Push(lua.LNil)
	L.Push(lua.LString(msg))
	return 2
}

// termField reads a string field from a Lua table ("" when absent).
func termField(L *lua.LState, tbl *lua.LTable, key string) string {
	if v, ok := L.GetField(tbl, key).(lua.LString); ok {
		return string(v)
	}
	return ""
}

// termSpawn{ shell = "pi", args = {...}, cwd = "...", label = "pi", location = "editor" } -> id, err
func termSpawn(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Terminal == nil {
			return termErr(L, "terminal API not available")
		}
		spec := L.OptTable(1, nil)
		if spec == nil {
			return termErr(L, "terminal.spawn expects a table: {shell, args, cwd, label}")
		}
		var args []string
		if t, ok := L.GetField(spec, "args").(*lua.LTable); ok {
			t.ForEach(func(_, v lua.LValue) {
				args = append(args, v.String())
			})
		}
		// env accepts a map (KEY = "value") or an array of "KEY=value" strings
		var env []string
		if t, ok := L.GetField(spec, "env").(*lua.LTable); ok {
			t.ForEach(func(k, v lua.LValue) {
				if ks, ok := k.(lua.LString); ok && ks != "" {
					env = append(env, string(ks)+"="+v.String())
					return
				}
				env = append(env, v.String())
			})
		}
		location := termField(L, spec, "location")
		switch location {
		case "", TerminalLocationPanel, TerminalLocationEditor:
		default:
			return termErr(L, `terminal.spawn: location must be "panel" or "editor"`)
		}
		id, err := p.Terminal.TerminalSpawn(TerminalSpawnSpec{
			Shell:    termField(L, spec, "shell"),
			Args:     args,
			Env:      env,
			Cwd:      termField(L, spec, "cwd"),
			Label:    termField(L, spec, "label"),
			Location: location,
		})
		if err != nil {
			return termErr(L, err.Error())
		}
		L.Push(lua.LString(id))
		return 1
	}
}

// termSend(id, text) -> true | nil, err
func termSend(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Terminal == nil {
			return termErr(L, "terminal API not available")
		}
		id := L.CheckString(1)
		text := L.CheckString(2)
		if err := p.Terminal.TerminalSend(id, text); err != nil {
			return termErr(L, err.Error())
		}
		L.Push(lua.LTrue)
		return 1
	}
}

// termRead(id, maxBytes?) -> tail | nil, err
func termRead(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Terminal == nil {
			return termErr(L, "terminal API not available")
		}
		id := L.CheckString(1)
		n := L.OptInt(2, 0)
		tail, err := p.Terminal.TerminalTail(id, n)
		if err != nil {
			return termErr(L, err.Error())
		}
		L.Push(lua.LString(tail))
		return 1
	}
}

// termScreen(id) -> text | nil, err
func termScreen(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Terminal == nil {
			return termErr(L, "terminal API not available")
		}
		id := L.CheckString(1)
		text, err := p.Terminal.TerminalScreen(id)
		if err != nil {
			return termErr(L, err.Error())
		}
		L.Push(lua.LString(text))
		return 1
	}
}

// termFocus(id) -> true | nil, err
func termFocus(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Terminal == nil {
			return termErr(L, "terminal API not available")
		}
		id := L.CheckString(1)
		if err := p.Terminal.TerminalFocus(id); err != nil {
			return termErr(L, err.Error())
		}
		L.Push(lua.LTrue)
		return 1
	}
}

// termClose(id) -> true | nil, err
func termClose(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Terminal == nil {
			return termErr(L, "terminal API not available")
		}
		id := L.CheckString(1)
		if err := p.Terminal.TerminalClose(id); err != nil {
			return termErr(L, err.Error())
		}
		L.Push(lua.LTrue)
		return 1
	}
}

// termList() -> {{id=, label=, cols=, rows=}, …}
func termList(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Terminal == nil {
			return termErr(L, "terminal API not available")
		}
		infos := p.Terminal.TerminalList()
		tbl := L.NewTable()
		for _, t := range infos {
			entry := L.NewTable()
			L.SetField(entry, "id", lua.LString(t.ID))
			L.SetField(entry, "label", lua.LString(t.Label))
			L.SetField(entry, "location", lua.LString(t.Location))
			L.SetField(entry, "cols", lua.LNumber(t.Cols))
			L.SetField(entry, "rows", lua.LNumber(t.Rows))
			tbl.Append(entry)
		}
		L.Push(tbl)
		return 1
	}
}
