package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/gdamore/tcell/v3"
)

const clipboardEditorText = "editor text"

// openSelectedEditorFile opens a file and selects all of it, so a Copy or Cut
// that wrongly falls through to the editor would be visible.
func openSelectedEditorFile(t *testing.T, h *testHarness) {
	t.Helper()
	f := filepath.Join(h.dir, "doc.txt")
	if err := os.WriteFile(f, []byte(clipboardEditorText+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	h.openFile(f)
	h.app.EditorGroup.SelectAll()
	h.redraw()
}

func assertEditorUnchanged(t *testing.T, h *testHarness) {
	t.Helper()
	ed := h.app.EditorGroup.Editor
	if got := ed.Buf.Lines[0]; got != clipboardEditorText {
		t.Fatalf("editor line = %q, want %q", got, clipboardEditorText)
	}
	if ed.Buf.Dirty {
		t.Fatal("editor buffer was modified")
	}
}

// typeAndSelect types text into the focused input and selects all of it.
func typeAndSelect(h *testHarness, text string) {
	h.typeText(text)
	h.pressKey(tcell.KeyHome, tcell.ModShift)
	h.redraw()
}

func TestCopyCommandCopiesAFocusedPluginInputSelection(t *testing.T) {
	h := newTestHarness(t, 160, 30)
	defer h.stop()
	clipboard.DisableSystem()
	openSelectedEditorFile(t, h)
	focusPluginInput(t, h)
	typeAndSelect(h, "hello input")
	clipboard.Set("before")

	h.exec("editor.copy")
	h.redraw()

	if got := clipboard.Get(); got != "hello input" {
		t.Fatalf("clipboard = %q, want the plugin input selection", got)
	}
	h.assertContains("hello input")
	assertEditorUnchanged(t, h)
}

func TestCutCommandCutsOnlyThePluginInputSelection(t *testing.T) {
	h := newTestHarness(t, 160, 30)
	defer h.stop()
	clipboard.DisableSystem()
	openSelectedEditorFile(t, h)
	focusPluginInput(t, h)
	typeAndSelect(h, "hello input")
	clipboard.Set("before")

	h.exec("editor.cut")
	h.redraw()

	if got := clipboard.Get(); got != "hello input" {
		t.Fatalf("clipboard = %q, want the plugin input selection", got)
	}
	h.assertNotContains("hello input")
	assertEditorUnchanged(t, h)
}

func TestClipboardCommandsWithoutPluginInputSelectionSkipTheEditor(t *testing.T) {
	h := newTestHarness(t, 160, 30)
	defer h.stop()
	clipboard.DisableSystem()
	openSelectedEditorFile(t, h)
	focusPluginInput(t, h)
	h.typeText("unselected")
	h.redraw()
	clipboard.Set("before")

	h.exec("editor.copy")
	h.exec("editor.cut")
	h.redraw()

	if got := clipboard.Get(); got != "before" {
		t.Fatalf("clipboard = %q, want it untouched with no input selection", got)
	}
	h.assertContains("unselected")
	assertEditorUnchanged(t, h)
}

func TestCopyAndCutGoToAPluginTabInput(t *testing.T) {
	h := newTestHarness(t, 160, 30)
	defer h.stop()
	clipboard.DisableSystem()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	src := filepath.Join(h.dir, "tab.lua")
	if err := os.WriteFile(src, []byte(`
		function show()
			require("ttt").open_tab({ title = "T", render = function(panel)
				panel:input({ placeholder = "say" })
			end })
		end
	`), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, src)
	p := h.app.PluginManager.FindPlugin("tab")
	if p == nil {
		t.Fatal("plugin not loaded")
	}
	t.Cleanup(p.Destroy)
	if err := p.State.DoString(`show()`); err != nil {
		t.Fatal(err)
	}
	h.redraw()
	typeAndSelect(h, "tabword")
	clipboard.Set("before")

	h.exec("editor.copy")
	if got := clipboard.Get(); got != "tabword" {
		t.Fatalf("clipboard after copy = %q, want the plugin tab input selection", got)
	}
	h.redraw()
	h.assertContains("tabword")

	clipboard.Set("before")
	h.exec("editor.cut")
	h.redraw()
	if got := clipboard.Get(); got != "tabword" {
		t.Fatalf("clipboard after cut = %q, want the plugin tab input selection", got)
	}
	h.assertNotContains("tabword")
}
