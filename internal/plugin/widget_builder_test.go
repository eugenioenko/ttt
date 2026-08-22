package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
	lua "github.com/yuin/gopher-lua"
)

func TestReconcileCreatesWidgets(t *testing.T) {
	ws := NewWidgetState()
	p := &Plugin{Name: "test", State: lua.NewState()}
	defer p.State.Close()

	descs := []WidgetDesc{
		{Kind: WidgetLabel, Key: "label:0", Text: "Hello"},
		{Kind: WidgetTree, Key: "tree:0", Items: []*widgets.TreeNode{
			{ID: "a", Label: "Alpha"},
		}},
		{Kind: WidgetButton, Key: "button:0", Label: "OK"},
	}

	root := ws.Reconcile(descs, p)
	if root == nil {
		t.Fatal("expected non-nil root")
	}
	if len(root.Children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(root.Children))
	}

	if _, ok := root.Children[0].(*widgets.LabelWidget); !ok {
		t.Error("expected child 0 to be LabelWidget")
	}
	if _, ok := root.Children[1].(*widgets.TreeWidget); !ok {
		t.Error("expected child 1 to be TreeWidget")
	}
	if _, ok := root.Children[2].(*widgets.ButtonWidget); !ok {
		t.Error("expected child 2 to be ButtonWidget")
	}
}

func TestReconcilePreservesTreeState(t *testing.T) {
	ws := NewWidgetState()
	p := &Plugin{Name: "test", State: lua.NewState()}
	defer p.State.Close()

	descs := []WidgetDesc{
		{Kind: WidgetTree, Key: "tree:0", Items: []*widgets.TreeNode{
			{ID: "a", Label: "Alpha", Expandable: true, Children: []*widgets.TreeNode{
				{ID: "a1", Label: "Alpha-1"},
			}},
			{ID: "b", Label: "Beta"},
		}},
	}

	ws.Reconcile(descs, p)

	tw := ws.items[0].(*widgets.TreeWidget)
	tw.Config.Items[0].Expanded = true

	descs2 := []WidgetDesc{
		{Kind: WidgetTree, Key: "tree:0", Items: []*widgets.TreeNode{
			{ID: "a", Label: "Alpha Updated", Expandable: true, Children: []*widgets.TreeNode{
				{ID: "a1", Label: "Alpha-1"},
				{ID: "a2", Label: "Alpha-2"},
			}},
			{ID: "b", Label: "Beta"},
		}},
	}

	ws.Reconcile(descs2, p)

	tw2 := ws.items[0].(*widgets.TreeWidget)
	if !tw2.Config.Items[0].Expanded {
		t.Error("expected node 'a' to remain expanded after reconcile")
	}
	if tw2.Config.Items[0].Label != "Alpha Updated" {
		t.Error("expected label to be updated")
	}
}

func TestReconcilePreservesInputText(t *testing.T) {
	ws := NewWidgetState()
	p := &Plugin{Name: "test", State: lua.NewState()}
	defer p.State.Close()

	descs := []WidgetDesc{
		{Kind: WidgetInput, Key: "input:0", Placeholder: "Type..."},
	}

	ws.Reconcile(descs, p)

	iw := ws.items[0].(*widgets.InputWidget)
	iw.SetText("user typed this")

	descs2 := []WidgetDesc{
		{Kind: WidgetInput, Key: "input:0", Placeholder: "New placeholder"},
	}

	ws.Reconcile(descs2, p)

	iw2 := ws.items[0].(*widgets.InputWidget)
	if iw2.Text() != "user typed this" {
		t.Errorf("expected text preserved, got %q", iw2.Text())
	}
	if iw2.Config.Placeholder != "New placeholder" {
		t.Errorf("expected placeholder updated, got %q", iw2.Config.Placeholder)
	}
}

func TestReconcileHandlesTypeChange(t *testing.T) {
	ws := NewWidgetState()
	p := &Plugin{Name: "test", State: lua.NewState()}
	defer p.State.Close()

	descs1 := []WidgetDesc{
		{Kind: WidgetLabel, Key: "label:0", Text: "Hello"},
		{Kind: WidgetTree, Key: "tree:0"},
	}
	ws.Reconcile(descs1, p)

	descs2 := []WidgetDesc{
		{Kind: WidgetLabel, Key: "label:0", Text: "Hello"},
		{Kind: WidgetButton, Key: "button:0", Label: "Click"},
	}
	ws.Reconcile(descs2, p)

	if _, ok := ws.items[1].(*widgets.ButtonWidget); !ok {
		t.Error("expected child 1 to be replaced with ButtonWidget")
	}
}

func TestReconcileEmptyDescriptors(t *testing.T) {
	ws := NewWidgetState()
	p := &Plugin{Name: "test", State: lua.NewState()}
	defer p.State.Close()

	root := ws.Reconcile(nil, p)
	if root == nil {
		t.Fatal("expected non-nil root even with empty descriptors")
	}
	if len(root.Children) != 0 {
		t.Errorf("expected 0 children, got %d", len(root.Children))
	}
}

func TestContainersApplyBoxModel(t *testing.T) {
	ws := NewWidgetState()
	p := &Plugin{Name: "test", State: lua.NewState()}
	defer p.State.Close()

	descs := []WidgetDesc{
		{Kind: WidgetVStack, Key: "vstack:0", MarginTop: 1, PaddingLeft: 2},
		{Kind: WidgetHStack, Key: "hstack:0", MarginBottom: 3},
		{Kind: WidgetScrollView, Key: "scrollview:0", PaddingRight: 4},
	}

	ws.Reconcile(descs, p)

	vs := ws.items[0].(*widgets.VStackWidget)
	if vs.Box.MarginTop != 1 || vs.Box.PaddingLeft != 2 {
		t.Errorf("vstack box model not applied on create: %+v", vs.Box)
	}
	hs := ws.items[1].(*widgets.HStackWidget)
	if hs.Box.MarginBottom != 3 {
		t.Errorf("hstack box model not applied on create: %+v", hs.Box)
	}
	sv := ws.items[2].(*widgets.ScrollViewWidget)
	if sv.Box.PaddingRight != 4 {
		t.Errorf("scrollview box model not applied on create: %+v", sv.Box)
	}

	descs2 := []WidgetDesc{
		{Kind: WidgetVStack, Key: "vstack:0", MarginTop: 5},
		{Kind: WidgetHStack, Key: "hstack:0", MarginBottom: 6},
		{Kind: WidgetScrollView, Key: "scrollview:0", PaddingRight: 7},
	}

	ws.Reconcile(descs2, p)

	vs2 := ws.items[0].(*widgets.VStackWidget)
	if vs2.Box.MarginTop != 5 {
		t.Errorf("vstack box model not applied on update: %+v", vs2.Box)
	}
	hs2 := ws.items[1].(*widgets.HStackWidget)
	if hs2.Box.MarginBottom != 6 {
		t.Errorf("hstack box model not applied on update: %+v", hs2.Box)
	}
	sv2 := ws.items[2].(*widgets.ScrollViewWidget)
	if sv2.Box.PaddingRight != 7 {
		t.Errorf("scrollview box model not applied on update: %+v", sv2.Box)
	}
}

func TestReconcileUpdatesLabelWidth(t *testing.T) {
	ws := NewWidgetState()
	p := &Plugin{Name: "test", State: lua.NewState()}
	defer p.State.Close()

	ws.Reconcile([]WidgetDesc{{Kind: WidgetLabel, Key: "label:0", Text: "hi", FixedWidth: 10}}, p)
	lw := ws.items[0].(*widgets.LabelWidget)
	if lw.FixedWidth != 10 {
		t.Fatalf("expected initial width 10, got %d", lw.FixedWidth)
	}

	ws.Reconcile([]WidgetDesc{{Kind: WidgetLabel, Key: "label:0", Text: "hi", FixedWidth: 20}}, p)
	lw2 := ws.items[0].(*widgets.LabelWidget)
	if lw2.FixedWidth != 20 {
		t.Errorf("expected width updated to 20 on reconcile, got %d", lw2.FixedWidth)
	}
}

func TestReconcileUpdatesReusableDropdownStateAndCallback(t *testing.T) {
	ws := NewWidgetState()
	p := &Plugin{Name: "test", State: lua.NewState()}
	defer p.State.Close()

	var shown []widgets.MenuEntry
	var commands []string
	p.ShowContextMenu = func(entries []widgets.MenuEntry, _, _ int, onCommand func(string)) {
		shown = entries
		onCommand(entries[0].Command)
	}

	checked, unchecked := true, false
	states := []struct {
		name    string
		label   string
		checked *bool
	}{
		{name: "checked", label: "Checked", checked: &checked},
		{name: "unchecked", label: "Unchecked", checked: &unchecked},
		{name: "omitted", label: "Omitted"},
		{name: "checked-again", label: "Checked again", checked: &checked},
	}

	var original *widgets.DropdownWidget
	for i, state := range states {
		desc := WidgetDesc{
			Kind:  WidgetDropdown,
			Key:   "dropdown:0",
			Label: state.label,
			Entries: []widgets.MenuEntry{{
				Label:   "界 Mode",
				Command: state.name,
				Checked: state.checked,
			}},
			OnMenu: func(command string) { commands = append(commands, command) },
		}
		ws.Reconcile([]WidgetDesc{desc}, p)

		dd := ws.items[0].(*widgets.DropdownWidget)
		if i == 0 {
			original = dd
		} else if dd != original {
			t.Fatalf("%s reconcile replaced reusable dropdown", state.name)
		}
		if dd.Config.Label != state.label {
			t.Errorf("%s label = %q, want %q", state.name, dd.Config.Label, state.label)
		}
		if dd.Config.OnMenu == nil {
			t.Fatalf("%s callback was not wired", state.name)
		}
		dd.Config.OnMenu(dd.Config.Entries, 0, 0)
		assertMenuEntryChecked(t, state.name, shown[0], state.checked)
		if got := commands[len(commands)-1]; got != state.name {
			t.Errorf("%s callback command = %q, want %q", state.name, got, state.name)
		}
	}
}

func TestReconcileUpdatesReusableTitleAndNodeMenus(t *testing.T) {
	ws := NewWidgetState()
	p := &Plugin{Name: "test", State: lua.NewState()}
	defer p.State.Close()

	var shown []widgets.MenuEntry
	p.ShowContextMenu = func(entries []widgets.MenuEntry, _, _ int, _ func(string)) {
		shown = entries
	}

	checked, unchecked := true, false
	initial := []WidgetDesc{
		{Kind: WidgetTitle, Key: "title:0", Text: "Modes", Entries: []widgets.MenuEntry{{Label: "Mode", Checked: &checked}}},
		{Kind: WidgetTree, Key: "tree:0", Items: []*widgets.TreeNode{{ID: "node", Label: "Node"}}, NodeMenu: []widgets.MenuEntry{{Label: "Mode", Checked: &checked}}},
		{Kind: WidgetList, Key: "list:0", Items: []*widgets.TreeNode{{ID: "node", Label: "Node"}}, NodeMenu: []widgets.MenuEntry{{Label: "Mode", Checked: &checked}}},
		{Kind: WidgetTable, Key: "table:0", Rows: [][]string{{"Row"}}, NodeMenu: []widgets.MenuEntry{{Label: "Mode", Checked: &checked}}},
	}
	ws.Reconcile(initial, p)

	updated := []WidgetDesc{
		{Kind: WidgetTitle, Key: "title:0", Text: "Modes", Entries: []widgets.MenuEntry{{Label: "Mode", Checked: &unchecked}}},
		{Kind: WidgetTree, Key: "tree:0", Items: []*widgets.TreeNode{{ID: "node", Label: "Node"}}},
		{Kind: WidgetList, Key: "list:0", Items: []*widgets.TreeNode{{ID: "node", Label: "Node"}}, NodeMenu: []widgets.MenuEntry{{Label: "Mode"}}},
		{Kind: WidgetTable, Key: "table:0", Rows: [][]string{{"Row"}}, NodeMenu: []widgets.MenuEntry{{Label: "Mode", Checked: &unchecked}}},
	}
	ws.Reconcile(updated, p)

	title := ws.items[0].(*widgets.TitleWidget)
	title.SetRect(widgets.Rect{X: 0, Y: 0, W: 30, H: 1})
	title.Render(newMockSurface(30, 1))
	title.HandleEvent(tcell.NewEventMouse(29, 0, tcell.Button1, tcell.ModNone))
	if len(shown) != 1 {
		t.Fatalf("title menu entries = %d, want 1", len(shown))
	}
	assertMenuEntryChecked(t, "title", shown[0], &unchecked)

	tree := ws.items[1].(*widgets.TreeWidget)
	if len(tree.Config.NodeMenu) != 0 || tree.Config.OnMenu != nil {
		t.Errorf("tree retained removed node menu: entries=%v callback=%v", tree.Config.NodeMenu, tree.Config.OnMenu != nil)
	}
	list := ws.items[2].(*widgets.TreeWidget)
	assertMenuEntryChecked(t, "list", list.Config.NodeMenu[0], nil)
	table := ws.items[3].(*widgets.TableWidget)
	assertMenuEntryChecked(t, "table", table.Config.NodeMenu[0], &unchecked)
}

func assertMenuEntryChecked(t *testing.T, surface string, entry widgets.MenuEntry, want *bool) {
	t.Helper()
	if want == nil {
		if entry.Checked != nil {
			t.Errorf("%s checked = %v, want omitted", surface, *entry.Checked)
		}
		return
	}
	if entry.Checked == nil || *entry.Checked != *want {
		t.Errorf("%s checked = %v, want %v", surface, entry.Checked, *want)
	}
}
