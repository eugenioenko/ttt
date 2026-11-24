package dap

import (
	"os"
	"path/filepath"
	"testing"
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
	targets, err := LoadTargets(dir)
	if err != nil || len(targets) != 1 || targets[0].Name != "fallback" {
		t.Fatalf("fallback: %v, %v", targets, err)
	}
	write(".ttt", `{// named targets
 "configurations":[{"name":"worker","type":"go","request":"launch","program":"${workspaceFolder}/cmd/worker","args":["${workspaceFolder}"],},],}`)
	targets, err = LoadTargets(dir)
	if err != nil || len(targets) != 1 || targets[0].Name != "worker" {
		t.Fatalf("native: %v, %v", targets, err)
	}
	args, err := targets[0].LaunchArguments()
	if err != nil || args["program"] != filepath.Join(dir, "cmd", "worker") || args["args"].([]any)[0] != dir {
		t.Fatalf("arguments: %v, %v", args, err)
	}
	write(".ttt", "{")
	if _, err := LoadTargets(dir); err == nil {
		t.Fatal("malformed native file fell back silently")
	}
}
