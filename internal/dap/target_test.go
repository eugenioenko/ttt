package dap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/config/debugadapter"
)

func TestAdapterLookup(t *testing.T) {
	settings := config.DefaultSettings()
	if len(settings.Debug.Adapters) != 0 {
		t.Fatal(settings.Debug)
	}
	_, err := LookupAdapter(settings.Debug.Adapters, "node")
	if err == nil || !strings.Contains(err.Error(), "node") || !strings.Contains(err.Error(), "Plugins") {
		t.Fatal(err)
	}
	for _, provider := range []string{"", "generic", "debugpy", "delve", "jsdebug", "typo"} {
		settings.Debug.Adapters["go"] = debugadapter.Config{Provider: provider}
		a, err := LookupAdapter(settings.Debug.Adapters, "go")
		if provider == "typo" {
			if err == nil {
				t.Fatal("accepted unknown provider")
			}
			continue
		}
		want := provider
		if want == "" {
			want = "generic"
		}
		if err != nil || a.Provider != want {
			t.Fatal(provider, a, err)
		}
	}
}

func TestWorkspaceTargets(t *testing.T) {
	root := t.TempDir()
	write := func(dir, text string) string {
		t.Helper()
		folder := filepath.Join(root, dir)
		if err := os.MkdirAll(folder, 0755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(folder, "launch.json")
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := write(".vscode", `{ // VS Code workspace
		"inputs": [], "compounds": [], "configurations": [
		{"name":"Custom","type":"my-tool","request":"attach","vendor":[{"path":"${workspaceFolder}/${vendor:path}","id":9007199254740993}]},
		],}`)
	got, targets, err := ReadLaunchFile(root)
	if err != nil || got != path || len(targets) != 1 {
		t.Fatal(got, targets, err)
	}
	args, err := (Adapter{}).Arguments(targets[0])
	if err != nil {
		t.Fatal(err)
	}
	vendor := args["vendor"].([]any)[0].(map[string]any)
	if vendor["path"] != root+"/${vendor:path}" || vendor["id"] != json.Number("9007199254740993") {
		t.Fatal(vendor)
	}
	path = write(".ttt", `{"configurations":[]}`)
	got, targets, err = ReadLaunchFile(root)
	if err != nil || got != path || len(targets) != 0 {
		t.Fatal(got, targets, err)
	}
	write(".ttt", "{")
	if _, _, err := ReadLaunchFile(root); err == nil {
		t.Fatal("invalid preferred file silently ignored")
	}
}

func TestRejectUnsupportedLaunchFeatures(t *testing.T) {
	for _, args := range []map[string]any{{"preLaunchTask": "build"}, {"nested": []any{"${input:choice}"}}} {
		if _, err := (Adapter{}).Arguments(Target{Request: "launch", Arguments: args}); err == nil {
			t.Fatal(args)
		}
	}
}

func TestFileTargetOverridesLeaveBaseRecipeIntact(t *testing.T) {
	a := Adapter{FileLaunch: map[string]any{"mode": "debug"}, FileLaunchOverrides: []debugadapter.FileLaunchOverride{
		{Pattern: "*_test.go", Arguments: map[string]any{"mode": "test"}},
		{Pattern: "*.go", Arguments: map[string]any{"mode": "fallback"}},
	}}
	target, ok := a.FileTarget("/work", "/work/sample_test.go", "go")
	if !ok || target.Arguments["mode"] != "test" || a.FileLaunch["mode"] != "debug" {
		t.Fatal(target, a)
	}
}
