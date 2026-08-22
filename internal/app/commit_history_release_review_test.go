package app

import (
	"context"
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

func TestCommitDetailClosedRequestCannotOverwriteReopenedCommit(t *testing.T) {
	group := ui.NewEditorGroupWidget(nil, 4, true, "relative")
	ref := strings.Repeat("a", 40)
	tabID := commitDetailTabID("/repo", ref)

	first := ui.NewCommitDetailWidget("/repo", ref, "aaaaaaa", false)
	first.Incarnation = 1
	group.OpenPluginTab(tabID, "Commit aaaaaaa", first)
	group.ClosePluginTab(tabID)

	reopened := ui.NewCommitDetailWidget("/repo", ref, "aaaaaaa", false)
	reopened.Incarnation = 2
	group.OpenPluginTab(tabID, "Commit aaaaaaa", reopened)
	app := &App{EditorGroup: group}

	app.ApplyCommitDetail(&CommitDetailResult{Incarnation: 2, Dir: "/repo", Ref: ref, Message: "new request succeeded"})
	app.ApplyCommitDetail(&CommitDetailResult{Incarnation: 1, Dir: "/repo", Ref: ref, Err: "older request failed"})

	if reopened.Error != "" || reopened.Message != "new request succeeded" {
		t.Fatalf("older closed-tab result overwrote reopened success: message=%q error=%q", reopened.Message, reopened.Error)
	}
}

func TestCommitDetailStaleSuccessCannotOverwriteReopenedFailure(t *testing.T) {
	group := ui.NewEditorGroupWidget(nil, 4, true, "relative")
	ref := strings.Repeat("f", 40)
	tabID := commitDetailTabID("/repo", ref)
	reopened := ui.NewCommitDetailWidget("/repo", ref, "fffffff", false)
	reopened.Incarnation = 2
	group.OpenPluginTab(tabID, "Commit fffffff", reopened)
	app := &App{EditorGroup: group}

	app.ApplyCommitDetail(&CommitDetailResult{Incarnation: 2, Dir: "/repo", Ref: ref, Err: "new request failed"})
	app.ApplyCommitDetail(&CommitDetailResult{Incarnation: 1, Dir: "/repo", Ref: ref, Message: "older request succeeded"})
	if reopened.Error != "new request failed" || reopened.Message != "" {
		t.Fatalf("stale success overwrote reopened failure: message=%q error=%q", reopened.Message, reopened.Error)
	}
}

func TestCommitDetailClosedContextRequestCannotOverwriteReopenedCommit(t *testing.T) {
	group := ui.NewEditorGroupWidget(nil, 4, true, "relative")
	ref := strings.Repeat("b", 40)
	tabID := commitDetailTabID("/repo", ref)

	first := ui.NewCommitDetailWidget("/repo", ref, "bbbbbbb", false)
	first.Incarnation = 1
	group.OpenPluginTab(tabID, "Commit bbbbbbb", first)
	group.ClosePluginTab(tabID)

	reopened := ui.NewCommitDetailWidget("/repo", ref, "bbbbbbb", false)
	reopened.Incarnation = 2
	reopened.SetDetail("subject", []ui.CommitDetailFile{{Path: "file.txt"}}, "")
	group.OpenPluginTab(tabID, "Commit bbbbbbb", reopened)
	app := &App{EditorGroup: group}
	key := ui.CommitDetailContextKey(reopened.Files[0])

	app.ApplyCommitDetailContext(&CommitDetailContextResult{
		Incarnation: 2, TabID: tabID, Dir: "/repo", Ref: ref, FileIndex: 0, FileKey: key,
		OldLines: []string{"old"}, NewLines: []string{"new"},
	})
	app.ApplyCommitDetailContext(&CommitDetailContextResult{
		Incarnation: 1, TabID: tabID, Dir: "/repo", Ref: ref, FileIndex: 0, FileKey: key,
		Err: "older request failed",
	})

	if reopened.Files[0].FullFileState != ui.CommitDetailFullFileLoaded {
		t.Fatal("older closed-tab context result erased the reopened tab's successful context")
	}
}

func TestCommitDetailStaleContextSuccessCannotOverwriteReopenedFailure(t *testing.T) {
	group := ui.NewEditorGroupWidget(nil, 4, true, "relative")
	ref := strings.Repeat("0", 40)
	tabID := commitDetailTabID("/repo", ref)
	reopened := ui.NewCommitDetailWidget("/repo", ref, "0000000", false)
	reopened.Incarnation = 2
	reopened.SetDetail("subject", []ui.CommitDetailFile{{Path: "file.txt"}}, "")
	group.OpenPluginTab(tabID, "Commit 0000000", reopened)
	app := &App{EditorGroup: group}
	key := ui.CommitDetailContextKey(reopened.Files[0])

	app.ApplyCommitDetailContext(&CommitDetailContextResult{
		Incarnation: 2, TabID: tabID, Dir: "/repo", Ref: ref, FileIndex: 0, FileKey: key,
		Err: "new request failed",
	})
	app.ApplyCommitDetailContext(&CommitDetailContextResult{
		Incarnation: 1, TabID: tabID, Dir: "/repo", Ref: ref, FileIndex: 0, FileKey: key,
		OldLines: []string{"old"}, NewLines: []string{"new"},
	})
	if reopened.Files[0].FullFileState != ui.CommitDetailFullFileFailed || reopened.Files[0].FullFileErr != "new request failed" {
		t.Fatalf("stale context success overwrote failure: state=%v err=%q", reopened.Files[0].FullFileState, reopened.Files[0].FullFileErr)
	}
}

func TestCommitDetailAuthoredTimestampPreservesGitSecondPrecision(t *testing.T) {
	group := ui.NewEditorGroupWidget(nil, 4, true, "relative")
	ref := strings.Repeat("c", 40)
	detail := ui.NewCommitDetailWidget("/repo", ref, "ccccccc", false)
	group.OpenPluginTab(commitDetailTabID("/repo", ref), "Commit ccccccc", detail)
	app := &App{EditorGroup: group}
	authored := time.Date(2026, time.August, 22, 3, 14, 15, 0, time.FixedZone("EDT", -4*60*60))

	app.ApplyCommitDetail(&CommitDetailResult{Dir: "/repo", Ref: ref, Message: "subject", AuthoredAt: authored})

	if detail.Metadata != "Authored Aug 22, 2026 at 3:14:15 AM -0400" {
		t.Fatalf("authored timestamp lost precision: %q", detail.Metadata)
	}
}

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
	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err == nil {
		t.Fatalf("git process %d is still alive after its detail tab closed", pid)
	}
}

func TestCommitDetailIncarnationsIncreaseAcrossRepeatedCloseReopen(t *testing.T) {
	group := ui.NewEditorGroupWidget(nil, 4, true, "relative")
	app := &App{EditorGroup: group}
	ref := strings.Repeat("e", 40)
	tabID := commitDetailTabID("/repo", ref)
	var previous uint64
	for range 25 {
		request := app.beginCommitDetailRequest(tabID)
		if request.Incarnation <= previous {
			t.Fatalf("incarnation %d did not increase after %d", request.Incarnation, previous)
		}
		previous = request.Incarnation
		detail := ui.NewCommitDetailWidget("/repo", ref, "eeeeeee", false)
		detail.Incarnation = request.Incarnation
		detail.OnClose = func() { app.cancelCommitDetailRequest(tabID, request.Incarnation) }
		group.OpenPluginTab(tabID, "Commit eeeeeee", detail)
		group.ClosePluginTab(tabID)
		select {
		case <-request.Context.Done():
		default:
			t.Fatalf("incarnation %d remained live after close", request.Incarnation)
		}
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

func TestShutdownGitReadsCancelsEveryLiveRequest(t *testing.T) {
	app := &App{Changes: NewChangesPanel()}
	first := app.beginCommitDetailRequest("first")
	second := app.beginCommitDetailRequest("second")
	fileContext, cancelFile := context.WithCancel(context.Background())
	app.Changes.commitFilesPending["file"] = commitFilesRequest{ID: 1, Cancel: cancelFile}

	app.ShutdownGitReads()
	for name, done := range map[string]<-chan struct{}{
		"first detail":  first.Context.Done(),
		"second detail": second.Context.Done(),
		"history file":  fileContext.Done(),
	} {
		select {
		case <-done:
		default:
			t.Fatalf("%s request remained live after shutdown", name)
		}
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
