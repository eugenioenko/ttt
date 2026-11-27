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
	folder    string
	languages []string
	points    chan map[string][]int
	closed    chan struct{}
	cancel    context.CancelFunc
}

func (r *debugRun) launch(ctx context.Context, cmd *exec.Cmd, adapter string, args map[string]any, stopped func(string, int)) error {
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
			path, line, err := stoppedLocation(request, session, id)
			finish()
			if err != nil {
				return err
			}
			if path != "" && line > 0 {
				if !filepath.IsAbs(path) {
					path = filepath.Join(r.folder, path)
				}
				stopped(filepath.Clean(path), line)
			}
		}
	}
}

func stoppedLocation(ctx context.Context, session *dap.Session, id *int) (string, int, error) {
	var threads struct{ Threads []struct{ ID int } }
	if id == nil {
		if err := session.Call(ctx, "threads", nil, &threads); err != nil {
			return "", 0, err
		}
	} else {
		threads.Threads = append(threads.Threads, struct{ ID int }{*id})
	}
	for _, thread := range threads.Threads {
		var trace struct {
			StackFrames []struct {
				Line   int
				Source struct{ Path string }
			}
		}
		err := session.Call(ctx, "stackTrace", map[string]int{"threadId": thread.ID, "levels": 1}, &trace)
		if err != nil {
			if id != nil || ctx.Err() != nil {
				return "", 0, err
			}
			continue
		}
		if len(trace.StackFrames) > 0 {
			f := trace.StackFrames[0]
			if f.Source.Path != "" && f.Line > 0 {
				return f.Source.Path, f.Line, nil
			}
		}
	}
	return "", 0, nil
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
