package plugin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func TestAutomationChecksActionPermissions(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{})
	defer cleanup()
	for _, action := range []string{"type", "paste", "screenshot", "debug", "session", "buffer", "commands", "command", "clipboard"} {
		err := p.State.DoString(`require("ttt.automation").run("` + action + `", "", function() end)`)
		if err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("%s: %v", action, err)
		}
	}
}

func TestAutomationCancellationAndUnload(t *testing.T) {
	for _, unload := range []bool{false, true} {
		p, _ := newTestPluginBase(PermissionSet{Input: true})
		posted := make(chan *PluginAsyncResult, 1)
		p.SetPostAsync(func(r *PluginAsyncResult) { posted <- r })
		started, cancelled := make(chan struct{}), make(chan struct{})
		p.RunAutomation = func(ctx context.Context, _, _ string) (json.RawMessage, error) {
			close(started)
			<-ctx.Done()
			close(cancelled)
			return nil, ctx.Err()
		}
		if err := p.State.DoString(`cancel = require("ttt.automation").run("wait", "30000", function(result, err) failure = err end)`); err != nil {
			t.Fatal(err)
		}
		<-started
		if unload {
			p.Destroy()
		} else if err := p.State.DoString(`cancel()`); err != nil {
			t.Fatal(err)
		}
		select {
		case <-cancelled:
		case <-time.After(time.Second):
			t.Fatal("automation did not cancel")
		}
		if !unload {
			select {
			case r := <-posted:
				r.Callback()
			case <-time.After(time.Second):
				t.Fatal("cancellation callback not posted")
			}
			if !strings.Contains(p.State.GetGlobal("failure").String(), context.Canceled.Error()) {
				t.Fatal("missing cancellation error")
			}
			p.Destroy()
		}
	}
}

func TestAutomationPostsResultOnMainThread(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{Input: true})
	defer cleanup()
	posted := make(chan *PluginAsyncResult, 1)
	p.PostAsync = func(r *PluginAsyncResult) { posted <- r }
	p.RunAutomation = func(_ context.Context, action, args string) (json.RawMessage, error) {
		if action != "type" || args != "世界" {
			t.Errorf("action=%q args=%q", action, args)
		}
		return json.RawMessage(`{"ok":true}`), nil
	}
	if err := p.State.DoString(`require("ttt.automation").run("type", "世界", function(result, err) done = result.ok; failure = err end)`); err != nil {
		t.Fatal(err)
	}
	if p.State.GetGlobal("done") != lua.LNil {
		t.Fatal("callback ran before main-thread delivery")
	}
	select {
	case r := <-posted:
		r.Callback()
	case <-time.After(time.Second):
		t.Fatal("callback not posted")
	}
	if p.State.GetGlobal("done") != lua.LTrue || p.State.GetGlobal("failure") != lua.LNil {
		t.Fatal("callback did not receive result")
	}
}

func TestAutomationDebugNeedsEditorAndTerminalRead(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{ScreenRead: true})
	defer cleanup()
	err := p.State.DoString(`require("ttt.automation").run("debug", "", function() end)`)
	if err == nil || !strings.Contains(err.Error(), "editor.read") {
		t.Fatalf("debug with screen.read only: %v", err)
	}
}
