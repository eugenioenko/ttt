package installer_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallerPlatformSelection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("installer requires a POSIX shell")
	}
	tests := []struct {
		name      string
		os        string
		arch      string
		env       []string
		binary    string
		directory string
		failure   string
	}{
		{name: "Termux", os: "Linux", arch: "aarch64", env: []string{"TERMUX_VERSION=0.118.3", "PREFIX=@/prefix"}, binary: "ttt-android-arm64", directory: "prefix/bin"},
		{name: "Termux custom directory", os: "Linux", arch: "arm64", env: []string{"TERMUX_VERSION=0.118.3", "PREFIX=@/prefix", "INSTALL_DIR=@/custom"}, binary: "ttt-android-arm64", directory: "custom"},
		{name: "Android environment", os: "Linux", arch: "aarch64", env: []string{"ANDROID_ROOT=/system", "PREFIX=@/prefix"}, binary: "ttt-android-arm64", directory: "prefix/bin"},
		{name: "Android without prefix", os: "Linux", arch: "aarch64", env: []string{"ANDROID_ROOT=/system"}, binary: "ttt-android-arm64", directory: "home/.local/bin"},
		{name: "unsupported Android architecture", os: "Linux", arch: "x86_64", env: []string{"TERMUX_VERSION=0.118.3"}, failure: "Unsupported Android architecture: amd64"},
		{name: "Linux with prefix", os: "Linux", arch: "aarch64", env: []string{"PREFIX=@/prefix"}, binary: "ttt-linux-arm64", directory: "home/.local/bin"},
		{name: "Linux Android source checkout", os: "Linux", arch: "x86_64", env: []string{"ANDROID_ROOT=@/android"}, binary: "ttt-linux-amd64", directory: "home/.local/bin"},
		{name: "Linux", os: "Linux", arch: "x86_64", binary: "ttt-linux-amd64", directory: "home/.local/bin"},
		{name: "macOS", os: "Darwin", arch: "arm64", env: []string{"INSTALL_DIR=@/custom"}, binary: "ttt-darwin-arm64", directory: "custom"},
		{name: "FreeBSD", os: "FreeBSD", arch: "amd64", env: []string{"INSTALL_DIR=@/custom"}, binary: "ttt-freebsd-amd64", directory: "custom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "bin")
			if err := os.Mkdir(bin, 0755); err != nil {
				t.Fatal(err)
			}
			mocks := map[string]string{
				"uname": `#!/bin/sh
case "$1" in
  -s) printf '%s\n' "$TEST_OS" ;;
  -m) printf '%s\n' "$TEST_ARCH" ;;
  *) exit 1 ;;
esac
`,
				"curl": `#!/bin/sh
printf '%s\n' "$*" >> "$TEST_DOWNLOADS"
case "$1" in
  -sSf) printf '{"tag_name":"v9.8.7"}\n' ;;
  -sSfL) printf 'installer fixture\n' > "$4" ;;
  *) exit 1 ;;
esac
`,
			}
			for name, body := range mocks {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("sh", "../../install.sh")
			cmd.Env = append(os.Environ(),
				"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"HOME="+filepath.Join(dir, "home"),
				"TERMUX_VERSION=", "ANDROID_ROOT=", "PREFIX=", "INSTALL_DIR=",
				"TEST_OS="+tt.os, "TEST_ARCH="+tt.arch,
				"TEST_DOWNLOADS="+filepath.Join(dir, "downloads"),
			)
			for _, env := range tt.env {
				cmd.Env = append(cmd.Env, strings.ReplaceAll(env, "@", dir))
			}
			output, err := cmd.CombinedOutput()
			if tt.failure != "" {
				if err == nil || !strings.Contains(string(output), tt.failure) {
					t.Fatalf("expected %q, got error %v and output:\n%s", tt.failure, err, output)
				}
				if _, err := os.Stat(filepath.Join(dir, "downloads")); !os.IsNotExist(err) {
					t.Fatalf("unsupported platform attempted a download: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("installer: %v\n%s", err, output)
			}
			downloads, err := os.ReadFile(filepath.Join(dir, "downloads"))
			if err != nil {
				t.Fatal(err)
			}
			url := "https://github.com/eugenioenko/ttt/releases/download/v9.8.7/" + tt.binary
			if !strings.Contains(string(downloads), url+" -o ") {
				t.Fatalf("expected download %s, got:\n%s", url, downloads)
			}
			installed := filepath.Join(dir, tt.directory, "ttt")
			content, err := os.ReadFile(installed)
			if err != nil || string(content) != "installer fixture\n" {
				t.Fatalf("installed binary: %q, error: %v", content, err)
			}
			info, err := os.Stat(installed)
			if err != nil || info.Mode()&0111 == 0 {
				t.Fatalf("installed binary is not executable: %v", err)
			}
		})
	}
}
