package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

func TestCallLuaFuncProtected(t *testing.T) {
	p := &Plugin{Name: "test"}
	p.State = lua.NewState()
	defer p.State.Close()

	fn := p.State.NewFunction(func(L *lua.LState) int {
		L.RaiseError("intentional")
		return 0
	})

	err := p.CallLuaFunc(fn)
	if err == nil {
		t.Fatal("expected error from crashing function")
	}
	if p.LastError == nil {
		t.Error("expected LastError to be set")
	}
}

func TestCallLuaFuncNil(t *testing.T) {
	p := &Plugin{Name: "test"}
	err := p.CallLuaFunc(nil)
	if err != nil {
		t.Errorf("expected no error for nil func, got %v", err)
	}
}

func TestTreeNodeToLuaRoundtrip(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	node := &widgets.TreeNode{
		ID:       "root",
		Label:    "Root Node",
		Icon:     "📁",
		Badge:    "3",
		Expanded: true,
		Children: []*widgets.TreeNode{
			{ID: "child1", Label: "Child 1"},
			{ID: "child2", Label: "Child 2", Muted: true},
		},
	}

	tbl := TreeNodeToLua(L, node)

	if tbl.RawGetString("id").String() != "root" {
		t.Error("expected id=root")
	}
	if tbl.RawGetString("label").String() != "Root Node" {
		t.Error("expected label=Root Node")
	}
	if tbl.RawGetString("icon").String() != "📁" {
		t.Error("expected icon")
	}

	children := tbl.RawGetString("children").(*lua.LTable)
	if children.Len() != 2 {
		t.Fatalf("expected 2 children, got %d", children.Len())
	}
}

func TestLuaTableToTreeNodes(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	err := L.DoString(`
		items = {
			{ id = "a", label = "Alpha", icon = "📁", children = {
				{ id = "a1", label = "Alpha-1" },
			}},
			{ id = "b", label = "Beta", muted = true },
		}
	`)
	if err != nil {
		t.Fatalf("lua error: %v", err)
	}

	tbl := L.GetGlobal("items").(*lua.LTable)
	nodes := LuaTableToTreeNodes(L, tbl)

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].ID != "a" || nodes[0].Label != "Alpha" {
		t.Errorf("node 0: got id=%s label=%s", nodes[0].ID, nodes[0].Label)
	}
	if len(nodes[0].Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(nodes[0].Children))
	}
	if nodes[0].Children[0].ID != "a1" {
		t.Errorf("child id: expected a1, got %s", nodes[0].Children[0].ID)
	}
	if !nodes[1].Muted {
		t.Error("expected node b to be muted")
	}
}

func TestLuaTableToTreeNodesParsesActions(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	err := L.DoString(`
		items = {
			{ id = "a", label = "Alpha", actions = {
				{ icon = "✕", command = "kill" },
			}},
		}
	`)
	if err != nil {
		t.Fatalf("lua error: %v", err)
	}

	tbl := L.GetGlobal("items").(*lua.LTable)
	nodes := LuaTableToTreeNodes(L, tbl)

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if len(nodes[0].Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(nodes[0].Actions))
	}
	action := nodes[0].Actions[0]
	if action.Icon != "✕" || action.Command != "kill" {
		t.Errorf("got action %+v, want {Icon: ✕, Command: kill}", action)
	}
}
