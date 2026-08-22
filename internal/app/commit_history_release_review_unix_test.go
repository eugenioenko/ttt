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
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
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

func TestClosingCommitDetailCancelsItsFullFileGitProcess(t *testing.T) {
	binDir := t.TempDir()
	pidPath := filepath.Join(binDir, "context.pid")
	script := "#!/bin/sh\nprintf '%s' \"$$\" > \"$REVIEW_CONTEXT_PID\"\nexec sleep 30\n"
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("REVIEW_CONTEXT_PID", pidPath)

	group := ui.NewEditorGroupWidget(nil, 4, true, "relative")
	sim := term.NewSimScreen()
	app := &App{EditorGroup: group, Screen: term.NewTcellScreenFrom(sim), Settings: &config.Settings{}}
	ref := strings.Repeat("c", 40)
	tabID := commitDetailTabID("/repo", ref)
	request := app.beginCommitDetailRequest(tabID)
	detail := ui.NewCommitDetailWidget("/repo", ref, "ccccccc", false)
	detail.Incarnation = request.Incarnation
	detail.SetDetail("subject", []ui.CommitDetailFile{{Status: "A", Path: "file.txt"}}, "")
	detail.OnClose = func() { app.cancelCommitDetailRequest(tabID, request.Incarnation) }
	app.wireCommitDetailContext(tabID, detail, request)
	group.OpenPluginTab(tabID, "Commit ccccccc", detail)
	detail.SetContextMode(ui.DiffContextFullFile)
	pid := waitReviewPID(t, pidPath)
	defer syscall.Kill(pid, syscall.SIGKILL)

	group.ClosePluginTab(tabID)
	waitReviewProcessExit(t, pid)
	select {
	case event := <-sim.EventQ():
		interrupt, ok := event.(*tcell.EventInterrupt)
		if !ok {
			t.Fatalf("context cancellation event = %T", event)
		}
		result, ok := interrupt.Data().(*CommitDetailContextResult)
		if !ok || !result.Canceled || result.Incarnation != request.Incarnation {
			t.Fatalf("context cancellation result = %#v", interrupt.Data())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled full-file context command was not reaped")
	}
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

func TestCanceledCommitHistoryPageReapsItsGitProcess(t *testing.T) {
	binDir := t.TempDir()
	pidPath := filepath.Join(binDir, "history-page.pid")
	script := "#!/bin/sh\nprintf '%s' \"$$\" > \"$REVIEW_HISTORY_PAGE_PID\"\nexec sleep 30\n"
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("REVIEW_HISTORY_PAGE_PID", pidPath)

	panel := NewChangesPanel()
	sim := term.NewSimScreen()
	panel.Screen = term.NewTcellScreenFrom(sim)
	panel.logDir = "/repo"
	panel.lastLogDir = "/repo"
	panel.logAnchor = git.ObjectID(strings.Repeat("a", 40))
	panel.logOffset = 10
	panel.logHasMore = true
	panel.logGen = 9
	panel.CommitLog.SetItems([]*widgets.TreeNode{historyLoadOlderNode(false, false)})
	panel.loadOlderHistory()
	pid := waitReviewPID(t, pidPath)
	defer syscall.Kill(pid, syscall.SIGKILL)

	panel.CancelHistoryRead()
	waitReviewProcessExit(t, pid)
	select {
	case event := <-sim.EventQ():
		interrupt, ok := event.(*tcell.EventInterrupt)
		if !ok {
			t.Fatalf("page cancellation event = %T", event)
		}
		result, ok := interrupt.Data().(*CommitLogResult)
		if !ok || !result.Canceled || !result.Append {
			t.Fatalf("page cancellation result = %#v", interrupt.Data())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled history page was not reaped")
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
