package e2e

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/gdamore/tcell/v3"
	lua "github.com/yuin/gopher-lua"
)

func TestPluginShowPanelLeavesAFocusedTabAlone(t *testing.T) {
	h := newTestHarness(t, 160, 24)
	defer h.stop()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	src := filepath.Join(h.dir, "chat.lua")
	if err := os.WriteFile(src, []byte(`
		local ttt = require("ttt")
		ttt.register({
			bottom = {
				title = "Chat",
				render = function(panel)
					panel:markdown({ text = "hello", follow = true })
					panel:input({ placeholder = "say", autofocus = true })
				end,
			},
		})
		function show() return ttt.show_panel("bottom") end
		function focus() return ttt.focus_panel("bottom") end
	`), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, src)
	p := h.app.PluginManager.FindPlugin("chat")
	if p == nil {
		t.Fatal("plugin not loaded")
	}
	defer p.Destroy()
	call := func(fn string) bool {
		t.Helper()
		if err := p.State.CallByParam(lua.P{Fn: p.State.GetGlobal(fn), NRet: 1, Protect: true}); err != nil {
			t.Fatal(err)
		}
		ret := p.State.Get(-1)
		p.State.Pop(1)
		return lua.LVAsBool(ret)
	}
	bottom := &h.app.BottomPanel.TabbedPanel

	h.app.ContentSplit.ShowBottom = true
	bottom.SetActivePanel("output")
	h.app.Root.SetFocus(bottom.ActiveWidget())
	if call("show") {
		t.Fatal("show_panel reported showing over a focused tab")
	}
	if bottom.ActivePanel != "output" {
		t.Fatalf("active panel = %q, want output to stay", bottom.ActivePanel)
	}
	h.redraw()
	h.assertContains("● Chat")

	if !call("focus") {
		t.Fatal("focus_panel reported false")
	}
	h.redraw()
	if bottom.ActivePanel != "plugin.chat" || h.app.Root.Focused != bottom.ActiveWidget() {
		t.Fatalf("active %q, focused %T: want the chat panel focused", bottom.ActivePanel, h.app.Root.Focused)
	}
	h.assertNotContains("● Chat")
	h.pressRune('h')
	h.pressRune('i')
	h.redraw()
	h.assertContains("hi")
	h.assertNotContains("say")

	h.app.FocusEditor()
	bottom.SetActivePanel("output")
	if !call("show") || bottom.ActivePanel != "plugin.chat" {
		t.Fatal("show_panel did not switch an unfocused bottom panel")
	}
	if h.app.Root.Focused == bottom.ActiveWidget() {
		t.Fatal("show_panel moved focus")
	}
}

func loadChatPlugin(t *testing.T, h *testHarness, body string) func(string) bool {
	t.Helper()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	src := filepath.Join(h.dir, "chat.lua")
	if err := os.WriteFile(src, []byte(`
		local ttt = require("ttt")
		`+body+`
		function show() return ttt.show_panel("bottom") end
		function focus() return ttt.focus_panel("bottom") end
	`), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, src)
	p := h.app.PluginManager.FindPlugin("chat")
	if p == nil {
		t.Fatal("plugin not loaded")
	}
	t.Cleanup(p.Destroy)
	return func(fn string) bool {
		t.Helper()
		if err := p.State.CallByParam(lua.P{Fn: p.State.GetGlobal(fn), NRet: 1, Protect: true}); err != nil {
			t.Fatal(err)
		}
		ret := p.State.Get(-1)
		p.State.Pop(1)
		return lua.LVAsBool(ret)
	}
}

func findLast(h *testHarness, text string) (int, int) {
	lines := strings.Split(h.screenText(), "\n")
	for y := len(lines) - 1; y >= 0; y-- {
		if x := displayColumnOf(lines[y], text); x >= 0 {
			return x, y
		}
	}
	return -1, -1
}

func TestPluginPanelClickKeepsTheClickedWidget(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	call := loadChatPlugin(t, h, `
		ttt.register({ bottom = { title = "Chat", render = function(panel)
			panel:list({ items = { { id = "a", label = "itemA" }, { id = "b", label = "itemB" } } })
			panel:input({ placeholder = "say", autofocus = true })
		end } })
	`)
	call("show")
	h.app.FocusEditor()
	h.redraw()
	x, y := findLast(h, "itemB")
	if x < 0 {
		t.Fatalf("list not on screen:\n%s", h.screenText())
	}
	h.click(x+1, y)
	h.pressRune('Q')
	h.redraw()
	if h.containsText("Q") && !h.containsText("say") {
		t.Errorf("click on list left keyboard focus on the autofocus input; screen:\n%s", h.screenText())
	}
}

func TestFocusPanelReachesTheInputWhenAlreadyFocused(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	call := loadChatPlugin(t, h, `
		ttt.register({ bottom = { title = "Chat", render = function(panel)
			panel:list({ items = { { id = "a", label = "itemA" }, { id = "b", label = "itemB" } } })
			panel:input({ placeholder = "say", autofocus = true })
		end } })
	`)
	call("focus")
	h.redraw()
	h.pressKey(tcell.KeyTab, tcell.ModNone) // move focus off the input
	h.redraw()
	if !call("focus") {
		t.Fatal("focus returned false")
	}
	h.redraw()
	h.pressRune('Q')
	h.redraw()
	if !h.containsText("Q") {
		t.Errorf("focus_panel did not land on autofocus input when panel already focused; screen:\n%s", h.screenText())
	}
}

func TestLaterAutofocusInputDoesNotStealFocus(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	call := loadChatPlugin(t, h, `
		mode = false
		ttt.register({ bottom = { title = "Chat", render = function(panel)
			panel:list({ items = { { id = "a", label = "itemA" }, { id = "b", label = "itemB" } } })
			if mode then panel:input({ placeholder = "say", autofocus = true }) end
		end } })
		function enable() mode = true return true end
	`)
	call("focus")
	h.redraw()
	h.redraw()
	call("enable")
	h.redraw()
	h.pressRune('Q')
	h.redraw()
	if h.containsText("Q") {
		t.Errorf("a later render moved focus from the list to the new autofocus input; screen:\n%s", h.screenText())
	}
}

func TestFocusPanelLeavesAnOpenDialogAlone(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	call := loadChatPlugin(t, h, `
		ttt.register({ bottom = { title = "Chat", render = function(panel)
			panel:input({ placeholder = "say", autofocus = true })
		end } })
	`)
	h.app.ShowInputDialog("Rename", "name", "", func(string) {})
	h.redraw()
	h.pressRune('a')
	call("focus")
	h.redraw()
	h.pressRune('b')
	h.redraw()
	if !h.containsText("ab") {
		t.Errorf("typing into the modal dialog stopped after focus_panel; screen:\n%s", h.screenText())
	}
}

func TestFocusPanelWorksWithTheFindBarOpen(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	call := loadChatPlugin(t, h, `
		ttt.register({ bottom = { title = "Chat", render = function(panel)
			panel:input({ placeholder = "say", autofocus = true })
		end } })
	`)
	h.app.FocusEditor()
	h.app.OpenFind()
	h.redraw()
	h.click(60, 10)
	if !call("focus") {
		t.Fatal("focus_panel refused with only the find bar open")
	}
	h.redraw()
	h.pressRune('Q')
	h.redraw()
	if h.containsText("say") {
		t.Errorf("Q did not reach the panel input; screen:\n%s", h.screenText())
	}
}

func TestFocusPanelReachesAnInputAddedInTheSameCallback(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	call := loadChatPlugin(t, h, `
		replying = false
		ttt.register({ bottom = { title = "Chat", render = function(panel)
			panel:list({ items = { { id = "a", label = "itemA" } } })
			if replying then panel:input({ placeholder = "say", autofocus = true }) end
		end } })
		function reply() replying = true return ttt.focus_panel("bottom") end
	`)
	call("show")
	h.app.FocusEditor()
	h.redraw()
	if !call("reply") {
		t.Fatal("focus_panel returned false")
	}
	h.redraw()
	h.pressRune('Q')
	h.redraw()
	if h.containsText("say") {
		t.Errorf("Q did not reach the input the callback added; screen:\n%s", h.screenText())
	}
}

func TestFlagOnAnOverflowedTabMarksTheChevron(t *testing.T) {
	h := newTestHarness(t, 120, 30)
	defer h.stop()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	src := filepath.Join(h.dir, "side.lua")
	if err := os.WriteFile(src, []byte(`
		local ttt = require("ttt")
		ttt.register({ sidebar = { title = "SideChat", render = function(panel)
			panel:label({ text = "hi" })
		end } })
		function show() return ttt.show_panel("sidebar") end
	`), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, src)
	p := h.app.PluginManager.FindPlugin("side")
	if p == nil {
		t.Fatal("not loaded")
	}
	defer p.Destroy()
	h.app.FocusSidebar()
	h.redraw()
	chevronMark := func() rune {
		for _, line := range strings.Split(h.screenText(), "\n") {
			if r := []rune(line); slices.Contains(r, '»') {
				return r[slices.Index(r, '»')-1]
			}
		}
		t.Fatalf("no overflow chevron:\n%s", h.screenText())
		return 0
	}
	if ids, _ := h.app.Sidebar.HiddenTabs(); !slices.Contains(ids, "plugin.side") {
		t.Fatalf("the plugin tab is not in the overflow at this width: %v", ids)
	}
	if m := chevronMark(); m != ' ' {
		t.Fatalf("chevron marked %q before any flag", m)
	}
	if err := p.State.CallByParam(lua.P{Fn: p.State.GetGlobal("show"), NRet: 1, Protect: true}); err != nil {
		t.Fatal(err)
	}
	ret := lua.LVAsBool(p.State.Get(-1))
	h.redraw()
	if ret || h.app.Sidebar.ActivePanel == "plugin.side" {
		t.Fatalf("show_panel switched a focused sidebar (returned %v)", ret)
	}
	if m := chevronMark(); m == ' ' {
		t.Errorf("flagged hidden tab left the chevron unmarked:\n%s", h.screenText())
	}
}
