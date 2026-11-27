package debugpy

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/eugenioenko/ttt/internal/dap"
)

func TestWorkspaceInterpreterAndExplicitOverride(t *testing.T) {
	folder := t.TempDir()
	interpreter := filepath.Join(folder, ".venv", "bin", "python")
	if runtime.GOOS == "windows" {
		interpreter = filepath.Join(folder, ".venv", "Scripts", "python.exe")
	}
	if err := os.MkdirAll(filepath.Dir(interpreter), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(interpreter, nil, 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		request string
		args    map[string]any
		want    string
	}{
		{"launch", map[string]any{"program": "main.py"}, interpreter},
		{"attach", map[string]any{"processId": 42}, interpreter},
		{"attach", map[string]any{"processId": 42, "python": "tools/python"}, filepath.Join(folder, "tools", "python")},
		{"launch", map[string]any{"module": "main", "python": "custom-python"}, "custom-python"},
	} {
		args, err := (Provider{}).Arguments(dap.Target{Folder: folder, Request: tc.request, Arguments: tc.args})
		if err != nil || args["python"] != tc.want {
			t.Fatalf("%s: python=%v err=%v", tc.request, args["python"], err)
		}
	}
}

func TestConsoleRedirect(t *testing.T) {
	enabled, disabled := true, false
	for _, tc := range []struct {
		console  string
		override *bool
		want     bool
	}{
		{"internalConsole", nil, true}, {"integratedTerminal", nil, false}, {"integratedTerminal", &enabled, true}, {"internalConsole", &disabled, false},
	} {
		args := map[string]any{"program": "main.py", "console": tc.console}
		if tc.override != nil {
			args["redirectOutput"] = *tc.override
		}
		out, err := (Provider{}).Arguments(dap.Target{Request: "launch", Folder: t.TempDir(), Arguments: args})
		if err != nil || out["redirectOutput"] != tc.want {
			t.Fatal(tc, out, err)
		}
	}
}
