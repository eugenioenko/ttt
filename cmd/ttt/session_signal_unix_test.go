//go:build !windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestTerminationSignalsSaveWorkspaceSession(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGHUP, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			dir := t.TempDir()
			configDir := filepath.Join(dir, "config")
			if err := os.Mkdir(configDir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(`{"editor":{"restoreTabs":true},"plugins":{"enabled":false}}`), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"first", "second"} {
				if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(name+" buffer"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			ready := filepath.Join(dir, "ready")
			script := fmt.Sprintf(`wait-for Explore; exec "Go to File"; type first.txt; key enter; wait-for "first buffer"; exec "Split: Right"; exec "Go to File"; type second.txt; key enter; wait-for "second buffer"; screenshot %q; wait 30000`, ready)
			cli, exited, pid := startCLI(t, dir, []string{"TTT_CONFIG_DIR=" + configDir}, "--size", "120x30", "--exec", script, dir)
			waitForCLI(t, cli, exited, func() bool {
				info, err := os.Stat(ready)
				return err == nil && info.Size() > 0
			})
			if err := syscall.Kill(pid, sig); err != nil {
				t.Fatal(err)
			}
			select {
			case <-exited:
			case <-time.After(5 * time.Second):
				t.Fatal("editor did not exit after termination signal")
			}

			paths, err := filepath.Glob(filepath.Join(configDir, "sessions", "*.json"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("saved sessions = %v, err = %v, want one session", paths, err)
			}
			data, err := os.ReadFile(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			var session config.EditorSessionState
			if err := json.Unmarshal(data, &session); err != nil {
				t.Fatal(err)
			}
			split := session.Layout
			if split == nil || split.First == nil || split.Second == nil ||
				len(split.First.Tabs) != 1 || len(split.Second.Tabs) != 1 ||
				filepath.Base(split.First.Tabs[0].Path) != "first.txt" || filepath.Base(split.Second.Tabs[0].Path) != "second.txt" {
				t.Fatalf("session did not save both split groups:\n%s", data)
			}
		})
	}
}
