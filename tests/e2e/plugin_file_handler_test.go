package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/plugin"
)

const fileHandlerPluginSource = `
local ttt = require("ttt")
local events = require("ttt.events")
opens, opened_event = 0, nil
events.on("file.open", function(path) opened_event = path end)
ttt.register({ file_handlers = {
	{ id = "pdf", title = "PDF Viewer", magic = "%PDF-", open = function(path, tab)
		opens = opens + 1
		tab.render = function(panel) panel:label("PDF page 1 of 3") end
	end },
	{ id = "broken", extensions = { "broken" }, open = function() error("pdfinfo not found") end },
} })
`

func newFileHandlerHarness(t *testing.T) (*testHarness, *plugin.Plugin) {
	t.Helper()
	h := newTestHarness(t, 100, 30)
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	src := filepath.Join(h.dir, "pdfview.lua")
	if err := os.WriteFile(src, []byte(fileHandlerPluginSource), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, src)
	p := h.app.PluginManager.FindPlugin("pdfview")
	if p == nil || len(p.FileHandlers) != 2 {
		t.Fatalf("plugin not loaded with handlers: %+v", p)
	}
	return h, p
}

func TestPluginFileHandlerOpensBinaryFile(t *testing.T) {
	h, p := newFileHandlerHarness(t)
	defer h.stop()
	defer p.Destroy()

	pdf := filepath.Join(h.dir, "paper.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.7\n\x00\x01\x02\xff"), 0644); err != nil {
		t.Fatal(err)
	}

	h.app.EditorGroup.OpenFile(pdf)
	h.redraw()
	h.assertContains("PDF page 1 of 3")
	h.assertContains("paper.pdf")
	h.assertNotContains("Binary file")
	if got := p.State.GetGlobal("opened_event").String(); got != pdf {
		t.Errorf("file.open received %q, want %q", got, pdf)
	}

	h.app.EditorGroup.OpenFile(pdf)
	h.redraw()
	if got := p.State.GetGlobal("opens").String(); got != "1" {
		t.Errorf("reopening the file called open %s times, want 1", got)
	}
}

func TestPluginFileHandlerErrorFallsBackToBinaryCard(t *testing.T) {
	h, p := newFileHandlerHarness(t)
	defer h.stop()
	defer p.Destroy()

	path := filepath.Join(h.dir, "data.broken")
	if err := os.WriteFile(path, []byte{0x00, 0x01, 0xff}, 0644); err != nil {
		t.Fatal(err)
	}

	h.app.EditorGroup.OpenFile(path)
	h.redraw()
	h.assertContains("Binary file")
	if p.LastError == nil {
		t.Error("handler error was not recorded on the plugin")
	}
}

func TestPluginFileHandlerIgnoresTextAndDisabledPlugins(t *testing.T) {
	h, p := newFileHandlerHarness(t)
	defer h.stop()
	defer p.Destroy()

	txt := filepath.Join(h.dir, "notes.broken")
	if err := os.WriteFile(txt, []byte("plain text\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(txt)
	h.redraw()
	h.assertContains("plain text")

	p.Enabled = false
	pdf := filepath.Join(h.dir, "other.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.7\n\x00\x01"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(pdf)
	h.redraw()
	h.assertContains("Binary file")
	h.assertNotContains("PDF page 1 of 3")
}

func TestPluginFileHandlerTabSurvivesReloadAndDisable(t *testing.T) {
	h, p := newFileHandlerHarness(t)
	defer h.stop()

	pdf := filepath.Join(h.dir, "paper.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.7\n\x00\x01\x02\xff"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(pdf)
	h.redraw()
	h.assertContains("PDF page 1 of 3")

	app.LoadPluginFromFile(h.app, filepath.Join(h.dir, "pdfview.lua"))
	reloaded := h.app.PluginManager.FindPlugin("pdfview")
	if reloaded == nil || reloaded == p {
		t.Fatal("plugin was not reloaded")
	}
	h.redraw()
	h.assertContains("PDF page 1 of 3")

	done := make(chan struct{})
	go func() {
		reloaded.Destroy()
		close(done)
	}()
	<-done
	h.redraw()
	h.assertContains("Binary file")
	h.assertNotContains("PDF page 1 of 3")
}

func TestPluginFileHandlerTabReleasedWhenPluginIsDisabledByErrors(t *testing.T) {
	h, p := newFileHandlerHarness(t)
	defer h.stop()

	pdf := filepath.Join(h.dir, "paper.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.7\n\x00\x01\x02\xff"), 0644); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.OpenFile(pdf)
	h.redraw()
	h.assertContains("PDF page 1 of 3")

	p.Enabled = false
	h.redraw()
	h.assertContains("Binary file")
	h.assertNotContains("PDF page 1 of 3")
}
