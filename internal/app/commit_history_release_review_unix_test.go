//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestClosingCommitDetailCancelsItsGitProcess(t *testing.T) {
	binDir := t.TempDir()
	pidPath := filepath.Join(binDir, "git.pid")
	script := "#!/bin/sh\nprintf '%s' \"$$\" > \"$REVIEW_GIT_PID\"\nexec sleep 30\n"
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("REVIEW_GIT_PID", pidPath)

	group := ui.NewEditorGroupWidget(nil, 4, true, "relative")
	sim := term.NewSimScreen()
	app := &App{
		EditorGroup: group,
		Screen:      term.NewTcellScreenFrom(sim),
		Settings:    &config.Settings{},
	}
	ref := strings.Repeat("d", 40)
	app.OpenCommitDetail("/repo", ref, "ddddddd")
	tabID := commitDetailTabID("/repo", ref)

	deadline := time.Now().Add(time.Second)
	var pid int
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidPath)
		if err == nil {
			pid, err = strconv.Atoi(string(data))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("fake git did not start")
	}
	defer syscall.Kill(pid, syscall.SIGKILL)

	group.ClosePluginTab(tabID)
	waitReviewProcessExit(t, pid)
}

func TestSupersededCommitHistoryCancelsItsGitProcess(t *testing.T) {
	binDir := t.TempDir()
	pidPath := filepath.Join(binDir, "history.pid")
	script := "#!/bin/sh\nprintf '%s' \"$$\" > \"$REVIEW_HISTORY_PID\"\nexec sleep 30\n"
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("REVIEW_HISTORY_PID", pidPath)

	panel := NewChangesPanel()
	sim := term.NewSimScreen()
	panel.Screen = term.NewTcellScreenFrom(sim)
	panel.groups = []changesGroup{{Dir: "/repo"}}
	panel.refreshCommitLog()
	pid := waitReviewPID(t, pidPath)
	defer syscall.Kill(pid, syscall.SIGKILL)

	panel.groups = nil
	panel.lastLogDir = ""
	panel.refreshCommitLog()
	waitReviewProcessExit(t, pid)
	select {
	case event := <-sim.EventQ():
		interrupt, ok := event.(*tcell.EventInterrupt)
		if !ok {
			t.Fatalf("history cancellation event = %T", event)
		}
		result, ok := interrupt.Data().(*CommitLogResult)
		if !ok || !result.Canceled {
			t.Fatalf("history cancellation result = %#v", interrupt.Data())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled history command was not reaped")
	}
}

func waitReviewPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, err := strconv.Atoi(string(data))
			if err != nil {
				t.Fatal(err)
			}
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("fake git did not start")
	return 0
}

func waitReviewProcessExit(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("git process %d is still alive", pid)
}
