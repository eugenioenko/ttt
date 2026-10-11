package e2e

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	tttimage "github.com/eugenioenko/ttt/internal/image"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	lua "github.com/yuin/gopher-lua"
)

func TestPluginImageShowsAltWithoutGraphics(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	src := filepath.Join(h.dir, "imgtab.lua")
	if err := os.WriteFile(src, []byte(`
		local ttt = require("ttt")
		function show()
			local g = ttt.graphics()
			ttt.open_tab({ title = "Preview", render = function(panel)
				panel:label("protocol: " .. g.protocol)
				panel:image({ data = img, alt = "Page 3 of 10" })
			end })
		end
	`), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, src)
	p := h.app.PluginManager.FindPlugin("imgtab")
	if p == nil {
		t.Fatal("plugin not loaded")
	}
	defer p.Destroy()

	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 32, 32))); err != nil {
		t.Fatal(err)
	}
	p.State.SetGlobal("img", lua.LString(buf.String()))
	if err := p.State.DoString(`show()`); err != nil {
		t.Fatal(err)
	}
	h.redraw()
	h.assertContains("protocol: none")
	h.assertContains("Page 3 of 10")
}

func TestPluginImageTransmitsOnceAndFreesOnPageChange(t *testing.T) {
	p := &plugin.Plugin{Name: "pages", Enabled: true}
	if err := p.InitFromSource(`function render(panel) panel:image({ data = img }) end`); err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	p.Graphics = func() plugin.GraphicsInfo {
		return plugin.GraphicsInfo{Protocol: tttimage.Kitty, CellW: 10, CellH: 20}
	}
	render := p.State.GetGlobal("render").(*lua.LFunction)
	panel := plugin.NewPluginPanelWidget(p, render, nil)

	layer := tttimage.NewLayer()
	frame := func() string {
		grid := make([][]term.Cell, 20)
		for y := range grid {
			grid[y] = make([]term.Cell, 60)
		}
		surface := ui.NewRenderSurface(grid, ui.Rect{W: 60, H: 20})
		surface.SetImageLayer(layer)
		layer.Begin()
		panel.Render(surface)
		var out bytes.Buffer
		if err := layer.Commit(&out); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	page := func(w int) lua.LString {
		var buf bytes.Buffer
		if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, w, 40))); err != nil {
			t.Fatal(err)
		}
		return lua.LString(buf.String())
	}

	p.State.SetGlobal("img", page(40))
	if out := frame(); strings.Count(out, "a=t") != 1 {
		t.Fatalf("first frame should transmit once, got %q", out)
	}
	if out := frame(); out != "" {
		t.Fatalf("an idle redraw re-sent %d bytes: %q", len(out), out)
	}

	p.State.SetGlobal("img", page(50))
	out := frame()
	if strings.Count(out, "a=t") != 1 || strings.Count(out, "d=I") != 1 {
		t.Fatalf("page change should free the old image data and send the new one, got %q", out)
	}

	panel.Close()
	layer.Begin()
	var closed bytes.Buffer
	if err := layer.Commit(&closed); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(closed.String(), "d=I") {
		t.Fatalf("closing the panel should free its image data, got %q", closed.String())
	}
}

func kittyGraphics() plugin.GraphicsInfo {
	return plugin.GraphicsInfo{Protocol: tttimage.Kitty, CellW: 10, CellH: 20}
}

func newImagePanel(t *testing.T) (*plugin.Plugin, *plugin.PluginPanelWidget) {
	t.Helper()
	p := &plugin.Plugin{Name: "pages", Enabled: true}
	if err := p.InitFromSource(`function render(panel) panel:image({ data = img }) end`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Destroy)
	p.Graphics = kittyGraphics
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 40, 40))); err != nil {
		t.Fatal(err)
	}
	p.State.SetGlobal("img", lua.LString(buf.String()))
	return p, plugin.NewPluginPanelWidget(p, p.State.GetGlobal("render").(*lua.LFunction), nil)
}

func placeOnLayer(t *testing.T, panel *plugin.PluginPanelWidget) *tttimage.Layer {
	t.Helper()
	layer := tttimage.NewLayer()
	grid := make([][]term.Cell, 20)
	for y := range grid {
		grid[y] = make([]term.Cell, 60)
	}
	surface := ui.NewRenderSurface(grid, ui.Rect{W: 60, H: 20})
	surface.SetImageLayer(layer)
	layer.Begin()
	panel.Render(surface)
	if err := layer.Commit(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	return layer
}

func assertImageFreed(t *testing.T, layer *tttimage.Layer) {
	t.Helper()
	layer.Begin()
	var out bytes.Buffer
	if err := layer.Commit(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "d=I") {
		t.Errorf("removing the panel should free its image data, got %q", out.String())
	}
}

func TestRemovingPluginPanelsFreesTheirImages(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	_, panel := newImagePanel(t)
	h.app.Sidebar.AddPanel("plugin.pages", "Pages", ui.NewWidgetAdapter(panel))
	layer := placeOnLayer(t, panel)

	h.app.RemovePluginPanels("pages")
	if h.app.Sidebar.HasPanel("plugin.pages") {
		t.Fatal("panel still in the sidebar")
	}
	assertImageFreed(t, layer)
}

func wiredImagePanel(t *testing.T, h *testHarness) (*plugin.Plugin, *plugin.PluginPanelWidget) {
	t.Helper()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	p, panel := newImagePanel(t)
	h.app.WirePlugin(p)
	// WirePlugin installs the harness's real graphics detection (none) and async decoding.
	p.Graphics = kittyGraphics
	p.RequestRedraw = nil
	return p, panel
}

func TestRemovingPluginPanelsClosesItsDrawer(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	p, panel := wiredImagePanel(t, h)
	p.OpenDrawer(panel, 40, 20, "right")
	if !h.app.Root.HasOverlay() {
		t.Fatal("drawer did not open")
	}
	layer := placeOnLayer(t, panel)

	h.app.RemovePluginPanels("other")
	if !h.app.Root.HasOverlay() {
		t.Fatal("removing another plugin's panels closed this drawer")
	}

	h.app.RemovePluginPanels("pages")
	if h.app.Root.HasOverlay() {
		t.Fatal("drawer still open")
	}
	assertImageFreed(t, layer)
}

func TestRemovingPluginPanelsClosesItsEditorTabs(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	p, panel := wiredImagePanel(t, h)
	p.OpenTab("Pages", panel)
	hasTab := func() bool {
		for i := range h.app.EditorGroup.TabCount() {
			if path, _ := h.app.EditorGroup.TabInfo(i); path == "Pages" {
				return true
			}
		}
		return false
	}
	if !hasTab() {
		t.Fatal("tab did not open")
	}
	layer := placeOnLayer(t, panel)

	h.app.RemovePluginPanels("other")
	if !hasTab() {
		t.Fatal("removing another plugin's panels closed this tab")
	}

	h.app.RemovePluginPanels("pages")
	if hasTab() {
		t.Fatal("tab still open")
	}
	assertImageFreed(t, layer)
}

func TestWirePluginReplacesStaleSidebarPanel(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	_, stale := newImagePanel(t)
	h.app.Sidebar.AddPanel("plugin.pages", "Pages", ui.NewWidgetAdapter(stale))
	layer := placeOnLayer(t, stale)

	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	p, fresh := newImagePanel(t)
	h.app.PluginManager.SidebarPanels = []plugin.SidebarRegistration{{ID: "plugin.pages", Title: "Pages", Widget: fresh}}
	h.app.WirePlugin(p)

	adapter, ok := h.app.Sidebar.PanelWidget("plugin.pages").(*ui.WidgetAdapter)
	if !ok || adapter.W != fresh {
		t.Fatal("sidebar still shows the stale panel")
	}
	assertImageFreed(t, layer)
}
