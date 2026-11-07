package plugin

import (
	"errors"
	"io"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

var (
	errProcClosed    = errors.New("process closed")
	errProcQueueFull = errors.New("process write queue full")
)

// sys.spawn gives plugins a long-lived child process with raw piped stdio —
// the missing middle between one-shot sys.exec and PTY-hosted
// terminal.spawn. It exists for interactive protocols (JSON-RPC agents over
// stdio, REPLs, streaming tools) where a conversation is exchanged with the
// child's pipes rather than a terminal.
//
//	local proc, err = sys.spawn("devin", {
//	  args = {"acp"},
//	  cwd = "/path",
//	  env = {FOO = "1"},
//	  on_stdout = function(chunk) end,
//	  on_stderr = function(chunk) end,
//	  on_exit   = function(code) end,
//	})
//	proc:write("hello\n")    -- queued; returns nil, error when the queue is full
//	proc:close_stdin()       -- child sees EOF after queued writes flush
//	proc:kill()
//	proc:pid()               -- number
//
// Streams pump on goroutines and deliver chunks on the main thread, so
// callbacks never interleave with rendering. Writes go through a bounded
// queue drained by a writer goroutine, so a stalled child cannot block the
// UI. Spawned processes are killed when the plugin is destroyed.
//
// The same system.exec binary allowlist gates spawning.

const procTypeName = "ttt.system.proc"
const procReadChunk = 32 * 1024
const procWriteQueueSize = 64

// procWriteMsg is one item in the writer queue. close asks the writer to
// close stdin after all queued writes have been sent.
type procWriteMsg struct {
	data  []byte
	close bool
}

type pluginProc struct {
	p    *Plugin
	proc SpawnedProcess

	onStdout *lua.LFunction
	onStderr *lua.LFunction
	onExit   *lua.LFunction

	writeCh chan procWriteMsg

	mu       sync.Mutex
	closed   bool // no more writes accepted
	eofs     int  // streams that reached EOF; reaping starts at 2
	reaped   bool
	pending  []func()
	draining bool
}

func checkProc(L *lua.LState) *pluginProc {
	ud := L.CheckUserData(1)
	pp, ok := ud.Value.(*pluginProc)
	if !ok {
		L.ArgError(1, "spawned process expected")
	}
	return pp
}

func sysSpawn(p *Plugin) lua.LGFunction {
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

		var args []string
		var env []string
		var cwd string
		var onStdout, onStderr, onExit *lua.LFunction
		if opts, ok := L.Get(2).(*lua.LTable); ok {
			if argsTbl, ok := L.GetField(opts, "args").(*lua.LTable); ok {
				argsTbl.ForEach(func(_, v lua.LValue) {
					args = append(args, v.String())
				})
			}
			switch v := L.GetField(opts, "env").(type) {
			case *lua.LNilType:
			case *lua.LTable:
				parsed, err := parseExecEnv(binary, v)
				if err != nil {
					L.ArgError(2, err.Error())
				}
				env = parsed
			default:
				L.ArgError(2, "env must be a table of strings")
			}
			if s, ok := L.GetField(opts, "cwd").(lua.LString); ok {
				cwd = string(s)
			}
			onStdout = optFunction(L, 2, opts, "on_stdout")
			onStderr = optFunction(L, 2, opts, "on_stderr")
			onExit = optFunction(L, 2, opts, "on_exit")
		}

		proc, err := p.System.Spawn(binary, args, SpawnOptions{Cwd: cwd, Env: env})
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}

		pp := &pluginProc{
			p:        p,
			proc:     proc,
			onStdout: onStdout,
			onStderr: onStderr,
			onExit:   onExit,
			writeCh:  make(chan procWriteMsg, procWriteQueueSize),
		}
		p.registerProc(pp)
		go pp.writeLoop()
		go pp.pump(proc.Stdout(), pp.onStdout, "stdout")
		go pp.pump(proc.Stderr(), pp.onStderr, "stderr")

		ud := L.NewUserData()
		ud.Value = pp
		L.SetMetatable(ud, L.GetTypeMetatable(procTypeName))
		L.Push(ud)
		return 1
	}
}

// writeLoop drains the write queue in order. A {close} message half-closes
// stdin; a closed channel (reaping, destroy) just exits.
func (pp *pluginProc) writeLoop() {
	for msg := range pp.writeCh {
		if msg.close {
			_ = pp.proc.CloseStdin()
			continue
		}
		if _, err := pp.proc.Write(msg.data); err != nil {
			p := pp.p
			p.SafePostAsync(&PluginAsyncResult{Plugin: p, Callback: func() {
				if p.State != nil {
					p.logError("proc write", err)
				}
			}})
			// The child is gone or the pipe is broken; further writes
			// can't succeed, so stop draining.
			return
		}
	}
}

// pump reads a stream to EOF, posting each chunk to the Lua callback on the
// main thread. A nil callback still drains so the child can't block on a
// full pipe.
func (pp *pluginProc) pump(r io.Reader, fn *lua.LFunction, name string) {
	buf := make([]byte, procReadChunk)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			p := pp.p
			pp.deliver(func() {
				if p.State == nil || fn == nil {
					return
				}
				if callErr := p.CallLuaFunc(fn, lua.LString(chunk)); callErr != nil {
					p.logError("proc "+name+" callback", callErr)
				}
			})
		}
		if err != nil {
			pp.streamEOF()
			return
		}
	}
}

// deliver queues fn to run on the main thread after every callback queued
// before it. Posts can overtake each other once the screen's event queue is
// full, so a process keeps a single drain posted at a time; that is what keeps
// each stream's chunks in order and on_exit after the final chunk.
func (pp *pluginProc) deliver(fn func()) {
	pp.mu.Lock()
	pp.pending = append(pp.pending, fn)
	if pp.draining {
		pp.mu.Unlock()
		return
	}
	pp.draining = true
	pp.mu.Unlock()
	pp.p.SafePostAsync(&PluginAsyncResult{Plugin: pp.p, Callback: pp.drain})
}

// drain runs the queued callbacks on the main thread, one batch per post so a
// chatty process cannot hold the event loop.
func (pp *pluginProc) drain() {
	pp.mu.Lock()
	batch := pp.pending
	pp.pending = nil
	pp.mu.Unlock()

	for _, fn := range batch {
		fn()
	}

	pp.mu.Lock()
	more := len(pp.pending) > 0
	pp.draining = more
	pp.mu.Unlock()
	if more {
		pp.p.SafePostAsync(&PluginAsyncResult{Plugin: pp.p, Callback: pp.drain})
	}
}

// streamEOF counts drained streams; when both reach EOF the child has exited
// and it is safe to reap (Wait must not run ahead of the readers).
func (pp *pluginProc) streamEOF() {
	pp.mu.Lock()
	pp.eofs++
	done := pp.eofs == 2
	pp.mu.Unlock()
	if done {
		go pp.reap()
	}
}

// reap waits for the child, reports the exit code, and releases the process
// from the plugin's tracking set.
func (pp *pluginProc) reap() {
	code, err := pp.proc.Wait()

	pp.mu.Lock()
	if pp.reaped {
		pp.mu.Unlock()
		return
	}
	pp.reaped = true
	pp.closed = true
	close(pp.writeCh)
	pp.mu.Unlock()

	p := pp.p
	p.unregisterProc(pp)
	if pp.onExit != nil {
		result := lua.LNumber(code)
		var errVal lua.LValue = lua.LNil
		if err != nil {
			errVal = lua.LString(err.Error())
		}
		pp.deliver(func() {
			if p.State == nil {
				return
			}
			if callErr := p.CallLuaFunc(pp.onExit, result, errVal); callErr != nil {
				p.logError("proc exit callback", callErr)
			}
		})
	}
}

// enqueue appends to the write queue; it fails when the process is closed or
// the queue is full rather than blocking the caller.
func (pp *pluginProc) enqueue(msg procWriteMsg) error {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	if pp.closed {
		return errProcClosed
	}
	select {
	case pp.writeCh <- msg:
		if msg.close {
			pp.closed = true
		}
		return nil
	default:
		return errProcQueueFull
	}
}

// markClosed refuses further writes; the queue keeps flushing what arrived.
func (pp *pluginProc) markClosed() {
	pp.mu.Lock()
	pp.closed = true
	pp.mu.Unlock()
}

func (p *Plugin) registerProc(pp *pluginProc) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.procs == nil {
		p.procs = make(map[*pluginProc]struct{})
	}
	p.procs[pp] = struct{}{}
}

func (p *Plugin) unregisterProc(pp *pluginProc) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.procs, pp)
}

// killProcsLocked terminates every spawned process; caller must hold p.mu.
func (p *Plugin) killProcsLocked() {
	for pp := range p.procs {
		pp.proc.Kill()
	}
	p.procs = nil
}

func procWrite(L *lua.LState) int {
	pp := checkProc(L)
	data := L.CheckString(2)
	if err := pp.enqueue(procWriteMsg{data: []byte(data)}); err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LTrue)
	return 1
}

func procCloseStdin(L *lua.LState) int {
	pp := checkProc(L)
	if err := pp.enqueue(procWriteMsg{close: true}); err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LTrue)
	return 1
}

func procKill(L *lua.LState) int {
	pp := checkProc(L)
	pp.markClosed()
	if err := pp.proc.Kill(); err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LTrue)
	return 1
}

func procPID(L *lua.LState) int {
	pp := checkProc(L)
	L.Push(lua.LNumber(pp.proc.PID()))
	return 1
}
