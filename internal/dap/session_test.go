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
		deadline, _ := ctx.Deadline()
		if err := peer.SetDeadline(deadline); err != nil {
			t.Fatal(err)
		}
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
		reply(receive("initialize"), map[string]any{"supportsConfigurationDoneRequest": true, "exceptionBreakpointFilters": []map[string]any{{"filter": "raised", "default": false}, {"filter": "uncaught", "default": true}}})
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
		filters := receive("setExceptionBreakpoints")
		if string(filters.Arguments) != `{"filters":["uncaught"]}` {
			t.Fatal(string(filters.Arguments))
		}
		reply(filters, nil)
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
		var ctx context.Context
		var cancel context.CancelFunc
		if blockedWrite {
			ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
		} else {
			ctx, cancel = context.WithCancel(context.Background())
		}
		t.Cleanup(cancel)
		t.Cleanup(func() { _ = client.Close(); _ = peer.Close() })
		if err := peer.SetDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
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

func TestDetachResumesStoppedThreadWithZeroOrMissingID(t *testing.T) {
	for _, missing := range []bool{false, true} {
		client, peer := net.Pipe()
		defer client.Close()
		defer peer.Close()
		if err := peer.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
		observed := make(chan struct{})
		s := SocketSession(client, func() {}, func(Event) { close(observed) }, SessionOptions{ResumeBeforeDetach: true})
		codec := jsonrpc.NewCodec(peer, peer)
		body := map[string]int{}
		if !missing {
			body["threadId"] = 0
		}
		if err := codec.Send(map[string]any{"type": "event", "event": "stopped", "body": body}); err != nil {
			t.Fatal(err)
		}
		<-observed
		closed := make(chan struct{})
		go func() { s.Close(false); close(closed) }()
		for _, cmd := range []string{"threads", "continue", "disconnect"} {
			if cmd == "threads" && !missing {
				continue
			}
			var p packet
			if err := codec.ReceiveInto(&p); err != nil || p.Command != cmd {
				t.Fatalf("wanted %s, got %s (%v)", cmd, p.Command, err)
			}
			if cmd == "continue" && string(p.Arguments) != `{"threadId":0}` {
				t.Fatal(string(p.Arguments))
			}
			if err := codec.Send(map[string]any{"type": "response", "success": true, "request_seq": p.Seq, "body": map[string]any{"threads": []map[string]int{{"id": 0}}}}); err != nil {
				t.Fatal(err)
			}
		}
		<-closed
	}
}
