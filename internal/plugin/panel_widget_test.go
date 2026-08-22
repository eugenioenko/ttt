package plugin

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type mockSurface struct {
	w, h  int
	cells map[[2]int]term.Cell
	texts []mockText
}

type mockText struct {
	x, y  int
	text  string
	style term.Style
}

func newMockSurface(w, h int) *mockSurface {
	return &mockSurface{w: w, h: h, cells: make(map[[2]int]term.Cell)}
}

func (s *mockSurface) Size() (int, int)                                              { return s.w, s.h }
func (s *mockSurface) Origin() (int, int)                                            { return 0, 0 }
func (s *mockSurface) SetCell(x, y int, c term.Cell)                                 { s.cells[[2]int{x, y}] = c }
func (s *mockSurface) DrawBorder(x, y, w, h int, b term.BorderSet, style term.Style) {}
func (s *mockSurface) ClearRect(x, y, w, h int, style term.Style)                    {}
func (s *mockSurface) Fill(c term.Cell)                                              {}
func (s *mockSurface) Sub(r widgets.Rect) widgets.Surface                            { return s }

func (s *mockSurface) DrawText(x, y int, text string, maxW int, style term.Style) int {
	s.texts = append(s.texts, mockText{x, y, text, style})
	return len([]rune(text))
}

func TestPluginPanelWidgetRender(t *testing.T) {
	p := &Plugin{
		Name:    "test",
		Granted: PermissionSet{PanelSidebar: true},
	}
	p.State = NewSandbox()
	defer p.State.Close()
	setupTTTModule(p.State, p)

	err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.register({
			sidebar = {
				title = "Test",
				render = function(panel)
					panel:text(0, 0, "Hello Plugin!", "default")
				end,
			},
		})
	`)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	pw := NewPluginPanelWidget(p, p.RenderFunc, p.EventFunc)
	surface := newMockSurface(40, 10)
	pw.Render(surface)

	if len(surface.texts) == 0 {
		t.Fatal("expected text to be drawn")
	}
	if surface.texts[0].text != "Hello Plugin!" {
		t.Errorf("expected 'Hello Plugin!', got %q", surface.texts[0].text)
	}
}

func TestPluginPanelWidgetRenderError(t *testing.T) {
	p := &Plugin{
		Name:    "broken",
		Granted: PermissionSet{PanelSidebar: true},
	}
	p.State = NewSandbox()
	defer p.State.Close()
	setupTTTModule(p.State, p)

	err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.register({
			sidebar = {
				title = "Broken",
				render = function(panel)
					error("intentional crash")
				end,
			},
		})
	`)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	pw := NewPluginPanelWidget(p, p.RenderFunc, p.EventFunc)
	surface := newMockSurface(80, 10)
	pw.Render(surface)

	found := false
	for _, text := range surface.texts {
		if text.style == term.StyleDanger {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error text to be drawn with danger style")
	}
}

func TestPluginPanelWidgetCellAPI(t *testing.T) {
	p := &Plugin{
		Name:    "cell-test",
		Granted: PermissionSet{PanelSidebar: true},
	}
	p.State = NewSandbox()
	defer p.State.Close()
	setupTTTModule(p.State, p)

	err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.register({
			sidebar = {
				title = "Cells",
				render = function(panel)
					panel:cell(0, 0, "X")
					panel:cell(1, 0, "Y", {style = "success"})
				end,
			},
		})
	`)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	pw := NewPluginPanelWidget(p, p.RenderFunc, p.EventFunc)
	surface := newMockSurface(40, 10)
	pw.Render(surface)

	c1, ok := surface.cells[[2]int{0, 0}]
	if !ok || c1.Ch != 'X' {
		t.Error("expected cell (0,0) to be 'X'")
	}

	c2, ok := surface.cells[[2]int{1, 0}]
	if !ok || c2.Ch != 'Y' {
		t.Error("expected cell (1,0) to be 'Y'")
	}
	if c2.Style != term.StyleSuccess {
		t.Errorf("expected success style, got %d", c2.Style)
	}
}

func TestTitleWidgetLuaFields(t *testing.T) {
	p := &Plugin{
		Name:    "title-test",
		Granted: PermissionSet{PanelSidebar: true},
	}
	p.State = NewSandbox()
	defer p.State.Close()
	setupTTTModule(p.State, p)

	err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.register({
			sidebar = {
				title = "Titles",
				render = function(panel)
					panel:title({
						text = "Section",
						badge = "3",
						icon = "+",
						padded = true,
						menu = {
							{ label = "Refresh", command = "refresh" },
						},
						on_menu = function(cmd) end,
					})
				end,
			},
		})
	`)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	proxy := NewPanelProxy(newMockSurface(40, 10), p)
	if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
		t.Fatalf("render failed: %v", err)
	}

	descs := proxy.Descriptors()
	if len(descs) != 1 {
		t.Fatalf("expected 1 descriptor, got %d", len(descs))
	}
	desc := descs[0]
	if desc.Kind != WidgetTitle {
		t.Fatalf("expected title widget, got %s", desc.Kind)
	}
	if desc.Text != "Section" {
		t.Errorf("expected text 'Section', got %q", desc.Text)
	}
	if desc.Badge != "3" {
		t.Errorf("expected badge '3', got %q", desc.Badge)
	}
	if desc.Icon != "+" {
		t.Errorf("expected icon '+', got %q", desc.Icon)
	}
	if !desc.Padded {
		t.Error("expected padded to be true")
	}
	if len(desc.Entries) != 1 || desc.Entries[0].Label != "Refresh" || desc.Entries[0].Command != "refresh" {
		t.Errorf("expected menu entry Refresh/refresh, got %+v", desc.Entries)
	}
	if desc.OnMenu == nil {
		t.Error("expected on_menu callback to be wired")
	}
}

func TestPluginWidgetMenusParseOptionalCheckedState(t *testing.T) {
	p := &Plugin{
		Name:    "checked-menu-test",
		Granted: PermissionSet{PanelSidebar: true},
	}
	p.State = NewSandbox()
	defer p.State.Close()
	setupTTTModule(p.State, p)

	err := p.State.DoString(`
		local ttt = require("ttt")
		local function menu_entries()
			return {
				{ label = "Omitted", command = "omitted" },
				{ label = "Unchecked", command = "unchecked", checked = false },
				{ separator = true },
				{ label = "Checked", command = "checked", checked = true },
			}
		end
		ttt.register({
			sidebar = {
				title = "Checked menus",
				render = function(panel)
					panel:title({ text = "Title", menu = menu_entries() })
					panel:dropdown({ label = "Dropdown", entries = menu_entries() })
					panel:tree({ items = {{ id = "tree", label = "Tree" }}, node_menu = menu_entries() })
					panel:list({ items = {{ id = "list", label = "List" }}, node_menu = menu_entries() })
					panel:table({
						columns = {{ label = "Column" }},
						rows = {{ "Row" }},
						node_menu = menu_entries(),
					})
				end,
			},
		})
	`)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	proxy := NewPanelProxy(newMockSurface(80, 20), p)
	if err := p.CallRenderWith(p.RenderFunc, proxy); err != nil {
		t.Fatalf("render failed: %v", err)
	}

	descs := proxy.Descriptors()
	if len(descs) != 5 {
		t.Fatalf("descriptors = %d, want title, dropdown, tree, list, and table", len(descs))
	}
	for _, desc := range descs {
		entries := desc.Entries
		if desc.Kind == WidgetTree || desc.Kind == WidgetList || desc.Kind == WidgetTable {
			entries = desc.NodeMenu
		}
		assertOptionalCheckedEntries(t, desc.Kind.String(), entries)
	}
}

func assertOptionalCheckedEntries(t *testing.T, surface string, entries []widgets.MenuEntry) {
	t.Helper()
	if len(entries) != 4 {
		t.Fatalf("%s entries = %d, want 4", surface, len(entries))
	}
	if entries[0].Checked != nil {
		t.Errorf("%s omitted checked = %v, want nil", surface, *entries[0].Checked)
	}
	if entries[1].Checked == nil || *entries[1].Checked {
		t.Errorf("%s unchecked entry = %+v, want non-nil false", surface, entries[1])
	}
	if !entries[2].Separator {
		t.Errorf("%s separator entry lost separator state", surface)
	}
	if entries[3].Checked == nil || !*entries[3].Checked {
		t.Errorf("%s checked entry = %+v, want non-nil true", surface, entries[3])
	}
}
