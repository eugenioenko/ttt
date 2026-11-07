package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/jsonrpc"
	lua "github.com/yuin/gopher-lua"
)

type rpcHarness struct {
	t      *testing.T
	p      *Plugin
	posted chan *PluginAsyncResult
	peer   net.Conn
}

func newRPCHarness(t *testing.T, perms PermissionSet) *rpcHarness {
	t.Helper()
	p, _ := newTestPluginBase(perms)
	h := &rpcHarness{t: t, p: p, posted: make(chan *PluginAsyncResult, 64)}
	p.PostAsync = func(r *PluginAsyncResult) { h.posted <- r }
	p.DialRPC = func(_ context.Context, endpoint string) (io.ReadWriteCloser, error) {
		if endpoint == "127.0.0.1:1" {
			return nil, errors.New("connection refused")
		}
		local, peer := net.Pipe()
		h.peer = peer
		return local, nil
	}
	t.Cleanup(p.Destroy)
	return h
}

func (h *rpcHarness) run(code string) {
	h.t.Helper()
	if err := h.p.State.DoString(`local ttt = require("ttt")
local rpc = require("ttt.rpc")
` + code); err != nil {
		h.t.Fatal(err)
	}
}

func (h *rpcHarness) global(name string) lua.LValue {
	return h.p.State.GetGlobal(name)
}

// pumpUntil runs posted callbacks on the test goroutine, standing in for the
// event loop, until done reports true.
func (h *rpcHarness) pumpUntil(done func() bool) {
	h.t.Helper()
	deadline := time.After(5 * time.Second)
	for !done() {
		select {
		case r := <-h.posted:
			r.Callback()
		case <-deadline:
			h.t.Fatal("timed out waiting for rpc callbacks")
		}
	}
}

func (h *rpcHarness) connectPeer(opts jsonrpc.Options) *jsonrpc.Conn {
	h.t.Helper()
	h.run(`
		rpc.connect("127.0.0.1:4250", {
			on_request = function(method, params)
				if method == "fail" then error("nope") end
				return { method = method, sum = params[1] + params[2] }
			end,
			on_notification = function(method, params)
				notes = (notes or "") .. method .. "=" .. tostring(params.n) .. ";"
			end,
			on_close = function(err) closed = err or "clean" end,
		}, function(c, err)
			conn, connect_err = c, err
		end)
	`)
	h.pumpUntil(func() bool { return h.global("conn") != lua.LNil })
	return jsonrpc.NewConn(h.peer, h.peer, opts)
}

var rpcPerms = PermissionSet{NetworkRPC: NetworkRPC{Endpoints: []string{"127.0.0.1:4250"}}}

func TestRPCPipelinedRequestsAndNotificationsKeepWireOrder(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	h := newRPCHarness(t, rpcPerms)
	h.run(`
		order = ""
		rpc.connect("127.0.0.1:4250", {
			on_request = function(method, params)
				order = order .. method .. ";"
				return params
			end,
			on_notification = function(method)
				order = order .. method .. ";"
			end,
		}, function(c) conn = c end)
	`)
	h.pumpUntil(func() bool { return h.global("conn") != lua.LNil })
	if err := h.peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var batch bytes.Buffer
	writer := jsonrpc.NewCodec(nil, &batch)
	for i, method := range []string{"request-a", "note-a", "request-b", "note-b", "request-c", "note-c"} {
		message := map[string]any{"jsonrpc": "2.0", "method": method}
		if i%2 == 0 {
			message["id"] = method
			message["params"] = method
		}
		if err := writer.Send(message); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.peer.Write(batch.Bytes()); err != nil {
		t.Fatal(err)
	}
	h.pumpUntil(func() bool { return strings.Count(h.global("order").String(), ";") == 6 })
	if got := h.global("order").String(); got != "request-a;note-a;request-b;note-b;request-c;note-c;" {
		t.Fatalf("peer handlers ran out of wire order: %q", got)
	}
	peer := jsonrpc.NewCodec(h.peer, h.peer)
	for range 3 {
		reply, err := peer.Receive()
		if err != nil {
			t.Fatal(err)
		}
		if reply.Error != nil || !bytes.Equal(reply.ID, reply.Result) {
			t.Fatalf("reply lost its request ID or result: %+v", reply)
		}
	}
}

func TestRPCQueuedRequestAfterDestroyDoesNotCallLua(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	called := false
	h.p.State.SetGlobal("record", h.p.State.NewFunction(func(*lua.LState) int {
		called = true
		return 0
	}))
	h.run(`rpc.connect("127.0.0.1:4250", {
		on_request = function() record(); return true end,
	}, function(c) conn = c end)`)
	h.pumpUntil(func() bool { return h.global("conn") != lua.LNil })
	peer := jsonrpc.NewCodec(h.peer, h.peer)
	if err := peer.Send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "queued"}); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-h.posted:
		h.p.Destroy()
		event.Callback()
	case <-time.After(5 * time.Second):
		t.Fatal("request was not queued for UI delivery")
	}
	if called {
		t.Fatal("queued request called Lua after plugin destruction")
	}
}

func TestRPCWithoutRequestHandlerRepliesMethodNotFound(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	h.run(`rpc.connect("127.0.0.1:4250", {}, function(c) conn = c end)`)
	h.pumpUntil(func() bool { return h.global("conn") != lua.LNil })
	if err := h.peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	peer := jsonrpc.NewCodec(h.peer, h.peer)
	if err := peer.Send(map[string]any{"jsonrpc": "2.0", "id": "missing", "method": "unknown"}); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-h.posted:
		event.Callback()
	case <-time.After(5 * time.Second):
		t.Fatal("request was not queued")
	}
	reply, err := peer.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if string(reply.ID) != `"missing"` || reply.Error == nil || reply.Error.Code != jsonrpc.CodeMethodNotFound {
		t.Fatalf("missing handler reply = %+v", reply)
	}
}

func TestRPCModuleRequiresPermission(t *testing.T) {
	h := newRPCHarness(t, PermissionSet{})
	h.run(`has_connect = rpc.connect ~= nil`)
	if h.global("has_connect") != lua.LFalse {
		t.Fatal("rpc.connect exposed without network.rpc")
	}
}

func TestRPCConnectRejectsUnlistedEndpoint(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	err := h.p.State.DoString(`require("ttt.rpc").connect("127.0.0.1:4251", {}, function() end)`)
	if err == nil || !strings.Contains(err.Error(), "network.rpc does not list") {
		t.Fatalf("error = %v", err)
	}
}

func TestRPCConnectReportsDialError(t *testing.T) {
	h := newRPCHarness(t, PermissionSet{NetworkRPC: NetworkRPC{Endpoints: []string{"127.0.0.1:1"}}})
	h.run(`rpc.connect("127.0.0.1:1", {}, function(c, err) got_conn, dial_err = c, err end)`)
	h.pumpUntil(func() bool { return h.global("dial_err") != lua.LNil })
	if h.global("got_conn") != lua.LNil || !strings.Contains(h.global("dial_err").String(), "refused") {
		t.Fatalf("conn = %v, err = %v", h.global("got_conn"), h.global("dial_err"))
	}
}

func TestRPCPluginCallsPeer(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	h.connectPeer(jsonrpc.Options{OnRequest: func(method string, params json.RawMessage) (any, error) {
		if method == "boom" {
			return nil, &jsonrpc.Error{Code: 1, Message: "peer failed"}
		}
		var p struct{ Name string }
		_ = json.Unmarshal(params, &p)
		return map[string]any{"greeting": "hi " + p.Name}, nil
	}})
	h.run(`
		conn:request("greet", { name = "ttt" }, function(result, err) greeting = result.greeting end)
		conn:request("boom", nil, function(result, err) boom_err = err end)
	`)
	h.pumpUntil(func() bool { return h.global("greeting") != lua.LNil && h.global("boom_err") != lua.LNil })
	if h.global("greeting").String() != "hi ttt" || h.global("boom_err").String() != "peer failed" {
		t.Fatalf("greeting = %v, boom_err = %v", h.global("greeting"), h.global("boom_err"))
	}
}

func TestRPCPeerCallsPluginAndNotifiesInOrder(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	peer := h.connectPeer(jsonrpc.Options{})
	for i := 1; i <= 3; i++ {
		if err := peer.Notify("tick", map[string]int{"n": i}); err != nil {
			t.Fatal(err)
		}
	}
	type result struct {
		raw json.RawMessage
		err error
	}
	results := make(chan result, 2)
	wake := &PluginAsyncResult{Callback: func() {}}
	go func() {
		raw, err := peer.Call(context.Background(), "add", []int{2, 3})
		results <- result{raw, err}
		h.posted <- wake
		raw, err = peer.Call(context.Background(), "fail", nil)
		results <- result{raw, err}
		h.posted <- wake
	}()
	var add, fail result
	h.pumpUntil(func() bool {
		select {
		case r := <-results:
			if add.raw == nil && add.err == nil {
				add = r
			} else {
				fail = r
			}
		default:
		}
		return fail.err != nil || fail.raw != nil
	})
	if string(add.raw) != `{"method":"add","sum":5}` {
		t.Fatalf("add = %s, %v", add.raw, add.err)
	}
	var rpcErr *jsonrpc.Error
	if !errors.As(fail.err, &rpcErr) || !strings.Contains(rpcErr.Message, "nope") {
		t.Fatalf("fail error = %v", fail.err)
	}
	if got := h.global("notes").String(); got != "tick=1;tick=2;tick=3;" {
		t.Fatalf("notifications = %q", got)
	}
}

func TestRPCCloseCallbacks(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	h.connectPeer(jsonrpc.Options{})
	h.peer.Close()
	h.pumpUntil(func() bool { return h.global("closed") != lua.LNil })
	if got := h.global("closed").String(); got != "clean" {
		t.Fatalf("peer hang-up reported %q", got)
	}

	h2 := newRPCHarness(t, rpcPerms)
	h2.connectPeer(jsonrpc.Options{})
	h2.run(`conn:close()`)
	h2.pumpUntil(func() bool { return h2.global("closed") != lua.LNil })
	h2.run(`ok, notify_err = conn:notify("late", {})`)
	if h2.global("ok") != lua.LFalse || h2.global("notify_err").String() != "connection closed" {
		t.Fatalf("notify after close = %v, %v", h2.global("ok"), h2.global("notify_err"))
	}
}

func TestRPCDestroyClosesConnections(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	peer := h.connectPeer(jsonrpc.Options{})
	h.p.mu.Lock()
	h.p.closeRPCConnsLocked()
	h.p.mu.Unlock()
	select {
	case <-peer.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("peer still connected after the plugin closed its connections")
	}
}

func TestRPCOversizedPeerMessageClosesConnection(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	h.connectPeer(jsonrpc.Options{})
	go func() { _, _ = h.peer.Write([]byte("Content-Length: 1000000000\r\n\r\n")) }()
	h.pumpUntil(func() bool { return h.global("closed") != lua.LNil })
	if got := h.global("closed").String(); !strings.Contains(got, "exceeds") {
		t.Fatalf("on_close reason = %q", got)
	}
}
