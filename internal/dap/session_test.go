package dap

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/jsonrpc"
)

func TestStartupAcceptsEitherLaunchReplyOrdering(t *testing.T) {
	for _, late := range []bool{false, true} {
		client, peer := net.Pipe()
		s := newSession(client, client, client, func() {}, nil)
		codec := jsonrpc.NewCodec(peer, peer)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		t.Cleanup(cancel)
		t.Cleanup(func() { _ = client.Close(); _ = peer.Close() })
		done := make(chan error, 1)
		go func() {
			done <- s.Start(ctx, "go", "launch", nil, func() error { _, err := s.SetBreakpoints(ctx, "file.go", nil); return err })
		}()
		receive := func(command string) packet {
			t.Helper()
			var p packet
			if err := codec.ReceiveInto(&p); err != nil || p.Command != command {
				t.Fatalf("wanted %s, received %+v (%v)", command, p, err)
			}
			return p
		}
		reply := func(p packet, body any) {
			t.Helper()
			if err := codec.Send(map[string]any{"type": "response", "success": true, "request_seq": p.Seq, "body": body}); err != nil {
				t.Fatal(err)
			}
		}
		reply(receive("initialize"), map[string]bool{"supportsConfigurationDoneRequest": true})
		launch := receive("launch")
		if !late {
			reply(launch, nil)
		}
		if err := codec.Send(map[string]string{"type": "event", "event": "initialized"}); err != nil {
			t.Fatal(err)
		}
		bp := receive("setBreakpoints")
		if string(bp.Arguments) != "{\"breakpoints\":[],\"source\":{\"path\":\"file.go\"}}" {
			t.Fatalf("invalid clear: %s", bp.Arguments)
		}
		reply(bp, map[string]any{"breakpoints": []any{}})
		reply(receive("configurationDone"), nil)
		if late {
			reply(launch, nil)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		_ = peer.Close()
	}
}

func TestAdapterStopsReadingOrExits(t *testing.T) {
	for _, blockedWrite := range []bool{false, true} {
		client, peer := net.Pipe()
		s := newSession(client, client, client, func() {}, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		done := make(chan error, 1)
		go func() { done <- s.Call(ctx, "threads", nil, nil) }()
		if !blockedWrite {
			var p packet
			if err := jsonrpc.NewCodec(peer, peer).ReceiveInto(&p); err != nil {
				t.Fatal(err)
			}
			_ = peer.Close()
		}
		select {
		case err := <-done:
			if blockedWrite && !errors.Is(err, context.DeadlineExceeded) || !blockedWrite && !errors.Is(err, io.EOF) {
				t.Fatalf("blocked=%t: %v", blockedWrite, err)
			}
		case <-time.After(time.Second):
			t.Fatal("request did not return")
		}
		cancel()
		_ = peer.Close()
		_ = client.Close()
	}
}
