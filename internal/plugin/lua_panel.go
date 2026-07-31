package plugin

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

const panelTypeName = "panel"

type PanelProxy struct {
	surface     widgets.Surface
	plugin      *Plugin
	descs       []WidgetDesc
	usedRaw     bool
	usedWidgets bool
	descCounts  map[WidgetKind]int
}

func (pp *PanelProxy) wrapSimpleCallback(fn *lua.LFunction) func() {
	p := pp.plugin
	return func() {
		p.CallLuaFunc(fn)
	}
}

func (pp *PanelProxy) wrapBoolCallback(fn *lua.LFunction) func(bool) {
	p := pp.plugin
	return func(b bool) {
		if p.State != nil {
			p.CallLuaFunc(fn, lua.LBool(b))
		}
	}
}

func (pp *PanelProxy) wrapStringCallback(fn *lua.LFunction) func(string) {
	p := pp.plugin
	return func(s string) {
		if p.State != nil {
			p.CallLuaFunc(fn, lua.LString(s))
		}
	}
}

func (pp *PanelProxy) wrapNodeCallback(fn *lua.LFunction) func(*widgets.TreeNode) {
	p := pp.plugin
	return func(node *widgets.TreeNode) {
		if p.State != nil {
			tbl := TreeNodeToLua(p.State, node)
			p.CallLuaFunc(fn, tbl)
		}
	}
}

func (pp *PanelProxy) wrapCommandCallback(fn *lua.LFunction) func(string, *widgets.TreeNode) {
	p := pp.plugin
	return func(command string, node *widgets.TreeNode) {
		if p.State != nil {
			tbl := TreeNodeToLua(p.State, node)
			p.CallLuaFunc(fn, lua.LString(command), tbl)
		}
	}
}

func NewPanelProxy(surface widgets.Surface, plugin *Plugin) *PanelProxy {
	return &PanelProxy{
		surface:    surface,
		plugin:     plugin,
		descCounts: make(map[WidgetKind]int),
	}
}

func (pp *PanelProxy) Descriptors() []WidgetDesc { return pp.descs }
func (pp *PanelProxy) UsedWidgets() bool         { return pp.usedWidgets }
func (pp *PanelProxy) UsedRaw() bool             { return pp.usedRaw }

func (pp *PanelProxy) appendDesc(kind WidgetKind, desc WidgetDesc) {
	idx := pp.descCounts[kind]
	pp.descCounts[kind] = idx + 1
	desc.Kind = kind
	desc.Key = fmt.Sprintf("%s:%d", kind, idx)
	pp.descs = append(pp.descs, desc)
	pp.usedWidgets = true
}

func RegisterPanelType(L *lua.LState) {
	mt := L.NewTypeMetatable(panelTypeName)
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"size":       panelSize,
		"cell":       panelCell,
		"text":       panelText,
		"clear":      panelClear,
		"label":      panelLabelWidget,
		"tree":       panelTreeWidget,
		"list":       panelListWidget,
		"button":     panelButtonWidget,
		"input":      panelInputWidget,
		"vstack":     panelVStackWidget,
		"box":        panelBoxWidget,
		"dropdown":   panelDropdownWidget,
		"title":      panelTitleWidget,
		"keyvalue":   panelKeyValueWidget,
		"scrollview": panelScrollViewWidget,
		"hstack":     panelHStackWidget,
		"divider":    panelDividerWidget,
		"progress":   panelProgressWidget,
		"table":      panelTableWidget,
		"checkbox":   panelCheckboxWidget,
		"redraw":     panelRedraw,
		"markdown":   panelMarkdownWidget,
	}))
}

func PushPanelProxy(L *lua.LState, proxy *PanelProxy) *lua.LUserData {
	ud := L.NewUserData()
	ud.Value = proxy
	L.SetMetatable(ud, L.GetTypeMetatable(panelTypeName))
	return ud
}

func checkPanelProxy(L *lua.LState) *PanelProxy {
	ud := L.CheckUserData(1)
	if proxy, ok := ud.Value.(*PanelProxy); ok {
		return proxy
	}
	L.ArgError(1, "panel expected")
	return nil
}

func resolveStyle(L *lua.LState, argPos int) term.Style {
	v := L.Get(argPos)
	if v == nil || v == lua.LNil {
		return term.StyleDefault
	}

	if tbl, ok := v.(*lua.LTable); ok {
		if s := L.GetField(tbl, "style"); s != lua.LNil {
			if mapped, ok := StyleByName(s.String()); ok {
				return mapped
			}
		}
	}

	if str, ok := v.(lua.LString); ok {
		if mapped, ok := StyleByName(string(str)); ok {
			return mapped
		}
	}

	return term.StyleDefault
}

func resolveStyleName(name string) term.Style {
	if s, ok := StyleByName(name); ok {
		return s
	}
	return term.StyleDefault
}

// Raw cell API

func panelSize(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}
	w, h := proxy.surface.Size()
	L.Push(lua.LNumber(w))
	L.Push(lua.LNumber(h))
	return 2
}

func panelCell(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}
	proxy.usedRaw = true
	x := int(L.CheckNumber(2))
	y := int(L.CheckNumber(3))
	ch := L.CheckString(4)
	style := resolveStyle(L, 5)

	runes := []rune(ch)
	if len(runes) > 0 {
		proxy.surface.SetCell(x, y, term.Cell{Ch: runes[0], Style: style})
	}
	return 0
}

func panelText(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}
	proxy.usedRaw = true
	x := int(L.CheckNumber(2))
	y := int(L.CheckNumber(3))
	text := L.CheckString(4)
	style := resolveStyle(L, 5)

	w, _ := proxy.surface.Size()
	proxy.surface.DrawText(x, y, text, w-x, style)
	return 0
}

func panelClear(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}
	proxy.usedRaw = true
	x := int(L.CheckNumber(2))
	y := int(L.CheckNumber(3))
	w := int(L.CheckNumber(4))
	h := int(L.CheckNumber(5))

	proxy.surface.ClearRect(x, y, w, h, term.StyleDefault)
	return 0
}

// Widget helpers

func panelLabelWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	desc := WidgetDesc{}
	arg := L.Get(2)

	switch v := arg.(type) {
	case lua.LString:
		desc.Text = string(v)
	case *lua.LTable:
		if t := L.GetField(v, "text"); t != lua.LNil {
			desc.Text = t.String()
		}
		if s := L.GetField(v, "style"); s != lua.LNil {
			desc.TextStyle = s.String()
		}
		if b := L.GetField(v, "badge"); b != lua.LNil {
			desc.Badge = b.String()
		}
		if w := L.GetField(v, "width"); w != lua.LNil {
			desc.FixedWidth = int(lua.LVAsNumber(w))
		}
		parseBorders(L, v, &desc)
		parseBoxModel(L, v, &desc)
	default:
		desc.Text = arg.String()
	}

	proxy.appendDesc(WidgetLabel, desc)
	return 0
}

func panelTitleWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	desc := WidgetDesc{}
	arg := L.Get(2)

	switch v := arg.(type) {
	case lua.LString:
		desc.Text = string(v)
	case *lua.LTable:
		if t := L.GetField(v, "text"); t != lua.LNil {
			desc.Text = t.String()
		}
		if b := L.GetField(v, "badge"); b != lua.LNil {
			desc.Badge = b.String()
		}
		if entries, ok := L.GetField(v, "menu").(*lua.LTable); ok {
			desc.Entries = parseLuaMenuEntries(L, entries)
		}
		if fn, ok := L.GetField(v, "on_menu").(*lua.LFunction); ok {
			desc.OnMenu = proxy.wrapStringCallback(fn)
		}
		if icon := L.GetField(v, "icon"); icon != lua.LNil {
			desc.Icon = icon.String()
		}
		if padded := L.GetField(v, "padded"); padded == lua.LTrue {
			desc.Padded = true
		}
		parseBoxModel(L, v, &desc)
	default:
		desc.Text = arg.String()
	}

	proxy.appendDesc(WidgetTitle, desc)
	return 0
}

func panelKeyValueWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	desc := WidgetDesc{}
	tbl := L.CheckTable(2)

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
		desc.KeyValueEntries = append(desc.KeyValueEntries, entry)
	})

	parseBoxModel(L, tbl, &desc)
	proxy.appendDesc(WidgetKeyValue, desc)
	return 0
}

func panelTreeWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	tbl := L.CheckTable(2)
	desc := WidgetDesc{Indent: 2}

	if items, ok := L.GetField(tbl, "items").(*lua.LTable); ok {
		desc.Items = LuaTableToTreeNodes(L, items)
	}
	if v := L.GetField(tbl, "indent"); v != lua.LNil {
		desc.Indent = int(lua.LVAsNumber(v))
	}
	if fn, ok := L.GetField(tbl, "on_select").(*lua.LFunction); ok {
		desc.OnSelect = proxy.wrapNodeCallback(fn)
	}
	if fn, ok := L.GetField(tbl, "on_expand").(*lua.LFunction); ok {
		desc.OnExpand = proxy.wrapNodeCallback(fn)
	}
	if fn, ok := L.GetField(tbl, "on_command").(*lua.LFunction); ok {
		desc.OnCommand = proxy.wrapCommandCallback(fn)
	}
	if menu, ok := L.GetField(tbl, "node_menu").(*lua.LTable); ok {
		desc.NodeMenu = parseLuaMenuEntries(L, menu)
	}
	if kc, ok := L.GetField(tbl, "key_commands").(*lua.LTable); ok {
		desc.KeyCommands = parseLuaKeyCommands(L, kc)
	}
	if v := L.GetField(tbl, "select_on_click"); v == lua.LTrue {
		desc.SelectOnClick = true
	}
	if v := L.GetField(tbl, "truncate_left"); v == lua.LTrue {
		desc.TruncateLeft = true
	}

	parseBoxModel(L, tbl, &desc)
	proxy.appendDesc(WidgetTree, desc)
	return 0
}

func panelListWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	tbl := L.CheckTable(2)
	desc := WidgetDesc{}

	if items, ok := L.GetField(tbl, "items").(*lua.LTable); ok {
		desc.Items = LuaTableToTreeNodes(L, items)
	}
	if fn, ok := L.GetField(tbl, "on_select").(*lua.LFunction); ok {
		desc.OnSelect = proxy.wrapNodeCallback(fn)
	}
	if fn, ok := L.GetField(tbl, "on_command").(*lua.LFunction); ok {
		desc.OnCommand = proxy.wrapCommandCallback(fn)
	}
	if menu, ok := L.GetField(tbl, "node_menu").(*lua.LTable); ok {
		desc.NodeMenu = parseLuaMenuEntries(L, menu)
	}
	if kc, ok := L.GetField(tbl, "key_commands").(*lua.LTable); ok {
		desc.KeyCommands = parseLuaKeyCommands(L, kc)
	}
	if v := L.GetField(tbl, "select_on_click"); v == lua.LTrue {
		desc.SelectOnClick = true
	}
	if v := L.GetField(tbl, "truncate_left"); v == lua.LTrue {
		desc.TruncateLeft = true
	}

	parseBoxModel(L, tbl, &desc)
	proxy.appendDesc(WidgetList, desc)
	return 0
}

func panelButtonWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	tbl := L.CheckTable(2)
	desc := WidgetDesc{}

	if v := L.GetField(tbl, "label"); v != lua.LNil {
		desc.Label = v.String()
	}
	if fn, ok := L.GetField(tbl, "on_click").(*lua.LFunction); ok {
		desc.OnClick = proxy.wrapSimpleCallback(fn)
	}

	parseBoxModel(L, tbl, &desc)
	proxy.appendDesc(WidgetButton, desc)
	return 0
}

func panelCheckboxWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	tbl := L.CheckTable(2)
	desc := WidgetDesc{}

	if v := L.GetField(tbl, "label"); v != lua.LNil {
		desc.Label = v.String()
	}
	if v := L.GetField(tbl, "checked"); v != lua.LNil {
		desc.Checked = lua.LVAsBool(v)
	}
	if v := L.GetField(tbl, "style"); v != lua.LNil {
		desc.StyleName = v.String()
	}
	if fn, ok := L.GetField(tbl, "on_change").(*lua.LFunction); ok {
		desc.OnChangeBool = proxy.wrapBoolCallback(fn)
	}

	parseBoxModel(L, tbl, &desc)
	proxy.appendDesc(WidgetCheckbox, desc)
	return 0
}

func panelInputWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	tbl := L.CheckTable(2)
	desc := WidgetDesc{}

	if v := L.GetField(tbl, "placeholder"); v != lua.LNil {
		desc.Placeholder = v.String()
	}
	if v := L.GetField(tbl, "prefix"); v != lua.LNil {
		desc.Prefix = v.String()
	}
	if fn, ok := L.GetField(tbl, "on_change").(*lua.LFunction); ok {
		desc.OnChange = proxy.wrapStringCallback(fn)
	}
	if fn, ok := L.GetField(tbl, "on_submit").(*lua.LFunction); ok {
		desc.OnSubmit = proxy.wrapStringCallback(fn)
	}
	if v := L.GetField(tbl, "clear_on_submit"); v != lua.LNil {
		desc.ClearOnSubmit = lua.LVAsBool(v)
	}

	parseBoxModel(L, tbl, &desc)
	proxy.appendDesc(WidgetInput, desc)
	return 0
}

func parseBorders(L *lua.LState, tbl *lua.LTable, desc *WidgetDesc) {
	if v := L.GetField(tbl, "border"); v != lua.LNil {
		desc.Border = lua.LVAsBool(v)
	}
	for _, field := range []struct {
		name string
		dst  *bool
	}{
		{"border_top", &desc.BorderTop},
		{"border_bottom", &desc.BorderBottom},
		{"border_left", &desc.BorderLeft},
		{"border_right", &desc.BorderRight},
	} {
		if v := L.GetField(tbl, field.name); v != lua.LNil {
			*field.dst = lua.LVAsBool(v)
		}
	}
}

func parseBoxModel(L *lua.LState, tbl *lua.LTable, desc *WidgetDesc) {
	for _, field := range []struct {
		name string
		dst  *int
	}{
		{"margin_top", &desc.MarginTop},
		{"margin_bottom", &desc.MarginBottom},
		{"margin_left", &desc.MarginLeft},
		{"margin_right", &desc.MarginRight},
		{"padding_top", &desc.PaddingTop},
		{"padding_bottom", &desc.PaddingBottom},
		{"padding_left", &desc.PaddingLeft},
		{"padding_right", &desc.PaddingRight},
	} {
		if v := L.GetField(tbl, field.name); v != lua.LNil {
			*field.dst = int(lua.LVAsNumber(v))
		}
	}
}

func parseLuaKeyCommands(_ *lua.LState, tbl *lua.LTable) map[rune]string {
	m := map[rune]string{}
	tbl.ForEach(func(k, v lua.LValue) {
		key := k.String()
		cmd := v.String()
		if len(key) == 1 && cmd != "" {
			m[rune(key[0])] = cmd
		}
	})
	return m
}

func parseLuaMenuEntries(L *lua.LState, tbl *lua.LTable) []widgets.MenuEntry {
	var entries []widgets.MenuEntry
	tbl.ForEach(func(_, v lua.LValue) {
		entry, ok := v.(*lua.LTable)
		if !ok {
			return
		}
		me := widgets.MenuEntry{}
		if label := L.GetField(entry, "label"); label != lua.LNil {
			me.Label = label.String()
		}
		if cmd := L.GetField(entry, "command"); cmd != lua.LNil {
			me.Command = cmd.String()
		}
		if sep := L.GetField(entry, "separator"); sep != lua.LNil {
			me.Separator = lua.LVAsBool(sep)
		}
		entries = append(entries, me)
	})
	return entries
}

func collectChildren(L *lua.LState, proxy *PanelProxy, fn *lua.LFunction) []WidgetDesc {
	child := &PanelProxy{
		surface:    proxy.surface,
		plugin:     proxy.plugin,
		descCounts: make(map[WidgetKind]int),
	}
	ud := PushPanelProxy(L, child)
	if err := L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, ud); err != nil {
		proxy.plugin.logError("widget builder", err)
	}
	return child.descs
}

func panelVStackWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	tbl := L.CheckTable(2)
	desc := WidgetDesc{}

	if fn, ok := L.GetField(tbl, "render").(*lua.LFunction); ok {
		desc.Children = collectChildren(L, proxy, fn)
	}
	if v := L.GetField(tbl, "gap"); v != lua.LNil {
		desc.Gap = int(lua.LVAsNumber(v))
	}

	parseBoxModel(L, tbl, &desc)
	proxy.appendDesc(WidgetVStack, desc)
	return 0
}

func panelHStackWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	tbl := L.CheckTable(2)
	desc := WidgetDesc{}

	if fn, ok := L.GetField(tbl, "render").(*lua.LFunction); ok {
		desc.Children = collectChildren(L, proxy, fn)
	}
	if v := L.GetField(tbl, "gap"); v != lua.LNil {
		desc.Gap = int(lua.LVAsNumber(v))
	}
	if v := L.GetField(tbl, "height"); v != lua.LNil {
		desc.FixedHeight = int(lua.LVAsNumber(v))
	}

	parseBoxModel(L, tbl, &desc)
	proxy.appendDesc(WidgetHStack, desc)
	return 0
}

func panelDividerWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	desc := WidgetDesc{}
	proxy.appendDesc(WidgetDivider, desc)
	return 0
}

func panelScrollViewWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	tbl := L.CheckTable(2)
	desc := WidgetDesc{}

	if fn, ok := L.GetField(tbl, "render").(*lua.LFunction); ok {
		desc.Children = collectChildren(L, proxy, fn)
	}

	parseBoxModel(L, tbl, &desc)
	proxy.appendDesc(WidgetScrollView, desc)
	return 0
}

func panelBoxWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	tbl := L.CheckTable(2)
	desc := WidgetDesc{}

	if fn, ok := L.GetField(tbl, "render").(*lua.LFunction); ok {
		desc.Children = collectChildren(L, proxy, fn)
	}
	parseBorders(L, tbl, &desc)
	if v := L.GetField(tbl, "height"); v != lua.LNil {
		desc.FixedHeight = int(lua.LVAsNumber(v))
	}
	parseBoxModel(L, tbl, &desc)

	proxy.appendDesc(WidgetBox, desc)
	return 0
}

func panelDropdownWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	tbl := L.CheckTable(2)
	desc := WidgetDesc{}

	if v := L.GetField(tbl, "label"); v != lua.LNil {
		desc.Label = v.String()
	}

	if entries, ok := L.GetField(tbl, "entries").(*lua.LTable); ok {
		desc.Entries = parseLuaMenuEntries(L, entries)
	}

	if fn, ok := L.GetField(tbl, "on_menu").(*lua.LFunction); ok {
		desc.OnMenu = proxy.wrapStringCallback(fn)
	}

	parseBoxModel(L, tbl, &desc)
	proxy.appendDesc(WidgetDropdown, desc)
	return 0
}

func panelProgressWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	tbl := L.CheckTable(2)
	desc := WidgetDesc{}

	if v := L.GetField(tbl, "value"); v != lua.LNil {
		desc.Value = float64(lua.LVAsNumber(v))
	}
	if v := L.GetField(tbl, "style"); v != lua.LNil {
		desc.StyleName = v.String()
	}
	if v := L.GetField(tbl, "char"); v != lua.LNil {
		s := v.String()
		if len([]rune(s)) > 0 {
			desc.Char = []rune(s)[0]
		}
	}

	parseBoxModel(L, tbl, &desc)
	proxy.appendDesc(WidgetProgress, desc)
	return 0
}

func panelTableWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	tbl := L.CheckTable(2)
	desc := WidgetDesc{}

	if cols, ok := L.GetField(tbl, "columns").(*lua.LTable); ok {
		cols.ForEach(func(_, v lua.LValue) {
			ct, ok := v.(*lua.LTable)
			if !ok {
				return
			}
			col := widgets.TableColumn{
				Label: L.GetField(ct, "label").String(),
			}
			if w := L.GetField(ct, "width"); w != lua.LNil {
				col.Width = int(lua.LVAsNumber(w))
			}
			if a := L.GetField(ct, "align"); a != lua.LNil {
				col.Align = a.String()
			}
			desc.Columns = append(desc.Columns, col)
		})
	}

	if rows, ok := L.GetField(tbl, "rows").(*lua.LTable); ok {
		rows.ForEach(func(_, rv lua.LValue) {
			rt, ok := rv.(*lua.LTable)
			if !ok {
				return
			}
			var row []string
			rt.ForEach(func(_, cv lua.LValue) {
				row = append(row, cv.String())
			})
			desc.Rows = append(desc.Rows, row)
		})
	}

	if fn, ok := L.GetField(tbl, "on_select").(*lua.LFunction); ok {
		luaFn := fn
		desc.OnSelectIndex = func(rowIndex int) {
			if proxy.plugin != nil && proxy.plugin.State != nil {
				proxy.plugin.CallLuaFunc(luaFn, lua.LNumber(rowIndex+1))
			}
		}
	}
	if fn, ok := L.GetField(tbl, "on_command").(*lua.LFunction); ok {
		luaFn := fn
		desc.OnCommandStr = func(command string, rowIndex int) {
			if proxy.plugin != nil && proxy.plugin.State != nil {
				proxy.plugin.CallLuaFunc(luaFn, lua.LString(command), lua.LNumber(rowIndex+1))
			}
		}
	}
	if menu, ok := L.GetField(tbl, "node_menu").(*lua.LTable); ok {
		desc.NodeMenu = parseLuaMenuEntries(L, menu)
	}
	if kc, ok := L.GetField(tbl, "key_commands").(*lua.LTable); ok {
		desc.KeyCommands = parseLuaKeyCommands(L, kc)
	}

	parseBoxModel(L, tbl, &desc)
	proxy.appendDesc(WidgetTable, desc)
	return 0
}

func panelMarkdownWidget(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}

	desc := WidgetDesc{}
	arg := L.Get(2)
	switch v := arg.(type) {
	case lua.LString:
		desc.MarkdownContent = string(v)
	case *lua.LTable:
		if t := L.GetField(v, "text"); t != lua.LNil {
			desc.MarkdownContent = t.String()
		}
		parseBoxModel(L, v, &desc)
	}
	proxy.appendDesc(WidgetMarkdown, desc)
	return 0
}

func panelRedraw(L *lua.LState) int {
	proxy := checkPanelProxy(L)
	if proxy == nil {
		return 0
	}
	if proxy.plugin != nil && proxy.plugin.RequestRedraw != nil {
		proxy.plugin.RequestRedraw()
	}
	return 0
}
