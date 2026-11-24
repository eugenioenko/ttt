package app

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/jsonrpc"
)

func TestQueuedVariableResultExpiresOnResume(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	s, err := dap.Connect(ctx, listener.Addr().String(), nil, dap.SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(false)
	peer, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = peer.Close() }()
	deadline, _ := ctx.Deadline()
	if err := peer.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	queued := make(chan func(), 1)
	r := &debugRun{ctx: ctx, session: s}
	a := &App{}
	a.debugger.run = r
	a.tasks.post = func(fn func()) error { queued <- fn; return nil }
	applied := false
	debugQuery(a, r, "variables", map[string]int{"variablesReference": 42}, func(_ struct{}) { applied = true })
	codec := jsonrpc.NewCodec(peer, peer)
	var req struct{ Seq int }
	if err := codec.ReceiveInto(&req); err != nil {
		t.Fatal(err)
	}
	if err := codec.Send(map[string]any{"type": "response", "success": true, "request_seq": req.Seq, "body": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	select {
	case apply := <-queued:
		a.onDebugEvent(r, "probe", dap.Event{Name: "continued"})
		apply()
		if applied {
			t.Fatal("expired variables repopulated the running session")
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
