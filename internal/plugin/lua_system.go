package plugin

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func setupSystemModule(L *lua.LState, p *Plugin) {
	loader := func(L *lua.LState) int {
		mod := L.NewTable()

		hasExec := len(p.Granted.SystemExec) > 0
		hasEnv := p.Granted.Check("system.env") == nil

		if hasExec {
			L.SetField(mod, "exec", L.NewFunction(sysExec(p)))
			L.SetField(mod, "exec_async", L.NewFunction(sysExecAsync(p)))
			L.SetField(mod, "request_exec", L.NewFunction(sysRequestExec(p)))
		}

		if hasEnv {
			L.SetField(mod, "env", L.NewFunction(sysEnv(p)))
		}

		L.Push(mod)
		return 1
	}

	L.PreloadModule("ttt.system", loader)
}

type execRequest struct {
	args               []string
	opts               ExecOptions
	timeout            time.Duration
	maxOutput          int64
	onStdout, onStderr *lua.LFunction
}

type execResult struct {
	stdout, stderr string
	exitCode       int
	err            error
	timedOut       bool
	cancelled      bool
	truncated      bool
}

// parseExecArgs reads the optional argv array and options table that follow
// the binary name, starting at stack index start.
func parseExecArgs(L *lua.LState, binary string, start int) execRequest {
	var req execRequest
	if argsTbl, ok := L.Get(start).(*lua.LTable); ok {
		argsTbl.ForEach(func(_, v lua.LValue) {
			req.args = append(req.args, v.String())
		})
	}
	optsTbl, ok := L.Get(start + 1).(*lua.LTable)
	if !ok {
		return req
	}
	if s, ok := L.GetField(optsTbl, "stdin").(lua.LString); ok {
		req.opts.Stdin = string(s)
	}
	switch v := L.GetField(optsTbl, "timeout_ms").(type) {
	case *lua.LNilType:
	case lua.LNumber:
		if v <= 0 {
			L.ArgError(start+1, "timeout_ms must be a positive number")
		}
		req.timeout = time.Duration(float64(v) * float64(time.Millisecond))
	default:
		L.ArgError(start+1, "timeout_ms must be a number")
	}
	switch v := L.GetField(optsTbl, "env").(type) {
	case *lua.LNilType:
	case *lua.LTable:
		env, err := parseExecEnv(binary, v)
		if err != nil {
			L.ArgError(start+1, err.Error())
		}
		req.opts.Env = env
	default:
		L.ArgError(start+1, "env must be a table of strings")
	}
	switch v := L.GetField(optsTbl, "max_output_bytes").(type) {
	case *lua.LNilType:
	case lua.LNumber:
		// Negated so NaN is rejected too.
		if !(v >= 1) {
			L.ArgError(start+1, "max_output_bytes must be at least 1")
		}
		req.maxOutput = math.MaxInt64
		if float64(v) < math.MaxInt64 {
			req.maxOutput = int64(v)
		}
	default:
		L.ArgError(start+1, "max_output_bytes must be a number")
	}
	req.onStdout = optFunction(L, start+1, optsTbl, "on_stdout")
	req.onStderr = optFunction(L, start+1, optsTbl, "on_stderr")
	return req
}

func optFunction(L *lua.LState, arg int, tbl *lua.LTable, name string) *lua.LFunction {
	switch v := L.GetField(tbl, name).(type) {
	case *lua.LNilType:
		return nil
	case *lua.LFunction:
		return v
	default:
		L.ArgError(arg, name+" must be a function")
		return nil
	}
}

func parseExecEnv(binary string, tbl *lua.LTable) ([]string, error) {
	var env []string
	var err error
	tbl.ForEach(func(k, v lua.LValue) {
		if err != nil {
			return
		}
		key, ok := k.(lua.LString)
		if !ok {
			err = fmt.Errorf("env keys must be strings")
			return
		}
		var val string
		switch v := v.(type) {
		case lua.LString:
			val = string(v)
		case lua.LNumber:
			val = v.String()
		default:
			err = fmt.Errorf("env value for %q must be a string", string(key))
			return
		}
		if err = validateExecEnv(binary, string(key), val); err == nil {
			env = append(env, string(key)+"="+val)
		}
	})
	slices.Sort(env)
	return env, err
}

// validateExecEnv rejects keys that change which code the child runs. The
// binary itself is resolved with the editor's PATH, but a child's own
// lookups and the dynamic loader would honor these, and git reads its
// config from GIT_* variables, bypassing the git argument checks.
func validateExecEnv(binary, key, val string) error {
	if key == "" || strings.ContainsAny(key, "=\x00") {
		return fmt.Errorf("env key %q must be non-empty and contain no '=' or NUL", key)
	}
	if strings.ContainsRune(val, 0) {
		return fmt.Errorf("env value for %q contains NUL", key)
	}
	upper := strings.ToUpper(key)
	blocked := upper == "PATH" || strings.HasPrefix(upper, "LD_") || strings.HasPrefix(upper, "DYLD_")
	if IsGitBinary(binary) && strings.HasPrefix(upper, "GIT_") {
		blocked = true
	}
	if blocked {
		return fmt.Errorf("env key %q is not allowed", key)
	}
	return nil
}

// IsGitBinary reports whether binary names git, the way a case-insensitive
// filesystem or Windows would resolve it.
func IsGitBinary(binary string) bool {
	return strings.TrimSuffix(strings.ToLower(filepath.Base(binary)), ".exe") == "git"
}

// finished is read and written only on the main loop: it is set before the
// final callback so a chunk still queued behind it is dropped.
func runExec(ctx context.Context, p *Plugin, binary string, req execRequest, finished *bool) execResult {
	if req.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.timeout)
		defer cancel()
	}
	childCtx, kill := context.WithCancel(ctx)
	defer kill()
	limit := newOutputLimit(req.maxOutput, kill)
	stdout := &execOutput{limit: limit}
	stderr := &execOutput{limit: limit}
	if req.onStdout != nil {
		stdout.deliver = p.streamTo(ctx, req.onStdout, finished, "exec on_stdout")
	}
	if req.onStderr != nil {
		stderr.deliver = p.streamTo(ctx, req.onStderr, finished, "exec on_stderr")
	}
	opts := req.opts
	opts.Stdout, opts.Stderr = stdout, stderr
	_, _, exitCode, err := p.System.Exec(childCtx, binary, req.args, opts)
	stdout.flush()
	stderr.flush()
	r := execResult{stdout: stdout.String(), stderr: stderr.String(), exitCode: exitCode, err: err}
	if limit.exceeded() {
		r.truncated = true
		r.exitCode = -1
		r.err = nil
	} else if ctxErr := ctx.Err(); ctxErr != nil && (err != nil || exitCode != 0) {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			r.timedOut = true
			r.err = fmt.Errorf("timed out after %s", req.timeout)
		} else {
			r.cancelled = true
			r.err = errors.New("cancelled")
		}
	}
	return r
}

func pushExecResult(L *lua.LState, r execResult) *lua.LTable {
	tbl := L.NewTable()
	if r.err != nil {
		L.SetField(tbl, "stdout", lua.LString(""))
		L.SetField(tbl, "stderr", lua.LString(r.err.Error()))
		L.SetField(tbl, "exit_code", lua.LNumber(-1))
	} else {
		L.SetField(tbl, "stdout", lua.LString(r.stdout))
		L.SetField(tbl, "stderr", lua.LString(r.stderr))
		L.SetField(tbl, "exit_code", lua.LNumber(r.exitCode))
	}
	if r.timedOut {
		L.SetField(tbl, "timed_out", lua.LTrue)
	}
	if r.cancelled {
		L.SetField(tbl, "cancelled", lua.LTrue)
	}
	if r.truncated {
		L.SetField(tbl, "truncated", lua.LTrue)
	}
	return tbl
}

func sysExec(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.System == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("system API not available"))
			return 2
		}
		binary := L.CheckString(1)
		if err := p.Granted.CheckExec(binary); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}

		req := parseExecArgs(L, binary, 2)
		if req.onStdout != nil || req.onStderr != nil {
			L.ArgError(3, "on_stdout and on_stderr need sys.exec_async")
		}
		finished := false
		L.Push(pushExecResult(L, runExec(context.Background(), p, binary, req, &finished)))
		return 1
	}
}

func sysExecAsync(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.System == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("system API not available"))
			return 2
		}
		binary := L.CheckString(1)
		if err := p.Granted.CheckExec(binary); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}

		var req execRequest
		var callback *lua.LFunction
		if fn, ok := L.Get(2).(*lua.LFunction); ok {
			callback = fn
		} else {
			req = parseExecArgs(L, binary, 2)
			if fn, ok := L.Get(3).(*lua.LFunction); ok {
				callback = fn
			} else {
				callback = L.CheckFunction(4)
			}
		}

		id, ctx, done := p.startExec()
		finished := false
		go func() {
			defer close(done)
			r := runExec(ctx, p, binary, req, &finished)
			p.finishExec(id)
			resultFn := func() {
				finished = true
				if p.State == nil {
					return
				}
				if callErr := p.CallLuaFunc(callback, pushExecResult(p.State, r)); callErr != nil {
					p.logError("async exec callback", callErr)
				}
			}
			p.SafePostAsync(&PluginAsyncResult{Plugin: p, Callback: resultFn})
		}()

		handle := L.NewTable()
		L.SetField(handle, "cancel", L.NewFunction(func(L *lua.LState) int {
			L.Push(lua.LBool(p.cancelExec(id)))
			return 1
		}))
		L.Push(handle)
		return 1
	}
}

type runningExec struct {
	cancel context.CancelFunc
	done   chan struct{}
}

const execKillWait = time.Second

func (p *Plugin) startExec() (int, context.Context, chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.execs == nil {
		p.execs = make(map[int]runningExec)
	}
	p.nextExecID++
	p.execs[p.nextExecID] = runningExec{cancel: cancel, done: done}
	return p.nextExecID, ctx, done
}

func (p *Plugin) finishExec(id int) {
	p.mu.Lock()
	e, ok := p.execs[id]
	delete(p.execs, id)
	p.mu.Unlock()
	if ok {
		e.cancel()
	}
}

func (p *Plugin) cancelExec(id int) bool {
	p.mu.Lock()
	e, running := p.execs[id]
	p.mu.Unlock()
	if running {
		e.cancel()
	}
	return running
}

// killExecs waits, up to execKillWait, for the children to be killed: the
// kill runs on another goroutine, and the editor exits right after Destroy.
func (p *Plugin) killExecs() {
	p.mu.Lock()
	execs := p.execs
	p.execs = nil
	p.mu.Unlock()
	for _, e := range execs {
		e.cancel()
	}
	deadline := time.After(execKillWait)
	for _, e := range execs {
		select {
		case <-e.done:
		case <-deadline:
			return
		}
	}
}

func sysEnv(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.System == nil {
			L.Push(lua.LString(""))
			return 1
		}
		name := L.CheckString(1)
		L.Push(lua.LString(p.System.Env(name)))
		return 1
	}
}
