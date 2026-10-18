package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
)

// openTintProbe themes the screen and opens a file whose second line a
// decoration can tint.
func openTintProbe(h *testHarness) string {
	h.t.Helper()
	theme := config.DefaultTheme()
	theme.ResolveColors()
	app.ApplyThemeStyles(h.app.Screen, theme)
	file := filepath.Join(h.dir, "probe.txt")
	if err := os.WriteFile(file, []byte("first probe\nsecond probe\nthird probe\n"), 0644); err != nil {
		h.t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(file)
	h.redraw()
	return file
}

func secondProbeLineTinted(h *testHarness) bool {
	h.t.Helper()
	cells, w, _ := h.screen.GetContents()
	bgOf := func(text string) any {
		for y := 0; y*w < len(cells); y++ {
			if x := displayColumnOf(h.screenRow(y), text); x >= 0 {
				return cells[y*w+x].Style.GetBackground()
			}
		}
		h.t.Fatalf("%q not on screen", text)
		return nil
	}
	return bgOf("second probe") != bgOf("third probe")
}

func TestPluginUninstallClearsDecorationsItsUninstallCallbackSets(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	file := openTintProbe(h)

	cfg := filepath.Join(h.dir, "config")
	dir := filepath.Join(cfg, "plugins", "deco")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"plugin.ttt.json": `{"name": "deco", "version": "1.0.0", "entry": "init.lua", "permissions": {"commands": true, "editor.decorations": true}}`,
		"init.lua": `
			local ttt = require("ttt")
			local deco = require("ttt.decorations")
			ttt.register({ commands = { { id = "deco.noop", title = "Deco: Noop", handler = function() end } } })
			ttt.on_uninstall(function() deco.set(FILE, { { line = 2 } }) end)
		`,
	}
	for name, content := range files {
		content = strings.ReplaceAll(content, "FILE", `"`+file+`"`)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	m := plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	h.app.PluginManager = m
	t.Cleanup(m.Shutdown)
	pending := m.LoadAll()
	if len(pending) != 1 {
		t.Fatalf("pending approvals = %d, want 1", len(pending))
	}
	if err := m.ApproveAndLoad(pending[0]); err != nil {
		t.Fatal(err)
	}
	h.app.WirePlugin(pending[0])
	h.redraw()

	h.app.PluginUninstallByName("deco")
	h.redraw()
	clicked := false
	for y := 0; y < 24 && !clicked; y++ {
		row := h.screenRow(y)
		if strings.Contains(row, "Cancel") {
			if x := displayColumnOf(row, "Uninstall"); x >= 0 {
				h.click(x+1, y)
				clicked = true
			}
		}
	}
	if !clicked {
		t.Fatal("no Uninstall button on screen")
	}
	h.redraw()

	if m.FindPlugin("deco") != nil {
		t.Fatal("plugin still installed")
	}
	if secondProbeLineTinted(h) {
		t.Error("a decoration set by the uninstall callback outlived the plugin")
	}
}

func TestPluginUpdateClearsTheOldVersionsDecorations(t *testing.T) {
	for _, tc := range []struct {
		name             string
		extraPermissions string
	}{
		{"applied", ""},
		{"awaiting approval", `, "fs.read": true`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, pdf := newUpdatablePluginHarness(t, tc.extraPermissions)
			defer h.stop()
			file := openTintProbe(h)
			h.app.EditorGroup.SetDecorations("plugin:pdfview", file, []ui.Decoration{{StartLine: 1, EndLine: 1, Style: term.StyleHighlight}})
			h.redraw()
			if !secondProbeLineTinted(h) {
				t.Fatal("decoration not drawn before the update")
			}

			h.awaitPluginUpdate("pdfview", pdf)
			h.app.DismissDialog()
			h.app.EditorGroup.OpenFile(file)
			h.redraw()

			if secondProbeLineTinted(h) {
				t.Error("the old version's decoration survived the update")
			}
		})
	}
}
