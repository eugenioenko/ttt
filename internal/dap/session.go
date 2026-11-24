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

type Session struct {
	jsAddr          string
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
	remoteDelve     bool
	stoppedThread   int
	breakpointFiles map[string]bool
}

func StartDelve(ctx context.Context, binary, folder string, onEvent func(Event)) (*Session, error) {
	return startListeningAdapter(ctx, binary, []string{"dap", "--listen=127.0.0.1:0"}, folder, "DAP server listening at: ", onEvent)
}
func startListeningAdapter(ctx context.Context, binary string, args []string, folder, prefix string, onEvent func(Event)) (*Session, error) {
	procCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(procCtx, binary, args...)
	cmd.Dir = folder
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
	return newSession(conn, conn, conn, kill, onEvent), nil
}

func newSession(r io.Reader, w io.Writer, conn io.Closer, kill func(), onEvent func(Event)) *Session {
	s := &Session{conn: conn, codec: jsonrpc.NewCodec(r, w), pending: make(map[int]chan packet), done: make(chan struct{}), initialized: make(chan struct{}), onEvent: onEvent, terminate: kill}
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
			if p.Command == "startDebugging" {
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
	if err := s.Call(ctx, "initialize", map[string]any{"supportsStartDebuggingRequest": adapter == "pwa-node" || adapter == "node", "adapterID": adapter, "clientID": "ttt", "linesStartAt1": true, "columnsStartAt1": true, "pathFormat": "path"}, &caps); err != nil {
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
			if s.remoteDelve && !terminateDebuggee {
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
