package plugin

import (
	"context"
	"fmt"
	"testing"
)

type mockSystemAPI struct {
	execBinary string
	execArgs   []string
	stdout     string
	stderr     string
	exitCode   int
	execErr    error
	envVars    map[string]string
	execStdin  string
	execEnv    []string
	// block makes Exec wait for its context, as a running child would.
	block    bool
	started  chan struct{}
	returned chan struct{}
}

func (m *mockSystemAPI) Exec(ctx context.Context, binary string, args []string, opts ExecOptions) (string, string, int, error) {
	m.execBinary = binary
	m.execArgs = args
	m.execStdin = opts.Stdin
	m.execEnv = opts.Env
	if m.returned != nil {
		defer close(m.returned)
	}
	if m.started != nil {
		close(m.started)
	}
	if m.block {
		<-ctx.Done()
		return "", "signal: killed", -1, nil
	}
	return m.stdout, m.stderr, m.exitCode, m.execErr
}

func (m *mockSystemAPI) Env(name string) string {
	if m.envVars != nil {
		return m.envVars[name]
	}
	return ""
}

func setupTestPluginWithSystem(perms PermissionSet, sys *mockSystemAPI) (*Plugin, func()) {
	p, cleanup := newTestPluginBase(perms)
	p.System = sys
	return p, cleanup
}

func TestSystemExec(t *testing.T) {
	mock := &mockSystemAPI{stdout: "container1\ncontainer2\n", exitCode: 0}
	p, cleanup := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"docker"}},
		mock,
	)
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		local result = sys.exec("docker", {"ps", "--format", "{{.Names}}"})
		_G.stdout = result.stdout
		_G.exit_code = result.exit_code
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("stdout").String() != "container1\ncontainer2\n" {
		t.Errorf("unexpected stdout: %q", p.State.GetGlobal("stdout").String())
	}
	if mock.execBinary != "docker" {
		t.Errorf("expected binary 'docker', got %q", mock.execBinary)
	}
	if len(mock.execArgs) != 3 || mock.execArgs[0] != "ps" {
		t.Errorf("unexpected args: %v", mock.execArgs)
	}
}

func TestSystemExecDeniedBinary(t *testing.T) {
	mock := &mockSystemAPI{}
	p, cleanup := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"docker"}},
		mock,
	)
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		local result, err = sys.exec("rm", {"-rf", "/"})
		assert(result == nil, "expected nil result")
		assert(err ~= nil, "expected error string")
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSystemExecError(t *testing.T) {
	mock := &mockSystemAPI{execErr: fmt.Errorf("command not found")}
	p, cleanup := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"missing"}},
		mock,
	)
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		local result = sys.exec("missing", {})
		_G.stderr = result.stderr
		_G.code = result.exit_code
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("code").String() != "-1" {
		t.Errorf("expected exit code -1, got %s", p.State.GetGlobal("code").String())
	}
}

func TestSystemEnv(t *testing.T) {
	mock := &mockSystemAPI{envVars: map[string]string{"HOME": "/home/test"}}
	p, cleanup := setupTestPluginWithSystem(
		PermissionSet{SystemEnv: true},
		mock,
	)
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		_G.home = sys.env("HOME")
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("home").String() != "/home/test" {
		t.Errorf("expected '/home/test', got %q", p.State.GetGlobal("home").String())
	}
}

func TestSystemNoExecPermission(t *testing.T) {
	mock := &mockSystemAPI{}
	p, cleanup := setupTestPluginWithSystem(PermissionSet{}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		sys.exec("ls", {})
	`)
	if err == nil {
		t.Fatal("expected error when system.exec not granted")
	}
}

func TestSystemNoEnvPermission(t *testing.T) {
	mock := &mockSystemAPI{}
	p, cleanup := setupTestPluginWithSystem(PermissionSet{}, mock)
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		sys.env("HOME")
	`)
	if err == nil {
		t.Fatal("expected error when system.env not granted")
	}
}

func TestSystemExecAsyncWithoutArgs(t *testing.T) {
	mock := &mockSystemAPI{stdout: "output"}

	ch := make(chan func(), 1)
	p, cleanup := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"echo"}},
		mock,
	)
	defer cleanup()

	p.PostAsync = func(result *PluginAsyncResult) {
		ch <- result.Callback
	}

	err := p.State.DoString(`
		local sys = require("ttt.system")
		sys.exec_async("echo", function(result)
			_G.got_stdout = result.stdout
		end)
	`)
	if err != nil {
		t.Fatalf("exec_async without args failed: %v", err)
	}

	callback := <-ch
	callback()

	if p.State.GetGlobal("got_stdout").String() != "output" {
		t.Errorf("expected stdout 'output', got %q", p.State.GetGlobal("got_stdout").String())
	}
}

func TestSystemExecAsyncRaceSafety(t *testing.T) {
	mock := &mockSystemAPI{stdout: "ok"}

	p, _ := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"echo"}},
		mock,
	)

	done := make(chan struct{})
	p.PostAsync = func(result *PluginAsyncResult) {
		close(done)
	}

	err := p.State.DoString(`
		local sys = require("ttt.system")
		sys.exec_async("echo", {"hello"}, function(result) end)
	`)
	if err != nil {
		t.Fatalf("exec_async call failed: %v", err)
	}

	// Wait for goroutine to finish posting, then destroy.
	// The race detector would flag if SafePostAsync and Destroy weren't synchronized.
	<-done
	p.Destroy()
}

func TestSystemExecAsyncNilStateSafety(t *testing.T) {
	mock := &mockSystemAPI{stdout: "ok"}

	ch := make(chan func(), 1)
	p, _ := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"echo"}},
		mock,
	)

	p.PostAsync = func(result *PluginAsyncResult) {
		ch <- result.Callback
	}

	err := p.State.DoString(`
		local sys = require("ttt.system")
		sys.exec_async("echo", {"hello"}, function(result) end)
	`)
	if err != nil {
		t.Fatalf("exec_async call failed: %v", err)
	}

	capturedCallback := <-ch

	p.Destroy()

	capturedCallback()
}

func TestSystemExecStdin(t *testing.T) {
	mock := &mockSystemAPI{stdout: "& helllo 3 1: hello, hell, he'll\n"}
	p, cleanup := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"aspell"}},
		mock,
	)
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		local result = sys.exec("aspell", {"pipe"}, {stdin = "!\n^helllo\n"})
		_G.stdout = result.stdout
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mock.execStdin != "!\n^helllo\n" {
		t.Errorf("stdin not forwarded, got %q", mock.execStdin)
	}
	if len(mock.execArgs) != 1 || mock.execArgs[0] != "pipe" {
		t.Errorf("opts table leaked into args: %v", mock.execArgs)
	}
}

func TestSystemExecWithoutStdin(t *testing.T) {
	mock := &mockSystemAPI{stdout: "ok"}
	p, cleanup := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"echo"}},
		mock,
	)
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		sys.exec("echo", {"hi"})
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mock.execStdin != "" {
		t.Errorf("expected empty stdin, got %q", mock.execStdin)
	}
}

func TestSystemExecAsyncStdin(t *testing.T) {
	mock := &mockSystemAPI{stdout: "piped"}

	ch := make(chan func(), 1)
	p, cleanup := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"aspell"}},
		mock,
	)
	defer cleanup()

	p.PostAsync = func(result *PluginAsyncResult) {
		ch <- result.Callback
	}

	err := p.State.DoString(`
		local sys = require("ttt.system")
		sys.exec_async("aspell", {"pipe"}, {stdin = "^word\n"}, function(result)
			_G.got_stdout = result.stdout
		end)
	`)
	if err != nil {
		t.Fatalf("exec_async with stdin failed: %v", err)
	}

	callback := <-ch
	callback()

	if mock.execStdin != "^word\n" {
		t.Errorf("stdin not forwarded, got %q", mock.execStdin)
	}
	if p.State.GetGlobal("got_stdout").String() != "piped" {
		t.Errorf("expected stdout 'piped', got %q", p.State.GetGlobal("got_stdout").String())
	}
}
