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

func TestRemovingPluginPanelsFreesTheirImages(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()
	p := &plugin.Plugin{Name: "pages", Enabled: true}
	if err := p.InitFromSource(`function render(panel) panel:image({ data = img }) end`); err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	p.Graphics = func() plugin.GraphicsInfo {
		return plugin.GraphicsInfo{Protocol: tttimage.Kitty, CellW: 10, CellH: 20}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 40, 40))); err != nil {
		t.Fatal(err)
	}
	p.State.SetGlobal("img", lua.LString(buf.String()))
	panel := plugin.NewPluginPanelWidget(p, p.State.GetGlobal("render").(*lua.LFunction), nil)
	h.app.Sidebar.AddPanel("plugin.pages", "Pages", ui.NewWidgetAdapter(panel))

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

	h.app.RemovePluginPanels("pages")
	if h.app.Sidebar.HasPanel("plugin.pages") {
		t.Fatal("panel still in the sidebar")
	}
	layer.Begin()
	var out bytes.Buffer
	if err := layer.Commit(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "d=I") {
		t.Errorf("removing the panel should free its image data, got %q", out.String())
	}
}
