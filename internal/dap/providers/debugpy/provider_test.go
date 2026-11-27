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
		{"launch", map[string]any{"module": "main", "python": "custom-python"}, "custom-python"},
	} {
		args, err := (Provider{}).Arguments(dap.Target{Folder: folder, Request: tc.request, Arguments: tc.args})
		if err != nil || args["python"] != tc.want {
			t.Fatalf("%s: python=%v err=%v", tc.request, args["python"], err)
		}
	}
}
