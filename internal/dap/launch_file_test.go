package dap

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/config/debugadapter"
)

func TestRankAdapters(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"go.mod", "App.csproj"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	adapters := map[string]debugadapter.Config{
		"gdb":        {Languages: []string{"c"}},
		"go":         {Languages: []string{"go"}, Markers: []string{"go.mod"}},
		"netcoredbg": {Markers: []string{"*.csproj"}},
		"node":       {Languages: []string{"javascript"}, Markers: []string{"package.json"}},
		"pwa-node":   {Languages: []string{"javascript"}},
	}
	tests := []struct {
		language string
		want     []string
		detected int
	}{
		{"", []string{"go", "netcoredbg", "gdb", "node", "pwa-node"}, 2},
		{"javascript", []string{"node", "pwa-node", "go", "netcoredbg", "gdb"}, 4},
		{"c", []string{"gdb", "go", "netcoredbg", "node", "pwa-node"}, 3},
	}
	for _, tt := range tests {
		names, detected := RankAdapters(adapters, root, tt.language)
		if !reflect.DeepEqual(names, tt.want) || detected != tt.detected {
			t.Errorf("language %q: got %v, %d; want %v, %d", tt.language, names, detected, tt.want, tt.detected)
		}
	}
}

func TestCreateLaunchFileWritesAdapterTemplates(t *testing.T) {
	root := t.TempDir()
	a := Adapter{
		FileLaunch: map[string]any{"program": "${file}"},
		Configurations: []map[string]any{
			{"name": "Launch package", "request": "launch", "program": "${workspaceFolder}"},
			{"type": "other", "request": "attach", "processId": "${command:pickProcess}"},
		},
	}
	if _, err := CreateLaunchFile(root, filepath.Join(root, "main.go"), "go", a); err != nil {
		t.Fatal(err)
	}
	targets, err := LoadTargets(root)
	if err != nil {
		t.Fatal(err)
	}
	got := make([][3]string, len(targets))
	for i, target := range targets {
		got[i] = [3]string{target.Name, target.Type, target.Request}
	}
	want := [][3]string{{"Launch package", "go", "launch"}, {"go", "go", "attach"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
	if targets[0].Arguments["program"] != "${workspaceFolder}" {
		t.Errorf("template placeholder expanded: %v", targets[0].Arguments["program"])
	}
	if _, ok := a.Configurations[1]["name"]; ok {
		t.Error("template in settings was modified")
	}
}

func TestCreateLaunchFileWithoutRecipeWritesStarter(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateLaunchFile(root, "", "gdb", Adapter{}); err != nil {
		t.Fatal(err)
	}
	targets, err := LoadTargets(root)
	if err != nil || len(targets) != 1 {
		t.Fatalf("targets = %v, %v", targets, err)
	}
	if got := targets[0]; got.Name != "gdb" || got.Type != "gdb" || got.Request != "launch" {
		t.Errorf("starter = %+v", got)
	}
}
