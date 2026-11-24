package providers

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/config/debugadapter"
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
	write(".vscode", `{"compounds":[],"inputs":[],"configurations":[{"name":"fallback","type":"go","request":"launch","program":"cmd/server"}]}`)
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
	provider, err := For(targets[0], map[string]debugadapter.Config{"go": {Provider: "delve"}})
	if err != nil {
		t.Fatal(err)
	}
	args, err := provider.Arguments(targets[0])
	if err != nil || args["program"] != filepath.Join(dir, "cmd", "worker") || args["args"].([]any)[0] != dir {
		t.Fatalf("arguments: %v, %v", args, err)
	}
	write(".ttt", `{"configurations":[],"compounds":[]}`)
	if _, err := dap.LoadTargets(dir); err != nil {
		t.Fatal(err)
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
		{"node", "launch", map[string]any{"program": "${input:entry}"}},
		{"node", "attach", map[string]any{"program": "app.js"}},
		{"node", "launch", map[string]any{"program": "app.js", "port": 9229}},
		{"node", "attach", map[string]any{"port": 65536}},
		{"node", "attach", map[string]any{"processId": 23, "address": "remote"}},
		{"debugpy", "launch", map[string]any{"program": "${file}"}},
		{"go", "launch", map[string]any{"program": ".", "preLaunchTask": "compile"}},
	} {
		t.Run(tc.adapter+"/"+tc.request, func(t *testing.T) {
			target := dap.Target{Type: tc.adapter, Request: tc.request, Arguments: tc.args}
			provider, err := For(target, map[string]debugadapter.Config{tc.adapter: {Provider: map[string]string{"node": "jsdebug", "debugpy": "debugpy", "go": "delve"}[tc.adapter]}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := provider.Arguments(target); err == nil {
				t.Fatalf("unexpectedly accepted %v", tc.args)
			}
		})
	}
}

func TestConfiguredAdapterUsesExplicitProvider(t *testing.T) {
	target := dap.Target{Type: "go", Request: "launch", Folder: "/project", Arguments: map[string]any{"custom": []any{"${workspaceFolder}/src", "${adapterVariable}"}}}
	var missing MissingAdapter
	if _, err := For(target, nil); !errors.As(err, &missing) || missing.Type != "go" {
		t.Fatalf("missing adapter: %v", err)
	}
	provider, err := For(target, map[string]debugadapter.Config{"go": {Command: []string{"custom"}}})
	if err != nil {
		t.Fatal(err)
	}
	args, err := provider.Arguments(target)
	if err != nil {
		t.Fatal(err)
	}
	if want := []any{"/project/src", "${adapterVariable}"}; !reflect.DeepEqual(args["custom"], want) {
		t.Fatalf("custom=%v", args["custom"])
	}
	for _, bad := range []map[string]any{{"preLaunchTask": "build"}, {"custom": []any{map[string]any{"value": "${input:id}"}}}} {
		target.Arguments = bad
		if _, err := provider.Arguments(target); err == nil {
			t.Fatalf("accepted %v", bad)
		}
	}
}
