package clipboard

import (
	"os"
	"path/filepath"
	"testing"
)

func init() {
	DisableSystem()
}

func TestSetGet(t *testing.T) {
	Set("hello")
	if got := Get(); got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
}

func TestOverwrite(t *testing.T) {
	Set("first")
	Set("second")
	if got := Get(); got != "second" {
		t.Fatalf("expected 'second', got %q", got)
	}
}

func TestEmpty(t *testing.T) {
	content = ""
	if got := Get(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestGetIgnoresImageOnlyClipboard(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"[ \"$1\" = --list-types ] && { echo image/png; exit 0; }\n" +
		"echo 'Clipboard content is not available as requested type \"text\"' >&2; exit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "wl-paste"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("WAYLAND_DISPLAY", "wayland-test")
	t.Setenv("DISPLAY", "")
	content = "stale"
	useSystem = true
	defer func() { useSystem = false }()

	if got := Get(); got != "" {
		t.Fatalf("Get() = %q, want no text for an image-only clipboard", got)
	}
}
