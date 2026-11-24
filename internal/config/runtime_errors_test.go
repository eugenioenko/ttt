package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadRejectsPartialThemeOnDecodeError(t *testing.T) {
	dir := withTempConfigDir(t)
	if err := os.Mkdir(filepath.Join(dir, "themes"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "themes", "default-dark.json")
	if err := os.WriteFile(path, []byte(`{"default":{"bg":"#123456"},"editor":42}`), 0600); err != nil {
		t.Fatal(err)
	}
	want := DefaultTheme()
	want.ResolveColors()
	if got := Load("").Theme; !reflect.DeepEqual(got, want) {
		t.Fatal("invalid theme applied partially instead of keeping defaults")
	}
	if err := os.WriteFile(path, []byte(`{"default":{"bg":"#123456"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := Load("").Theme.Default.Bg; got != "#123456" {
		t.Fatalf("repaired theme background = %q", got)
	}
}

func TestEnsureConfigFileReportsFailureAndPreservesExisting(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(parent, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "settings.json")
	if err := EnsureConfigFile(path, "{}\n"); err == nil {
		t.Fatal("failed config creation reported success")
	}
	if err := os.Remove(parent); err != nil {
		t.Fatal(err)
	}
	if err := EnsureConfigFile(path, "{}\n"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureConfigFile(path, "replacement"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "{}\n" {
		t.Fatalf("existing config overwritten: %q, %v", data, err)
	}
}

func TestParseKeyBindingsDropsStaleStepsAfterError(t *testing.T) {
	bindings := []KeyBinding{{Key: "ctrl+s", Command: "file.save"}, {Key: "ctrl+x", Command: "editor.cut"}}
	if err := ParseKeyBindings(bindings); err != nil {
		t.Fatal(err)
	}
	bindings[0].Key = "ctrl+not-a-key"
	if err := ParseKeyBindings(bindings); err == nil {
		t.Fatal("invalid binding accepted")
	}
	if len(bindings[0].Steps) != 0 || len(bindings[1].Steps) != 1 {
		t.Fatalf("invalid binding retained stale steps or valid binding lost: %+v", bindings)
	}
}
