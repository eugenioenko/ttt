package plugin

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/jsonrpc"
	lua "github.com/yuin/gopher-lua"
)

func TestRPCDisconnectBeforeContextCancellationDeliversCallback(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	local, peer := net.Pipe()
	defer peer.Close()
	if err := peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	c := &rpcConn{p: h.p, transport: local}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	h.p.registerRPCConn(c)
	allowClose := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(allowClose) }) }
	t.Cleanup(release)
	c.conn = jsonrpc.NewConn(local, local, jsonrpc.Options{OnClose: func(err error) {
		<-allowClose
		c.handleClose(err)
	}})
	L := h.p.State
	ud := L.NewUserData()
	ud.Value = c
	L.SetMetatable(ud, L.GetTypeMetatable(rpcConnTypeName))
	L.SetGlobal("conn", ud)
	h.run(`conn:request("pending", {}, function(_, err)
		request_err = err; callback_count = (callback_count or 0) + 1
	end)`)
	if _, err := jsonrpc.NewCodec(peer, peer).Receive(); err != nil {
		t.Fatal(err)
	}
	peer.Close()
	select {
	case event := <-h.posted:
		if c.ctx.Err() != nil {
			t.Fatal("context canceled before the request callback was queued")
		}
		release()
		select {
		case <-c.ctx.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("connection did not close")
		}
		event.Callback()
	case <-time.After(5 * time.Second):
		t.Fatal("disconnected request did not queue a callback")
	}
	if h.global("request_err") != lua.LString("connection closed") || h.global("callback_count") != lua.LNumber(1) {
		t.Fatalf("disconnect callback = %v, count = %v", h.global("request_err"), h.global("callback_count"))
	}
}

func TestRPCInboxOverflowDropsQueuedMessages(t *testing.T) {
	for _, kind := range []string{"notifications", "requests", "params bytes", "ID bytes"} {
		t.Run(kind, func(t *testing.T) {
			h := newRPCHarness(t, rpcPerms)
			h.connectPeer(jsonrpc.Options{})
			c := h.global("conn").(*lua.LUserData).Value.(*rpcConn)
			count := 257
			params := json.RawMessage(`{"n":1}`)
			id := json.RawMessage(`1`)
			if kind == "params bytes" {
				count = 2
				params = json.RawMessage(`"` + strings.Repeat("x", rpcMaxMessageSize/2) + `"`)
			}
			if kind == "ID bytes" {
				count = 2
				id = json.RawMessage(`"` + strings.Repeat("x", rpcMaxMessageSize/2) + `"`)
			}
			for range count {
				if kind == "requests" || kind == "ID bytes" {
					c.handleRequest(jsonrpc.Message{ID: id, Method: "add", Params: params}, func(any, error) {
						t.Error("discarded request was answered")
					})
				} else {
					c.handleNotification("tick", params)
				}
			}
			h.pumpUntil(func() bool { return h.global("closed") != lua.LNil })
			if !strings.Contains(h.global("closed").String(), "incoming queue is full") {
				t.Fatalf("overflow reason = %v", h.global("closed"))
			}
			if h.global("notes") != lua.LNil {
				t.Fatal("overflowed connection executed queued notifications")
			}
			c.mu.Lock()
			defer c.mu.Unlock()
			if len(c.inbox) != 0 || c.inboxCount != 0 || c.inboxBytes != 0 {
				t.Fatal("overflow retained queued callbacks or their budget")
			}
		})
	}
}

func TestRPCInboxOverflowFromPeer(t *testing.T) {
	for _, kind := range []string{"notification", "request without handler"} {
		t.Run(kind, func(t *testing.T) {
			h := newRPCHarness(t, rpcPerms)
			if kind == "notification" {
				h.connectPeer(jsonrpc.Options{})
			} else {
				connectRPCWithoutPeerReader(h)
			}
			if err := h.peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			peer := jsonrpc.NewCodec(h.peer, h.peer)
			for i := range rpcInboundQueueSize + 1 {
				message := map[string]any{"jsonrpc": "2.0", "method": "tick", "params": map[string]int{"n": 1}}
				if kind != "notification" {
					message["id"] = i
				}
				if err := peer.Send(message); err != nil && i != rpcInboundQueueSize {
					t.Fatal(err)
				}
			}
			c := h.global("conn").(*lua.LUserData).Value.(*rpcConn)
			select {
			case <-c.ctx.Done():
			case <-time.After(5 * time.Second):
				t.Fatal("peer traffic exceeded the incoming limit without closing")
			}
			h.pumpUntil(func() bool { return h.global("closed") != lua.LNil })
			if !strings.Contains(h.global("closed").String(), "incoming queue is full") {
				t.Fatalf("peer overflow close = %v", h.global("closed"))
			}
		})
	}
}

func TestRPCInboxCountsActiveCallbacksAndReleasesBudget(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	h.connectPeer(jsonrpc.Options{})
	c := h.global("conn").(*lua.LUserData).Value.(*rpcConn)
	for range 2 {
		for range rpcInboundQueueSize {
			c.handleNotification("tick", json.RawMessage(`{"n":1}`))
		}
		(<-h.posted).Callback()
		if c.ctx.Err() != nil {
			t.Fatal("draining did not release the incoming budget")
		}
	}
	called := false
	c.enqueueMessage(rpcMaxMessageSize/2, func() {
		c.enqueueMessage(rpcMaxMessageSize/2+1, func() { called = true })
	})
	h.pumpUntil(func() bool { return h.global("closed") != lua.LNil })
	if called || !strings.Contains(h.global("closed").String(), "incoming queue is full") {
		t.Fatal("active callback was excluded from the incoming budget")
	}
}

func TestRPCUnloadReleasesIncomingMessages(t *testing.T) {
	h := newRPCHarness(t, rpcPerms)
	h.connectPeer(jsonrpc.Options{})
	c := h.global("conn").(*lua.LUserData).Value.(*rpcConn)
	c.handleNotification("tick", json.RawMessage(`{"n":1}`))
	event := <-h.posted
	h.p.Destroy()
	event.Callback()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inboxCount != 0 || c.inboxBytes != 0 {
		t.Fatal("unload retained queued message budgets")
	}
	for _, callback := range c.inbox {
		if callback.size >= 0 {
			t.Fatal("unload retained a queued message")
		}
	}
}

func TestRPCFullOutboxRejectsRequestAndClosesOnLostReply(t *testing.T) {
	for _, action := range []string{"request", "reply"} {
		t.Run(action, func(t *testing.T) {
			h := newRPCHarness(t, rpcPerms)
			connectRPCWithoutPeerReader(h)
			if err := h.peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			h.run(`for i = 1, 64 do assert(conn:notify("fill", {})) end`)
			c := h.global("conn").(*lua.LUserData).Value.(*rpcConn)
			if action == "request" {
				h.run(`conn:request("overflow", {}, function(_, err) request_err = err end)`)
				h.pumpUntil(func() bool { return h.global("request_err") != lua.LNil })
				if !strings.Contains(h.global("request_err").String(), "queue is full") || c.ctx.Err() != nil {
					t.Fatalf("full request = %v, connection = %v", h.global("request_err"), c.ctx.Err())
				}
			} else {
				peer := jsonrpc.NewCodec(h.peer, h.peer)
				if err := peer.Send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "missing"}); err != nil {
					t.Fatal(err)
				}
				h.pumpUntil(func() bool { return h.global("closed") != lua.LNil })
				if !strings.Contains(h.global("closed").String(), "queue is full") {
					t.Fatalf("lost reply close = %v", h.global("closed"))
				}
			}
		})
	}
}
