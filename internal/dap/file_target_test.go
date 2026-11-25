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

func TestSavedFileRecipeKeepsWorkspacePaths(t *testing.T) {
	for _, name := range []string{"main.js", "src/main.js"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, name)
			a := Adapter{FileLaunch: map[string]any{"cwd": "${fileDirname}", "args": []any{"${file}"}}}
			if _, err := CreateLaunchFile(root, file, "node", a); err != nil {
				t.Fatal(err)
			}
			targets, err := LoadTargets(root)
			if err != nil || len(targets) != 1 {
				t.Fatalf("load targets = %v, %v", targets, err)
			}
			args, err := a.Arguments(targets[0])
			if err != nil {
				t.Fatal(err)
			}
			if got := filepath.Clean(args["args"].([]any)[0].(string)); got != file {
				t.Errorf("file argument = %q, want %q", got, file)
			}
			if got := filepath.Clean(args["cwd"].(string)); got != filepath.Dir(file) {
				t.Errorf("cwd = %q, want %q", got, filepath.Dir(file))
			}
		})
	}
}
