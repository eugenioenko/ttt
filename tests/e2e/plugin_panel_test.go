package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestPluginPanelSidebarEvents(t *testing.T) {
	h := newTestHarness(t, 80, 40)
	defer h.stop()

	// Create a plugin directory with a manifest and Lua file
	pluginDir := filepath.Join(h.dir, "test-plugin")
	requireNoError(t, os.MkdirAll(pluginDir, 0755))
	requireNoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.ttt.json"), []byte(`{
		"name": "test-panel",
		"description": "test",
		"version": "0.1.0",
		"entry": "init.lua",
		"permissions": { "panel.sidebar": true }
	}`), 0644))
	requireNoError(t, os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte(`
		local ttt = require("ttt")
		ttt.register({
			sidebar = {
				title = "Test",
				render = function(panel)
					panel:list({
						items = {
							{ id = "a", label = "Alpha" },
							{ id = "b", label = "Beta" },
							{ id = "c", label = "Gamma" },
							{ id = "d", label = "Delta" },
							{ id = "e", label = "Epsilon" },
						},
					})
				end,
			},
		})
	`), 0644))

	manifest, err := plugin.LoadManifest(pluginDir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	p := &plugin.Plugin{
		Name:     manifest.Name,
		Dir:      pluginDir,
		Manifest: manifest,
		Granted:  manifest.Permissions,
	}
	if err := p.Init(); err != nil {
		t.Fatalf("init plugin: %v", err)
	}
	defer p.Destroy()

	if p.SidebarTitle == "" {
		t.Fatal("expected sidebar title to be set")
	}
	if p.RenderFunc == nil {
		t.Fatal("expected render func to be set")
	}

	pw := plugin.NewPluginPanelWidget(p, p.RenderFunc, p.EventFunc)
	adapter := ui.NewWidgetAdapter(pw)

	h.app.Sidebar.AddPanel("plugin.test-panel", "Test", adapter)
	h.app.Sidebar.SetActivePanel("plugin.test-panel")
	h.app.ShowSidebar()
	h.redraw()

	// Verify the panel rendered the list items
	screen := h.screenText()
	if !strings.Contains(screen, "Alpha") {
		t.Fatal("expected 'Alpha' to be visible in sidebar")
	}
	if !strings.Contains(screen, "Epsilon") {
		t.Log("Epsilon may be scrolled off, that's ok")
	}

	// Get sidebar content area
	sidebarRect := h.app.Sidebar.GetRect()
	t.Logf("sidebar rect: %+v", sidebarRect)

	// Find where "Alpha" is rendered
	alphaY := -1
	for y := 0; y < 40; y++ {
		row := h.screenRow(y)
		if strings.Contains(row, "Alpha") {
			alphaY = y
			break
		}
	}
	if alphaY < 0 {
		t.Fatal("could not find Alpha row on screen")
	}
	t.Logf("Alpha is at row %d", alphaY)

	// Click on Alpha - this should be consumed by the tree widget
	down := tcell.NewEventMouse(sidebarRect.X+5, alphaY, tcell.Button1, tcell.ModNone)
	result := h.app.Root.HandleEvent(down)
	t.Logf("click on Alpha result: %d", result)

	up := tcell.NewEventMouse(sidebarRect.X+5, alphaY, tcell.ButtonNone, tcell.ModNone)
	h.app.Root.HandleEvent(up)
	h.redraw()

	// Click on Beta
	betaY := alphaY + 1
	down2 := tcell.NewEventMouse(sidebarRect.X+5, betaY, tcell.Button1, tcell.ModNone)
	result2 := h.app.Root.HandleEvent(down2)
	t.Logf("click on Beta result: %d", result2)

	up2 := tcell.NewEventMouse(sidebarRect.X+5, betaY, tcell.ButtonNone, tcell.ModNone)
	h.app.Root.HandleEvent(up2)
	h.redraw()

	// Wheel scroll
	wheelDown := tcell.NewEventMouse(sidebarRect.X+5, alphaY, tcell.WheelDown, tcell.ModNone)
	wheelResult := h.app.Root.HandleEvent(wheelDown)
	t.Logf("wheel down result: %d", wheelResult)
	h.redraw()

	// Now set focus to sidebar and try keyboard
	h.app.Root.SetFocus(h.app.Sidebar)

	// Arrow down should work
	downKey := tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone)
	keyResult := h.app.Root.HandleEvent(downKey)
	t.Logf("arrow down result: %d", keyResult)
	h.redraw()

	// Tab should cycle focus
	tabKey := tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone)
	tabResult := h.app.Root.HandleEvent(tabKey)
	t.Logf("tab result: %d", tabResult)
	h.redraw()

	// If we got here without panics, basic event routing works
	// Check that events were consumed (result=1 means EventConsumed)
	if result == 0 {
		t.Error("click on Alpha was NOT consumed - events are not reaching the tree widget")
	}
	if wheelResult == 0 {
		t.Error("wheel scroll was NOT consumed - scroll events are not reaching the tree widget")
	}
	if keyResult == 0 {
		t.Error("arrow down key was NOT consumed - key events are not reaching the tree widget")
	}
}
