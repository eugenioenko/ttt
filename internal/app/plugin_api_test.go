package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func TestContextMenuItemFromWidgetEntryPreservesOptionalCheckedState(t *testing.T) {
	checked, unchecked := true, false
	tests := []struct {
		name    string
		checked *bool
		want    int
	}{
		{name: "omitted", want: 0},
		{name: "unchecked", checked: &unchecked, want: ui.MenuUnchecked},
		{name: "checked", checked: &checked, want: ui.MenuChecked},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := contextMenuItemFromWidgetEntry(widgets.MenuEntry{Label: "Mode", Command: "mode", Checked: test.checked})
			if item.Checked != test.want {
				t.Fatalf("checked indicator = %d, want %d", item.Checked, test.want)
			}
		})
	}
}

func TestSetPathNilDeletesKey(t *testing.T) {
	m := map[string]any{
		"lsp": map[string]any{
			"servers": map[string]any{
				"go":   map[string]any{"command": []any{"gopls"}},
				"rust": map[string]any{"command": []any{"rust-analyzer"}},
			},
		},
	}

	setPath(m, []string{"lsp", "servers", "go"}, nil)

	servers := m["lsp"].(map[string]any)["servers"].(map[string]any)
	if _, exists := servers["go"]; exists {
		t.Error("expected 'go' key to be deleted")
	}
	if _, exists := servers["rust"]; !exists {
		t.Error("expected 'rust' key to remain")
	}
}

func TestGetPathNestedValue(t *testing.T) {
	m := map[string]any{
		"lsp": map[string]any{
			"servers": map[string]any{
				"go": map[string]any{"command": []any{"gopls"}},
			},
		},
	}

	val, ok := getPath(m, []string{"lsp", "servers", "go"})
	if !ok {
		t.Fatal("expected to find lsp.servers.go")
	}
	srv, ok := val.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", val)
	}
	cmd := srv["command"].([]any)
	if len(cmd) != 1 || cmd[0] != "gopls" {
		t.Errorf("expected [gopls], got %v", cmd)
	}

	_, ok = getPath(m, []string{"lsp", "servers", "nonexistent"})
	if ok {
		t.Error("expected false for nonexistent key")
	}
}

func TestPluginFilesystemAPI_PathRestriction(t *testing.T) {
	tmpDir := t.TempDir()
	allowed := filepath.Join(tmpDir, "workspace")
	os.MkdirAll(allowed, 0755)
	os.WriteFile(filepath.Join(allowed, "test.txt"), []byte("hello"), 0644)

	forbidden := filepath.Join(tmpDir, "secrets")
	os.MkdirAll(forbidden, 0755)
	os.WriteFile(filepath.Join(forbidden, "key.pem"), []byte("secret"), 0644)

	api := NewPluginFilesystemAPI(allowed)

	t.Run("read allowed", func(t *testing.T) {
		content, err := api.ReadFile(filepath.Join(allowed, "test.txt"))
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if content != "hello" {
			t.Errorf("expected 'hello', got %q", content)
		}
	})

	t.Run("read forbidden", func(t *testing.T) {
		_, err := api.ReadFile(filepath.Join(forbidden, "key.pem"))
		if err == nil {
			t.Fatal("expected access denied error")
		}
	})

	t.Run("write allowed", func(t *testing.T) {
		err := api.WriteFile(filepath.Join(allowed, "out.txt"), "data")
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
	})

	t.Run("write forbidden", func(t *testing.T) {
		err := api.WriteFile(filepath.Join(forbidden, "evil.txt"), "data")
		if err == nil {
			t.Fatal("expected access denied error")
		}
	})

	t.Run("exists allowed", func(t *testing.T) {
		if !api.FileExists(filepath.Join(allowed, "test.txt")) {
			t.Error("expected true for existing allowed file")
		}
	})

	t.Run("exists forbidden returns false", func(t *testing.T) {
		if api.FileExists(filepath.Join(forbidden, "key.pem")) {
			t.Error("expected false for forbidden path")
		}
	})

	t.Run("list allowed", func(t *testing.T) {
		entries, err := api.ListDir(allowed)
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if len(entries) == 0 {
			t.Error("expected entries in allowed dir")
		}
	})

	t.Run("list forbidden", func(t *testing.T) {
		_, err := api.ListDir(forbidden)
		if err == nil {
			t.Fatal("expected access denied error")
		}
	})

	t.Run("path traversal blocked", func(t *testing.T) {
		_, err := api.ReadFile(filepath.Join(allowed, "..", "secrets", "key.pem"))
		if err == nil {
			t.Fatal("expected path traversal to be blocked")
		}
	})

	t.Run("absolute path outside roots blocked", func(t *testing.T) {
		_, err := api.ReadFile("/etc/passwd")
		if err == nil {
			t.Fatal("expected absolute path outside roots to be blocked")
		}
	})
}

func TestPluginFilesystemAPI_MultipleRoots(t *testing.T) {
	tmpDir := t.TempDir()
	root1 := filepath.Join(tmpDir, "project")
	root2 := filepath.Join(tmpDir, "plugin")
	outside := filepath.Join(tmpDir, "outside")

	for _, d := range []string{root1, root2, outside} {
		os.MkdirAll(d, 0755)
		os.WriteFile(filepath.Join(d, "file.txt"), []byte("data"), 0644)
	}

	api := NewPluginFilesystemAPI(root1, root2)

	if _, err := api.ReadFile(filepath.Join(root1, "file.txt")); err != nil {
		t.Errorf("root1 should be accessible: %v", err)
	}
	if _, err := api.ReadFile(filepath.Join(root2, "file.txt")); err != nil {
		t.Errorf("root2 should be accessible: %v", err)
	}
	if _, err := api.ReadFile(filepath.Join(outside, "file.txt")); err == nil {
		t.Error("outside should not be accessible")
	}
}

func TestPluginFilesystemAPI_SymlinkEscape(t *testing.T) {
	tmpDir := t.TempDir()
	allowed := filepath.Join(tmpDir, "workspace")
	secrets := filepath.Join(tmpDir, "secrets")
	os.MkdirAll(allowed, 0755)
	os.MkdirAll(secrets, 0755)
	os.WriteFile(filepath.Join(secrets, "key.pem"), []byte("secret"), 0644)

	symlink := filepath.Join(allowed, "escape")
	if err := os.Symlink(secrets, symlink); err != nil {
		t.Skip("symlinks not supported")
	}

	api := NewPluginFilesystemAPI(allowed)

	_, err := api.ReadFile(filepath.Join(symlink, "key.pem"))
	if err == nil {
		t.Fatal("expected symlink escape to be blocked")
	}
}

func TestPluginNetworkAPI_SSRFProtection(t *testing.T) {
	api := NewPluginNetworkAPI()

	tests := []struct {
		name    string
		url     string
		blocked bool
	}{
		{"https allowed", "https://example.com/api", false},
		{"http allowed", "http://example.com/api", false},
		{"file scheme blocked", "file:///etc/passwd", true},
		{"ftp scheme blocked", "ftp://evil.com/file", true},
		{"localhost blocked", "http://localhost/admin", true},
		{"127.0.0.1 blocked", "http://127.0.0.1/admin", true},
		{"169.254 metadata blocked", "http://169.254.169.254/latest/meta-data/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := api.validateURL(tt.url)
			if tt.blocked && err == nil {
				t.Errorf("expected URL %q to be blocked", tt.url)
			}
			if !tt.blocked && err != nil {
				t.Errorf("expected URL %q to be allowed, got: %v", tt.url, err)
			}
		})
	}
}

func TestPluginSystemAPI_ArgumentInjection(t *testing.T) {
	api := NewPluginSystemAPI()

	tests := []struct {
		name    string
		binary  string
		args    []string
		blocked bool
	}{
		{"safe git args", "git", []string{"status"}, false},
		{"safe git log", "git", []string{"log", "--oneline", "-5"}, false},
		{"git fsmonitor injection", "git", []string{"-c", "core.fsmonitor=!malicious"}, true},
		{"git sshCommand injection", "git", []string{"-c", "core.sshCommand=evil"}, true},
		{"git pager injection", "git", []string{"-c", "core.pager=!cmd"}, true},
		{"git upload-pack", "git", []string{"clone", "--upload-pack=evil"}, true},
		{"git receive-pack", "git", []string{"push", "--receive-pack=evil"}, true},
		{"general =! injection", "/usr/bin/some-tool", []string{"--config=!evil"}, true},
		{"safe non-git args", "docker", []string{"ps", "-a"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := api.validateArgs(tt.binary, tt.args)
			if tt.blocked && err == nil {
				t.Error("expected argument to be blocked")
			}
			if !tt.blocked && err != nil {
				t.Errorf("expected argument to be allowed, got: %v", err)
			}
		})
	}
}

// TestPluginSystemAPI_ExecStdin exercises the real subprocess path. The
// Lua-level tests in internal/plugin use a mock SystemAPI, so this is the only
// coverage that stdin actually reaches the child process.
func TestPluginSystemAPI_ExecStdin(t *testing.T) {
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("cat not available")
	}
	api := NewPluginSystemAPI()

	stdout, _, code, err := api.Exec("cat", nil, "hello from stdin\n")
	if err != nil || code != 0 {
		t.Fatalf("exec cat: err=%v code=%d", err, code)
	}
	if stdout != "hello from stdin\n" {
		t.Errorf("stdin not piped to child, got %q", stdout)
	}

	// With no stdin the child must see EOF immediately. If Stdin were left
	// attached to the terminal instead, this would block forever.
	stdout, _, code, err = api.Exec("cat", nil, "")
	if err != nil || code != 0 {
		t.Fatalf("exec cat with no stdin: err=%v code=%d", err, code)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
}
