package plugin

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
	lua "github.com/yuin/gopher-lua"
)

func newTextPanel(t *testing.T) (*PluginPanelWidget, *mockSurface) {
	t.Helper()
	p := &Plugin{Name: "test", Granted: PermissionSet{PanelSidebar: true}}
	p.State = NewSandbox()
	t.Cleanup(p.State.Close)
	setupTTTModule(p.State, p)
	err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.register({
			sidebar = {
				title = "Test",
				render = function(panel)
					panel:text(0, 0, "alpha beta")
					panel:text(0, 1, "gamma delta")
				end,
			},
		})
	`)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	pw := NewPluginPanelWidget(p, p.RenderFunc, p.EventFunc)
	pw.SetRect(widgets.Rect{X: 10, Y: 5, W: 20, H: 3})
	surface := newMockSurface(20, 3)
	pw.Render(surface)
	return pw, surface
}

func mouse(pw *PluginPanelWidget, x, y int, btn tcell.ButtonMask) widgets.EventResult {
	return pw.HandleEvent(tcell.NewEventMouse(x, y, btn, tcell.ModNone))
}

func TestPanelDragSelectsDrawnTextAndCopies(t *testing.T) {
	clipboard.DisableSystem()
	pw, surface := newTextPanel(t)

	mouse(pw, 16, 5, tcell.Button1)
	if got := mouse(pw, 14, 6, tcell.Button1); got != widgets.EventCaptured {
		t.Fatalf("drag result = %v, want captured", got)
	}
	mouse(pw, 14, 6, tcell.ButtonNone)
	pw.Render(surface)

	if c := surface.cells[[2]int{6, 0}]; c.BgStyle != term.StylePluginSelection || c.Ch != 'b' {
		t.Errorf("selection start cell = %+v, want highlighted 'b'", c)
	}
	if c := surface.cells[[2]int{5, 1}]; c.BgStyle == term.StylePluginSelection {
		t.Errorf("cell past the selection end is highlighted")
	}

	if got := pw.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlC, "", tcell.ModCtrl)); got != widgets.EventConsumed {
		t.Fatalf("ctrl+c result = %v, want consumed", got)
	}
	if got, want := clipboard.Get(), "beta\ngamma"; got != want {
		t.Errorf("clipboard = %q, want %q", got, want)
	}
	if pw.CopyTextSelection() {
		t.Error("selection should clear after copying")
	}
}

func TestPanelClickDoesNotSelect(t *testing.T) {
	pw, _ := newTextPanel(t)
	mouse(pw, 12, 5, tcell.Button1)
	mouse(pw, 12, 5, tcell.ButtonNone)
	if pw.CopyTextSelection() {
		t.Error("a click without a drag selected text")
	}
}

func TestPanelSelectAllCopiesEveryLine(t *testing.T) {
	clipboard.DisableSystem()
	pw, _ := newTextPanel(t)
	pw.SelectAllText()
	if !pw.CopyTextSelection() {
		t.Fatal("select all made no selection")
	}
	if got, want := clipboard.Get(), "alpha beta\ngamma delta\n"; got != want {
		t.Errorf("clipboard = %q, want %q", got, want)
	}
}

func TestPanelPressWhoseReleaseWentElsewhereIsForgotten(t *testing.T) {
	for name, lose := range map[string]func(*PluginPanelWidget){
		"focus moved":      func(pw *PluginPanelWidget) { pw.SetFocused(false) },
		"capture canceled": func(pw *PluginPanelWidget) { pw.CancelPointerCapture() },
	} {
		t.Run(name, func(t *testing.T) {
			pw, _ := newTextPanel(t)
			pw.SetFocused(true)
			mouse(pw, 12, 5, tcell.Button1)
			lose(pw)
			if got := mouse(pw, 14, 6, tcell.Button1); got == widgets.EventCaptured {
				t.Fatal("the next press was taken for a drag from the forgotten one")
			}
		})
	}
}

func newInputPanel(t *testing.T) (*PluginPanelWidget, *mockSurface) {
	t.Helper()
	p := &Plugin{Name: "test", Granted: PermissionSet{PanelSidebar: true}}
	p.State = NewSandbox()
	t.Cleanup(p.State.Close)
	setupTTTModule(p.State, p)
	err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.register({ sidebar = { title = "Test", render = function(panel)
			panel:label("alpha beta")
			panel:input({ value = "typed text", autofocus = true })
		end } })
	`)
	if err != nil {
		t.Fatal(err)
	}
	pw := NewPluginPanelWidget(p, p.RenderFunc, p.EventFunc)
	pw.SetRect(widgets.Rect{X: 10, Y: 5, W: 30, H: 3})
	surface := newMockSurface(30, 3)
	pw.Render(surface)
	pw.SetFocused(true)
	pw.FocusAutofocusInput()
	pw.Render(surface)
	return pw, surface
}

func TestPanelLeavesInputDragsToTheInput(t *testing.T) {
	pw, _ := newInputPanel(t)
	mouse(pw, 14, 6, tcell.Button1)
	if got := mouse(pw, 18, 6, tcell.Button1); got == widgets.EventCaptured {
		t.Fatal("the panel captured a drag inside the input")
	}
	mouse(pw, 18, 6, tcell.ButtonNone)
	if pw.sel.active {
		t.Fatal("a drag inside the input selected panel text")
	}
}

func TestFocusedInputSelectionWinsOverPanelSelection(t *testing.T) {
	clipboard.DisableSystem()
	pw, _ := newInputPanel(t)
	pw.sel.selectAll()
	pw.FocusedInput().SelectAll()
	clipboard.Set("before")
	if pw.CopyTextSelection() {
		t.Fatal("the panel copied although the focused input has a selection")
	}
	pw.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlC, "", tcell.ModCtrl))
	if got := clipboard.Get(); got != "typed text" {
		t.Fatalf("clipboard = %q, want the input's selection", got)
	}
}

func TestSelectAllSelectsTheFocusedInput(t *testing.T) {
	pw, _ := newInputPanel(t)
	pw.SelectAllText()
	if pw.sel.active || !pw.FocusedInput().HasSelection() {
		t.Fatalf("panel selection %v, input selection %v: want only the input selected", pw.sel.active, pw.FocusedInput().HasSelection())
	}
}

func TestCopyingAnEmptySelectionClearsTheClipboard(t *testing.T) {
	clipboard.DisableSystem()
	clipboard.Set("stale")
	pw, _ := newTextPanel(t)
	pw.sel.anchor, pw.sel.end, pw.sel.active = cellPos{x: 15, y: 2}, cellPos{x: 18, y: 2}, true
	if !pw.CopyTextSelection() || clipboard.Get() != "" {
		t.Fatalf("clipboard = %q, want it emptied by a blank selection", clipboard.Get())
	}
}

func TestSelectionAfterARenderErrorCopiesTheError(t *testing.T) {
	clipboard.DisableSystem()
	p := &Plugin{Name: "test", Granted: PermissionSet{PanelSidebar: true}}
	p.State = NewSandbox()
	t.Cleanup(p.State.Close)
	setupTTTModule(p.State, p)
	if err := p.State.DoString(`
		local ttt = require("ttt")
		broken = false
		ttt.register({ sidebar = { title = "Test", render = function(panel)
			if broken then error("boom") end
			panel:text(0, 0, "old text")
		end } })
	`); err != nil {
		t.Fatal(err)
	}
	pw := NewPluginPanelWidget(p, p.RenderFunc, p.EventFunc)
	pw.SetRect(widgets.Rect{X: 0, Y: 0, W: 40, H: 2})
	surface := newMockSurface(40, 2)
	pw.Render(surface)
	p.State.SetGlobal("broken", lua.LTrue)
	pw.Render(surface)
	pw.SelectAllText()
	pw.CopyTextSelection()
	if got := clipboard.Get(); !strings.Contains(got, "Plugin error") || strings.Contains(got, "old text") {
		t.Fatalf("clipboard = %q, want the error that is on screen", got)
	}
}

func newTablePanel(t *testing.T) (*PluginPanelWidget, *mockSurface, *Plugin) {
	t.Helper()
	p := &Plugin{Name: "test", Granted: PermissionSet{PanelSidebar: true}}
	p.State = NewSandbox()
	t.Cleanup(p.State.Close)
	setupTTTModule(p.State, p)
	err := p.State.DoString(`
		local ttt = require("ttt")
		activated = 0
		ttt.register({
			sidebar = {
				title = "Test",
				render = function(panel)
					panel:table({
						columns = {{ label = "Id", width = 6 }, { label = "Name" }},
						rows = {{ "0123456789abcdef", "alpha" }, { "42", "beta" }},
						on_activate = function() activated = activated + 1 end,
					})
				end,
			},
		})
	`)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	pw := NewPluginPanelWidget(p, p.RenderFunc, p.EventFunc)
	pw.SetRect(widgets.Rect{X: 10, Y: 5, W: 20, H: 6})
	surface := newMockSurface(20, 6)
	pw.Render(surface)
	return pw, surface, p
}

func doubleClickPanel(pw *PluginPanelWidget, x, y int) {
	for range 2 {
		mouse(pw, x, y, tcell.Button1)
		mouse(pw, x, y, tcell.ButtonNone)
	}
}

func TestPanelTableCellCopiesFullValue(t *testing.T) {
	clipboard.DisableSystem()
	clipboard.Set("before")
	pw, surface, p := newTablePanel(t)

	doubleClickPanel(pw, 11, 7)
	pw.Render(surface)
	if clipboard.Get() != "before" {
		t.Fatal("double-click touched the clipboard")
	}
	if v := p.State.GetGlobal("activated"); v != lua.LNumber(0) {
		t.Fatalf("double-click fired on_activate %v times", v)
	}
	for x := range 7 {
		c := surface.cells[[2]int{x, 2}]
		if hl := c.BgStyle == term.StylePluginSelection; hl != (x < 6) {
			t.Fatalf("cell %d %q highlighted=%v, want the drawn id only", x, c.Ch, hl)
		}
	}
	if c := surface.cells[[2]int{5, 2}]; c.Ch != '…' {
		t.Fatalf("truncated id should end in an ellipsis, got %q", c.Ch)
	}

	if got := pw.HandleEvent(tcell.NewEventKey(tcell.KeyCtrlC, "", tcell.ModCtrl)); got != widgets.EventConsumed {
		t.Fatalf("ctrl+c result = %v, want consumed", got)
	}
	if got := clipboard.Get(); got != "0123456789abcdef" {
		t.Fatalf("clipboard = %q, want the full value", got)
	}

	if got := pw.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, "", tcell.ModNone)); got != widgets.EventConsumed {
		t.Fatalf("escape result = %v, want consumed", got)
	}
	if pw.CopyTextSelection() {
		t.Fatal("escape kept the cell selection")
	}

	doubleClickPanel(pw, 11, 8)
	mouse(pw, 25, 10, tcell.Button1)
	mouse(pw, 25, 10, tcell.ButtonNone)
	if pw.CopyTextSelection() {
		t.Fatal("a click below the rows kept the cell selection")
	}
}

func TestPanelDrawsWidgetSelectionsInPluginSelectionStyle(t *testing.T) {
	p := &Plugin{Name: "test", Granted: PermissionSet{PanelSidebar: true}}
	p.State = NewSandbox()
	t.Cleanup(p.State.Close)
	setupTTTModule(p.State, p)
	err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.register({
			sidebar = {
				title = "Test",
				render = function(panel)
					panel:input({ value = "hi界", autofocus = true })
				end,
			},
		})
	`)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	pw := NewPluginPanelWidget(p, p.RenderFunc, p.EventFunc)
	pw.SetRect(widgets.Rect{W: 20, H: 3})
	surface := newMockSurface(20, 3)
	pw.SetFocused(true)
	pw.Render(surface)
	pw.FocusAutofocusInput()
	pw.SelectAllText()
	pw.Render(surface)

	selected := 0
	for _, c := range surface.cells {
		if c.Style == term.StyleSelection || c.BgStyle == term.StyleSelection {
			t.Fatalf("cell %q still uses the editor selection style", c.Ch)
		}
		if c.Style == term.StylePluginSelection || c.BgStyle == term.StylePluginSelection {
			selected++
		}
	}
	if want := textwidth.String("hi界"); selected != want {
		t.Fatalf("cells in the plugin selection style = %d, want %d, both columns of the wide rune included", selected, want)
	}
}
