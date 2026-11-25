package app

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/jsonrpc"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func inspectionTestPeer(t *testing.T) (*App, *debugRun, *jsonrpc.Codec, chan func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	s, err := dap.Connect(ctx, listener.Addr().String(), nil, dap.SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close(false) })
	peer, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = peer.Close() })
	deadline, _ := ctx.Deadline()
	if err := peer.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	queued := make(chan func(), 1)
	r := &debugRun{ctx: ctx, session: s}
	a := &App{}
	a.debugger.run = r
	a.tasks.post = func(fn func()) error { queued <- fn; return nil }
	return a, r, jsonrpc.NewCodec(peer, peer), queued
}

func TestQueuedVariableResultExpiresOnResume(t *testing.T) {
	a, r, codec, queued := inspectionTestPeer(t)
	applied := false
	debugQuery(a, r, "variables", map[string]int{"variablesReference": 42}, func(_ struct{}) { applied = true })
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
	case <-r.ctx.Done():
		t.Fatal(r.ctx.Err())
	}
}

func TestStepPublishesStackAndExpandedVariablesTogether(t *testing.T) {
	a, r, codec, queued := inspectionTestPeer(t)
	p := &a.debugger.panel
	old := &widgets.TreeNode{ID: `scope:"Locals"/"obj"`, Expanded: true, Expandable: true}
	p.frames = []debugFrame{{ID: 1}}
	p.scopes = []*widgets.TreeNode{{Label: "previous locals"}}
	p.values = map[*widgets.TreeNode]debugVariable{old: {VariablesReference: 55}}
	r.stepping = true
	a.onDebugEvent(r, "probe", dap.Event{Name: "continued"})
	a.onDebugEvent(r, "probe", dap.Event{Name: "stopped", Body: json.RawMessage(`{"threadId":1}`)})
	reply := func(command, body string) {
		t.Helper()
		if len(p.frames) != 1 || p.frames[0].ID != 1 || len(p.scopes) != 1 || p.scopes[0].Label != "previous locals" || !p.stale {
			t.Fatalf("inspection changed before %s response", command)
		}
		var req struct {
			Seq     int
			Command string
		}
		if err := codec.ReceiveInto(&req); err != nil {
			t.Fatal(err)
		}
		if req.Command != command {
			t.Fatalf("wanted %s, got %s", command, req.Command)
		}
		if err := codec.Send(map[string]any{"type": "response", "success": true, "request_seq": req.Seq, "body": json.RawMessage(body)}); err != nil {
			t.Fatal(err)
		}
		select {
		case apply := <-queued:
			apply()
		case <-r.ctx.Done():
			t.Fatal(r.ctx.Err())
		}
	}
	reply("threads", `{"threads":[{"id":1,"name":"main"}]}`)
	reply("stackTrace", `{"stackFrames":[{"id":2,"line":7}]}`)
	reply("scopes", `{"scopes":[{"name":"Locals","variablesReference":100}]}`)
	reply("variables", `{"variables":[{"name":"obj","variablesReference":101}]}`)
	reply("variables", `{"variables":[{"name":"value","value":"42"}]}`)
	if p.stale || len(p.frames) != 1 || p.frames[0].ID != 2 || len(p.scopes) != 1 || len(p.scopes[0].Children) != 1 {
		t.Fatal("new stack and locals missing after final reply")
	}
	obj := p.scopes[0].Children[0]
	if !obj.Expanded || len(obj.Children) != 1 || obj.Children[0].Label != "value = 42" {
		t.Fatalf("expanded object: %+v", obj)
	}
	if _, ok := p.values[old]; ok {
		t.Fatal("previous stop's variable reference survived")
	}
}
