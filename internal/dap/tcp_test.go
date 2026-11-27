package dap

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/jsonrpc"
)

func TestTCPAdapterDisconnectLeavesServerAvailable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	adapter := Adapter{Transport: "tcp", Address: listener.Addr().String()}
	for range 2 {
		session, err := adapter.Open(ctx, Target{}, nil, nil, SessionOptions{RunInTerminal: func(context.Context, TerminalRequest) (int, error) {
			t.Error("TCP peer invoked local terminal")
			return 0, nil
		}})
		if err != nil {
			t.Fatal(err)
		}
		peer, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		deadline, _ := ctx.Deadline()
		if err := peer.SetDeadline(deadline); err != nil {
			t.Fatal(err)
		}
		codec := jsonrpc.NewCodec(peer, peer)
		if session.SupportsRunInTerminal() {
			t.Fatal("advertised local terminal capability to a TCP peer")
		}
		if err := codec.Send(map[string]any{"type": "request", "seq": 1, "command": "runInTerminal", "arguments": map[string]any{"args": []string{"tool"}}}); err != nil {
			t.Fatal(err)
		}
		var rejection packet
		if err := codec.ReceiveInto(&rejection); err != nil || rejection.Success || rejection.RequestSeq != 1 || rejection.Type != "response" {
			t.Fatal(rejection, err)
		}
		done := make(chan error, 1)
		go func() { done <- session.Disconnect(ctx, false) }()
		var request struct {
			Seq       int
			Command   string
			Arguments struct{ TerminateDebuggee *bool }
		}
		if err := codec.ReceiveInto(&request); err != nil {
			t.Fatal(err)
		}
		if request.Command != "disconnect" || request.Arguments.TerminateDebuggee == nil || *request.Arguments.TerminateDebuggee {
			t.Fatal(request)
		}
		if err := codec.Send(map[string]any{"type": "response", "request_seq": request.Seq, "success": true}); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		_ = peer.Close()
	}
}
