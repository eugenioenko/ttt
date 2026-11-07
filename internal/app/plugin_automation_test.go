package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/plugin"
	lua "github.com/yuin/gopher-lua"
)

func automationTestPlugin(t *testing.T, a *App) *plugin.Plugin {
	t.Helper()
	p := &plugin.Plugin{Enabled: true, State: lua.NewState()}
	t.Cleanup(func() { p.State.Close() })
	p.Editor = NewPluginEditorAPI(a)
	return p
}

func TestPluginAutomationAcknowledgesPasteAndUndo(t *testing.T) {
	a := newListenTestApp(t)
	p := automationTestPlugin(t, a)
	if _, err := a.runPluginAutomation(context.Background(), p, "paste", "héllo\n世界"); err != nil {
		t.Fatal(err)
	}
	raw, err := a.runPluginAutomation(context.Background(), p, "buffer", "")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.Text != "héllo\n世界" {
		t.Fatalf("buffer=%q", result.Text)
	}
	if _, err := a.runPluginAutomation(context.Background(), p, "command", "editor.undo"); err != nil {
		t.Fatal(err)
	}
	raw, err = a.runPluginAutomation(context.Background(), p, "buffer", "")
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(raw, &result)
	if result.Text != "" {
		t.Fatalf("undo left %q", result.Text)
	}
}

func TestPluginAutomationCannotActDuringApproval(t *testing.T) {
	a := newListenTestApp(t)
	p := automationTestPlugin(t, a)
	if err := runExecMain(a, func() error { a.PendingPluginApprovals = []*plugin.Plugin{{Name: "pending"}}; return nil }); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"type", "paste", "command", "rclick", "hover"} {
		args := "hello"
		if action == "rclick" || action == "hover" {
			args = "2 2"
		}
		if _, err := a.runPluginAutomation(context.Background(), p, action, args); err == nil || !strings.Contains(err.Error(), "permissions are pending") {
			t.Fatalf("%s: %v", action, err)
		}
	}
}
