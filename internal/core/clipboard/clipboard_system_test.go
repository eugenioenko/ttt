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
		{"list types", "wl-paste", "echo started > \"$0.pid\"\nprintf partial\nexec sleep 5\n", false},
		{"paste", "wl-paste", "if [ \"$1\" = --list-types ]; then echo text/plain; exit 0; fi\necho started > \"$0.pid\"\nprintf partial\nexec sleep 5\n", false},
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

func TestWaylandClipboardText(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux clipboard command selection")
	}

	for _, tc := range []struct {
		name     string
		offered  string
		mimeType string
		response string
		want     string
	}{
		{name: "PNG", offered: "image/png\n"},
		{name: "SVG", offered: "image/svg+xml\n"},
		{name: "UTF-8 plain text", offered: "text/html\nimage/png\ntext/plain\ntext/plain;charset=utf-8\n", mimeType: "text/plain;charset=utf-8", response: "café", want: "café"},
		{name: "plain text with parameters", offered: "text/html\ntext/plain; charset=utf-8\n", mimeType: "text/plain; charset=utf-8", response: "café", want: "café"},
		{name: "other text", offered: "text/csv\n", mimeType: "text/csv", response: "a,b", want: "a,b"},
		{name: "XWayland text", offered: "UTF8_STRING\n", mimeType: "UTF8_STRING", response: "café", want: "café"},
		{name: "empty", want: "fallback"},
		{name: "unavailable", offered: "unavailable", want: "fallback"},
		{name: "text read fails", offered: "text/plain\nimage/png\n", mimeType: "text/plain", response: "unavailable", want: "fallback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			script := "#!/bin/sh\n" +
				"if [ \"$1\" = --list-types ]; then\n" +
				"    [ \"$TTT_TEST_CLIPBOARD_TYPES\" = unavailable ] && exit 1\n" +
				"    printf '%s' \"$TTT_TEST_CLIPBOARD_TYPES\"\n" +
				"    exit 0\n" +
				"fi\n" +
				"if [ \"$1\" != --no-newline ] || [ \"$2\" != --type ] || [ \"$3\" != \"$TTT_TEST_CLIPBOARD_TYPE\" ] || [ -z \"$3\" ]; then\n" +
				"    printf 'unexpected paste request'\n" +
				"    exit 0\n" +
				"fi\n" +
				"[ \"$TTT_TEST_CLIPBOARD_RESPONSE\" = unavailable ] && exit 1\n" +
				"printf '%s' \"$TTT_TEST_CLIPBOARD_RESPONSE\"\n"
			if err := os.WriteFile(filepath.Join(dir, "wl-paste"), []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir)
			t.Setenv("WAYLAND_DISPLAY", "clipboard-test")
			t.Setenv("DISPLAY", "")
			t.Setenv("TTT_TEST_CLIPBOARD_TYPES", tc.offered)
			t.Setenv("TTT_TEST_CLIPBOARD_TYPE", tc.mimeType)
			t.Setenv("TTT_TEST_CLIPBOARD_RESPONSE", tc.response)
			previousSystem, previousContent := useSystem, content
			useSystem, content = true, "fallback"
			t.Cleanup(func() { useSystem, content = previousSystem, previousContent })

			if got := Get(); got != tc.want {
				t.Fatalf("Get() = %q, want %q", got, tc.want)
			}
		})
	}
}
