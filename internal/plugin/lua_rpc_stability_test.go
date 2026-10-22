package plugin

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/jsonrpc"
	lua "github.com/yuin/gopher-lua"
)

func TestRPCConnectDestroyBeforeDialStarts(t *testing.T) {
	if os.Getenv("TTT_RPC_DIAL_DESTROY_HELPER") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestRPCConnectDestroyBeforeDialStarts$")
		cmd.Env = append(os.Environ(), "TTT_RPC_DIAL_DESTROY_HELPER=1", "GODEBUG=asyncpreemptoff=1", "GOGC=off")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("connect followed by unload crashed: %v\n%s", err, output)
		}
		return
	}
	runtime.GOMAXPROCS(1)
	p, _ := newTestPluginBase(rpcPerms)
	t.Cleanup(p.Destroy)
	called := make(chan error, 1)
	p.DialRPC = func(ctx context.Context, _ string) (io.ReadWriteCloser, error) {
		called <- ctx.Err()
		return nil, ctx.Err()
	}
	if err := p.State.DoString(`require("ttt.rpc").connect("127.0.0.1:4250", {}, function() end)`); err != nil {
		t.Fatal(err)
	}
	p.Destroy()
	select {
	case err := <-called:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("dial after unload received %v, want canceled context", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("dial did not finish after unload")
	}
}

func connectRPCWithoutPeerReader(h *rpcHarness) {
	h.t.Helper()
	h.run(`rpc.connect("127.0.0.1:4250", {
		on_close = function(err) close_count = (close_count or 0) + 1; closed = err or "clean" end,
	}, function(c) conn = c end)`)
	h.pumpUntil(func() bool { return h.global("conn") != lua.LNil })
}

func TestRPCNotificationsKeepQueueOrder(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	connectRPCWithoutPeerReader(h)
	if err := h.peer.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	h.run(`for i = 1, 3 do assert(conn:notify("tick", { n = i })) end`)
	peer := jsonrpc.NewCodec(h.peer, h.peer)
	for _, want := range []string{`{"n":1}`, `{"n":2}`, `{"n":3}`} {
		msg, err := peer.Receive()
		if err != nil {
			t.Fatal(err)
		}
		if msg.Method != "tick" || string(msg.Params) != want {
			t.Fatalf("notification = %+v, want tick %s", msg, want)
		}
	}
}

func TestRPCNotificationPrecedesFollowingRequest(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	connectRPCWithoutPeerReader(h)
	if err := h.peer.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	h.run(`assert(conn:notify("before", {}))
		conn:request("after", {}, function() reply_done = true end)`)
	peer := jsonrpc.NewCodec(h.peer, h.peer)
	first, err := peer.Receive()
	if err != nil || first.Method != "before" || first.HasID() {
		t.Fatalf("first outbound message = %+v, %v", first, err)
	}
	second, err := peer.Receive()
	if err != nil || second.Method != "after" || !second.HasID() {
		t.Fatalf("second outbound message = %+v, %v", second, err)
	}
	if err := peer.Send(jsonrpc.Reply{JSONRPC: "2.0", ID: second.ID, Result: []byte("null")}); err != nil {
		t.Fatal(err)
	}
	h.pumpUntil(func() bool { return h.global("reply_done") == lua.LTrue })
}

func TestRPCOutboxBoundsQueuedBytesAndReleasesFrames(t *testing.T) {
	failure := make(chan error, 1)
	q := newRPCOutbox(context.Background(), rpcWriter{onError: func(err error) { failure <- err }})
	defer q.stop()
	if err := q.WriteFrame("header", make([]byte, rpcMaxMessageSize-len("header"))); err != nil {
		t.Fatal(err)
	}
	if err := q.WriteFrame("next", []byte("body")); !errors.Is(err, errRPCQueueFull) {
		t.Fatalf("full byte budget returned %v", err)
	}
	if !errors.Is(<-failure, errRPCQueueFull) {
		t.Fatal("byte overflow did not report the connection failure")
	}
	q.stop()
	if len(q.frames) != 0 || q.bytes != 0 || q.pending != 0 {
		t.Fatal("closed outbox retained queued frames")
	}
	if err := q.WriteFrame("late", nil); !errors.Is(err, errRPCClosed) {
		t.Fatal("closed outbox accepted another frame")
	}
}

func TestRPCNotificationQueueOverflowClosesConnection(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	connectRPCWithoutPeerReader(h)
	h.p.State.SetGlobal("count", lua.LNumber(rpcOutboundQueueSize+2))
	h.run(`for i = 1, count do
		local ok, err = conn:notify("blocked", {})
		if not ok then queue_err = err; break end
	end`)
	if !strings.Contains(h.global("queue_err").String(), "queue is full") {
		t.Fatalf("queue overflow error = %v", h.global("queue_err"))
	}
	h.pumpUntil(func() bool { return h.global("closed") != lua.LNil })
	if !strings.Contains(h.global("closed").String(), "queue is full") || h.global("close_count") != lua.LNumber(1) {
		t.Fatalf("queue overflow close = %v, count = %v", h.global("closed"), h.global("close_count"))
	}
	h.run(`ok, err = conn:notify("late", {})`)
	if h.global("ok") != lua.LFalse || h.global("err").String() != "connection closed" {
		t.Fatal("closed queue accepted another notification")
	}
}

type rpcFailingTransport struct {
	*rpcCloseProbe
	mode      string
	deadlines chan time.Time
}

func (c *rpcFailingTransport) SetWriteDeadline(deadline time.Time) error {
	c.deadlines <- deadline
	if c.mode == "deadline error" {
		return errors.New("deadline setup failed")
	}
	if c.mode == "timeout" {
		deadline = time.Now().Add(-time.Second)
	}
	return c.Conn.SetWriteDeadline(deadline)
}

func (c *rpcFailingTransport) Write(data []byte) (int, error) {
	if c.mode == "partial" {
		return len(data) / 2, nil
	}
	return c.Conn.Write(data)
}

func TestRPCWriteFailuresCloseTransport(t *testing.T) {
	for _, tc := range []struct{ action, failure, want string }{
		{"notification", "timeout", "timeout"},
		{"request", "timeout", "timeout"},
		{"reply", "timeout", "timeout"},
		{"notification", "partial", "short write"},
		{"notification", "deadline error", "deadline setup failed"},
	} {
		t.Run(tc.action+"/"+tc.failure, func(t *testing.T) {
			h := newRPCHarness(t, rpcPerms)
			var transport *rpcFailingTransport
			h.p.DialRPC = func(context.Context, string) (io.ReadWriteCloser, error) {
				local, peer := net.Pipe()
				h.peer = peer
				transport = &rpcFailingTransport{
					rpcCloseProbe: &rpcCloseProbe{Conn: local, closed: make(chan struct{})},
					mode:          tc.failure, deadlines: make(chan time.Time, 4),
				}
				return transport, nil
			}
			connectRPCWithoutPeerReader(h)
			start := time.Now()
			switch tc.action {
			case "notification":
				h.run(`assert(conn:notify("blocked", {}))`)
			case "request":
				h.run(`conn:request("blocked", {}, function() end)`)
			case "reply":
				peer := jsonrpc.NewCodec(h.peer, h.peer)
				if err := peer.Send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "unknown"}); err != nil {
					t.Fatal(err)
				}
			}
			h.pumpUntil(func() bool { return h.global("closed") != lua.LNil })
			if !strings.Contains(h.global("closed").String(), tc.want) || h.global("close_count") != lua.LNumber(1) {
				t.Fatalf("write failure close = %v, count = %v", h.global("closed"), h.global("close_count"))
			}
			select {
			case <-transport.closed:
			default:
				t.Fatal("failed write did not close the transport")
			}
			select {
			case deadline := <-transport.deadlines:
				if !deadline.After(start) || deadline.After(time.Now().Add(rpcWriteTimeout)) {
					t.Fatalf("unexpected write deadline: %v", deadline)
				}
			default:
				t.Fatal("socket write did not set a deadline")
			}
		})
	}
}

func TestRPCWriterTimeoutWithoutSocketDeadline(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	failure := make(chan error, 1)
	var once sync.Once
	writer := rpcWriter{
		transport: struct{ io.ReadWriteCloser }{local}, timeout: 20 * time.Millisecond,
		onError: func(err error) { once.Do(func() { failure <- err; local.Close() }) },
	}
	done := make(chan error, 1)
	go func() { _, err := writer.Write([]byte("blocked")); done <- err }()
	select {
	case err := <-done:
		if err == nil || !errors.Is(<-failure, errRPCWriteTimeout) {
			t.Fatalf("deadline-less write returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("write timeout did not close a transport without socket deadlines")
	}
}

func TestRPCNotifyDoesNotBlockLuaOnNonReadingPeer(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	connectRPCWithoutPeerReader(h)
	done := make(chan error, 1)
	go func() { done <- h.p.State.DoString(`notify_ok, notify_err = conn:notify("blocked", {})`) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
		if h.global("notify_ok") != lua.LTrue {
			t.Fatalf("notification rejected: %v", h.global("notify_err"))
		}
	case <-time.After(time.Second):
		h.peer.Close()
		<-done
		t.Fatal("notification blocked the Lua thread while the peer was not reading")
	}
}

type rpcCloseProbe struct {
	net.Conn
	closed chan struct{}
	once   sync.Once
}

func (c *rpcCloseProbe) Close() error {
	c.once.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

func TestRPCReadLoopExitClosesTransport(t *testing.T) {
	for _, reason := range []string{"malformed", "oversized", "EOF"} {
		t.Run(reason, func(t *testing.T) {
			h := newRPCHarness(t, rpcPerms)
			var transport *rpcCloseProbe
			h.p.DialRPC = func(context.Context, string) (io.ReadWriteCloser, error) {
				local, peer := net.Pipe()
				h.peer = peer
				transport = &rpcCloseProbe{Conn: local, closed: make(chan struct{})}
				return transport, nil
			}
			connectRPCWithoutPeerReader(h)
			switch reason {
			case "malformed":
				_, _ = h.peer.Write([]byte("Content-Length: invalid\r\n"))
			case "oversized":
				_, _ = h.peer.Write([]byte("Content-Length: 1000000000\r\n"))
			case "EOF":
				h.peer.Close()
			}
			h.pumpUntil(func() bool { return h.global("closed") != lua.LNil })
			select {
			case <-transport.closed:
			default:
				t.Fatal("read loop unregistered the connection without closing its transport")
			}
		})
	}
}
