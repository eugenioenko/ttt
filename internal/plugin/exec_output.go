package plugin

import (
	"bytes"
	"context"
	"sync"
	"unicode/utf8"

	lua "github.com/yuin/gopher-lua"
)

// execChunkMax bounds a streamed chunk when a line has no newline in sight.
const execChunkMax = 64 * 1024

// outputLimit is a byte budget shared by a child's stdout and stderr.
type outputLimit struct {
	mu   sync.Mutex
	left int64
	hit  bool
	kill context.CancelFunc
}

func newOutputLimit(max int64, kill context.CancelFunc) *outputLimit {
	if max <= 0 {
		return nil
	}
	return &outputLimit{left: max, kill: kill}
}

// take returns how many of n bytes fit; the first overflow kills the child.
func (l *outputLimit) take(n int) int {
	if l == nil {
		return n
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if int64(n) <= l.left {
		l.left -= int64(n)
		return n
	}
	kept := int(l.left)
	l.left = 0
	if !l.hit {
		l.hit = true
		l.kill()
	}
	return kept
}

func (l *outputLimit) exceeded() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.hit
}

// execOutput collects one output stream, or with deliver set hands it on in
// chunks that end at a newline, so a callback never sees a line or a UTF-8
// sequence cut in two unless the line outgrows execChunkMax.
type execOutput struct {
	limit   *outputLimit
	buf     bytes.Buffer
	deliver func(chunk string)
	pending []byte
}

func (o *execOutput) Write(p []byte) (int, error) {
	n := len(p)
	p = p[:o.limit.take(n)]
	if o.deliver == nil {
		o.buf.Write(p)
		return n, nil
	}
	o.pending = append(o.pending, p...)
	for {
		window := o.pending[:min(len(o.pending), execChunkMax+1)]
		if i := bytes.LastIndexByte(window, '\n'); i >= 0 {
			o.send(i + 1)
		} else if len(o.pending) > execChunkMax {
			o.send(runeBoundary(o.pending, execChunkMax))
		} else {
			return n, nil
		}
	}
}

func (o *execOutput) flush() {
	if o.deliver != nil && len(o.pending) > 0 {
		o.send(len(o.pending))
	}
}

func (o *execOutput) String() string { return o.buf.String() }

func (o *execOutput) send(n int) {
	chunk := string(o.pending[:n])
	o.pending = append(o.pending[:0], o.pending[n:]...)
	o.deliver(chunk)
}

func runeBoundary(b []byte, at int) int {
	for i := at; i > 0 && i > at-utf8.UTFMax; i-- {
		if utf8.RuneStart(b[i]) {
			return i
		}
	}
	return at
}

// streamTo returns a deliver func that runs fn with each chunk on the main
// loop and waits for it, so a child that outpaces the plugin blocks on its
// pipe instead of piling output up in memory. Chunks stop once ctx is done
// or the exec's final callback has run.
func (p *Plugin) streamTo(ctx context.Context, fn *lua.LFunction, finished *bool, label string) func(string) {
	return func(chunk string) {
		if ctx.Err() != nil {
			return
		}
		delivered := make(chan struct{})
		p.SafePostAsync(&PluginAsyncResult{Plugin: p, Callback: func() {
			defer close(delivered)
			if p.State == nil || *finished || ctx.Err() != nil {
				return
			}
			if err := p.CallLuaFunc(fn, lua.LString(chunk)); err != nil {
				p.logError(label, err)
			}
		}})
		select {
		case <-delivered:
		case <-ctx.Done():
		}
	}
}
