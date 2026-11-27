package app

import (
	"context"
	"encoding/json"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/config/debugadapter"
	"net"
	"os"
	"path/filepath"
	"reflect"
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
	client, peer := net.Pipe()
	session := dap.NewSession(client, nil)
	t.Cleanup(session.Close)
	t.Cleanup(func() { _ = peer.Close() })
	deadline, _ := ctx.Deadline()
	if err := peer.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	queued := make(chan func(), 1)
	r := &debugRun{ctx: ctx, session: session}
	a := &App{}
	a.debugger.run = r
	a.tasks.post = func(fn func()) error { queued <- fn; return nil }
	return a, r, jsonrpc.NewCodec(peer, peer), queued
}

func TestQueuedVariableResultExpiresOnResume(t *testing.T) {
	a, r, codec, queued := inspectionTestPeer(t)
	applied := false
	debugQuery(a, r, "variables", map[string]int{"variablesReference": 42}, func(_ struct{}, _ error) { applied = true })
	var req struct{ Seq int }
	if err := codec.ReceiveInto(&req); err != nil {
		t.Fatal(err)
	}
	if err := codec.Send(map[string]any{"type": "response", "success": true, "request_seq": req.Seq, "body": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	select {
	case apply := <-queued:
		a.debugResumed(r)
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
	a.debugResumed(r)
	a.showDebugStop(r, 1, []debugFrame{{ID: 2, Line: 7}})
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

func TestSteppingSelectsFrameProvidingLocals(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	_, r, codec, queued := inspectionTestPeer(t)
	a.debugger.run = r
	a.tasks.post = func(fn func()) error { queued <- fn; return nil }
	a.ensureDebugPanel()
	frames := []debugFrame{{ID: 0, Name: "callee"}, {ID: 42, Name: "caller"}}
	p := &a.debugger.panel
	check := func(id int, label string) {
		t.Helper()
		var req struct {
			Seq       int
			Command   string
			Arguments struct{ FrameID int }
		}
		if err := codec.ReceiveInto(&req); err != nil {
			t.Fatal(err)
		}
		if req.Command != "scopes" || req.Arguments.FrameID != id {
			t.Fatal(req)
		}
		if err := codec.Send(map[string]any{"type": "response", "success": true, "request_seq": req.Seq, "body": map[string]any{"scopes": []any{map[string]string{"name": label}}}}); err != nil {
			t.Fatal(err)
		}
		select {
		case apply := <-queued:
			apply()
		case <-r.ctx.Done():
			t.Fatal(r.ctx.Err())
		}
		if p.frameNodes[p.stack.Selected()].ID != id || len(p.scopes) != 1 || p.scopes[0].Label != label || p.stack.Selected() == nil {
			t.Fatalf("selection and locals diverged for %s", label)
		}
	}
	a.showDebugStop(r, 1, frames)
	check(0, "callee locals")
	p.stack.SelectByID("frame:42")
	p.stack.ActivateSelected()
	check(42, "caller locals")
	a.selectDebugThread(2)
	var trace struct {
		Seq     int
		Command string
	}
	if err := codec.ReceiveInto(&trace); err != nil || trace.Command != "stackTrace" {
		t.Fatalf("stack request: %+v %v", trace, err)
	}
	if err := codec.Send(map[string]any{"type": "response", "request_seq": trace.Seq, "success": false, "message": "thread exited"}); err != nil {
		t.Fatal(err)
	}
	select {
	case apply := <-queued:
		apply()
	case <-r.ctx.Done():
		t.Fatal(r.ctx.Err())
	}
	if *r.thread != 1 || p.frameID != 42 || p.stale || p.scopes[0].Label != "caller locals" {
		t.Fatal("failed thread selection replaced the previous inspection")
	}
	r.stepping = true
	a.debugResumed(r)
	a.showDebugStop(r, 1, frames)
	check(0, "new callee locals")
}

func TestDebugFileOutsideWorkspaceKeepsOnlyItsBreakpoints(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Debug.Adapters["fixture"] = debugadapter.Config{Languages: []string{"c"}, FileLaunch: map[string]any{"program": "${file}"}}
	a := buildTestApp(t, settings)
	file := filepath.Join(t.TempDir(), "outside.c")
	if err := os.WriteFile(file, []byte("int main(void) {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.EditorGroup.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	target, ok := a.currentFileDebugTarget()
	if !ok || a.Workspace.FolderForFile(file) != nil || target.File != file {
		t.Fatal(target, ok)
	}
	run := debugRun{file: target.File, folder: target.Folder, languages: []string{"c"}, points: make(chan map[string][]int, 1)}
	a.debugger.points = map[string]map[int]*debugBreakpoint{file: {1: {}}, filepath.Join(target.Folder, "other.c"): {3: {}}}
	a.queueDebugPoints(&run)
	if got := <-run.points; !reflect.DeepEqual(got, map[string][]int{file: {1}}) {
		t.Fatal(got)
	}
}

func TestContinueSelectsAnotherPausedPeer(t *testing.T) {
	a, root, _, _ := inspectionTestPeer(t)
	thread := 7
	root.thread = &thread
	child := &debugRun{ctx: root.ctx, thread: &thread}
	a.debugger.peers = []*debugRun{child}
	a.debugger.selected = child
	a.debugResumed(child)
	if a.activeDebugPeer() != root || child.thread != nil {
		t.Fatal("resuming child lost the paused root")
	}
	child.thread = &thread
	a.debugResumed(root)
	if a.activeDebugPeer() != child || root.thread != nil {
		t.Fatal("resuming root lost the paused child")
	}
}
