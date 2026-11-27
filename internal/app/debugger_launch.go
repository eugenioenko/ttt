package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/eugenioenko/ttt/internal/dap"
)

type debugRun struct {
	ctx            context.Context
	session        *dap.Session
	thread         *int
	lastThread     *int
	generation     uint64
	busy, stepping bool

	folder    string
	languages []string
	points    chan map[string][]int
	closed    chan struct{}
	cancel    context.CancelFunc
}

func (r *debugRun) launch(ctx context.Context, cmd *exec.Cmd, adapter string, args map[string]any, ready func(*dap.Session), stopped func(int, []debugFrame), resumed func()) error {
	initial := <-r.points
	initSignal, terminated := make(chan struct{}), make(chan struct{})
	var initOnce, endOnce sync.Once
	stops := make(chan *int, 1)
	session, err := dap.StartStdio(cmd, func(e dap.Event) {
		switch e.Name {
		case "initialized":
			initOnce.Do(func() { close(initSignal) })
		case "terminated":
			endOnce.Do(func() { close(terminated) })
		case "continued":
			resumed()
		case "stopped":
			var body struct{ ThreadID *int }
			if json.Unmarshal(e.Body, &body) == nil {
				select {
				case <-stops:
				default:
				}
				stops <- body.ThreadID
			}
		}
	})
	if err != nil {
		return err
	}
	defer func() {
		stop, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = session.Disconnect(stop, true)
	}()
	startup, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var caps struct {
		SupportsConfigurationDoneRequest bool
		ExceptionBreakpointFilters       []struct {
			Filter  string
			Default bool
		}
	}
	if err = session.Call(startup, "initialize", map[string]any{"adapterID": adapter, "clientID": "ttt", "linesStartAt1": true, "columnsStartAt1": true, "pathFormat": "path"}, &caps); err != nil {
		return err
	}
	launch := make(chan error, 1)
	go func(result chan<- error) { result <- session.Call(startup, "launch", args, nil) }(launch)
	// Some adapters acknowledge launch only after configurationDone.
	initialized := initSignal
	configured := false
	var sent map[string][]int
	for !configured || launch != nil {
		select {
		case err := <-launch:
			if err != nil {
				return err
			}
			launch = nil
		case <-initialized:
			initialized = nil
			select {
			case initial = <-r.points:
			default:
			}
			if err = syncDebugPoints(startup, session, nil, initial); err != nil {
				return err
			}
			sent = initial
			if len(caps.ExceptionBreakpointFilters) > 0 {
				filters := make([]string, 0, len(caps.ExceptionBreakpointFilters))
				for _, f := range caps.ExceptionBreakpointFilters {
					if f.Default {
						filters = append(filters, f.Filter)
					}
				}
				if err = session.Call(startup, "setExceptionBreakpoints", map[string]any{"filters": filters}, nil); err != nil {
					return err
				}
			}
			if caps.SupportsConfigurationDoneRequest {
				if err = session.Call(startup, "configurationDone", struct{}{}, nil); err != nil {
					return err
				}
			}
			configured = true
		case <-terminated:
			return nil
		case <-session.Done():
			return io.EOF
		case <-startup.Done():
			return startup.Err()
		}
	}
	cancel()
	ready(session)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-terminated:
			return nil
		case <-session.Done():
			select {
			case <-terminated:
				return nil
			default:
				return io.EOF
			}
		case next := <-r.points:
			request, finish := context.WithTimeout(ctx, 10*time.Second)
			err := syncDebugPoints(request, session, sent, next)
			finish()
			if err != nil {
				return err
			}
			sent = next
		case id := <-stops:
			request, finish := context.WithTimeout(ctx, 10*time.Second)
			tid, frames, err := stoppedFrames(request, session, id)
			finish()
			if err != nil {
				return err
			}
			if tid != nil {
				for i := range frames {
					p := frames[i].Source.Path
					if p != "" && !filepath.IsAbs(p) {
						frames[i].Source.Path = filepath.Join(r.folder, p)
					}
				}
				stopped(*tid, frames)
			}
		}
	}
}

func stoppedFrames(ctx context.Context, session *dap.Session, id *int) (*int, []debugFrame, error) {
	var threads struct{ Threads []struct{ ID int } }
	if id == nil {
		if err := session.Call(ctx, "threads", nil, &threads); err != nil {
			return nil, nil, err
		}
	} else {
		threads.Threads = append(threads.Threads, struct{ ID int }{*id})
	}
	for _, thread := range threads.Threads {
		var trace struct {
			StackFrames []debugFrame
		}
		err := session.Call(ctx, "stackTrace", map[string]int{"threadId": thread.ID, "levels": 50}, &trace)
		if err != nil {
			if id != nil || ctx.Err() != nil {
				return nil, nil, err
			}
			continue
		}
		if id != nil || len(trace.StackFrames) > 0 {
			return &thread.ID, trace.StackFrames, nil
		}
	}
	return nil, nil, nil
}

func syncDebugPoints(ctx context.Context, s *dap.Session, prev, next map[string][]int) error {
	changes := make(map[string][]int)
	for path, lines := range next {
		if !slices.Equal(prev[path], lines) {
			changes[path] = lines
		}
	}
	for path := range prev {
		if _, ok := next[path]; !ok {
			changes[path] = nil
		}
	}
	for path, lines := range changes {
		list := make([]map[string]int, len(lines))
		for i, line := range lines {
			list[i] = map[string]int{"line": line}
		}
		if err := s.Call(ctx, "setBreakpoints", map[string]any{"source": map[string]string{"path": path}, "breakpoints": list}, nil); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

func (a *App) DebugPause()    { a.debugControl("pause") }
func (a *App) DebugContinue() { a.debugControl("continue") }
func (a *App) DebugStepOver() { a.debugControl("next") }
func (a *App) DebugStepIn()   { a.debugControl("stepIn") }
func (a *App) DebugStepOut()  { a.debugControl("stepOut") }
func (a *App) debugControl(cmd string) {
	r := a.debugger.run
	if r == nil || r.session == nil || r.busy || (cmd == "pause") != (r.thread == nil) {
		return
	}
	if r.stepping && cmd != "pause" {
		return
	}
	if cmd == "next" || cmd == "stepIn" || cmd == "stepOut" {
		r.stepping = true
	}
	r.busy = true
	a.refreshDebugControls()
	generation := r.generation
	tid := r.thread
	if tid == nil {
		tid = r.lastThread
	}
	post := a.tasks.post
	go func() {
		ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
		defer cancel()
		var err error
		if tid == nil {
			var result struct{ Threads []struct{ ID int } }
			err = r.session.Call(ctx, "threads", nil, &result)
			if err == nil && len(result.Threads) > 0 {
				tid = &result.Threads[0].ID
			}
		}
		if err == nil && tid != nil {
			err = r.session.Call(ctx, cmd, map[string]int{"threadId": *tid}, nil)
		}

		_ = post(func() {
			if a.debugger.run != r || r.ctx.Err() != nil || r.generation != generation {
				return
			}
			r.busy = false
			a.refreshDebugControls()
			if err != nil {
				if cmd != "pause" {
					r.stepping = false
				}
				a.StatusError(err.Error())
				return
			}
			if cmd != "pause" {
				r.generation++
				a.debugResumed(r)
			}
		})
	}()
}
