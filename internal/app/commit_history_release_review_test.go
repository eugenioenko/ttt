package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/ui"
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
