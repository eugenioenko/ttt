package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/plugin"
	lua "github.com/yuin/gopher-lua"
)

func loadFormsPlugin(t *testing.T, h *testHarness) *plugin.Plugin {
	t.Helper()
	cfg := filepath.Join(h.dir, "config")
	h.app.PluginManager = plugin.NewManager(filepath.Join(cfg, "plugins"), filepath.Join(cfg, "plugins.ttt.json"))
	src := filepath.Join(h.dir, "forms.lua")
	if err := os.WriteFile(src, []byte(`
		local ttt = require("ttt")
		typed = ""
		local function render_form(panel)
			panel:label("Form tab")
			panel:input({ placeholder = "type here", autofocus = true, on_change = function(t) typed = t end })
		end
		function open_form(focus)
			ttt.open_tab({ id = "form", title = "Form", focus = focus, render = render_form })
		end
		ttt.register({
			sidebar = { title = "Forms", render = function(panel)
				panel:list({
					items = { { id = "first", label = "First" }, { id = "form", label = "Open the form" } },
					on_select = function(node) if node.id == "form" then open_form() end end,
				})
			end },
			commands = { { id = "forms.open", title = "Forms: Open", handler = function() open_form() end } },
		})
	`), 0644); err != nil {
		t.Fatal(err)
	}
	app.LoadPluginFromFile(h.app, src)
	p := h.app.PluginManager.FindPlugin("forms")
	if p == nil {
		t.Fatal("plugin not loaded")
	}
	t.Cleanup(p.Destroy)
	return p
}

func screenRowOf(t *testing.T, h *testHarness, text string) int {
	t.Helper()
	screen := h.screenText()
	i := strings.Index(screen, text)
	if i < 0 {
		t.Fatalf("%q not on screen:\n%s", text, screen)
	}
	return strings.Count(screen[:i], "\n")
}

func typeInto(h *testHarness, text string) {
	for _, r := range text {
		h.pressRune(r)
	}
}

func typedText(p *plugin.Plugin) string { return lua.LVAsString(p.State.GetGlobal("typed")) }

func TestPluginOpenTabFromSidebarClickFocusesItsInput(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()
	p := loadFormsPlugin(t, h)
	h.app.Sidebar.SetActivePanel("plugin.forms")
	h.app.FocusSidebar()
	h.redraw()

	h.click(4, screenRowOf(t, h, "Open the form"))
	if h.app.Root.Focused != h.app.EditorGroup {
		t.Fatalf("focus = %T after the click opened a tab, want the editor group", h.app.Root.Focused)
	}
	typeInto(h, "abc")
	if got := typedText(p); got != "abc" {
		t.Fatalf("typed %q into the form input, want abc", got)
	}

	h.click(4, screenRowOf(t, h, "First"))
	if h.app.Root.Focused != h.app.Sidebar.ActiveWidget() {
		t.Fatalf("a later sidebar click left focus on %T", h.app.Root.Focused)
	}
}

func TestPluginOpenTabFromCommandFocusesItsInput(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()
	p := loadFormsPlugin(t, h)
	h.app.FocusSidebar()

	h.exec("forms.open")
	typeInto(h, "hi")
	if got := typedText(p); got != "hi" {
		t.Fatalf("typed %q into the form input, want hi (focus %T)", got, h.app.Root.Focused)
	}
}

func TestPluginOpenTabFocusFalseAndFocusTab(t *testing.T) {
	h := newTestHarness(t, 100, 24)
	defer h.stop()
	p := loadFormsPlugin(t, h)
	h.app.FocusSidebar()
	sidebar := h.app.Root.Focused

	if err := p.State.DoString(`open_form(false)`); err != nil {
		t.Fatal(err)
	}
	h.redraw()
	if path, _ := h.app.EditorGroup.TabInfo(h.app.EditorGroup.ActiveTabIndex()); path != "plugin:forms:form" {
		t.Fatalf("active tab = %q, want the form", path)
	}
	if h.app.Root.Focused != sidebar {
		t.Fatalf("focus = false moved focus to %T", h.app.Root.Focused)
	}

	if err := p.State.DoString(`found = require("ttt").focus_tab("form")`); err != nil {
		t.Fatal(err)
	}
	h.redraw()
	if p.State.GetGlobal("found") != lua.LTrue {
		t.Fatal("focus_tab did not find the open tab")
	}
	typeInto(h, "ok")
	if got := typedText(p); got != "ok" {
		t.Fatalf("typed %q after focus_tab, want ok (focus %T)", got, h.app.Root.Focused)
	}
}
