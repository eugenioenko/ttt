package providers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/dap"
)

func TestWorkspaceLaunchSelection(t *testing.T) {
	dir := t.TempDir()
	write := func(sub, data string) {
		t.Helper()
		path := filepath.Join(dir, sub)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "launch.json"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(".vscode", `{"configurations":[{"name":"fallback","type":"go","request":"launch","program":"cmd/server"}]}`)
	targets, err := dap.LoadTargets(dir)
	if err != nil || len(targets) != 1 || targets[0].Name != "fallback" {
		t.Fatalf("fallback: %v, %v", targets, err)
	}
	write(".ttt", `{// named targets
 "configurations":[{"name":"worker","type":"go","request":"launch","program":"${workspaceFolder}/cmd/worker","args":["${workspaceFolder}"],},],}`)
	targets, err = dap.LoadTargets(dir)
	if err != nil || len(targets) != 1 || targets[0].Name != "worker" {
		t.Fatalf("native: %v, %v", targets, err)
	}
	provider, err := For(targets[0])
	if err != nil {
		t.Fatal(err)
	}
	args, err := provider.Arguments(targets[0])
	if err != nil || args["program"] != filepath.Join(dir, "cmd", "worker") || args["args"].([]any)[0] != dir {
		t.Fatalf("arguments: %v, %v", args, err)
	}
	write(".ttt", `{"configurations":[],"compounds":[]}`)
	if _, err := dap.LoadTargets(dir); err == nil {
		t.Fatal("silently ignored unsupported compound configuration")
	}
	write(".ttt", "{")
	if _, err := dap.LoadTargets(dir); err == nil {
		t.Fatal("malformed native file fell back silently")
	}
}

func TestRejectUnsupportedTargetBeforeStartup(t *testing.T) {
	for _, tc := range []struct {
		adapter, request string
		args             map[string]any
	}{
		{"node", "attach", map[string]any{"program": "app.js"}},
		{"node", "launch", map[string]any{"program": "app.js", "port": 9229}},
		{"node", "attach", map[string]any{"port": 65536}},
		{"node", "attach", map[string]any{"processId": 23, "address": "remote"}},
		{"debugpy", "launch", map[string]any{"program": "${file}"}},
		{"go", "launch", map[string]any{"program": ".", "preLaunchTask": "compile"}},
	} {
		t.Run(tc.adapter+"/"+tc.request, func(t *testing.T) {
			target := dap.Target{Type: tc.adapter, Request: tc.request, Arguments: tc.args}
			provider, err := For(target)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := provider.Arguments(target); err == nil {
				t.Fatalf("unexpectedly accepted %v", tc.args)
			}
		})
	}
}
