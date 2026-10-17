package plugin

import (
	"fmt"
	"math"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/eugenioenko/ttt/internal/term"
)

func setupDecorationsModule(L *lua.LState, p *Plugin) {
	loader := func(L *lua.LState) int {
		mod := L.NewTable()

		if p.Granted.Check("editor.decorations") == nil {
			L.SetField(mod, "set", L.NewFunction(decorationsSet(p)))
			L.SetField(mod, "clear", L.NewFunction(decorationsClear(p)))
		}

		L.Push(mod)
		return 1
	}

	L.PreloadModule("ttt.decorations", loader)
}

func decorationsSet(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		path := L.CheckString(1)
		itemsTbl := L.CheckTable(2)

		var items []DecorationItem
		n := itemsTbl.Len()
		for i := 1; i <= n; i++ {
			tbl, ok := itemsTbl.RawGetInt(i).(*lua.LTable)
			if !ok {
				L.ArgError(2, "items must be tables")
			}
			items = append(items, luaTableToDecorationItem(L, tbl, i))
		}

		if p.SetDecorations != nil {
			p.SetDecorations(path, items)
		}
		return 0
	}
}

func decorationsClear(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		path := L.OptString(1, "")
		if p.ClearDecorations != nil {
			p.ClearDecorations(path)
		}
		return 0
	}
}

// luaTableToDecorationItem converts a 1-based Lua item. With note it draws
// the note under line. Otherwise, without col it tints lines line..end_line;
// with col it highlights text up to end_col (exclusive) on end_line. end_line
// may be math.huge for the end of the file.
func luaTableToDecorationItem(L *lua.LState, tbl *lua.LTable, i int) DecorationItem {
	field := func(name string) (int, bool) {
		v := L.GetField(tbl, name)
		if v == lua.LNil {
			return 0, false
		}
		n, ok := v.(lua.LNumber)
		f := float64(n)
		if !ok || f < 1 || (f != math.Floor(f) && !math.IsInf(f, 1)) {
			L.ArgError(2, fmt.Sprintf("item %d: %s must be a whole number >= 1", i, name))
		}
		return int(min(f, math.MaxInt32)), true
	}
	line, ok := field("line")
	if !ok {
		L.ArgError(2, fmt.Sprintf("item %d: line is required", i))
	}
	if v := L.GetField(tbl, "note"); v != lua.LNil {
		item := DecorationItem{StartLine: line - 1, EndLine: line - 1, Note: v.String(), Style: term.StyleNote}
		if _, ok := v.(lua.LString); !ok || strings.TrimSpace(item.Note) == "" {
			L.ArgError(2, fmt.Sprintf("item %d: note must be text", i))
		}
		for _, name := range []string{"end_line", "col", "end_col"} {
			if L.GetField(tbl, name) != lua.LNil {
				L.ArgError(2, fmt.Sprintf("item %d: a note takes only line and style, not %s", i, name))
			}
		}
		item.Style = luaDecorationStyle(L, tbl, i, item.Style)
		if item.Style == term.StyleHide || item.Style == term.StyleReverse {
			L.ArgError(2, fmt.Sprintf("item %d: a note can't use style %s", i, L.GetField(tbl, "style").String()))
		}
		return item
	}
	endLine, ok := field("end_line")
	if !ok {
		endLine = line
	}
	if endLine < line {
		L.ArgError(2, fmt.Sprintf("item %d: end_line is before line", i))
	}
	item := DecorationItem{StartLine: line - 1, EndLine: endLine - 1, Style: term.StyleHighlight}

	col, hasCol := field("col")
	endCol, hasEndCol := field("end_col")
	switch {
	case hasCol != hasEndCol:
		L.ArgError(2, fmt.Sprintf("item %d: col and end_col go together", i))
	case hasCol:
		if endLine == line && endCol <= col {
			L.ArgError(2, fmt.Sprintf("item %d: text range is empty", i))
		}
		item.Text, item.StartCol, item.EndCol = true, col-1, endCol-1
		item.Style = term.StyleHighlightStrong
	}

	item.Style = luaDecorationStyle(L, tbl, i, item.Style)
	return item
}

func luaDecorationStyle(L *lua.LState, tbl *lua.LTable, i int, def term.Style) term.Style {
	v := L.GetField(tbl, "style")
	if v == lua.LNil {
		return def
	}
	style, ok := StyleByName(v.String())
	if !ok {
		L.ArgError(2, fmt.Sprintf("item %d: unknown style %s", i, v.String()))
	}
	return style
}
