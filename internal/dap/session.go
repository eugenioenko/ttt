package dap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/eugenioenko/ttt/internal/jsonrpc"
)

type packet struct {
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

type Session struct {
	codec          *jsonrpc.Codec
	onEvent        func(Event)
	closeTransport func()
	closeOnce      sync.Once
	done           chan struct{}
	err            error
	writer         chan struct{}
	seq            int
	mu             sync.Mutex
	pending        map[int]chan packet
}

// NewSession takes ownership of the stream. Closing it must interrupt both reads
// and writes. onEvent runs on the reader and must not wait for another packet.
func NewSession(stream io.ReadWriteCloser, onEvent func(Event)) *Session {
	return newSession(stream, stream, func() { _ = stream.Close() }, onEvent)
}

func newSession(r io.Reader, w io.Writer, closeTransport func(), onEvent func(Event)) *Session {
	s := &Session{codec: jsonrpc.NewCodec(r, w), closeTransport: closeTransport, onEvent: onEvent,
		done: make(chan struct{}), writer: make(chan struct{}, 1), pending: make(map[int]chan packet)}
	s.codec.SetMaxContentLength(32 << 20)
	go s.read()
	return s
}

func (s *Session) Done() <-chan struct{} { return s.done }

func (s *Session) Close() { s.finish(io.ErrClosedPipe) }

func (s *Session) finish(err error) {
	s.closeOnce.Do(func() {
		s.err = err
		s.closeTransport()
		close(s.done)
	})
}

func (s *Session) read() {
	for {
		var p packet
		if err := s.codec.ReceiveInto(&p); err != nil {
			s.finish(err)
			return
		}
		switch p.Type {
		case "response":
			s.mu.Lock()
			if reply := s.pending[p.RequestSeq]; reply != nil {
				reply <- p
				delete(s.pending, p.RequestSeq)
			}
			s.mu.Unlock()
		case "event":
			if s.onEvent != nil {
				s.onEvent(Event{Name: p.Event, Body: p.Body})
			}
		case "request":
			// No reverse-request capability is advertised by this client yet.
			err := s.write(context.Background(), func(seq int) any {
				return map[string]any{"type": "response", "seq": seq, "request_seq": p.Seq, "command": p.Command, "success": false, "message": "unsupported request"}
			})
			if err != nil {
				return
			}
		}
	}
}

func (s *Session) write(ctx context.Context, message func(int) any) error {
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
	if !stop() {
		return ctx.Err()
	}
	if err != nil {
		s.finish(err)
	}
	return err
}

func (s *Session) Call(ctx context.Context, command string, args, out any) error {
	var arguments json.RawMessage
	if args != nil {
		var err error
		arguments, err = json.Marshal(args)
		if err != nil {
			return err
		}
	}
	var id int
	reply := make(chan packet, 1)
	err := s.write(ctx, func(seq int) any {
		id = seq
		s.mu.Lock()
		s.pending[id] = reply
		s.mu.Unlock()
		return struct {
			Type      string          `json:"type"`
			Seq       int             `json:"seq"`
			Command   string          `json:"command"`
			Arguments json.RawMessage `json:"arguments,omitempty"`
		}{"request", seq, command, arguments}
	})
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
	defer s.Close()
	return s.Call(ctx, "disconnect", map[string]bool{"terminateDebuggee": terminateDebuggee}, nil)
}
