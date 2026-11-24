package dap

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net"
	"os/exec"
	"slices"
	"strings"
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
	options         SessionOptions
	peerAddress     string
	child           *Session
	desired         map[string][]int
	closing         bool
	conn            io.Closer
	codec           *jsonrpc.Codec
	mu              sync.Mutex
	seq             int
	pending         map[int]chan packet
	done            chan struct{}
	initialized     chan struct{}
	onEvent         func(Event)
	terminate       func()
	closeOnce       sync.Once
	stoppedThread   int
	breakpointFiles map[string]bool
}

func ListenProcess(ctx context.Context, binary string, args []string, folder, prefix string, onEvent func(Event), options SessionOptions) (*Session, error) {
	procCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(procCtx, binary, args...)
	cmd.Dir = folder
	cmd.Stderr = adapterStderr(onEvent)
	cmd.WaitDelay = time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	address := make(chan string, 1)
	exited := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if addr, ok := strings.CutPrefix(scanner.Text(), prefix); ok {
				select {
				case address <- addr:
				default:
				}
			}
		}
		_ = cmd.Wait()
		close(exited)
	}()
	kill := func() { cancel(); _ = stdout.Close(); <-exited }
	var addr string
	select {
	case addr = <-address:
	case <-ctx.Done():
		kill()
		return nil, ctx.Err()
	case <-exited:
		cancel()
		return nil, fmt.Errorf("%s exited before listening: %v", binary, cmd.ProcessState)
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		kill()
		return nil, err
	}
	return SocketSession(conn, kill, onEvent, options), nil
}

func newSession(r io.Reader, w io.Writer, conn io.Closer, kill func(), onEvent func(Event)) *Session {
	return makeSession(r, w, conn, kill, onEvent, "", SessionOptions{})
}
func makeSession(r io.Reader, w io.Writer, conn io.Closer, kill func(), onEvent func(Event), addr string, options SessionOptions) *Session {
	s := &Session{options: options, peerAddress: addr, conn: conn, codec: jsonrpc.NewCodec(r, w), pending: make(map[int]chan packet), done: make(chan struct{}), initialized: make(chan struct{}), onEvent: onEvent, terminate: kill}
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
				var body struct{ ThreadID int }
				if json.Unmarshal(p.Body, &body) == nil {
					s.mu.Lock()
					s.stoppedThread = body.ThreadID
					s.mu.Unlock()
				}
			case "continued", "terminated":
				s.mu.Lock()
				s.stoppedThread = 0
				s.mu.Unlock()
			}
			if s.onEvent != nil {
				s.onEvent(Event{p.Event, p.Body})
			}
		case "request":
			if p.Command == "startDebugging" && s.options.ChildTarget != nil {
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
	case "stackTrace", "scopes", "threads", "variables", "pause", "continue", "next", "stepIn", "stepOut":
		s.mu.Lock()
		child := s.child
		s.mu.Unlock()
		if child != nil {
			return child.Call(ctx, command, args, out)
		}
	}
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
	err := s.Call(ctx, "setBreakpoints", map[string]any{"source": map[string]string{"path": path}, "breakpoints": points}, &out)
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
	var caps struct{ SupportsConfigurationDoneRequest bool }
	if err := s.Call(ctx, "initialize", map[string]any{"supportsStartDebuggingRequest": s.options.ChildTarget != nil, "adapterID": adapter, "clientID": "ttt", "linesStartAt1": true, "columnsStartAt1": true, "pathFormat": "path"}, &caps); err != nil {
		return err
	}
	launched := make(chan error, 1)
	go func() { launched <- s.Call(ctx, request, args, nil) }()
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
				thread := s.stoppedThread
				s.mu.Unlock()
				if thread != 0 {
					_ = s.Call(ctx, "continue", map[string]int{"threadId": thread}, nil)
				}
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
	ChildTarget        func(json.RawMessage) (Target, error)
	ResumeBeforeDetach bool
}

func SocketSession(conn net.Conn, cleanup func(), onEvent func(Event), options SessionOptions) *Session {
	return makeSession(conn, conn, conn, cleanup, onEvent, conn.RemoteAddr().String(), options)
}

func Connect(ctx context.Context, addr string, onEvent func(Event), options SessionOptions) (*Session, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return SocketSession(conn, func() {}, onEvent, options), nil
}
func (s *Session) startChild(p packet) {
	err := s.attachChild(p.Arguments)
	response := map[string]any{"type": "response", "seq": s.nextSeq(), "command": p.Command, "request_seq": p.Seq, "success": err == nil}
	if err != nil {
		response["message"] = err.Error()
	}
	_ = s.codec.Send(response)
}
func (s *Session) attachChild(raw json.RawMessage) error {
	target, err := s.options.ChildTarget(raw)
	if err != nil {
		return err
	}
	addr := s.peerAddress
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	child := newSession(conn, conn, conn, func() {}, s.onEvent)
	s.mu.Lock()
	if s.child != nil || s.closing {
		s.mu.Unlock()
		_ = conn.Close()
		return fmt.Errorf("debug target already active or closing")
	}
	points := maps.Clone(s.desired)
	s.child = child
	s.mu.Unlock()
	err = child.Start(ctx, target.Type, target.Request, target.Arguments, func() error {
		for path, lines := range points {
			if _, err := child.SetBreakpoints(ctx, path, lines); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_ = conn.Close()
		s.mu.Lock()
		if s.child == child {
			s.child = nil
		}
		s.mu.Unlock()
	}
	return err
}
