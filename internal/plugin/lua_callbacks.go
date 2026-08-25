package plugin

import (
	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

func (p *Plugin) CallLuaFunc(fn *lua.LFunction, args ...lua.LValue) error {
	if p.State == nil || fn == nil {
		return nil
	}
	err := p.State.CallByParam(lua.P{
		Fn:      fn,
		NRet:    0,
		Protect: true,
	}, args...)
	if err != nil {
		p.LastError = err
		p.logError("callback", err)
	}
	return err
}

func (p *Plugin) CallOnInstall() {
	if p.InstallFunc != nil {
		p.CallLuaFunc(p.InstallFunc)
	}
}

func TreeNodeToLua(L *lua.LState, node *widgets.TreeNode) *lua.LTable {
	tbl := L.NewTable()
	L.SetField(tbl, "id", lua.LString(node.ID))
	L.SetField(tbl, "label", lua.LString(node.Label))
	if node.Icon != "" {
		L.SetField(tbl, "icon", lua.LString(node.Icon))
	}
	if node.Badge != "" {
		L.SetField(tbl, "badge", lua.LString(node.Badge))
	}
	L.SetField(tbl, "expanded", lua.LBool(node.Expanded))
	L.SetField(tbl, "muted", lua.LBool(node.Muted))

	if len(node.Children) > 0 {
		children := L.NewTable()
		for _, child := range node.Children {
			children.Append(TreeNodeToLua(L, child))
		}
		L.SetField(tbl, "children", children)
	}
	return tbl
}

func LuaTableToTreeNodes(L *lua.LState, tbl *lua.LTable) []*widgets.TreeNode {
	var nodes []*widgets.TreeNode
	tbl.ForEach(func(_, v lua.LValue) {
		if item, ok := v.(*lua.LTable); ok {
			nodes = append(nodes, luaTableToTreeNode(L, item))
		}
	})
	return nodes
}

func luaTableToTreeNode(L *lua.LState, tbl *lua.LTable) *widgets.TreeNode {
	node := &widgets.TreeNode{}

	if v := L.GetField(tbl, "id"); v != lua.LNil {
		node.ID = v.String()
	}
	if v := L.GetField(tbl, "label"); v != lua.LNil {
		node.Label = v.String()
	}
	if v := L.GetField(tbl, "icon"); v != lua.LNil {
		node.Icon = v.String()
	}
	if v := L.GetField(tbl, "badge"); v != lua.LNil {
		node.Badge = v.String()
	}
	if v := L.GetField(tbl, "muted"); v != lua.LNil {
		node.Muted = lua.LVAsBool(v)
	}
	if v := L.GetField(tbl, "expanded"); v != lua.LNil {
		node.Expanded = lua.LVAsBool(v)
	}
	if v := L.GetField(tbl, "expandable"); v != lua.LNil {
		node.Expandable = lua.LVAsBool(v)
	}

	if v := L.GetField(tbl, "icon_style"); v != lua.LNil {
		if s, ok := StyleByName(v.String()); ok {
			node.IconStyle = s
		}
	}

	if actions, ok := L.GetField(tbl, "actions").(*lua.LTable); ok {
		actions.ForEach(func(_, v lua.LValue) {
			at, ok := v.(*lua.LTable)
			if !ok {
				return
			}
			action := widgets.Action{}
			if icon := L.GetField(at, "icon"); icon != lua.LNil {
				action.Icon = icon.String()
			}
			if command := L.GetField(at, "command"); command != lua.LNil {
				action.Command = command.String()
			}
			node.Actions = append(node.Actions, action)
		})
	}

	if children, ok := L.GetField(tbl, "children").(*lua.LTable); ok {
		node.Children = LuaTableToTreeNodes(L, children)
		if len(node.Children) > 0 {
			node.Expandable = true
		}
	}

	return node
}
