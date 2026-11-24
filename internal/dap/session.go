package dap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/eugenioenko/ttt/internal/jsonrpc"
)

type packet struct {
	Arguments  json.RawMessage
	Type       string
	Seq        int
	Command    string
	RequestSeq int `json:"request_seq"`
	Event      string
	Body       json.RawMessage
	Success    bool
	Message    string
}

type Event struct {
	Name string
	Body json.RawMessage
}

type adapterStderr func(Event)

func (output adapterStderr) Write(data []byte) (int, error) {
	if output != nil {
		body, err := json.Marshal(map[string]string{"category": "stderr", "output": string(data)})
		if err != nil {
			return 0, err
		}
		output(Event{Name: "output", Body: body})
	}
	return len(data), nil
}

type Session struct {
	options           SessionOptions
	child             *Session
	desired           map[string][]int
	closing           bool
	conn              io.Closer
	codec             *jsonrpc.Codec
	mu                sync.Mutex
	seq               int
	pending           map[int]chan packet
	done              chan struct{}
	initialized       chan struct{}
	onEvent           func(Event)
	terminate         func()
	closeOnce         sync.Once
	stoppedThread     *int
	stopped           bool
	supportsTerminate bool
	breakpointFiles   map[string]bool
}

func newSession(r io.Reader, w io.Writer, conn io.Closer, kill func(), onEvent func(Event)) *Session {
	return makeSession(r, w, conn, kill, onEvent, SessionOptions{})
}
func makeSession(r io.Reader, w io.Writer, conn io.Closer, kill func(), onEvent func(Event), options SessionOptions) *Session {
	s := &Session{options: options, conn: conn, codec: jsonrpc.NewCodec(r, w), pending: make(map[int]chan packet), done: make(chan struct{}), initialized: make(chan struct{}), onEvent: onEvent, terminate: kill}
	s.codec.SetMaxContentLength(32 << 20)
	go s.read()
	return s
}

func (s *Session) read() {
	defer close(s.done)
	for {
		var p packet
		if s.codec.ReceiveInto(&p) != nil {
			return
		}
		switch p.Type {
		case "response":
			s.mu.Lock()
			reply := s.pending[p.RequestSeq]
			delete(s.pending, p.RequestSeq)
			s.mu.Unlock()
			if reply != nil {
				reply <- p
			}
		case "event":
			if p.Event == "initialized" {
				select {
				case <-s.initialized:
				default:
					close(s.initialized)
				}
			}
			switch p.Event {
			case "stopped":
				var body struct{ ThreadID *int }
				if json.Unmarshal(p.Body, &body) == nil {
					s.mu.Lock()
					s.stopped = true
					s.stoppedThread = body.ThreadID
					s.mu.Unlock()
				}
			case "continued", "terminated":
				s.mu.Lock()
				s.stopped = false
				s.stoppedThread = nil
				s.mu.Unlock()
			}
			if s.onEvent != nil {
				s.onEvent(Event{p.Event, p.Body})
			}
		case "request":
			if p.Command == "runInTerminal" && s.options.RunInTerminal != nil {
				go s.runTerminal(p)
				continue
			}
			if p.Command == "startDebugging" && s.options.openChild != nil {
				go s.startChild(p)
				continue
			}
			if err := s.codec.Send(map[string]any{"type": "response", "seq": s.nextSeq(), "command": p.Command, "request_seq": p.Seq, "success": false, "message": "unsupported request"}); err != nil {
				return
			}
		}
	}
}
func (s *Session) nextSeq() int { s.mu.Lock(); defer s.mu.Unlock(); s.seq++; return s.seq }
func (s *Session) Call(ctx context.Context, command string, args any, out any) error {
	switch command {
	case "initialize", "launch", "attach", "configurationDone", "disconnect", "terminate":
	default:
		s.mu.Lock()
		child := s.child
		s.mu.Unlock()
		if child != nil {
			return child.Call(ctx, command, args, out)
		}
	}
	return s.call(ctx, command, args, out)
}
func (s *Session) call(ctx context.Context, command string, args, out any) error {
	return s.sendCall(ctx, command, args, out, nil)
}
func (s *Session) sendCall(ctx context.Context, command string, args, out any, sent chan struct{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id := s.nextSeq()
	reply := make(chan packet, 1)
	s.mu.Lock()
	s.pending[id] = reply
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.pending, id); s.mu.Unlock() }()
	// A partial frame cannot be retried. Closing the stream interrupts a blocked
	// write when an adapter stops reading, including during editor shutdown.
	stop := context.AfterFunc(ctx, func() { _ = s.conn.Close() })
	err := s.codec.Send(map[string]any{"type": "request", "seq": id, "command": command, "arguments": args})
	if !stop() {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	if sent != nil {
		close(sent)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return io.EOF
	case p := <-reply:
		if !p.Success {
			var detail struct{ Error struct{ Format string } }
			_ = json.Unmarshal(p.Body, &detail)
			return fmt.Errorf("%s: %s %s", command, p.Message, detail.Error.Format)
		}
		if out != nil {
			return json.Unmarshal(p.Body, out)
		}
		return nil
	}
}

type Breakpoint struct {
	ID       *int
	Source   struct{ Path string }
	Verified bool
	Line     int
	Message  string
}

func (s *Session) SetBreakpoints(ctx context.Context, path string, lines []int) ([]Breakpoint, error) {
	s.mu.Lock()
	if s.desired == nil {
		s.desired = make(map[string][]int)
	}
	s.desired[path] = slices.Clone(lines)
	child := s.child
	s.mu.Unlock()
	if child != nil {
		return child.SetBreakpoints(ctx, path, lines)
	}
	points := make([]map[string]int, 0, len(lines))
	for _, line := range lines {
		points = append(points, map[string]int{"line": line})
	}
	var out struct{ Breakpoints []Breakpoint }
	err := s.call(ctx, "setBreakpoints", map[string]any{"source": map[string]string{"path": path}, "breakpoints": points}, &out)
	if err == nil {
		s.mu.Lock()
		if len(lines) > 0 {
			if s.breakpointFiles == nil {
				s.breakpointFiles = make(map[string]bool)
			}
			s.breakpointFiles[path] = true
		} else {
			delete(s.breakpointFiles, path)
		}
		s.mu.Unlock()
	}
	return out.Breakpoints, err
}
func (s *Session) Start(ctx context.Context, adapter, request string, args map[string]any, configure func() error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var caps struct {
		SupportsConfigurationDoneRequest, SupportsTerminateRequest bool
		ExceptionBreakpointFilters                                 []struct {
			Filter  string
			Default bool
		}
	}
	if err := s.Call(ctx, "initialize", map[string]any{"supportsRunInTerminalRequest": s.options.RunInTerminal != nil, "supportsStartDebuggingRequest": s.options.openChild != nil, "adapterID": adapter, "clientID": "ttt", "linesStartAt1": true, "columnsStartAt1": true, "pathFormat": "path"}, &caps); err != nil {
		return err
	}
	s.mu.Lock()
	s.supportsTerminate = caps.SupportsTerminateRequest
	s.mu.Unlock()
	launched := make(chan error, 1)
	sent := make(chan struct{})
	go func() { launched <- s.sendCall(ctx, request, args, nil, sent) }()
	select {
	case <-sent:
	case err := <-launched:
		if err != nil {
			return err
		}
		launched = nil
	case <-ctx.Done():
		return ctx.Err()
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.done:
			return io.EOF
		case err := <-launched:
			if err != nil {
				return err
			}
			launched = nil
		case <-s.initialized:
			if err := configure(); err != nil {
				return err
			}
			if len(caps.ExceptionBreakpointFilters) > 0 {
				filters := []string{}
				for _, f := range caps.ExceptionBreakpointFilters {
					if f.Default {
						filters = append(filters, f.Filter)
					}
				}
				if err := s.call(ctx, "setExceptionBreakpoints", map[string]any{"filters": filters}, nil); err != nil {
					return err
				}
			}
			if caps.SupportsConfigurationDoneRequest {
				if err := s.Call(ctx, "configurationDone", struct{}{}, nil); err != nil {
					return err
				}
			}
			if launched == nil {
				return nil
			}
			select {
			case err := <-launched:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
func (s *Session) Done() <-chan struct{} { return s.done }
func (s *Session) Close(terminateDebuggee bool) {
	s.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		finished := make(chan struct{})
		go func() {
			s.mu.Lock()
			s.closing = true
			supportsTerminate := s.supportsTerminate
			child := s.child
			s.mu.Unlock()
			if child != nil {
				child.Close(terminateDebuggee)
			}
			if s.options.ResumeBeforeDetach && !terminateDebuggee {
				s.mu.Lock()
				files := slices.Collect(maps.Keys(s.breakpointFiles))
				s.mu.Unlock()
				for _, path := range files {
					_, _ = s.SetBreakpoints(ctx, path, nil)
				}
				s.mu.Lock()
				stopped, thread := s.stopped, s.stoppedThread
				s.mu.Unlock()
				if stopped && thread == nil {
					var reply struct{ Threads []struct{ ID int } }
					if s.call(ctx, "threads", struct{}{}, &reply) == nil && len(reply.Threads) > 0 {
						thread = &reply.Threads[0].ID
					}
				}
				if stopped && thread != nil {
					_ = s.Call(ctx, "continue", map[string]int{"threadId": *thread}, nil)
				}
			}
			if terminateDebuggee && supportsTerminate {
				termCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
				_ = s.call(termCtx, "terminate", struct{}{}, nil)
				cancel()
			}
			_ = s.Call(ctx, "disconnect", map[string]bool{"terminateDebuggee": terminateDebuggee}, nil)
			close(finished)
		}()
		select {
		case <-finished:
		case <-ctx.Done():
		}
		_ = s.conn.Close()
		s.terminate()
	})
}

type SessionOptions struct {
	OnChildFailure     func(error)
	AdapterType        string
	openChild          func(context.Context) (*Session, error)
	RunInTerminal      func(TerminalRequest) (int, error)
	ResumeBeforeDetach bool
}

func SocketSession(conn net.Conn, cleanup func(), onEvent func(Event), options SessionOptions) *Session {
	addr := conn.RemoteAddr()
	if addr.Network() == "tcp" || (addr.Network() == "unix" && addr.String() != "" && addr.String() != "@") {
		options.openChild = func(ctx context.Context) (*Session, error) {
			c, err := (&net.Dialer{}).DialContext(ctx, addr.Network(), addr.String())
			if err != nil {
				return nil, err
			}
			return SocketSession(c, func() {}, onEvent, options), nil
		}
	}
	return makeSession(conn, conn, conn, cleanup, onEvent, options)
}

func Connect(ctx context.Context, addr string, onEvent func(Event), options SessionOptions) (*Session, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return SocketSession(conn, func() {}, onEvent, options), nil
}

var errChildActive = errors.New("debug target already active or closing")

func (s *Session) startChild(p packet) {
	err := s.attachChild(p.Arguments)
	response := map[string]any{"type": "response", "seq": s.nextSeq(), "command": p.Command, "request_seq": p.Seq, "success": err == nil}
	if err != nil {
		response["message"] = err.Error()
		s.mu.Lock()
		idle := s.child == nil && !s.closing
		s.mu.Unlock()
		if idle && !errors.Is(err, errChildActive) && s.options.OnChildFailure != nil {
			s.options.OnChildFailure(err)
		}
	}
	_ = s.codec.Send(response)
}
func (s *Session) attachChild(raw json.RawMessage) error {
	var request struct {
		Request       string
		Configuration map[string]any
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		return err
	}
	if request.Request != "launch" && request.Request != "attach" || request.Configuration == nil {
		return fmt.Errorf("invalid startDebugging request")
	}
	target := Target{Type: s.options.AdapterType, Request: request.Request, Arguments: request.Configuration}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	child, err := s.options.openChild(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.child != nil || s.closing {
		s.mu.Unlock()
		child.Close(false)
		return errChildActive
	}
	points := maps.Clone(s.desired)
	s.child = child
	s.mu.Unlock()
	err = child.Start(ctx, target.Type, target.Request, target.Arguments, func() error {
		for path, lines := range points {
			if _, err := child.SetBreakpoints(ctx, path, lines); err != nil && s.onEvent != nil {
				body, _ := json.Marshal(map[string]string{"category": "stderr", "output": path + ": " + err.Error() + "\n"})
				s.onEvent(Event{"output", body})
			}
		}
		return nil
	})
	if err != nil {
		child.Close(false)
		s.mu.Lock()
		s.child = nil
		s.mu.Unlock()
		return err
	}
	go func() {
		<-child.Done()
		child.Close(false)
		s.mu.Lock()
		active := s.child == child && !s.closing
		if s.child == child {
			s.child = nil
		}
		s.mu.Unlock()
		if active && s.onEvent != nil {
			s.onEvent(Event{Name: "terminated"})
		}
	}()
	return err
}

// OnCleanup registers resource cleanup before the session is handed to its caller.
func (s *Session) OnCleanup(clean func()) {
	prev := s.terminate
	s.terminate = func() { prev(); clean() }
}

type TerminalRequest struct {
	Kind, Title, Cwd            string
	Args                        []string
	Env                         map[string]*string
	ArgsCanBeInterpretedByShell bool
}

func (s *Session) runTerminal(p packet) {
	var req TerminalRequest
	err := json.Unmarshal(p.Arguments, &req)
	var pid int
	if err == nil {
		if req.Kind == "external" || req.ArgsCanBeInterpretedByShell || len(req.Args) == 0 || req.Args[0] == "" {
			err = fmt.Errorf("expected an integrated terminal and literal argv")
		} else {
			pid, err = s.options.RunInTerminal(req)
		}
	}
	reply := map[string]any{"seq": s.nextSeq(), "type": "response", "command": p.Command, "request_seq": p.Seq, "success": err == nil}
	if err != nil {
		reply["message"] = err.Error()
	} else {
		reply["body"] = map[string]int{"processId": pid}
	}
	_ = s.codec.Send(reply)
}
