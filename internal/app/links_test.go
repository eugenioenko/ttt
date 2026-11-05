package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOpenURLCommand(t *testing.T) {
	for _, raw := range []string{"", "file:///tmp/file", "javascript:alert(1)", "ftp://example.com", "https:relative", "https:///missing-host", "https://bad\nURL"} {
		if cmd := openURLCommand(raw, "windows"); cmd != nil {
			t.Errorf("accepted unsupported URL %q", raw)
		}
	}
	raw := "https://a.test/&calc.exe?q=hello&lang=en"
	for _, tt := range []struct {
		platform string
		args     []string
	}{
		{"windows", []string{"rundll32.exe", "url.dll,FileProtocolHandler", raw}},
		{"darwin", []string{"open", raw}},
		{"linux", []string{"xdg-open", raw}},
	} {
		cmd := openURLCommand(raw, tt.platform)
		if cmd == nil || !reflect.DeepEqual(cmd.Args, tt.args) {
			t.Errorf("%s command = %v, want %v", tt.platform, cmd, tt.args)
		}
	}
}

func TestResolveFileLinks(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target.go")
	if err := os.WriteFile(file, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ path, dir, want string }{
		{"target.go", dir, file}, {file, "", file},
		{"missing.go", dir, ""}, {"target.go", "", ""}, {"", dir, ""},
	} {
		if got := resolveFilePath(tt.path, tt.dir); got != tt.want {
			t.Errorf("resolve(%q, %q) = %q, want %q", tt.path, tt.dir, got, tt.want)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if got := resolveFilePath("~/", ""); got != home {
			t.Errorf("home path = %q, want %q", got, home)
		}
	}
}
