package plugin

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// mockSpawnedProcess is a test SpawnedProcess: configurable streams and exit
// code, recorded writes, and signal channels for synchronization with the
// plugin's pump/writer goroutines.
type mockSpawnedProcess struct {
	stdin    *bytes.Buffer
	stdout   io.Reader
	stderr   io.Reader
	code     int
	writeErr error

	mu          sync.Mutex
	killed      bool
	stdinClosed bool

	wrote  chan struct{}
	closed chan struct{}
	waited chan struct{}
}

func (m *mockSpawnedProcess) Write(data []byte) (int, error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	n, err := m.stdin.Write(data)
	if m.wrote != nil {
		select {
		case m.wrote <- struct{}{}:
		default:
		}
	}
	return n, err
}

func (m *mockSpawnedProcess) CloseStdin() error {
	m.mu.Lock()
	m.stdinClosed = true
	m.mu.Unlock()
	if m.closed != nil {
		close(m.closed)
	}
	return nil
}

func (m *mockSpawnedProcess) Kill() error {
	m.mu.Lock()
	m.killed = true
	m.mu.Unlock()
	return nil
}

func (m *mockSpawnedProcess) killedState() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.killed
}

func (m *mockSpawnedProcess) PID() int          { return 4242 }
func (m *mockSpawnedProcess) Stdout() io.Reader { return m.stdout }
func (m *mockSpawnedProcess) Stderr() io.Reader { return m.stderr }

func (m *mockSpawnedProcess) Wait() (int, error) {
	if m.waited != nil {
		close(m.waited)
	}
	return m.code, nil
}

// postChan routes async results to a channel so tests can run callbacks on
// the test goroutine, matching the production UI-thread delivery.
func postChan() (func(*PluginAsyncResult), chan *PluginAsyncResult) {
	ch := make(chan *PluginAsyncResult, 64)
	return func(r *PluginAsyncResult) { ch <- r }, ch
}

// runPosted runs up to n posted callbacks, waiting for each.
func runPosted(t *testing.T, ch chan *PluginAsyncResult, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case r := <-ch:
			r.Callback()
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for async callback %d of %d", i+1, n)
		}
	}
}

// runPostedUntil runs posted callbacks until done reports true. Process
// callbacks are batched, so how many posts carry them is not fixed.
func runPostedUntil(t *testing.T, ch chan *PluginAsyncResult, done func() bool) {
	t.Helper()
	for !done() {
		select {
		case r := <-ch:
			r.Callback()
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for async callbacks")
		}
	}
}

func waitFor(t *testing.T, ch chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func blockingSpawnedProcess(t *testing.T) *mockSpawnedProcess {
	t.Helper()
	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()
	mp := &mockSpawnedProcess{
		stdin: &bytes.Buffer{}, stdout: stdout, stderr: stderr,
		waited: make(chan struct{}),
	}
	t.Cleanup(func() {
		stdoutWriter.Close()
		stderrWriter.Close()
		waitFor(t, mp.waited, "process reaping")
	})
	return mp
}

func TestSpawnRejectsInvalidOptions(t *testing.T) {
	for _, opts := range []string{
		`{env = {PATH = "/tmp"}}`,
		`{env = {Path = "/tmp"}}`,
		`{env = {LD_PRELOAD = "/tmp/evil.so"}}`,
		`{env = {DYLD_INSERT_LIBRARIES = "/tmp/evil.dylib"}}`,
		`{env = {GIT_CONFIG_COUNT = 1}}`,
		`{env = {GIT_SSH_COMMAND = "evil"}}`,
		`{env = {[""] = "x"}}`,
		`{env = {["A=B"] = "x"}}`,
		`{env = {["A\0B"] = "x"}}`,
		`{env = {A = "x\0y"}}`,
		`{env = {A = true}}`,
		`{env = {"positional"}}`,
		`{env = "A=x"}`,
		`{on_stdout = true}`,
		`{on_stderr = "callback"}`,
		`{on_exit = {}}`,
	} {
		t.Run(opts, func(t *testing.T) {
			mock := &mockSystemAPI{}
			p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"git"}}, mock)
			defer cleanup()
			if err := p.State.DoString(`require("ttt.system").spawn("git", ` + opts + `)`); err == nil {
				t.Fatal("expected invalid spawn options to raise an error")
			}
			if mock.spawnBinary != "" {
				t.Fatal("host Spawn called despite invalid options")
			}
		})
	}
}

func TestSpawnDeniedWithoutExecGrant(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{})
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		_G.spawn_fn = sys.spawn
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("spawn_fn") != lua.LNil {
		t.Error("expected sys.spawn to be nil without system.exec grant")
	}
}

func TestSpawnDeniedBinary(t *testing.T) {
	mock := &mockSystemAPI{}
	p, cleanup := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"docker"}},
		mock,
	)
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		local proc, err = sys.spawn("rm", {})
		_G.proc = proc
		_G.err = err
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("proc") != lua.LNil {
		t.Error("expected nil proc for a binary outside the allowlist")
	}
	if p.State.GetGlobal("err") == lua.LNil {
		t.Error("expected an error string")
	}
	if mock.spawnBinary != "" {
		t.Error("host Spawn should not have been called")
	}
}

func TestSpawnPassesOptions(t *testing.T) {
	mock := &mockSystemAPI{}
	p, cleanup := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"devin"}},
		mock,
	)
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		local proc, err = sys.spawn("devin", {
			args = {"acp"},
			cwd = "/tmp/ws",
			env = {FOO = "bar", PORT = 5433, GIT_DIR = "/tmp/ws/.git"},
		})
		assert(proc ~= nil, tostring(err))
		_G.pid = proc:pid()
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mock.spawnBinary != "devin" {
		t.Errorf("expected binary 'devin', got %q", mock.spawnBinary)
	}
	if len(mock.spawnArgs) != 1 || mock.spawnArgs[0] != "acp" {
		t.Errorf("unexpected args: %v", mock.spawnArgs)
	}
	if mock.spawnOpts.Cwd != "/tmp/ws" {
		t.Errorf("unexpected cwd: %q", mock.spawnOpts.Cwd)
	}
	if strings.Join(mock.spawnOpts.Env, ",") != "FOO=bar,GIT_DIR=/tmp/ws/.git,PORT=5433" {
		t.Errorf("unexpected env: %v", mock.spawnOpts.Env)
	}
	if p.State.GetGlobal("pid") != lua.LNumber(4242) {
		t.Errorf("unexpected pid: %v", p.State.GetGlobal("pid"))
	}
}

func TestSpawnSpawnError(t *testing.T) {
	mock := &mockSystemAPI{spawnErr: io.EOF}
	p, cleanup := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"devin"}},
		mock,
	)
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		local proc, err = sys.spawn("devin", {})
		_G.proc = proc
		_G.err = err
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("proc") != lua.LNil {
		t.Error("expected nil proc on spawn error")
	}
	if p.State.GetGlobal("err") == lua.LNil {
		t.Error("expected an error string")
	}
}

func TestSpawnStreamsToCallbacks(t *testing.T) {
	mp := &mockSpawnedProcess{
		stdin:  &bytes.Buffer{},
		stdout: strings.NewReader("hello stdout"),
		stderr: strings.NewReader("oops stderr"),
		code:   7,
	}
	mock := &mockSystemAPI{spawnProc: mp}
	p, cleanup := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"cat"}},
		mock,
	)
	defer cleanup()
	post, ch := postChan()
	p.PostAsync = post

	err := p.State.DoString(`
		local sys = require("ttt.system")
		_G.out, _G.errs, _G.code = {}, {}, nil
		local proc = sys.spawn("cat", {
			on_stdout = function(c) table.insert(_G.out, c) end,
			on_stderr = function(c) table.insert(_G.errs, c) end,
			on_exit = function(code) _G.code = code end,
		})
		assert(proc ~= nil)
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	runPostedUntil(t, ch, func() bool { return p.State.GetGlobal("code") != lua.LNil })

	out := p.State.GetGlobal("out").(*lua.LTable)
	if out.Len() != 1 || out.RawGetInt(1).String() != "hello stdout" {
		t.Errorf("unexpected stdout chunks: %v", out)
	}
	errs := p.State.GetGlobal("errs").(*lua.LTable)
	if errs.Len() != 1 || errs.RawGetInt(1).String() != "oops stderr" {
		t.Errorf("unexpected stderr chunks: %v", errs)
	}
	if p.State.GetGlobal("code") != lua.LNumber(7) {
		t.Errorf("unexpected exit code: %v", p.State.GetGlobal("code"))
	}
}

func TestProcWriteAndCloseStdin(t *testing.T) {
	mp := blockingSpawnedProcess(t)
	mp.wrote = make(chan struct{}, 8)
	mp.closed = make(chan struct{})
	mock := &mockSystemAPI{spawnProc: mp}
	p, cleanup := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"cat"}},
		mock,
	)
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		_G.proc = sys.spawn("cat", {})
		assert(_G.proc ~= nil)
		assert(_G.proc:write("line one\n") == true)
		assert(_G.proc:write("line two\n") == true)
		assert(_G.proc:close_stdin() == true)
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	waitFor(t, mp.wrote, "stdin write")
	waitFor(t, mp.closed, "stdin close")
	if got := mp.stdin.String(); got != "line one\nline two\n" {
		t.Errorf("unexpected stdin: %q", got)
	}
}

func TestProcWriteAfterCloseFails(t *testing.T) {
	mp := blockingSpawnedProcess(t)
	mock := &mockSystemAPI{spawnProc: mp}
	p, cleanup := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"cat"}},
		mock,
	)
	defer cleanup()

	err := p.State.DoString(`
		local sys = require("ttt.system")
		local proc = sys.spawn("cat", {})
		assert(proc:close_stdin() == true)
		_G.ok, _G.err = proc:write("too late")
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.State.GetGlobal("ok") != lua.LNil {
		t.Error("expected write after close_stdin to fail")
	}
	if p.State.GetGlobal("err") == lua.LNil {
		t.Error("expected an error string")
	}
}

func TestDestroyKillsSpawnedProcesses(t *testing.T) {
	mp := blockingSpawnedProcess(t)
	mock := &mockSystemAPI{spawnProc: mp}
	p, _ := setupTestPluginWithSystem(
		PermissionSet{SystemExec: []string{"cat"}},
		mock,
	)
	// No cleanup: Destroy closes the Lua state itself.

	err := p.State.DoString(`
		local sys = require("ttt.system")
		_G.proc = sys.spawn("cat", {})
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	p.Destroy()
	if !mp.killedState() {
		t.Error("expected spawned process to be killed on plugin destroy")
	}
}

func TestProcWriteErrorPostedToMainThread(t *testing.T) {
	for _, destroy := range []bool{false, true} {
		name := "active"
		if destroy {
			name = "destroyed"
		}
		t.Run(name, func(t *testing.T) {
			p, _ := newTestPluginBase(PermissionSet{})
			defer p.Destroy()
			post, ch := postChan()
			p.SetPostAsync(post)
			var logs []string
			p.Log = func(_, message string) { logs = append(logs, message) }
			pp := &pluginProc{
				p: p, proc: &mockSpawnedProcess{writeErr: io.ErrClosedPipe},
				writeCh: make(chan procWriteMsg, 1),
			}
			pp.writeCh <- procWriteMsg{data: []byte("hello")}
			close(pp.writeCh)
			done := make(chan struct{})
			go func() {
				defer close(done)
				pp.writeLoop()
			}()
			waitFor(t, done, "failed write")
			if p.errorCount != 0 || len(logs) != 0 {
				t.Fatal("write error logged before main-thread delivery")
			}
			if destroy {
				p.Destroy()
			}
			runPosted(t, ch, 1)
			want := 1
			if destroy {
				want = 0
			}
			if p.errorCount != want || len(logs) != want {
				t.Fatalf("error count=%d logs=%v, want %d", p.errorCount, logs, want)
			}
		})
	}
}

func TestProcEnqueueWhileReaping(t *testing.T) {
	for i := 0; i < 100; i++ {
		pp := &pluginProc{
			p: &Plugin{}, proc: &mockSpawnedProcess{},
			writeCh: make(chan procWriteMsg, 1),
		}
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			if err := pp.enqueue(procWriteMsg{data: []byte("hello")}); err != nil && !errors.Is(err, errProcClosed) {
				t.Errorf("enqueue: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			pp.reap()
		}()
		close(start)
		wg.Wait()
		if err := pp.enqueue(procWriteMsg{}); !errors.Is(err, errProcClosed) {
			t.Fatalf("write after reaping: %v", err)
		}
	}
}

// The screen queue delivers overflow posts from goroutines, so posts can run in
// any order. Running every post in reverse must still give each stream's
// chunks in order and on_exit last.
func TestProcCallbacksKeepOrderWhenPostsAreReordered(t *testing.T) {
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"cat"}}, &mockSystemAPI{})
	defer cleanup()
	var posted []*PluginAsyncResult
	p.PostAsync = func(r *PluginAsyncResult) { posted = append(posted, r) }
	pp := &pluginProc{p: p}

	var got []string
	for _, name := range []string{"out:1", "err:1", "out:2", "out:3", "exit"} {
		pp.deliver(func() { got = append(got, name) })
	}
	for len(posted) > 0 {
		batch := posted
		posted = nil
		for i := len(batch) - 1; i >= 0; i-- {
			batch[i].Callback()
		}
	}

	if want := []string{"out:1", "err:1", "out:2", "out:3", "exit"}; !slices.Equal(got, want) {
		t.Fatalf("callbacks ran as %v, want %v", got, want)
	}
}

// A callback queued while the queue drains goes out in a new post instead of
// keeping the event loop in the current one.
func TestProcDrainYieldsBetweenBatches(t *testing.T) {
	p, cleanup := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"cat"}}, &mockSystemAPI{})
	defer cleanup()
	var posted []*PluginAsyncResult
	p.PostAsync = func(r *PluginAsyncResult) { posted = append(posted, r) }
	pp := &pluginProc{p: p}

	ran := 0
	pp.deliver(func() {
		ran++
		pp.deliver(func() { ran++ })
	})
	if len(posted) != 1 {
		t.Fatalf("posts = %d, want 1", len(posted))
	}
	posted[0].Callback()
	if ran != 1 || len(posted) != 2 {
		t.Fatalf("after one drain: ran %d, posts %d; want 1 and a second post", ran, len(posted))
	}
	posted[1].Callback()
	if ran != 2 {
		t.Fatalf("ran = %d, want 2", ran)
	}
}

// A drain still queued on the screen when the plugin is destroyed must not
// call into the closed Lua state.
func TestProcDrainAfterDestroySkipsCallbacks(t *testing.T) {
	mp := &mockSpawnedProcess{
		stdin:  &bytes.Buffer{},
		stdout: strings.NewReader("late chunk"),
		stderr: strings.NewReader(""),
		waited: make(chan struct{}),
	}
	p, _ := setupTestPluginWithSystem(PermissionSet{SystemExec: []string{"cat"}}, &mockSystemAPI{spawnProc: mp})
	post, ch := postChan()
	p.SetPostAsync(post)

	err := p.State.DoString(`
		local sys = require("ttt.system")
		sys.spawn("cat", {
			on_stdout = function(c) _G.out = c end,
			on_exit = function() _G.exited = true end,
		})
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	waitFor(t, mp.waited, "process reaping")
	var queued *PluginAsyncResult
	select {
	case queued = <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("no drain posted")
	}

	p.Destroy()
	queued.Callback()
}
