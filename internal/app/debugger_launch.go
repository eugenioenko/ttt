package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/eugenioenko/ttt/internal/dap"
)

type debugThread struct {
	ID   int
	Name string
}

type debugRun struct {
	resumeControl func(context.Context, *dap.Session, string, int) error
	threads       []debugThread
	bindings      map[string]map[int]debugBinding
	onBindings    func(string, []int, []debugBinding)

	output         map[string]string
	terminalIDs    []string
	stopChildren   func()
	frames         []debugFrame
	ctx            context.Context
	session        *dap.Session
	thread         *int
	lastThread     *int
	generation     uint64
	busy, stepping bool
	attached       bool

	file      string
	folder    string
	languages []string
	points    chan map[string][]int
	closed    chan struct{}
	cancel    context.CancelFunc
}

func (r *debugRun) runSession(ctx context.Context, open func(context.Context, func(dap.Event)) (*dap.Session, error), target dap.Target, args map[string]any, ready func(*dap.Session), stopped func(int, []debugFrame, []debugThread), resumed func()) error {
	var initial map[string][]int
	select {
	case initial = <-r.points:
	case <-ctx.Done():
		return ctx.Err()
	}
	initSignal, terminated := make(chan struct{}), make(chan struct{})
	var initOnce, endOnce sync.Once
	type stopEvent struct {
		all        bool
		threadID   *int
		generation uint64
	}
	var eventMu sync.Mutex
	var generation uint64
	var paused bool
	var pausedThread *int
	stops := make(chan stopEvent, 1)
	var pausedIDs []int
	queueStop := func(all bool, id *int, gen uint64) {
		select {
		case <-stops:
		default:
		}
		stops <- stopEvent{all, id, gen}
	}

	applyContinuation := func(tid *int, all *bool) {
		generation++
		if tid != nil && all != nil && !*all {
			pausedIDs = slices.DeleteFunc(pausedIDs, func(id int) bool { return id == *tid })
		} else {
			pausedIDs = nil
		}
		if len(pausedIDs) > 0 {
			id := pausedIDs[0]
			if pausedThread != nil && slices.Contains(pausedIDs, *pausedThread) {
				id = *pausedThread
			}
			pausedThread = &id
			queueStop(false, &id, generation)
		} else {
			paused = false
			resumed()
		}
	}
	r.resumeControl = func(request context.Context, s *dap.Session, cmd string, id int) error {
		eventMu.Lock()
		before := generation
		eventMu.Unlock()
		return s.CallWithResponse(request, cmd, map[string]int{"threadId": id}, func(raw json.RawMessage) {
			var result struct{ AllThreadsContinued *bool }
			if len(raw) > 0 {
				_ = json.Unmarshal(raw, &result)
			}
			eventMu.Lock()
			defer eventMu.Unlock()
			if generation == before {
				applyContinuation(&id, result.AllThreadsContinued)
			}
		})
	}
	startup, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	session, err := open(startup, func(e dap.Event) {
		switch e.Name {
		case "initialized":
			initOnce.Do(func() { close(initSignal) })
		case "terminated":
			endOnce.Do(func() { close(terminated) })
		case "continued":
			eventMu.Lock()
			var body struct {
				ThreadID            *int
				AllThreadsContinued *bool
			}
			_ = json.Unmarshal(e.Body, &body)
			applyContinuation(body.ThreadID, body.AllThreadsContinued)
			eventMu.Unlock()
		case "stopped":
			var body struct {
				ThreadID          *int
				AllThreadsStopped bool
			}
			if json.Unmarshal(e.Body, &body) == nil {
				eventMu.Lock()
				generation++
				paused, pausedThread = true, body.ThreadID
				if body.ThreadID != nil && !slices.Contains(pausedIDs, *body.ThreadID) {
					pausedIDs = append(pausedIDs, *body.ThreadID)
				}
				queueStop(body.AllThreadsStopped, body.ThreadID, generation)
				eventMu.Unlock()
			}
		}
	})
	if err != nil {
		return err
	}
	var caps struct {
		SupportsConfigurationDoneRequest, SupportsTerminateRequest bool
		ExceptionBreakpointFilters                                 []struct {
			Filter  string
			Default bool
		}
	}
	var sent map[string][]int
	defer func() {
		if r.stopChildren != nil {
			r.stopChildren()
		}
		stop, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if r.attached && session.ResumeBeforeDetach() {
			_ = syncDebugPoints(stop, session, sent, nil, nil)
			eventMu.Lock()
			isPaused, tid := paused, pausedThread
			eventMu.Unlock()
			if isPaused {
				if tid == nil {
					var threads struct{ Threads []struct{ ID int } }
					if session.Call(stop, "threads", nil, &threads) == nil && len(threads.Threads) > 0 {
						tid = &threads.Threads[0].ID
					}
				}
				if tid != nil {
					_ = session.Call(stop, "continue", map[string]int{"threadId": *tid}, nil)
				}
			}
		}
		if !r.attached && caps.SupportsTerminateRequest {
			term, finish := context.WithTimeout(stop, 500*time.Millisecond)
			if session.Call(term, "terminate", struct{}{}, nil) == nil {
				select {
				case <-terminated:
				case <-session.Done():
				case <-term.Done():
				}
			}
			finish()
		}
		_ = session.Disconnect(stop, !r.attached)
	}()
	if err = session.Call(startup, "initialize", map[string]any{"adapterID": target.Type, "clientID": "ttt", "supportsStartDebuggingRequest": session.SupportsStartDebugging(), "supportsRunInTerminalRequest": session.SupportsRunInTerminal(), "linesStartAt1": true, "columnsStartAt1": true, "pathFormat": "path"}, &caps); err != nil {
		return err
	}
	launch := make(chan error, 1)
	go func(result chan<- error) { result <- session.Call(startup, target.Request, args, nil) }(launch)
	// Some adapters acknowledge launch only after configurationDone.
	initialized := initSignal
	configured := false
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
			if err = syncDebugPoints(startup, session, nil, initial, r.onBindings); err != nil {
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
			err := syncDebugPoints(request, session, sent, next, r.onBindings)
			finish()
			if err != nil {
				return err
			}
			sent = next
		case event := <-stops:
			request, finish := context.WithTimeout(ctx, 10*time.Second)
			tid, frames, err := stoppedFrames(request, session, event.threadID)
			var listed struct{ Threads []debugThread }
			if event.all && err == nil {
				_ = session.Call(request, "threads", nil, &listed)
			}
			finish()
			// Keep the freshness check and UI delivery ordered against a concurrent resume.
			eventMu.Lock()
			if generation != event.generation {
				eventMu.Unlock()
				continue
			}
			if err != nil {
				eventMu.Unlock()
				return err
			}
			if tid != nil {
				for i := range frames {
					p := frames[i].Source.Path
					if p != "" && !filepath.IsAbs(p) {
						frames[i].Source.Path = filepath.Join(r.folder, p)
					}
				}
				if event.all && len(listed.Threads) > 0 {
					pausedIDs = nil
				}
				if !slices.Contains(pausedIDs, *tid) {
					pausedIDs = append(pausedIDs, *tid)
				}
				for _, t := range listed.Threads {
					if !slices.Contains(pausedIDs, t.ID) {
						pausedIDs = append(pausedIDs, t.ID)
					}
				}
				threads := slices.Clone(listed.Threads)
				for _, id := range pausedIDs {
					if !slices.ContainsFunc(threads, func(t debugThread) bool { return t.ID == id }) {
						threads = append(threads, debugThread{ID: id, Name: fmt.Sprint(id)})
					}
				}
				stopped(*tid, frames, threads)
			}
			eventMu.Unlock()
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

func syncDebugPoints(ctx context.Context, s *dap.Session, prev, next map[string][]int, apply func(string, []int, []debugBinding)) error {
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
		var result struct{ Breakpoints []debugBinding }
		if err := s.Call(ctx, "setBreakpoints", map[string]any{"source": map[string]string{"path": path}, "breakpoints": list}, &result); err != nil {
			if apply != nil && ctx.Err() == nil && len(lines) > 0 {
				bindings := make([]debugBinding, len(lines))
				for i := range bindings {
					bindings[i].Message = err.Error()
				}
				apply(path, lines, bindings)
				continue
			}
			return fmt.Errorf("%s: %w", path, err)
		}
		if apply != nil {
			apply(path, lines, result.Breakpoints)
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
	r := a.activeDebugPeer()
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
			if cmd == "pause" {
				err = r.session.Call(ctx, cmd, map[string]int{"threadId": *tid}, nil)
			} else {
				err = r.resumeControl(ctx, r.session, cmd, *tid)
			}
		}

		_ = post(func() {
			if r.ctx.Err() != nil || r.generation != generation {
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

		})
	}()
}
