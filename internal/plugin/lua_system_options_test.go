package plugin

import (
	"strings"
	"testing"
	"time"
)

func asyncResults(p *Plugin) chan func() {
	ch := make(chan func(), 4)
	p.PostAsync = func(result *PluginAsyncResult) { ch <- result.Callback }
	return ch
}

func TestSystemExecEnv(t *testing.T) {
	mock := &mockSystemAPI{stdout: "ok"}
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql"}}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		sys.exec("psql", {"-c", "select 1"}, {env = {PGPASSWORD = "s3cret", PGPORT = 5433}})
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(mock.execEnv, ","); got != "PGPASSWORD=s3cret,PGPORT=5433" {
		t.Errorf("env = %q", got)
	}
	if strings.Contains(strings.Join(mock.execArgs, " "), "s3cret") {
		t.Errorf("env value leaked into args: %v", mock.execArgs)
	}
}

func TestSystemExecAsyncEnv(t *testing.T) {
	mock := &mockSystemAPI{}
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql"}}, mock)
	defer cleanup()
	ch := asyncResults(p)

	if err := p.State.DoString(`
		require("ttt.system").exec_async("psql", {}, {env = {PGPASSWORD = "s3cret"}}, function() end)
	`); err != nil {
		t.Fatal(err)
	}
	(<-ch)()
	if got := strings.Join(mock.execEnv, ","); got != "PGPASSWORD=s3cret" {
		t.Errorf("env = %q", got)
	}
}

func TestSystemExecEnvRejected(t *testing.T) {
	tests := []struct {
		binary string
		env    string
	}{
		{"psql", `{[""] = "x"}`},
		{"psql", `{["A=B"] = "x"}`},
		{"psql", `{["A\0B"] = "x"}`},
		{"psql", `{A = "x\0y"}`},
		{"psql", `{A = true}`},
		{"psql", `{"positional"}`},
		{"psql", `"PGPASSWORD=x"`},
		{"psql", `{PATH = "/tmp"}`},
		{"psql", `{Path = "/tmp"}`},
		{"psql", `{LD_PRELOAD = "/tmp/evil.so"}`},
		{"psql", `{DYLD_INSERT_LIBRARIES = "/tmp/evil.dylib"}`},
		{"git", `{GIT_SSH_COMMAND = "evil"}`},
		{"/usr/bin/git", `{GIT_CONFIG_COUNT = "1"}`},
	}
	for _, tt := range tests {
		t.Run(tt.binary+" "+tt.env, func(t *testing.T) {
			mock := &mockSystemAPI{}
			p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{tt.binary}}, mock)
			defer cleanup()
			err := p.State.DoString(`require("ttt.system").exec("` + tt.binary + `", {}, {env = ` + tt.env + `})`)
			if err == nil {
				t.Fatal("expected invalid env to raise an error")
			}
			if mock.execBinary != "" {
				t.Fatal("binary ran despite invalid env")
			}
		})
	}
}

func TestSystemExecEnvGitVarsAllowedForOtherBinaries(t *testing.T) {
	mock := &mockSystemAPI{}
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"lazytool"}}, mock)
	defer cleanup()
	if err := p.State.DoString(`require("ttt.system").exec("lazytool", {}, {env = {GIT_DIR = "/repo/.git"}})`); err != nil {
		t.Fatal(err)
	}
}

func TestSystemExecTimeout(t *testing.T) {
	mock := &mockSystemAPI{block: true}
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql"}}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local r = require("ttt.system").exec("psql", {}, {timeout_ms = 20})
		_G.timed_out = r.timed_out
		_G.cancelled = r.cancelled
		_G.code = r.exit_code
		_G.stderr = r.stderr
	`)
	if err != nil {
		t.Fatal(err)
	}
	if p.State.GetGlobal("timed_out").String() != "true" || p.State.GetGlobal("cancelled").String() != "nil" {
		t.Errorf("timed_out = %s, cancelled = %s", p.State.GetGlobal("timed_out"), p.State.GetGlobal("cancelled"))
	}
	if p.State.GetGlobal("code").String() != "-1" {
		t.Errorf("exit_code = %s, want -1", p.State.GetGlobal("code"))
	}
	if got := p.State.GetGlobal("stderr").String(); got != "timed out after 20ms" {
		t.Errorf("stderr = %q", got)
	}
}

func TestSystemExecTimeoutNotReportedOnSuccess(t *testing.T) {
	mock := &mockSystemAPI{stdout: "fast"}
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql"}}, mock)
	defer cleanup()
	err := p.State.DoString(`
		local r = require("ttt.system").exec("psql", {}, {timeout_ms = 5000})
		assert(r.stdout == "fast" and r.exit_code == 0 and r.timed_out == nil)
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSystemExecInvalidTimeout(t *testing.T) {
	for _, v := range []string{"0", "-5", `"100"`} {
		mock := &mockSystemAPI{}
		p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql"}}, mock)
		err := p.State.DoString(`require("ttt.system").exec("psql", {}, {timeout_ms = ` + v + `})`)
		cleanup()
		if err == nil {
			t.Errorf("timeout_ms = %s: expected an error", v)
		}
	}
}

func TestSystemExecAsyncTimeout(t *testing.T) {
	mock := &mockSystemAPI{block: true}
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql"}}, mock)
	defer cleanup()
	ch := asyncResults(p)

	err := p.State.DoString(`
		require("ttt.system").exec_async("psql", {}, {timeout_ms = 20}, function(r)
			_G.timed_out = r.timed_out
		end)
	`)
	if err != nil {
		t.Fatal(err)
	}
	(<-ch)()
	if p.State.GetGlobal("timed_out").String() != "true" {
		t.Errorf("timed_out = %s", p.State.GetGlobal("timed_out"))
	}
}

func TestSystemExecAsyncCancel(t *testing.T) {
	mock := &mockSystemAPI{block: true, started: make(chan struct{})}
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql"}}, mock)
	defer cleanup()
	ch := asyncResults(p)

	err := p.State.DoString(`
		_G.handle = require("ttt.system").exec_async("psql", {"-c", "select pg_sleep(60)"}, function(r)
			_G.cancelled = r.cancelled
			_G.timed_out = r.timed_out
			_G.stderr = r.stderr
			_G.code = r.exit_code
		end)
	`)
	if err != nil {
		t.Fatal(err)
	}
	<-mock.started
	if err := p.State.DoString(`_G.first = handle:cancel(); _G.second = handle.cancel()`); err != nil {
		t.Fatal(err)
	}
	if p.State.GetGlobal("first").String() != "true" {
		t.Errorf("cancel() of a running child = %s, want true", p.State.GetGlobal("first"))
	}
	select {
	case cb := <-ch:
		cb()
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not end the child")
	}
	if p.State.GetGlobal("cancelled").String() != "true" || p.State.GetGlobal("timed_out").String() != "nil" {
		t.Errorf("cancelled = %s, timed_out = %s", p.State.GetGlobal("cancelled"), p.State.GetGlobal("timed_out"))
	}
	if p.State.GetGlobal("stderr").String() != "cancelled" || p.State.GetGlobal("code").String() != "-1" {
		t.Errorf("stderr = %q, exit_code = %s", p.State.GetGlobal("stderr").String(), p.State.GetGlobal("code"))
	}
	if err := p.State.DoString(`assert(handle:cancel() == false)`); err != nil {
		t.Errorf("cancel() after completion: %v", err)
	}
}

func TestSystemExecAsyncWithoutArgsReturnsHandle(t *testing.T) {
	mock := &mockSystemAPI{}
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"uptime"}}, mock)
	defer cleanup()
	ch := asyncResults(p)
	if err := p.State.DoString(`
		local h = require("ttt.system").exec_async("uptime", function() end)
		assert(type(h) == "table" and type(h.cancel) == "function")
	`); err != nil {
		t.Fatal(err)
	}
	(<-ch)()
}

func TestSystemExecAsyncKilledOnDestroy(t *testing.T) {
	mock := &mockSystemAPI{block: true, started: make(chan struct{}), returned: make(chan struct{})}
	p, _ := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"psql"}}, mock)
	ch := asyncResults(p)

	if err := p.State.DoString(`require("ttt.system").exec_async("psql", {}, function() error("callback ran after Destroy") end)`); err != nil {
		t.Fatal(err)
	}
	<-mock.started
	p.Destroy()
	select {
	case <-mock.returned:
	default:
		t.Fatal("Destroy returned before the running child was killed")
	}
	select {
	case cb := <-ch:
		cb()
	default:
	}
	if p.errorCount != 0 {
		t.Fatal("callback ran after Destroy")
	}
}
