// Package jjtest creates isolated Jujutsu repositories for tests.
package jjtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Require skips the test when jj is not installed, unless TTT_REQUIRE_JJ is
// set, so CI cannot pass by silently skipping every jj test.
func Require(t testing.TB) {
	t.Helper()
	if _, err := exec.LookPath("jj"); err != nil {
		if os.Getenv("TTT_REQUIRE_JJ") != "" {
			t.Fatalf("jj is required: %v", err)
		}
		t.Skip("jj not installed")
	}
}

// Isolate points jj at an empty config so the developer's settings and
// identity never leak into tests. Commits then use jj's empty identity.
func Isolate(t testing.TB) {
	t.Helper()
	config := filepath.Join(t.TempDir(), "jj-config.toml")
	if err := os.WriteFile(config, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JJ_CONFIG", config)
}

// Init creates a non-colocated jj repository in a fresh directory.
func Init(t testing.TB) string {
	t.Helper()
	Require(t)
	Isolate(t)
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	Run(t, dir, "git", "init")
	return dir
}

func Run(t testing.TB, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("jj", append([]string{"--no-pager", "--color=never", "--quiet"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("jj %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func Write(t testing.TB, dir, path, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
