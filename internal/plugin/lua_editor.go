package plugin

import (
	lua "github.com/yuin/gopher-lua"
)

func setupEditorModule(L *lua.LState, p *Plugin) {
	loader := func(L *lua.LState) int {
		mod := L.NewTable()

		hasRead := p.Granted.Check("editor.read") == nil
		hasWrite := p.Granted.Check("editor.write") == nil

		if hasRead {
			L.SetField(mod, "buffer_text", L.NewFunction(editorBufferText(p)))
			L.SetField(mod, "buffer_lines", L.NewFunction(editorBufferLines(p)))
			L.SetField(mod, "current_line", L.NewFunction(editorCurrentLine(p)))
			L.SetField(mod, "get_line", L.NewFunction(editorGetLine(p)))
			L.SetField(mod, "line_count", L.NewFunction(editorLineCount(p)))
			L.SetField(mod, "viewport", L.NewFunction(editorViewport(p)))
			L.SetField(mod, "cursor", L.NewFunction(editorCursor(p)))
			L.SetField(mod, "selection", L.NewFunction(editorSelection(p)))
			L.SetField(mod, "selection_text", L.NewFunction(editorSelectionText(p)))
			L.SetField(mod, "file_path", L.NewFunction(editorFilePath(p)))
			L.SetField(mod, "file_name", L.NewFunction(editorFileName(p)))
			L.SetField(mod, "language", L.NewFunction(editorLanguage(p)))
			L.SetField(mod, "byte_to_col", L.NewFunction(editorByteToCol(p)))
			L.SetField(mod, "col_to_byte", L.NewFunction(editorColToByte(p)))
			L.SetField(mod, "register_context_menu", L.NewFunction(editorRegisterContextMenu(p)))
			L.SetField(mod, "get_cursors", L.NewFunction(editorGetCursors(p)))
			L.SetField(mod, "diff", L.NewFunction(editorDiff(p)))
		}

		if hasWrite {
			L.SetField(mod, "scroll_to", L.NewFunction(editorScrollTo(p)))
			L.SetField(mod, "scroll_by", L.NewFunction(editorScrollBy(p)))
			L.SetField(mod, "insert", L.NewFunction(editorInsert(p)))
			L.SetField(mod, "set_line", L.NewFunction(editorSetLine(p)))
			L.SetField(mod, "replace", L.NewFunction(editorReplace(p)))
			L.SetField(mod, "set_cursor", L.NewFunction(editorSetCursor(p)))
			L.SetField(mod, "set_selection", L.NewFunction(editorSetSelection(p)))
			L.SetField(mod, "clear_selection", L.NewFunction(editorClearSelection(p)))
			L.SetField(mod, "begin_undo_group", L.NewFunction(editorBeginUndoGroup(p)))
			L.SetField(mod, "end_undo_group", L.NewFunction(editorEndUndoGroup(p)))
			L.SetField(mod, "add_cursor", L.NewFunction(editorAddCursor(p)))
			L.SetField(mod, "clear_cursors", L.NewFunction(editorClearCursors(p)))
			L.SetField(mod, "set_search", L.NewFunction(editorSetSearch(p)))
			L.SetField(mod, "clear_search", L.NewFunction(editorClearSearch(p)))
			L.SetField(mod, "go_to_diff_hunk", L.NewFunction(editorGoToDiffHunk(p)))
		}

		L.Push(mod)
		return 1
	}

	L.PreloadModule("ttt.editor", loader)
}

func editorBufferText(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		L.Push(lua.LString(p.Editor.BufferText()))
		return 1
	}
}

func editorBufferLines(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		lines := p.Editor.BufferLines()
		tbl := L.NewTable()
		for _, line := range lines {
			tbl.Append(lua.LString(line))
		}
		L.Push(tbl)
		return 1
	}
}

func editorCurrentLine(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		L.Push(lua.LString(p.Editor.CurrentLine()))
		return 1
	}
}

func editorGetLine(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		n := int(L.CheckNumber(1)) - 1
		L.Push(lua.LString(p.Editor.GetLine(n)))
		return 1
	}
}

func editorLineCount(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNumber(0))
			return 1
		}
		L.Push(lua.LNumber(p.Editor.LineCount()))
		return 1
	}
}

func editorCursor(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		line, col := p.Editor.CursorPos()
		tbl := L.NewTable()
		L.SetField(tbl, "line", lua.LNumber(line+1))
		L.SetField(tbl, "col", lua.LNumber(col+1))
		L.Push(tbl)
		return 1
	}
}

func editorSelection(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		tbl := L.NewTable()
		active, sl, sc, el, ec := p.Editor.Selection()
		L.SetField(tbl, "active", lua.LBool(active))
		L.SetField(tbl, "start_line", lua.LNumber(sl+1))
		L.SetField(tbl, "start_col", lua.LNumber(sc+1))
		L.SetField(tbl, "end_line", lua.LNumber(el+1))
		L.SetField(tbl, "end_col", lua.LNumber(ec+1))
		L.Push(tbl)
		return 1
	}
}

func editorSelectionText(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		L.Push(lua.LString(p.Editor.SelectionText()))
		return 1
	}
}

func editorFilePath(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		L.Push(lua.LString(p.Editor.FilePath()))
		return 1
	}
}

func editorFileName(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		L.Push(lua.LString(p.Editor.FileName()))
		return 1
	}
}

func editorLanguage(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		L.Push(lua.LString(p.Editor.Language()))
		return 1
	}
}

// isUTF8Lead reports whether b is a UTF-8 lead byte (start of a rune), i.e.
// not a 0x80..0xBF continuation byte.
func isUTF8Lead(b byte) bool {
	return b < 0x80 || b >= 0xC0
}

// editorByteToCol converts a 1-based byte offset into a 1-based rune column for
// the given line text. It counts the UTF-8 lead bytes in text[1..b-1] and adds
// one. The offset is clamped to [1, #text+1]. This is a pure helper (no editor
// state) so plugins can map byte offsets from Lua string functions to the
// rune/visual columns the editor APIs expect.
func editorByteToCol(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		text := L.CheckString(1)
		b := L.CheckInt(2)
		if b < 1 {
			b = 1
		}
		if b > len(text)+1 {
			b = len(text) + 1
		}
		col := 1
		for i := 0; i < b-1; i++ {
			if isUTF8Lead(text[i]) {
				col++
			}
		}
		L.Push(lua.LNumber(col))
		return 1
	}
}

// editorColToByte converts a 1-based rune column into the 1-based byte offset
// where that rune starts, for the given line text. It walks the string counting
// lead bytes and clamps the column to [1, runeCount+1] (a column past the end
// maps to #text+1). Inverse of editorByteToCol.
func editorColToByte(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		text := L.CheckString(1)
		c := L.CheckInt(2)
		if c < 1 {
			c = 1
		}
		runes := 0
		for i := 0; i < len(text); i++ {
			if isUTF8Lead(text[i]) {
				runes++
				if runes == c {
					L.Push(lua.LNumber(i + 1))
					return 1
				}
			}
		}
		// Column at or past the end: the (runeCount+1)-th rune starts past the
		// last byte.
		L.Push(lua.LNumber(len(text) + 1))
		return 1
	}
}

func editorViewport(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		top, bottom, height := p.Editor.Viewport()
		tbl := L.NewTable()
		L.SetField(tbl, "top_line", lua.LNumber(top+1))
		L.SetField(tbl, "bottom_line", lua.LNumber(bottom+1))
		L.SetField(tbl, "height", lua.LNumber(height))
		L.Push(tbl)
		return 1
	}
}

func editorScrollTo(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			return 0
		}
		line := int(L.CheckNumber(1)) - 1
		p.Editor.ScrollTo(line)
		return 0
	}
}

func editorScrollBy(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			return 0
		}
		delta := int(L.CheckNumber(1))
		p.Editor.ScrollBy(delta)
		return 0
	}
}

func editorSetLine(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		n := int(L.CheckNumber(1)) - 1
		text := L.CheckString(2)
		p.Editor.SetLine(n, text)
		return 0
	}
}

func editorInsert(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		line := int(L.CheckNumber(1)) - 1
		col := int(L.CheckNumber(2)) - 1
		text := L.CheckString(3)
		p.Editor.Insert(line, col, text)
		return 0
	}
}

func editorReplace(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		sl := int(L.CheckNumber(1)) - 1
		sc := int(L.CheckNumber(2)) - 1
		el := int(L.CheckNumber(3)) - 1
		ec := int(L.CheckNumber(4)) - 1
		text := L.CheckString(5)
		p.Editor.Replace(sl, sc, el, ec, text)
		return 0
	}
}

func editorSetCursor(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		line := int(L.CheckNumber(1)) - 1
		col := int(L.CheckNumber(2)) - 1
		p.Editor.SetCursor(line, col)
		return 0
	}
}

func editorSetSelection(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		sl := int(L.CheckNumber(1)) - 1
		sc := int(L.CheckNumber(2)) - 1
		el := int(L.CheckNumber(3)) - 1
		ec := int(L.CheckNumber(4)) - 1
		p.Editor.SetSelection(sl, sc, el, ec)
		return 0
	}
}

// editorRegisterContextMenu stores a provider function invoked when the editor
// context menu opens. The provider receives (line, col, word) with 1-based
// line/col and must return an array of item tables:
//
//	{ label = "...", on_select = function() ... end }  -- clickable item
//	{ separator = true }                               -- divider
func editorRegisterContextMenu(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		fn := L.CheckFunction(1)
		p.EditorContextProvider = fn
		return 0
	}
}

// EditorContextMenuItems invokes the registered context-menu provider on the
// main thread with the given 1-based line/col and word, returning the entries
// it produced. Each non-separator entry's OnSelect wraps invoking the returned
// Lua closure via the plugin's safe-call path. Returns nil if no provider is
// registered or it produced nothing.
func (p *Plugin) EditorContextMenuItems(line, col int, word string) []ContextMenuEntry {
	if p.State == nil || p.EditorContextProvider == nil {
		return nil
	}

	L := p.State
	err := L.CallByParam(lua.P{
		Fn:      p.EditorContextProvider,
		NRet:    1,
		Protect: true,
	}, lua.LNumber(line), lua.LNumber(col), lua.LString(word))
	if err != nil {
		p.LastError = err
		p.logError("context menu", err)
		return nil
	}

	ret := L.Get(-1)
	L.Pop(1)
	tbl, ok := ret.(*lua.LTable)
	if !ok {
		return nil
	}

	var entries []ContextMenuEntry
	tbl.ForEach(func(_, v lua.LValue) {
		itemTbl, ok := v.(*lua.LTable)
		if !ok {
			return
		}
		if sep := L.GetField(itemTbl, "separator"); sep != lua.LNil && lua.LVAsBool(sep) {
			entries = append(entries, ContextMenuEntry{Separator: true})
			return
		}
		label := ""
		if lv := L.GetField(itemTbl, "label"); lv != lua.LNil {
			label = lv.String()
		}
		if label == "" {
			return
		}
		var onSelect func()
		if fn, ok := L.GetField(itemTbl, "on_select").(*lua.LFunction); ok {
			onSelect = func() { p.CallLuaFunc(fn) }
		}
		entries = append(entries, ContextMenuEntry{Label: label, OnSelect: onSelect})
	})

	return entries
}

func editorClearSelection(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		p.Editor.ClearSelection()
		return 0
	}
}

func editorBeginUndoGroup(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			return 0
		}
		p.Editor.BeginUndoGroup()
		return 0
	}
}

func editorEndUndoGroup(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			return 0
		}
		p.Editor.EndUndoGroup()
		return 0
	}
}

func editorAddCursor(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			return 0
		}
		line := int(L.CheckNumber(1)) - 1
		col := int(L.CheckNumber(2)) - 1
		p.Editor.AddCursor(line, col)
		return 0
	}
}

func editorGetCursors(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		cursors := p.Editor.GetCursors()
		tbl := L.NewTable()
		for _, c := range cursors {
			entry := L.NewTable()
			L.SetField(entry, "line", lua.LNumber(c.Line+1))
			L.SetField(entry, "col", lua.LNumber(c.Col+1))
			tbl.Append(entry)
		}
		L.Push(tbl)
		return 1
	}
}

func editorClearCursors(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			return 0
		}
		p.Editor.ClearCursors()
		return 0
	}
}

func editorSetSearch(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			return 0
		}
		pattern := L.CheckString(1)
		useRegex := false
		if L.GetTop() >= 2 {
			useRegex = L.OptBool(2, false)
		}
		p.Editor.SetSearch(pattern, useRegex)
		return 0
	}
}

func editorClearSearch(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			return 0
		}
		p.Editor.ClearSearch()
		return 0
	}
}

// editorDiff returns the active diff's change blocks and the cursor's
// position among them, or nil when the active tab is not a diff. Plugins read
// this instead of buffer lines because a diff tab has no buffer.
func editorDiff(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			return 1
		}
		hunks := p.Editor.DiffHunks()
		if hunks == nil {
			L.Push(lua.LNil)
			return 1
		}
		tbl := L.NewTable()
		list := L.NewTable()
		for i, h := range hunks {
			item := L.NewTable()
			L.SetField(item, "file_path", lua.LString(h.FilePath))
			L.SetField(item, "kind", lua.LString(h.Kind))
			list.RawSetInt(i+1, item)
		}
		L.SetField(tbl, "hunks", list)
		L.SetField(tbl, "current_hunk", lua.LNumber(p.Editor.DiffHunkAt()))
		L.Push(tbl)
		return 1
	}
}

// editorGoToDiffHunk moves to a 1-based change block in the active diff,
// revealing a collapsed file when needed. It never changes buffer contents.
func editorGoToDiffHunk(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Editor == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("editor API not available"))
			return 2
		}
		index := L.CheckInt(1)
		if err := p.Editor.GoToDiffHunk(index); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}
}
