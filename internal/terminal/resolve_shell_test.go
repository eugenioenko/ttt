package terminal

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveShellDefault(t *testing.T) {
	resolved, err := resolveShell("")
	if err != nil {
		t.Fatalf("resolveShell(\"\") unexpected error: %v", err)
	}
	if resolved == "" {
		t.Fatal("expected non-empty resolved shell for default")
	}
}

func TestResolveShellWithPathSeparators(t *testing.T) {
	paths := []string{"/bin/sh", "/usr/bin/bash", "./custom_shell"}
	if runtime.GOOS == "windows" {
		paths = append(paths, `C:\Windows\System32\cmd.exe`, `.\custom_shell.bat`, `C:custom_shell.bat`)
	}
	for _, p := range paths {
		resolved, err := resolveShell(p)
		if err != nil {
			t.Fatalf("resolveShell(%q) unexpected error: %v", p, err)
		}
		if resolved != p {
			t.Fatalf("resolveShell(%q) = %q, want %q", p, resolved, p)
		}
	}
}

func TestResolveShellBareCommandNotFound(t *testing.T) {
	_, err := resolveShell("nonexistent_shell_cmd_xyz_12345")
	if err == nil {
		t.Fatal("expected error for nonexistent bare shell command, got nil")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Logf("got error: %v (expected ErrNotFound or similar)", err)
	}
}

func TestResolveShellBareCommandResolves(t *testing.T) {
	target := "sh"
	if runtime.GOOS == "windows" {
		target = "cmd"
	}
	resolved, err := resolveShell(target)
	if err != nil {
		t.Fatalf("resolveShell(%q) unexpected error: %v", target, err)
	}
	if !filepath.IsAbs(resolved) {
		t.Fatalf("expected absolute path for resolved shell %q, got %q", target, resolved)
	}
}

func TestNewWithWorkingDirAndBareShell(t *testing.T) {
	shells := []string{"sh"}
	if runtime.GOOS == "windows" {
		shells = []string{"cmd", "powershell"}
	}
	for _, target := range shells {
		t.Run(target, func(t *testing.T) {
			tmp := t.TempDir()
			term, err := New(target, 80, 24, 100, nil, tmp)
			if err != nil {
				t.Fatalf("New(%q, dir=%q) failed: %v", target, tmp, err)
			}
			defer term.Close()

			if term.pt == nil {
				t.Fatal("expected pty to be initialized")
			}
			if term.cmd == nil || term.cmd.Process == nil {
				t.Fatal("expected process to be started")
			}
		})
	}
}
