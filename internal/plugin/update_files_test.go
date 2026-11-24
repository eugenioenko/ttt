package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplacePluginDirectory(t *testing.T) {
	for _, mode := range []string{"success", "copy-failure", "backup-failure", "replace-failure", "rollback-failure"} {
		t.Run(mode, func(t *testing.T) {
			parent := t.TempDir()
			dst := writePluginDir(t, parent, "p")
			src := writePluginDir(t, t.TempDir(), "p")
			write := func(path, text string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(dst, "main.ttt.lua"), "old code")
			write(filepath.Join(dst, "obsolete.lua"), "old only")
			write(filepath.Join(src, "main.ttt.lua"), "new code")
			write(filepath.Join(src, "added.lua"), "new only")
			if mode == "copy-failure" {
				src = filepath.Join(src, "main.ttt.lua")
			}
			injected := errors.New("rename failed")
			calls := 0
			backup := ""
			rename := func(from, to string) error {
				calls++
				if calls == 1 {
					backup = to
				}
				if (mode == "backup-failure" && calls == 1) || ((mode == "replace-failure" || mode == "rollback-failure") && calls == 2) || (mode == "rollback-failure" && calls == 3) {
					return injected
				}
				return os.Rename(from, to)
			}
			err := replacePluginDirectoryWithRename(src, dst, rename)
			if mode == "success" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("failed update reported success")
			}
			if strings.Contains(mode, "failure") && mode != "copy-failure" && !errors.Is(err, injected) {
				t.Fatalf("error = %v, want rename failure", err)
			}
			check := func(dir, name, want string) {
				t.Helper()
				data, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil || string(data) != want {
					t.Fatalf("%s/%s = %q, error %v, want %q", dir, name, data, err, want)
				}
			}
			switch mode {
			case "success":
				check(dst, "main.ttt.lua", "new code")
				check(dst, "added.lua", "new only")
				if _, err := os.Stat(filepath.Join(dst, "obsolete.lua")); !os.IsNotExist(err) {
					t.Fatalf("obsolete file remains: %v", err)
				}
			case "rollback-failure":
				check(backup, "main.ttt.lua", "old code")
				check(backup, "obsolete.lua", "old only")
				if !strings.Contains(err.Error(), backup) {
					t.Fatalf("error omits recovery path: %v", err)
				}
			default:
				check(dst, "main.ttt.lua", "old code")
				check(dst, "obsolete.lua", "old only")
				if _, err := os.Stat(filepath.Join(dst, "added.lua")); !os.IsNotExist(err) {
					t.Fatalf("failed update mixed new files into installation: %v", err)
				}
			}
			if mode != "rollback-failure" {
				entries, err := os.ReadDir(parent)
				if err != nil || len(entries) != 1 || entries[0].Name() != "p" {
					t.Fatalf("staging files remain: %v, error %v", entries, err)
				}
			}
		})
	}
}
