package dap

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestGenericFileLaunchRecipe(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "src", "sample.xyz")
	recipe := map[string]any{"args": []any{"${file}", "--cwd=${workspaceFolder}"}, "env": map[string]any{"SOURCE_DIR": "${fileDirname}"}}
	a := Adapter{FileLaunch: recipe}
	target, ok := a.FileTarget(root, file, "custom")
	if !ok {
		t.Fatal("generic recipe unavailable")
	}
	args, err := a.Arguments(target)
	want := map[string]any{"args": []any{file, "--cwd=" + root}, "env": map[string]any{"SOURCE_DIR": filepath.Dir(file)}}
	if err != nil || !reflect.DeepEqual(args, want) {
		t.Fatalf("got %v, %v", args, err)
	}
	target.file = ""
	saved, err := a.Arguments(target)
	if err != nil || saved["args"].([]any)[0] != "${file}" {
		t.Fatal("saved target unexpectedly expanded file placeholder")
	}
}
