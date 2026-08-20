package plugin

import (
	lua "github.com/yuin/gopher-lua"
)

func setupBookmarksModule(L *lua.LState, p *Plugin) {
	loader := func(L *lua.LState) int {
		mod := L.NewTable()

		if p.Granted.Check("editor.bookmarks") == nil {
			L.SetField(mod, "set", L.NewFunction(bookmarksSet(p)))
			L.SetField(mod, "get", L.NewFunction(bookmarksGet(p)))
			L.SetField(mod, "remove", L.NewFunction(bookmarksRemove(p)))
			L.SetField(mod, "set_all", L.NewFunction(bookmarksSetAll(p)))
			L.SetField(mod, "get_all", L.NewFunction(bookmarksGetAll(p)))
			L.SetField(mod, "clear", L.NewFunction(bookmarksClear(p)))
		}

		L.Push(mod)
		return 1
	}

	L.PreloadModule("ttt.bookmarks", loader)
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

func bookmarksSet(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		line := L.CheckInt(1) - 1
		icon := firstRune(L.CheckString(2))
		style, _ := StyleByName(L.CheckString(3))
		if p.SetBookmark != nil {
			p.SetBookmark(line, icon, style)
		}
		return 0
	}
}

func bookmarksGet(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		line := L.CheckInt(1) - 1
		if p.GetBookmark == nil {
			L.Push(lua.LNil)
			return 1
		}
		icon, style, ok := p.GetBookmark(line)
		if !ok {
			L.Push(lua.LNil)
			return 1
		}
		tbl := L.NewTable()
		L.SetField(tbl, "icon", lua.LString(string(icon)))
		L.SetField(tbl, "style", lua.LString(NameByStyle(style)))
		L.Push(tbl)
		return 1
	}
}

func bookmarksRemove(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		line := L.CheckInt(1) - 1
		if p.RemoveBookmark != nil {
			p.RemoveBookmark(line)
		}
		return 0
	}
}

func bookmarksSetAll(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		itemsTbl := L.CheckTable(1)

		var items []BookmarkItem
		itemsTbl.ForEach(func(_, v lua.LValue) {
			tbl, ok := v.(*lua.LTable)
			if !ok {
				return
			}
			items = append(items, luaTableToBookmarkItem(L, tbl))
		})

		if p.SetAllBookmarks != nil {
			p.SetAllBookmarks(items)
		}
		return 0
	}
}

func bookmarksGetAll(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		result := L.NewTable()
		if p.GetAllBookmarks == nil {
			L.Push(result)
			return 1
		}
		for _, it := range p.GetAllBookmarks() {
			tbl := L.NewTable()
			L.SetField(tbl, "line", lua.LNumber(it.Line+1))
			L.SetField(tbl, "icon", lua.LString(string(it.Icon)))
			L.SetField(tbl, "style", lua.LString(NameByStyle(it.Style)))
			result.Append(tbl)
		}
		L.Push(result)
		return 1
	}
}

func bookmarksClear(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.ClearBookmarks != nil {
			p.ClearBookmarks()
		}
		return 0
	}
}

func luaTableToBookmarkItem(L *lua.LState, tbl *lua.LTable) BookmarkItem {
	item := BookmarkItem{Line: luaFieldInt(L, tbl, "line", 1) - 1}
	if v := L.GetField(tbl, "icon"); v != lua.LNil {
		item.Icon = firstRune(v.String())
	}
	if v := L.GetField(tbl, "style"); v != lua.LNil {
		if style, ok := StyleByName(v.String()); ok {
			item.Style = style
		}
	}
	return item
}
