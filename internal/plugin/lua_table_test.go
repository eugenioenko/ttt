package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	lua "github.com/yuin/gopher-lua"
)

func TestLuaTableOptions(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	err := p.State.DoString(`
		local ttt = require("ttt")
		activated = nil
		ttt.register({ sidebar = { title = "T", render = function(panel)
			panel:table({
				columns = {{ label = "Name" }},
				rows = {{ "a" }, { "b" }, { "c" }},
				row_styles = { nil, "muted" },
				selected = 3,
				loading = true,
				empty = "No keys yet",
				inset = 1,
				on_activate = function(i) activated = i end,
			})
		end } })
	`)
	if err != nil {
		t.Fatal(err)
	}
	proxy := NewPanelProxy(newMockSurface(80, 20), p)
	if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
		t.Fatal(err)
	}
	tbl := NewWidgetState().Reconcile(proxy.Descriptors(), p).Children[0].(*widgets.TableWidget)

	if tbl.SelectedIndex() != 2 {
		t.Errorf("selected = %d, want row 3", tbl.SelectedIndex())
	}
	if len(tbl.Config.RowStyles) != 2 || tbl.Config.RowStyles[0] != 0 || tbl.Config.RowStyles[1] != term.StyleMuted {
		t.Errorf("row styles = %v", tbl.Config.RowStyles)
	}
	if !tbl.Config.Loading || tbl.Config.EmptyText != "No keys yet" || tbl.Config.Inset != 1 {
		t.Errorf("loading %v, empty %q, inset %d", tbl.Config.Loading, tbl.Config.EmptyText, tbl.Config.Inset)
	}
	tbl.Config.OnActivate(1)
	if v := p.State.GetGlobal("activated"); v != lua.LNumber(2) {
		t.Errorf("on_activate got %v, want 1-based row 2", v)
	}
}

func TestLuaTableSelectionIsControlledOnlyWhenGiven(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	err := p.State.DoString(`
		local ttt = require("ttt")
		sel = nil
		ttt.register({ sidebar = { title = "T", render = function(panel)
			panel:table({ columns = {{ label = "N" }}, rows = {{ "a" }, { "b" }, { "c" }}, selected = sel })
		end } })
	`)
	if err != nil {
		t.Fatal(err)
	}
	ws := NewWidgetState()
	render := func() *widgets.TableWidget {
		proxy := NewPanelProxy(newMockSurface(80, 20), p)
		if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
			t.Fatal(err)
		}
		return ws.Reconcile(proxy.Descriptors(), p).Children[0].(*widgets.TableWidget)
	}
	tbl := render()
	tbl.SetSelectedIndex(1)
	if tbl = render(); tbl.SelectedIndex() != 1 {
		t.Fatalf("without selected the table keeps its own row, got %d", tbl.SelectedIndex())
	}
	p.State.SetGlobal("sel", lua.LNumber(1))
	if tbl = render(); tbl.SelectedIndex() != 0 {
		t.Fatalf("selected = 1 should move to the first row, got %d", tbl.SelectedIndex())
	}
}

func TestLuaTableSelectionClampsWhenRowsShrink(t *testing.T) {
	p, done := newTestPluginBase(PermissionSet{PanelSidebar: true})
	defer done()
	err := p.State.DoString(`
		local ttt = require("ttt")
		rows = {{ "a" }, { "b" }, { "c" }}
		ttt.register({ sidebar = { title = "T", render = function(panel)
			panel:table({ columns = {{ label = "N" }}, rows = rows, selected = 3 })
		end } })
	`)
	if err != nil {
		t.Fatal(err)
	}
	ws := NewWidgetState()
	render := func() *widgets.TableWidget {
		proxy := NewPanelProxy(newMockSurface(80, 20), p)
		if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
			t.Fatal(err)
		}
		return ws.Reconcile(proxy.Descriptors(), p).Children[0].(*widgets.TableWidget)
	}
	render()
	if err := p.State.DoString(`rows = {{ "a" }}`); err != nil {
		t.Fatal(err)
	}
	if got := render().SelectedIndex(); got != 0 {
		t.Fatalf("selected = %d after the rows shrank to one, want 0", got)
	}
}
