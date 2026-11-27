package dap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/eugenioenko/ttt/internal/jsonrpc"
)

type packet struct {
	Arguments  json.RawMessage
	Type       string
	Seq        json.Number
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

type pendingCall struct {
	reply     chan packet
	onSuccess func(json.RawMessage)
}

type Session struct {
	options        SessionOptions
	cleanupMu      sync.Mutex
	cleaned        bool
	cleanups       []func()
	reverseCtx     context.Context
	cancelReverse  context.CancelFunc
	codec          *jsonrpc.Codec
	onEvent        func(Event)
	closeTransport func()
	closeOnce      sync.Once
	done           chan struct{}
	err            error
	writer         chan struct{}
	seq            int
	mu             sync.Mutex
	pending        map[int]pendingCall
}

// NewSession takes ownership of the stream. Closing it must interrupt both reads
// and writes. onEvent runs on the reader and must not wait for another packet.
func NewSession(stream io.ReadWriteCloser, onEvent func(Event)) *Session {
	return newSession(stream, stream, func() { _ = stream.Close() }, onEvent, SessionOptions{})
}

func newSession(r io.Reader, w io.Writer, closeTransport func(), onEvent func(Event), options SessionOptions) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{options: options, reverseCtx: ctx, cancelReverse: cancel, codec: jsonrpc.NewCodec(r, w), closeTransport: closeTransport, onEvent: onEvent,
		done: make(chan struct{}), writer: make(chan struct{}, 1), pending: make(map[int]pendingCall)}
	s.codec.SetMaxContentLength(32 << 20)
	go s.read()
	return s
}

func (s *Session) Done() <-chan struct{} { return s.done }

func (s *Session) Close() { s.finish(io.ErrClosedPipe) }

func (s *Session) finish(err error) {
	s.closeOnce.Do(func() {
		s.cancelReverse()
		s.err = err
		s.closeTransport()
		s.cleanupMu.Lock()
		s.cleaned = true
		cleanups := s.cleanups
		s.cleanups = nil
		s.cleanupMu.Unlock()
		for _, cleanup := range cleanups {
			cleanup()
		}
		close(s.done)
	})
}

func (s *Session) read() {
	requests := make(chan packet, 16)
	defer close(requests)
	go func() {
		for p := range requests {
			if s.reverseCtx.Err() != nil {
				return
			}
			response := s.reverseResponse(s.reverseCtx, p)
			err := s.write(context.Background(), nil, func(seq int) any {
				response["seq"] = seq
				return response
			})
			if err != nil {
				return
			}
		}
	}()

	for {
		var p packet
		if err := s.codec.ReceiveInto(&p); err != nil {
			s.finish(err)
			return
		}
		switch p.Type {
		case "response":
			s.mu.Lock()
			call, ok := s.pending[p.RequestSeq]
			delete(s.pending, p.RequestSeq)
			s.mu.Unlock()
			if ok {
				if p.Success && call.onSuccess != nil {
					call.onSuccess(p.Body)
				}
				call.reply <- p
			}
		case "event":
			if s.onEvent != nil {
				s.onEvent(Event{Name: p.Event, Body: p.Body})
			}
		case "request":
			if !(p.Command == "runInTerminal" && s.SupportsRunInTerminal() || p.Command == "startDebugging" && s.SupportsStartDebugging()) {
				p.Arguments = nil
			}
			p.Body = nil
			select {
			case requests <- p:
			default:
				s.finish(errors.New("too many unanswered DAP reverse requests"))
				return
			}
		}
	}
}

func (s *Session) write(ctx context.Context, afterWrite func(), message func(int) any) error {
	select {
	case s.writer <- struct{}{}:
		defer func() { <-s.writer }()
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return s.err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return s.err
	default:
	}
	s.seq++
	// A partial frame cannot be retried. Interrupt the transport if its writer stalls.
	stop := context.AfterFunc(ctx, s.Close)
	err := s.codec.Send(message(s.seq))
	if err == nil && afterWrite != nil {
		afterWrite()
	}
	if !stop() {
		return ctx.Err()
	}
	if err != nil {
		s.finish(err)
	}
	return err
}

func (s *Session) Call(ctx context.Context, command string, args, out any) error {
	return s.call(ctx, command, args, out, nil, nil)
}

// CallWithResponse handles a successful response on the reader before subsequent events.
// afterWrite runs once the request frame has been sent, before onSuccess.
// Neither callback may wait for another packet.
func (s *Session) CallWithResponse(ctx context.Context, command string, args any, afterWrite func(), onSuccess func(json.RawMessage)) error {
	return s.call(ctx, command, args, nil, afterWrite, onSuccess)
}

func (s *Session) call(ctx context.Context, command string, args, out any, afterWrite func(), onSuccess func(json.RawMessage)) error {
	var arguments json.RawMessage
	if args != nil {
		var err error
		arguments, err = json.Marshal(args)
		if err != nil {
			return err
		}
	}
	var written chan struct{}
	if afterWrite != nil {
		written = make(chan struct{})
		handle := onSuccess
		onSuccess = func(body json.RawMessage) {
			<-written
			if handle != nil {
				handle(body)
			}
		}
	}
	var id int
	reply := make(chan packet, 1)
	err := s.write(ctx, afterWrite, func(seq int) any {
		id = seq
		s.mu.Lock()
		s.pending[id] = pendingCall{reply, onSuccess}
		s.mu.Unlock()
		return struct {
			Type      string          `json:"type"`
			Seq       int             `json:"seq"`
			Command   string          `json:"command"`
			Arguments json.RawMessage `json:"arguments,omitempty"`
		}{"request", seq, command, arguments}
	})
	if written != nil {
		close(written)
	}
	defer func() { s.mu.Lock(); delete(s.pending, id); s.mu.Unlock() }()
	if err != nil {
		return err
	}
	var p packet
	select {
	case p = <-reply:
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		// Adapters can close stdout immediately after their last response.
		select {
		case p = <-reply:
		default:
			return s.err
		}
	}
	if !p.Success {
		return fmt.Errorf("%s: %s", command, p.Message)
	}
	if out != nil {
		return json.Unmarshal(p.Body, out)
	}
	return nil
}

func (s *Session) Disconnect(ctx context.Context, terminateDebuggee bool) error {
	s.cancelReverse()
	defer s.Close()
	return s.Call(ctx, "disconnect", map[string]bool{"terminateDebuggee": terminateDebuggee}, nil)
}
