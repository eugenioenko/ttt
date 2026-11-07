package plugin

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"
)

const rpcWriteTimeout = 10 * time.Second

var errRPCWriteTimeout = errors.New("RPC write timed out")
var errRPCQueueFull = errors.New("RPC outbound queue is full")

type rpcWriter struct {
	transport io.ReadWriteCloser
	timeout   time.Duration
	onError   func(error)
}

func (w rpcWriter) Write(data []byte) (int, error) {
	// Closing the transport also bounds writes on injected transports that do
	// not implement socket deadlines. A partial frame must never be reused.
	timer := time.AfterFunc(w.timeout, func() { w.onError(errRPCWriteTimeout) })
	defer timer.Stop()
	if socket, ok := w.transport.(interface{ SetWriteDeadline(time.Time) error }); ok {
		if err := socket.SetWriteDeadline(time.Now().Add(w.timeout)); err != nil {
			w.onError(err)
			return 0, err
		}
	}
	n, err := w.transport.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		w.onError(err)
	}
	return n, err
}

type rpcFrame struct {
	header string
	body   []byte
}

type rpcOutbox struct {
	rpcWriter
	ctx     context.Context
	frames  chan rpcFrame
	mu      sync.Mutex
	stopped bool
	pending int
	bytes   int
}

func newRPCOutbox(ctx context.Context, writer rpcWriter) *rpcOutbox {
	return &rpcOutbox{rpcWriter: writer, ctx: ctx, frames: make(chan rpcFrame, rpcOutboundQueueSize)}
}

func (q *rpcOutbox) WriteFrame(header string, body []byte) error {
	q.mu.Lock()
	if q.stopped || q.ctx.Err() != nil {
		q.mu.Unlock()
		return errRPCClosed
	}
	size := len(header) + len(body)
	if q.pending >= cap(q.frames) || size > rpcMaxMessageSize-q.bytes {
		q.mu.Unlock()
		return errRPCQueueFull
	}
	q.pending++
	q.bytes += size
	q.frames <- rpcFrame{header: header, body: body}
	q.mu.Unlock()
	return nil
}

func (q *rpcOutbox) run() {
	for {
		select {
		case <-q.ctx.Done():
			return
		case frame := <-q.frames:
			if q.ctx.Err() != nil {
				return
			}
			if _, err := q.rpcWriter.Write([]byte(frame.header)); err != nil {
				return
			}
			if _, err := q.rpcWriter.Write(frame.body); err != nil {
				return
			}
			q.mu.Lock()
			if !q.stopped {
				q.pending--
				q.bytes -= len(frame.header) + len(frame.body)
			}
			q.mu.Unlock()
		}
	}
}

func (q *rpcOutbox) stop() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.stopped = true
	q.pending, q.bytes = 0, 0
	for {
		select {
		case <-q.frames:
		default:
			return
		}
	}
}
