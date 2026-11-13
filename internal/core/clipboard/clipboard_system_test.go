package clipboard

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSystemClipboardTimeout(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux clipboard command selection")
	}

	for _, tc := range []struct {
		name    string
		command string
		script  string
		copy    bool
	}{
		{"copy", "wl-copy", "echo started > \"$0.pid\"\nexec sleep 5\n", true},
		{"paste", "wl-paste", "echo started > \"$0.pid\"\nprintf partial\nexec sleep 5\n", false},
		{"paste with inherited pipe", "wl-paste", "sleep 5 &\necho $! > \"$0.pid\"\nprintf partial\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tc.command)
			if err := os.WriteFile(path, []byte("#!/bin/sh\n"+tc.script), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("WAYLAND_DISPLAY", "clipboard-test")
			t.Setenv("DISPLAY", "")
			previousSystem, previousContent := useSystem, content
			useSystem, content = true, "fallback"
			t.Cleanup(func() {
				useSystem, content = previousSystem, previousContent
				if data, err := os.ReadFile(path + ".pid"); err == nil {
					if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
						if process, err := os.FindProcess(pid); err == nil {
							_ = process.Kill()
						}
					}
				}
			})

			start := time.Now()
			if tc.copy {
				content = "previous"
				Set("fallback")
				useSystem = false
			}
			got := Get()
			if elapsed := time.Since(start); elapsed > 3*time.Second {
				t.Errorf("clipboard blocked for %s", elapsed)
			}
			if got != "fallback" {
				t.Errorf("clipboard = %q, want local fallback", got)
			}
			if _, err := os.Stat(path + ".pid"); err != nil {
				t.Fatalf("clipboard helper did not start: %v", err)
			}
		})
	}
}
