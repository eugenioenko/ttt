package plugin

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

var processStart = time.Now()

func osTime() int64 {
	return time.Now().Unix()
}

func osClock() float64 {
	return time.Since(processStart).Seconds()
}

func osDate(format string) string {
	t := time.Now()
	r := strings.NewReplacer(
		"%Y", t.Format("2006"),
		"%m", t.Format("01"),
		"%d", t.Format("02"),
		"%H", t.Format("15"),
		"%M", t.Format("04"),
		"%S", t.Format("05"),
		"%c", t.Format("Mon Jan  2 15:04:05 2006"),
		"%A", t.Format("Monday"),
		"%a", t.Format("Mon"),
		"%B", t.Format("January"),
		"%b", t.Format("Jan"),
		"%p", t.Format("PM"),
		"%I", t.Format("03"),
		"%Z", t.Format("MST"),
		"%%", "%",
	)
	return r.Replace(format)
}

func NewSandbox() *lua.LState {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})

	for _, pair := range []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage},
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
		{lua.CoroutineLibName, lua.OpenCoroutine},
	} {
		L.Push(L.NewFunction(pair.fn))
		L.Push(lua.LString(pair.name))
		L.Call(1, 0)
	}

	for _, name := range []string{
		"dofile", "loadfile", "load", "loadstring",
		"getfenv", "setfenv",
		"rawset", "rawget",
		"print",
	} {
		L.SetGlobal(name, lua.LNil)
	}

	pkg := L.GetGlobal("package")
	if tbl, ok := pkg.(*lua.LTable); ok {
		loaders := L.GetField(tbl, "loaders")
		if lt, ok := loaders.(*lua.LTable); ok {
			preload := lt.RawGetInt(1)
			for i := lt.Len(); i >= 1; i-- {
				lt.Remove(i)
			}
			lt.RawSetInt(1, preload)
		}
	}

	osMod := L.NewTable()
	L.SetField(osMod, "time", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(float64(osTime())))
		return 1
	}))
	L.SetField(osMod, "clock", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(osClock()))
		return 1
	}))
	L.SetField(osMod, "date", L.NewFunction(func(L *lua.LState) int {
		format := L.OptString(1, "%c")
		L.Push(lua.LString(osDate(format)))
		return 1
	}))
	L.SetGlobal("os", osMod)

	cryptoMod := L.NewTable()
	L.SetField(cryptoMod, "random_bytes", L.NewFunction(func(L *lua.LState) int {
		n := L.CheckInt(1)
		if n < 1 || n > 1024 {
			L.ArgError(1, "byte count must be between 1 and 1024")
			return 0
		}
		buf := make([]byte, n)
		if _, err := rand.Read(buf); err != nil {
			L.RaiseError("crypto error: %s", err.Error())
			return 0
		}
		L.Push(lua.LString(hex.EncodeToString(buf)))
		return 1
	}))
	L.SetField(cryptoMod, "uuid", L.NewFunction(func(L *lua.LState) int {
		var buf [16]byte
		if _, err := rand.Read(buf[:]); err != nil {
			L.RaiseError("crypto error: %s", err.Error())
			return 0
		}
		buf[6] = (buf[6] & 0x0f) | 0x40
		buf[8] = (buf[8] & 0x3f) | 0x80
		s := hex.EncodeToString(buf[:])
		uuid := s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
		L.Push(lua.LString(uuid))
		return 1
	}))
	L.SetGlobal("crypto", cryptoMod)

	return L
}

func setupTTTModule(L *lua.LState, p *Plugin) {
	RegisterPanelType(L)

	loader := func(L *lua.LState) int {
		mod := L.NewTable()

		L.SetField(mod, "register", L.NewFunction(func(L *lua.LState) int {
			tbl := L.CheckTable(1)

			sidebar := L.GetField(tbl, "sidebar")
			if st, ok := sidebar.(*lua.LTable); ok {
				if err := p.Granted.Check("panel.sidebar"); err != nil {
					L.ArgError(1, "panel.sidebar permission not granted")
					return 0
				}
				if title := L.GetField(st, "title"); title != lua.LNil {
					p.SidebarTitle = title.String()
				}
				if fn, ok := L.GetField(st, "render").(*lua.LFunction); ok {
					p.RenderFunc = fn
				}
				if fn, ok := L.GetField(st, "on_event").(*lua.LFunction); ok {
					p.EventFunc = fn
				}
				if actions, ok := L.GetField(st, "actions").(*lua.LTable); ok {
					p.SidebarMenuEntries = parseLuaMenuEntries(L, actions)
				}
				if fn, ok := L.GetField(st, "on_action").(*lua.LFunction); ok {
					p.sidebarMenuFunc = fn
				}
			}

			bottom := L.GetField(tbl, "bottom")
			if bt, ok := bottom.(*lua.LTable); ok {
				if err := p.Granted.Check("panel.bottom"); err != nil {
					L.ArgError(1, "panel.bottom permission not granted")
					return 0
				}
				if title := L.GetField(bt, "title"); title != lua.LNil {
					p.BottomTitle = title.String()
				}
				if fn, ok := L.GetField(bt, "render").(*lua.LFunction); ok {
					p.BottomRenderFunc = fn
				}
				if fn, ok := L.GetField(bt, "on_event").(*lua.LFunction); ok {
					p.BottomEventFunc = fn
				}
			}

			commands := L.GetField(tbl, "commands")
			if ct, ok := commands.(*lua.LTable); ok {
				if err := p.Granted.Check("commands"); err != nil {
					L.ArgError(1, "commands permission not granted")
					return 0
				}
				ct.ForEach(func(_ lua.LValue, v lua.LValue) {
					entry, ok := v.(*lua.LTable)
					if !ok {
						return
					}
					id := L.GetField(entry, "id")
					title := L.GetField(entry, "title")
					handler, hOk := L.GetField(entry, "handler").(*lua.LFunction)
					if id == lua.LNil || title == lua.LNil || !hOk {
						return
					}
					fn := handler
					p.Commands = append(p.Commands, PluginCommand{
						ID:      id.String(),
						Title:   title.String(),
						Handler: func() error { return p.CallLuaFunc(fn) },
					})
				})
			}

			keybindings := L.GetField(tbl, "keybindings")
			if kt, ok := keybindings.(*lua.LTable); ok {
				if err := p.Granted.Check("keybindings"); err != nil {
					L.ArgError(1, "keybindings permission not granted")
					return 0
				}
				kt.ForEach(func(_ lua.LValue, v lua.LValue) {
					entry, ok := v.(*lua.LTable)
					if !ok {
						return
					}
					key := L.GetField(entry, "key")
					cmd := L.GetField(entry, "command")
					if key == lua.LNil || cmd == lua.LNil {
						return
					}
					p.PluginKeybindings = append(p.PluginKeybindings, PluginKeybinding{
						Key:     key.String(),
						Command: cmd.String(),
					})
				})
			}

			return 0
		}))

		L.SetField(mod, "log", L.NewFunction(func(L *lua.LState) int {
			nargs := L.GetTop()
			var level, message string
			if nargs >= 2 {
				level = L.CheckString(1)
				message = L.CheckString(2)
			} else {
				level = "info"
				message = L.CheckString(1)
			}
			if p.Log != nil {
				p.Log(level, message)
			}
			return 0
		}))

		L.SetField(mod, "notify", L.NewFunction(func(L *lua.LState) int {
			msg := L.CheckString(1)
			level := L.OptString(2, "info")
			if p.Notify != nil {
				p.Notify(msg, level)
			}
			return 0
		}))

		L.SetField(mod, "set_status_item", L.NewFunction(func(L *lua.LState) int {
			side := L.CheckString(1)
			id := L.CheckString(2)
			text := L.CheckString(3)
			priority := 1000
			var onClick func()
			if opts := L.OptTable(4, nil); opts != nil {
				if pv := L.GetField(opts, "priority"); pv != lua.LNil {
					if n, ok := pv.(lua.LNumber); ok {
						priority = int(n)
					}
				}
				if cb := L.GetField(opts, "on_click"); cb != lua.LNil {
					if fn, ok := cb.(*lua.LFunction); ok {
						onClick = func() {
							if err := L.CallByParam(lua.P{Fn: fn, Protect: true}); err != nil {
								p.logError("status_item.on_click", err)
							}
						}
					}
				}
			}
			if p.SetStatusItem != nil {
				p.SetStatusItem(side, p.Name+":"+id, text, priority, onClick)
			}
			return 0
		}))

		L.SetField(mod, "remove_status_item", L.NewFunction(func(L *lua.LState) int {
			id := L.CheckString(1)
			if p.RemoveStatusItem != nil {
				p.RemoveStatusItem(p.Name + ":" + id)
			}
			return 0
		}))

		L.SetField(mod, "set_echo", L.NewFunction(func(L *lua.LState) int {
			text := L.CheckString(1)
			if p.SetEcho != nil {
				p.SetEcho(text)
			}
			return 0
		}))

		L.SetField(mod, "clear_echo", L.NewFunction(func(L *lua.LState) int {
			if p.SetEcho != nil {
				p.SetEcho("")
			}
			return 0
		}))

		L.SetField(mod, "exec_command", L.NewFunction(func(L *lua.LState) int {
			if err := p.Granted.Check("commands"); err != nil {
				L.ArgError(1, "commands permission not granted")
				return 0
			}
			id := L.CheckString(1)
			ok := false
			if p.ExecCommand != nil {
				ok = p.ExecCommand(id)
			}
			L.Push(lua.LBool(ok))
			return 1
		}))

		L.SetField(mod, "command_line", newCommandLineModule(L, p))

		L.SetField(mod, "list_commands", L.NewFunction(func(L *lua.LState) int {
			if err := p.Granted.Check("commands"); err != nil {
				L.ArgError(1, "commands permission not granted")
				return 0
			}
			tbl := L.NewTable()
			if p.ListCommands != nil {
				for _, cmd := range p.ListCommands() {
					entry := L.NewTable()
					L.SetField(entry, "id", lua.LString(cmd.ID))
					L.SetField(entry, "title", lua.LString(cmd.Title))
					tbl.Append(entry)
				}
			}
			L.Push(tbl)
			return 1
		}))

		L.SetField(mod, "show_info", L.NewFunction(func(L *lua.LState) int {
			title := L.CheckString(1)
			tbl := L.CheckTable(2)
			var entries []widgets.KeyValueEntry
			tbl.ForEach(func(_, v lua.LValue) {
				row, ok := v.(*lua.LTable)
				if !ok {
					return
				}
				entry := widgets.KeyValueEntry{}
				if k := L.GetField(row, "key"); k != lua.LNil {
					entry.Key = k.String()
				}
				if val := L.GetField(row, "value"); val != lua.LNil {
					entry.Value = val.String()
				}
				entries = append(entries, entry)
			})
			if p.ShowInfoDialog != nil {
				p.ShowInfoDialog(title, entries)
			}
			return 0
		}))

		L.SetField(mod, "confirm", L.NewFunction(func(L *lua.LState) int {
			message := L.CheckString(1)
			callback := L.CheckFunction(2)
			confirmLabel := L.OptString(3, "OK")
			cancelLabel := L.OptString(4, "Cancel")
			if p.ShowConfirmDialog != nil {
				p.ShowConfirmDialog(message, confirmLabel, cancelLabel, func() {
					if err := p.CallLuaFunc(callback); err != nil {
						p.logError("confirm callback", err)
					}
				})
			}
			return 0
		}))

		L.SetField(mod, "open_drawer", L.NewFunction(func(L *lua.LState) int {
			if err := p.Granted.Check("panel.drawer"); err != nil {
				L.ArgError(1, "panel.drawer permission not granted")
				return 0
			}
			tbl := L.CheckTable(1)
			renderFunc, ok := L.GetField(tbl, "render").(*lua.LFunction)
			if !ok {
				L.ArgError(1, "render function required")
				return 0
			}
			width := 40
			minWidth := 20
			if w := L.GetField(tbl, "width"); w != lua.LNil {
				if n, ok := w.(lua.LNumber); ok {
					width = int(n)
				}
			}
			if mw := L.GetField(tbl, "min_width"); mw != lua.LNil {
				if n, ok := mw.(lua.LNumber); ok {
					minWidth = int(n)
				}
			}
			side := "right"
			if s := L.GetField(tbl, "side"); s != lua.LNil {
				if sv := s.String(); sv == "left" || sv == "right" {
					side = sv
				}
			}
			panel := NewPluginPanelWidget(p, renderFunc, nil)
			if p.OpenDrawer != nil {
				p.OpenDrawer(panel, width, minWidth, side)
			} else {
				p.pendingDrawer = &pendingDrawerCall{
					panel: panel, width: width, minWidth: minWidth, side: side,
				}
			}
			return 0
		}))

		L.SetField(mod, "close_drawer", L.NewFunction(func(L *lua.LState) int {
			if p.CloseDrawer != nil {
				p.CloseDrawer()
			}
			return 0
		}))

		L.SetField(mod, "open_tab", L.NewFunction(func(L *lua.LState) int {
			if err := p.Granted.Check("panel.editor"); err != nil {
				L.ArgError(1, "panel.editor permission not granted")
				return 0
			}
			tbl := L.CheckTable(1)
			title := "Plugin"
			if t := L.GetField(tbl, "title"); t != lua.LNil {
				title = t.String()
			}
			renderFunc, ok := L.GetField(tbl, "render").(*lua.LFunction)
			if !ok {
				L.ArgError(1, "render function required")
				return 0
			}
			var eventFunc *lua.LFunction
			if fn, ok := L.GetField(tbl, "on_event").(*lua.LFunction); ok {
				eventFunc = fn
			}
			if p.OpenTab != nil {
				panel := NewPluginPanelWidget(p, renderFunc, eventFunc)
				p.OpenTab(title, panel)
			}
			return 0
		}))

		L.SetField(mod, "close_tab", L.NewFunction(func(L *lua.LState) int {
			id := L.CheckString(1)
			if p.CloseTab != nil {
				p.CloseTab(id)
			}
			return 0
		}))

		L.SetField(mod, "click", L.NewFunction(func(L *lua.LState) int {
			x := L.CheckInt(1)
			y := L.CheckInt(2)
			if p.SimulateClick != nil {
				p.SimulateClick(x, y)
			}
			return 0
		}))

		L.SetField(mod, "drag", L.NewFunction(func(L *lua.LState) int {
			x1 := L.CheckInt(1)
			y1 := L.CheckInt(2)
			x2 := L.CheckInt(3)
			y2 := L.CheckInt(4)
			if p.SimulateDrag != nil {
				p.SimulateDrag(x1, y1, x2, y2)
			}
			return 0
		}))

		L.SetField(mod, "screenshot", L.NewFunction(func(L *lua.LState) int {
			path := L.CheckString(1)
			if p.ScreenshotToFile != nil {
				if err := p.ScreenshotToFile(path); err != nil {
					L.ArgError(1, err.Error())
				}
			}
			return 0
		}))

		L.SetField(mod, "debug", L.NewFunction(func(L *lua.LState) int {
			path := L.CheckString(1)
			if p.DebugDumpToFile != nil {
				if err := p.DebugDumpToFile(path); err != nil {
					L.ArgError(1, err.Error())
				}
			}
			return 0
		}))

		L.SetField(mod, "quit", L.NewFunction(func(L *lua.LState) int {
			if p.QuitApp != nil {
				p.QuitApp()
			}
			return 0
		}))

		L.SetField(mod, "open_file", L.NewFunction(func(L *lua.LState) int {
			path := L.CheckString(1)
			line := L.OptInt(2, 0)
			readonly := false
			if L.Get(3) == lua.LTrue {
				readonly = true
			}
			if p.OpenFile != nil {
				p.OpenFile(path, line, readonly)
			}
			return 0
		}))

		L.SetField(mod, "open_diff", L.NewFunction(func(L *lua.LState) int {
			if err := p.Granted.Check("panel.editor"); err != nil {
				L.ArgError(1, "panel.editor permission not granted")
				return 0
			}
			title := L.CheckString(1)
			oldTbl := L.CheckTable(2)
			newTbl := L.CheckTable(3)
			filePath := L.OptString(4, "")
			oldLines := luaTableToStrings(L, oldTbl)
			newLines := luaTableToStrings(L, newTbl)
			if p.OpenDiff != nil {
				p.OpenDiff(title, oldLines, newLines, filePath)
			}
			return 0
		}))

		L.SetField(mod, "open_readonly", L.NewFunction(func(L *lua.LState) int {
			if err := p.Granted.Check("panel.editor"); err != nil {
				L.ArgError(1, "panel.editor permission not granted")
				return 0
			}
			title := L.CheckString(1)
			linesTbl := L.CheckTable(2)
			filePath := L.OptString(3, "")
			lines := luaTableToStrings(L, linesTbl)
			if p.OpenReadOnly != nil {
				p.OpenReadOnly(title, filePath, lines)
			}
			return 0
		}))

		L.SetField(mod, "plugin_dir", L.NewFunction(func(L *lua.LState) int {
			L.Push(lua.LString(p.Dir))
			return 1
		}))

		L.SetField(mod, "set_timeout", L.NewFunction(func(L *lua.LState) int {
			ms := L.CheckInt(1)
			fn := L.CheckFunction(2)
			id := p.SetTimeout(ms, func() { p.CallLuaFunc(fn) })
			L.Push(lua.LNumber(id))
			return 1
		}))

		L.SetField(mod, "set_interval", L.NewFunction(func(L *lua.LState) int {
			ms := L.CheckInt(1)
			fn := L.CheckFunction(2)
			id := p.SetInterval(ms, func() { p.CallLuaFunc(fn) })
			L.Push(lua.LNumber(id))
			return 1
		}))

		clearTimer := L.NewFunction(func(L *lua.LState) int {
			p.ClearTimer(L.CheckInt(1))
			return 0
		})
		L.SetField(mod, "clear_timeout", clearTimer)
		L.SetField(mod, "clear_interval", clearTimer)

		L.SetField(mod, "on_install", L.NewFunction(func(L *lua.LState) int {
			fn := L.CheckFunction(1)
			p.InstallFunc = fn
			return 0
		}))

		L.SetField(mod, "on_uninstall", L.NewFunction(func(L *lua.LState) int {
			fn := L.CheckFunction(1)
			p.UninstallFunc = fn
			return 0
		}))

		L.SetField(mod, "markdown", L.NewFunction(func(L *lua.LState) int {
			if p.RenderMarkdown == nil {
				L.Push(L.NewTable())
				return 1
			}
			text := L.CheckString(1)
			rendered := p.RenderMarkdown(text)
			result := L.NewTable()
			for i, line := range rendered {
				lineTable := L.NewTable()
				for j, span := range line.Spans {
					spanTable := L.NewTable()
					L.SetField(spanTable, "text", lua.LString(span.Text))
					L.SetField(spanTable, "style", lua.LString(NameByStyle(span.Style)))
					lineTable.RawSetInt(j+1, spanTable)
				}
				result.RawSetInt(i+1, lineTable)
			}
			L.Push(result)
			return 1
		}))

		L.Push(mod)
		return 1
	}

	L.PreloadModule("ttt", loader)

	allowedModules := map[string]bool{
		"ttt":             true,
		"ttt.editor":      true,
		"ttt.diagnostics": true,
		"ttt.fs":          true,
		"ttt.system":      true,
		"ttt.net":         true,
		"ttt.events":      true,
		"ttt.json":        true,
		"ttt.settings":    true,
	}

	origRequire := L.GetGlobal("require")
	L.SetGlobal("require", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		if !allowedModules[name] {
			L.ArgError(1, fmt.Sprintf("module %q is not available", name))
			return 0
		}
		L.Push(origRequire)
		L.Push(lua.LString(name))
		L.Call(1, 1)
		return 1
	}))
}

func luaTableToStrings(_ *lua.LState, tbl *lua.LTable) []string {
	var result []string
	tbl.ForEach(func(k, v lua.LValue) {
		if _, ok := k.(lua.LNumber); ok {
			result = append(result, v.String())
		}
	})
	return result
}
