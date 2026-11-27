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
		session, err := adapter.Open(ctx, "", nil)
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
		done := make(chan error, 1)
		go func() { done <- session.Disconnect(ctx, false) }()
		codec := jsonrpc.NewCodec(peer, peer)
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
