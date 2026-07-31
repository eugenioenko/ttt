package plugin

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

type commandLineRecorder struct {
	shown    bool
	prefix   string
	text     string
	hidden   int
	active   bool
	onChange func(string)
	onSubmit func(string)
	onCancel func()
	onKey    func(*tcell.EventKey) bool
}

func setupCommandLinePlugin(t *testing.T, perms PermissionSet) (*Plugin, *commandLineRecorder, func()) {
	t.Helper()
	p, cleanup := newTestPluginBase(perms)
	rec := &commandLineRecorder{}
	p.ShowCommandLine = func(opts CommandLineOptions) {
		rec.shown = true
		rec.prefix = opts.Prefix
		rec.text = opts.Text
		rec.onChange = opts.OnChange
		rec.onSubmit = opts.OnSubmit
		rec.onCancel = opts.OnCancel
		rec.onKey = opts.OnKey
		rec.active = true
	}
	p.HideCommandLine = func() {
		rec.hidden++
		rec.active = false
	}
	p.SetCommandLineText = func(text string) { rec.text = text }
	p.CommandLineActive = func() bool { return rec.active }
	return p, rec, cleanup
}

func TestCommandLineShow(t *testing.T) {
	p, rec, cleanup := setupCommandLinePlugin(t, PermissionSet{Keybindings: true})
	defer cleanup()

	err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.command_line.show({ prefix = "/", text = "seed" })
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if !rec.shown {
		t.Fatal("expected show to be called")
	}
	if rec.prefix != "/" || rec.text != "seed" {
		t.Fatalf("expected prefix %q text %q, got %q / %q", "/", "seed", rec.prefix, rec.text)
	}
}

func TestCommandLineShowDefaultsToColonPrefix(t *testing.T) {
	p, rec, cleanup := setupCommandLinePlugin(t, PermissionSet{Keybindings: true})
	defer cleanup()

	if err := p.State.DoString(`require("ttt").command_line.show({})`); err != nil {
		t.Fatalf("error: %v", err)
	}
	if rec.prefix != ":" {
		t.Fatalf("expected default prefix %q, got %q", ":", rec.prefix)
	}
}

func TestCommandLineCallbacks(t *testing.T) {
	p, rec, cleanup := setupCommandLinePlugin(t, PermissionSet{Keybindings: true})
	defer cleanup()

	err := p.State.DoString(`
		local ttt = require("ttt")
		_G.changed = ""
		_G.submitted = ""
		_G.cancelled = false
		ttt.command_line.show({
			on_change = function(text) _G.changed = text end,
			on_submit = function(text) _G.submitted = text end,
			on_cancel = function() _G.cancelled = true end,
		})
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	rec.onChange("ab")
	rec.onSubmit("wq")
	rec.onCancel()

	if got := p.State.GetGlobal("changed").String(); got != "ab" {
		t.Errorf("on_change: expected %q, got %q", "ab", got)
	}
	if got := p.State.GetGlobal("submitted").String(); got != "wq" {
		t.Errorf("on_submit: expected %q, got %q", "wq", got)
	}
	if got := p.State.GetGlobal("cancelled").String(); got != "true" {
		t.Errorf("on_cancel: expected true, got %q", got)
	}
}

// A callback that raises must be contained: it is logged, not propagated.
func TestCommandLineCallbackErrorIsContained(t *testing.T) {
	p, rec, cleanup := setupCommandLinePlugin(t, PermissionSet{Keybindings: true})
	defer cleanup()

	var logged []string
	p.Log = func(level, message string) { logged = append(logged, level+": "+message) }

	err := p.State.DoString(`
		require("ttt").command_line.show({
			on_submit = function() error("boom") end,
		})
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	rec.onSubmit("x") // must not panic

	found := false
	for _, l := range logged {
		if strings.Contains(l, "command_line.on_submit") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the callback error to be logged, got %v", logged)
	}
}

func TestCommandLineHideAndSetText(t *testing.T) {
	p, rec, cleanup := setupCommandLinePlugin(t, PermissionSet{Keybindings: true})
	defer cleanup()

	err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.command_line.show({})
		ttt.command_line.set_text("noh")
		_G.was_active = ttt.command_line.active()
		ttt.command_line.hide()
		_G.still_active = ttt.command_line.active()
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if rec.text != "noh" {
		t.Errorf("expected set_text %q, got %q", "noh", rec.text)
	}
	if rec.hidden != 1 {
		t.Errorf("expected 1 hide, got %d", rec.hidden)
	}
	if p.State.GetGlobal("was_active").String() != "true" {
		t.Error("expected active() to be true while open")
	}
	if p.State.GetGlobal("still_active").String() != "false" {
		t.Error("expected active() to be false after hide")
	}
}

func TestCommandLineWithoutHostCallbacksIsSafe(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{Keybindings: true})
	defer cleanup()

	// Nothing wired (pre-WirePlugin): calls must be no-ops, not panics.
	err := p.State.DoString(`
		local ttt = require("ttt")
		ttt.command_line.show({})
		ttt.command_line.set_text("x")
		ttt.command_line.hide()
		_G.active = ttt.command_line.active()
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("active").String() != "false" {
		t.Error("expected active() to be false with no host callback")
	}
}

func TestCommandLinePermissionDenied(t *testing.T) {
	for _, snippet := range []string{
		`ttt.command_line.show({})`,
		`ttt.command_line.hide()`,
		`ttt.command_line.set_text("x")`,
		`ttt.command_line.active()`,
	} {
		p, rec, cleanup := setupCommandLinePlugin(t, PermissionSet{})
		err := p.State.DoString("local ttt = require(\"ttt\")\n" + snippet)
		if err == nil {
			t.Errorf("%s: expected a permission error", snippet)
		} else if !strings.Contains(err.Error(), "keybindings permission not granted") {
			t.Errorf("%s: expected a keybindings permission error, got %v", snippet, err)
		}
		if rec.shown || rec.hidden > 0 {
			t.Errorf("%s: host callback must not run without permission", snippet)
		}
		cleanup()
	}
}

// on_key is what makes an incremental search possible: the plugin has to see
// C-s, C-g and DEL as they arrive, not just the final text on submit.
func TestCommandLineOnKeyConsumes(t *testing.T) {
	p, rec, cleanup := setupCommandLinePlugin(t, PermissionSet{Keybindings: true})
	defer cleanup()

	err := p.State.DoString(`
		local ttt = require("ttt")
		seen = {}
		ttt.command_line.show({ on_key = function(ev)
			seen[#seen + 1] = ev.key .. "/" .. tostring(ev.mod)
			return ev.key == "Ctrl-S"
		end })
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if rec.onKey == nil {
		t.Fatal("expected on_key to be wired through")
	}

	if got := rec.onKey(tcell.NewEventKey(tcell.KeyCtrlS, "", tcell.ModCtrl)); !got {
		t.Error("expected ctrl+s to be consumed")
	}
	if got := rec.onKey(tcell.NewEventKey(tcell.KeyRune, "a", tcell.ModNone)); got {
		t.Error("expected a declined key to fall through")
	}

	// The event shape must match key.press so a plugin's key normalisation works
	// unchanged in both places.
	if err := p.State.DoString(`
		assert(seen[1] == "Ctrl-S/ctrl", "got " .. tostring(seen[1]))
		assert(seen[2] == "a/nil", "got " .. tostring(seen[2]))
	`); err != nil {
		t.Fatalf("event shape: %v", err)
	}
}

func TestCommandLineOnKeyAbsentIsNil(t *testing.T) {
	p, rec, cleanup := setupCommandLinePlugin(t, PermissionSet{Keybindings: true})
	defer cleanup()

	if err := p.State.DoString(`require("ttt").command_line.show({})`); err != nil {
		t.Fatalf("error: %v", err)
	}
	if rec.onKey != nil {
		t.Fatal("expected on_key to stay nil when not supplied")
	}
}

func TestCommandLineOnKeyErrorDeclines(t *testing.T) {
	p, rec, cleanup := setupCommandLinePlugin(t, PermissionSet{Keybindings: true})
	defer cleanup()

	err := p.State.DoString(`
		require("ttt").command_line.show({ on_key = function() error("boom") end })
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// A throwing hook must not consume the key, or a broken plugin would wedge
	// the prompt with no way to type or cancel.
	if rec.onKey(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone)) {
		t.Error("expected a failing on_key to decline the key")
	}
}
