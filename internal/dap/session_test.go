package dap

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/jsonrpc"
)

type adapter struct {
	t *testing.T
	*jsonrpc.Codec
	net.Conn
}

func pipeSession(t *testing.T, onEvent func(Event)) (*Session, adapter) {
	t.Helper()
	client, peer := net.Pipe()
	if err := peer.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	s := NewSession(client, onEvent)
	t.Cleanup(s.Close)
	t.Cleanup(func() { _ = peer.Close() })
	return s, adapter{t, jsonrpc.NewCodec(peer, peer), peer}
}

func (a adapter) receive() packet {
	a.t.Helper()
	var p packet
	if err := a.ReceiveInto(&p); err != nil {
		a.t.Fatal(err)
	}
	return p
}

func (a adapter) send(msg any) {
	a.t.Helper()
	if err := a.Send(msg); err != nil {
		a.t.Fatal(err)
	}
}

func (a adapter) reply(p packet) {
	a.send(map[string]any{"type": "response", "request_seq": p.Seq, "success": true, "body": p.Command})
}

type blockedRequestWriter struct {
	io.Writer
	started chan struct{}
}

func (w blockedRequestWriter) Write(data []byte) (int, error) {
	select {
	case <-w.started:
	default:
		close(w.started)
	}
	return w.Writer.Write(data)
}

func TestControlSnapshotIncludesStopWhileRequestWriteIsBlocked(t *testing.T) {
	client, peer := net.Pipe()
	t.Cleanup(func() { _ = peer.Close() })
	if err := peer.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	writer := blockedRequestWriter{client, make(chan struct{})}
	var generation atomic.Int32
	handled := make(chan struct{})
	session := newSession(client, writer, func() { _ = client.Close() }, func(Event) { generation.Add(1); close(handled) }, SessionOptions{})
	t.Cleanup(session.Close)
	adapter := adapter{t, jsonrpc.NewCodec(peer, peer), peer}
	result := make(chan error, 1)
	var snapshot int32
	go func() {
		result <- session.CallWithResponse(t.Context(), "continue", map[string]int{"threadId": 1}, func() { snapshot = generation.Load() }, func(json.RawMessage) {
			if snapshot != 1 {
				t.Errorf("continue used generation %d, want the stop before transmission", snapshot)
			}
		})
	}()
	<-writer.started
	adapter.send(map[string]any{"type": "event", "event": "stopped", "body": map[string]int{"threadId": 2}})
	<-handled
	request := adapter.receive()
	adapter.reply(request)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestInterleavedRepliesAndEvents(t *testing.T) {
	events := make(chan Event, 1)
	s, a := pipeSession(t, func(e Event) { events <- e })
	results := make(chan error, 2)
	for _, command := range []string{"threads", "scopes"} {
		go func() {
			var result string
			err := s.Call(t.Context(), command, nil, &result)
			if err == nil && result != command {
				err = errors.New("wrong response: " + result)
			}
			results <- err
		}()
	}
	first, second := a.receive(), a.receive()
	if first.Seq != "1" || second.Seq != "2" {
		t.Fatal(first, second)
	}
	a.reply(second)
	a.send(map[string]any{"type": "event", "seq": "12", "event": "stopped", "body": map[string]int{"threadId": 1}})
	a.reply(first)
	_ = a.Close()
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	select {
	case e := <-events:
		if e.Name != "stopped" || string(e.Body) != `{"threadId":1}` {
			t.Fatal(e)
		}
	default:
		t.Fatal("missing event")
	}
}

func TestCanceledRequestIgnoresLateResponse(t *testing.T) {
	s, a := pipeSession(t, nil)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Call(ctx, "threads", nil, nil) }()
	p := a.receive()
	// Receiving the next request proves the first one has finished writing.
	go func() { done <- s.Disconnect(t.Context(), false) }()
	disconnect := a.receive()
	if disconnect.Command != "disconnect" {
		t.Fatal(disconnect)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	a.reply(p)
	a.reply(disconnect)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	<-s.Done()
}

func TestAdapterStopsReadingOrExits(t *testing.T) {
	for _, blocked := range []bool{true, false} {
		s, a := pipeSession(t, nil)
		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		done := make(chan error, 1)
		go func() { done <- s.Call(ctx, "threads", nil, nil) }()
		want := error(context.DeadlineExceeded)
		if !blocked {
			a.receive()
			_ = a.Close()
			want = io.EOF
		}
		if err := <-done; !errors.Is(err, want) {
			t.Fatalf("blocked=%t: %v", blocked, err)
		}
		cancel()
		<-s.Done()
	}
}

func TestStdioAdapter(t *testing.T) {
	if os.Getenv("TTT_TEST_DAP_CHILD") == "1" {
		codec := jsonrpc.NewCodec(os.Stdin, os.Stdout)
		var p packet
		if codec.ReceiveInto(&p) != nil || p.Command != "disconnect" {
			os.Exit(1)
		}
		if codec.Send(map[string]any{"type": "response", "request_seq": p.Seq, "success": true}) != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestStdioAdapter$")
	cmd.Env = append(os.Environ(), "TTT_TEST_DAP_CHILD=1", "GORACE=atexit_sleep_ms=0")
	s, err := StartStdio(cmd, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if err := s.Disconnect(ctx, false); err != nil {
		t.Fatal(err)
	}
	if !cmd.ProcessState.Success() {
		t.Fatal(cmd.ProcessState)
	}
}

func TestCloseReleasesPendingCalls(t *testing.T) {
	s, a := pipeSession(t, nil)
	done := make(chan error, 2)
	for range 2 {
		go func() { done <- s.Call(t.Context(), "threads", nil, nil) }()
		a.receive()
	}
	s.Close()
	for range 2 {
		if err := <-done; !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal(err)
		}
	}
}

func TestUnsupportedAdapterRequest(t *testing.T) {
	_, a := pipeSession(t, nil)
	a.send(map[string]any{"type": "request", "seq": 19, "command": "runInTerminal"})
	a.send(map[string]any{"type": "event", "event": "output"})
	p := a.receive()
	if p.Type != "response" || p.RequestSeq != 19 || p.Command != "runInTerminal" || p.Success || p.Message == "" {
		t.Fatal(p)
	}
}

func TestStdioAdapterExitWithInheritedStdout(t *testing.T) {
	switch os.Getenv("TTT_TEST_DAP_STDOUT") {
	case "descendant":
		time.Sleep(5 * time.Second)
		os.Exit(0)
	case "adapter":
		child := exec.Command(os.Args[0], "-test.run=^TestStdioAdapterExitWithInheritedStdout$")
		child.Env = append(os.Environ(), "TTT_TEST_DAP_STDOUT=descendant")
		child.Stdout = os.Stdout
		if child.Start() != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestStdioAdapterExitWithInheritedStdout$")
	cmd.Env = append(os.Environ(), "TTT_TEST_DAP_STDOUT=adapter", "GORACE=atexit_sleep_ms=0")
	s, err := StartStdio(cmd, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	select {
	case <-s.Done():
		if !cmd.ProcessState.Success() {
			t.Fatal(cmd.ProcessState)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("adapter exit hidden by descendant")
	}
}
