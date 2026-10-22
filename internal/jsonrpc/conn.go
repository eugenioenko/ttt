package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
)

var ErrClosed = errors.New("connection closed")

// Options are passed to NewConn rather than assigned afterwards because the
// read loop starts inside NewConn: a peer that writes or hangs up immediately
// would otherwise be handled before the caller installed its handlers.
type Options struct {
	// OnNotification runs on the read loop, so notifications are delivered in
	// order; it must not block on a Call to the same peer.
	OnNotification func(method string, params json.RawMessage)
	// OnRequest runs on its own goroutine. Its result is marshaled into the
	// reply; an *Error is sent as is, any other error as an internal error.
	// A nil OnRequest answers every request with method-not-found.
	OnRequest func(method string, params json.RawMessage) (any, error)
	// DispatchRequest, when set, replaces OnRequest and runs on the read loop
	// to preserve ordering with notifications. It must not block or Call the
	// same peer. reply may be called later; it writes off the read loop.
	DispatchRequest func(method string, params json.RawMessage, reply func(any, error))
	// OnCancel runs when a Call's context ends before its reply arrives.
	OnCancel func(id int)
	// OnClose runs once when the read loop ends, after pending Calls are
	// released and before Done is closed.
	OnClose func(err error)
	// MaxMessageSize bounds incoming messages in bytes; 0 means no limit. A
	// larger message ends the connection.
	MaxMessageSize int
}

type Conn struct {
	codec   *Codec
	opts    Options
	mu      sync.Mutex
	nextID  int
	pending map[int]chan Message
	done    chan struct{}
}

func NewConn(r io.Reader, w io.Writer, opts Options) *Conn {
	codec := NewCodec(r, w)
	codec.SetMaxContentLength(opts.MaxMessageSize)
	c := &Conn{
		codec:   codec,
		opts:    opts,
		nextID:  1,
		pending: make(map[int]chan Message),
		done:    make(chan struct{}),
	}
	go c.readLoop()
	return c
}

func (c *Conn) Done() <-chan struct{} { return c.done }

func (c *Conn) readLoop() {
	defer close(c.done)
	for {
		msg, err := c.codec.Receive()
		if err != nil {
			slog.Debug("jsonrpc read loop exit", "err", err)

			// Release in-flight callers before OnClose: it may block on a lock
			// held by a caller that cannot make progress until its Call
			// returns.
			c.mu.Lock()
			for _, ch := range c.pending {
				close(ch)
			}
			c.pending = make(map[int]chan Message)
			c.mu.Unlock()

			if c.opts.OnClose != nil {
				c.opts.OnClose(err)
			}
			return
		}
		if msg.IsNotification() {
			if c.opts.OnNotification != nil {
				c.opts.OnNotification(msg.Method, msg.Params)
			}
			continue
		}
		if msg.IsRequest() {
			if c.opts.DispatchRequest != nil {
				var once sync.Once
				c.opts.DispatchRequest(msg.Method, msg.Params, func(result any, err error) {
					once.Do(func() { go c.sendReply(msg, result, err) })
				})
				continue
			}
			// Off the read loop: a peer blocked writing to us may not be
			// reading, and a reply written from here could then deadlock
			// both sides.
			go c.answer(msg)
			continue
		}
		var id int
		if !msg.HasID() || json.Unmarshal(msg.ID, &id) != nil {
			continue
		}
		c.mu.Lock()
		ch, ok := c.pending[id]
		if ok {
			delete(c.pending, id)
		}
		c.mu.Unlock()
		if ok {
			ch <- msg
		}
	}
}

func (c *Conn) answer(req Message) {
	if c.opts.OnRequest == nil {
		c.sendReply(req, nil, &Error{Code: CodeMethodNotFound, Message: "method not supported: " + req.Method})
		return
	}
	result, err := c.opts.OnRequest(req.Method, req.Params)
	c.sendReply(req, result, err)
}

func (c *Conn) sendReply(req Message, result any, err error) {
	reply := Reply{JSONRPC: "2.0", ID: req.ID}
	if err != nil {
		var rpcErr *Error
		if !errors.As(err, &rpcErr) {
			rpcErr = &Error{Code: CodeInternalError, Message: err.Error()}
		}
		reply.Error = rpcErr
	} else if reply.Result, err = json.Marshal(result); err != nil {
		reply.Result = nil
		reply.Error = &Error{Code: CodeInternalError, Message: err.Error()}
	}
	if err := c.codec.Send(reply); err != nil {
		slog.Debug("jsonrpc reply failed", "method", req.Method, "err", err)
	}
}

func (c *Conn) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	ch := make(chan Message, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	err := c.codec.Send(Request{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		c.forget(id)
		return nil, err
	}

	var msg Message
	var ok bool
	select {
	case msg, ok = <-ch:
	case <-ctx.Done():
		c.forget(id)
		if c.opts.OnCancel != nil {
			c.opts.OnCancel(id)
		}
		return nil, ctx.Err()
	case <-c.done:
		// The read loop empties the pending map when it exits, so a call
		// registered after that point would otherwise wait for a reply that
		// can never arrive.
		c.forget(id)
		return nil, ErrClosed
	}
	if !ok {
		return nil, ErrClosed
	}
	if msg.Error != nil {
		return nil, msg.Error
	}
	return msg.Result, nil
}

func (c *Conn) forget(id int) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

func (c *Conn) Notify(method string, params any) error {
	return c.codec.Send(Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
}
