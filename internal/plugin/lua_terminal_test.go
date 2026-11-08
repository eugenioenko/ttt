package plugin

import (
	"errors"
	"sort"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

type mockTerminalAPI struct {
	shell    string
	args     []string
	env      []string
	cwd      string
	label    string
	location string

	sentID   string
	sentText string
	tail     string
	tailMax  int
	screen   string
	focused  string
	closed   string
	infos    []TerminalInfo
	spawnErr error
}

func (m *mockTerminalAPI) TerminalSpawn(spec TerminalSpawnSpec) (string, error) {
	m.shell, m.args, m.env, m.cwd, m.label, m.location = spec.Shell, spec.Args, spec.Env, spec.Cwd, spec.Label, spec.Location
	if m.spawnErr != nil {
		return "", m.spawnErr
	}
	return "terminal-7", nil
}

func (m *mockTerminalAPI) TerminalSend(id, text string) error {
	m.sentID, m.sentText = id, text
	return nil
}

func (m *mockTerminalAPI) TerminalTail(id string, maxBytes int) (string, error) {
	m.tailMax = maxBytes
	return m.tail, nil
}

func (m *mockTerminalAPI) TerminalScreen(id string) (string, error) {
	return m.screen, nil
}

func (m *mockTerminalAPI) TerminalClose(id string) error {
	m.closed = id
	return nil
}

func (m *mockTerminalAPI) TerminalFocus(id string) error {
	m.focused = id
	return nil
}

func (m *mockTerminalAPI) TerminalList() []TerminalInfo { return m.infos }

func setupTestPluginWithTerminal(perms PermissionSet, api *mockTerminalAPI) (*Plugin, func()) {
	p, cleanup := newTestPluginBase(perms)
	p.Terminal = api
	return p, cleanup
}

func TestTerminalSpawnPassesTheSpec(t *testing.T) {
	mock := &mockTerminalAPI{}
	p, cleanup := setupTestPluginWithTerminal(
		PermissionSet{TerminalControl: true},
		mock,
	)
	defer cleanup()

	err := p.State.DoString(`
		local term = require("ttt.terminal")
		_G.id, _G.err = term.spawn{
			shell = "pi",
			args = {"--model", "openrouter/free"},
			cwd = "/tmp/project",
			label = "pi",
			env = {FORSETI_AGENT_NAME = "planner", EXTRA = "1"},
		}
	`)
	if err != nil {
		t.Fatalf("lua: %v", err)
	}
	if got := p.State.GetGlobal("err"); got != lua.LNil {
		t.Fatalf("unexpected error: %v", got)
	}
	if id := p.State.GetGlobal("id").String(); id != "terminal-7" {
		t.Fatalf("id = %q, want terminal-7", id)
	}
	if mock.shell != "pi" {
		t.Errorf("shell = %q", mock.shell)
	}
	if len(mock.args) != 2 || mock.args[0] != "--model" || mock.args[1] != "openrouter/free" {
		t.Errorf("args = %v", mock.args)
	}
	if mock.cwd != "/tmp/project" {
		t.Errorf("cwd = %q", mock.cwd)
	}
	if mock.label != "pi" {
		t.Errorf("label = %q", mock.label)
	}
	sort.Strings(mock.env) // Lua map iteration order is unspecified
	if len(mock.env) != 2 || mock.env[0] != "EXTRA=1" || mock.env[1] != "FORSETI_AGENT_NAME=planner" {
		t.Errorf("env = %v", mock.env)
	}
}

func TestTerminalSpawnLocation(t *testing.T) {
	for _, tt := range []struct {
		location, want, err string
	}{
		{"", "", ""},
		{"panel", "panel", ""},
		{"editor", "editor", ""},
		{"sidebar", "", `terminal.spawn: location must be "panel" or "editor"`},
	} {
		t.Run(tt.location, func(t *testing.T) {
			mock := &mockTerminalAPI{location: "unset"}
			p, cleanup := setupTestPluginWithTerminal(PermissionSet{TerminalControl: true}, mock)
			defer cleanup()
			p.State.SetGlobal("location", lua.LString(tt.location))
			if err := p.State.DoString(`
				local term = require("ttt.terminal")
				_G.id, _G.err = term.spawn{shell = "edamame", args = {"README.md"}, location = location}
			`); err != nil {
				t.Fatalf("lua: %v", err)
			}
			if tt.err != "" {
				if got := p.State.GetGlobal("err").String(); got != tt.err {
					t.Fatalf("err = %q, want %q", got, tt.err)
				}
				if mock.location != "unset" {
					t.Fatal("an invalid location must not spawn")
				}
				return
			}
			if mock.location != tt.want {
				t.Fatalf("location = %q, want %q", mock.location, tt.want)
			}
		})
	}
}

func TestTerminalSpawnErrorIsReturned(t *testing.T) {
	mock := &mockTerminalAPI{spawnErr: errors.New("boom")}
	p, cleanup := setupTestPluginWithTerminal(
		PermissionSet{TerminalControl: true},
		mock,
	)
	defer cleanup()

	if err := p.State.DoString(`
		local term = require("ttt.terminal")
		local id, err = term.spawn{shell = "pi"}
		_G.id = id
		_G.err = err
	`); err != nil {
		t.Fatalf("lua: %v", err)
	}
	if p.State.GetGlobal("id") != lua.LNil {
		t.Fatalf("id should be nil on error, got %v", p.State.GetGlobal("id"))
	}
	if got := p.State.GetGlobal("err").String(); got != "boom" {
		t.Fatalf("err = %q, want boom", got)
	}
}

func TestTerminalSendReadFocusList(t *testing.T) {
	mock := &mockTerminalAPI{
		tail:  "pane tail",
		infos: []TerminalInfo{{ID: "terminal-0", Label: "pi", Location: "editor", Cols: 80, Rows: 24}},
	}
	p, cleanup := setupTestPluginWithTerminal(
		PermissionSet{TerminalControl: true, TerminalRead: true},
		mock,
	)
	defer cleanup()

	err := p.State.DoString(`
		local term = require("ttt.terminal")
		term.send("terminal-3", "review this\n")
		_G.tail = term.read("terminal-3", 4000)
		_G.screen = term.screen("terminal-3")
		term.focus("terminal-3")
		term.close("terminal-3")
		local list = term.list()
		_G.first_id = list[1].id
		_G.first_label = list[1].label
		_G.first_cols = list[1].cols
		_G.first_location = list[1].location
	`)
	if err != nil {
		t.Fatalf("lua: %v", err)
	}
	if mock.sentID != "terminal-3" || mock.sentText != "review this\n" {
		t.Fatalf("send got (%q, %q)", mock.sentID, mock.sentText)
	}
	if mock.tailMax != 4000 {
		t.Errorf("read maxBytes = %d, want 4000", mock.tailMax)
	}
	if p.State.GetGlobal("tail").String() != "pane tail" {
		t.Errorf("tail = %q", p.State.GetGlobal("tail").String())
	}
	if mock.focused != "terminal-3" {
		t.Errorf("focus = %q", mock.focused)
	}
	if mock.closed != "terminal-3" {
		t.Errorf("close = %q", mock.closed)
	}
	if p.State.GetGlobal("first_id").String() != "terminal-0" {
		t.Errorf("list id = %q", p.State.GetGlobal("first_id").String())
	}
	if p.State.GetGlobal("first_label").String() != "pi" {
		t.Errorf("list label = %q", p.State.GetGlobal("first_label").String())
	}
	if int(p.State.GetGlobal("first_cols").(lua.LNumber)) != 80 {
		t.Errorf("list cols = %v", p.State.GetGlobal("first_cols"))
	}
	if got := p.State.GetGlobal("first_location").String(); got != "editor" {
		t.Errorf("list location = %q", got)
	}
}

func TestTerminalPermissionsOmitTheFunctions(t *testing.T) {
	// read-only: spawn/send/focus must be absent (capability check idiom)
	ro, cleanup := setupTestPluginWithTerminal(PermissionSet{TerminalRead: true}, &mockTerminalAPI{})
	defer cleanup()
	if err := ro.State.DoString(`
		local term = require("ttt.terminal")
		_G.has_spawn = term.spawn ~= nil
		_G.has_read = term.read ~= nil
	`); err != nil {
		t.Fatalf("lua: %v", err)
	}
	if ro.State.GetGlobal("has_spawn") == lua.LTrue {
		t.Fatal("spawn must be absent without terminal.control")
	}
	if ro.State.GetGlobal("has_read") != lua.LTrue {
		t.Fatal("read must be present with terminal.read")
	}

	// control-only: read/list must be absent
	ctl, cleanup2 := setupTestPluginWithTerminal(PermissionSet{TerminalControl: true}, &mockTerminalAPI{})
	defer cleanup2()
	if err := ctl.State.DoString(`
		local term = require("ttt.terminal")
		_G.has_read = term.read ~= nil
		_G.has_spawn = term.spawn ~= nil
	`); err != nil {
		t.Fatalf("lua: %v", err)
	}
	if ctl.State.GetGlobal("has_read") == lua.LTrue {
		t.Fatal("read must be absent without terminal.read")
	}
	if ctl.State.GetGlobal("has_spawn") != lua.LTrue {
		t.Fatal("spawn must be present with terminal.control")
	}
}
