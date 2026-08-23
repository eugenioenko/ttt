package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func TestApplyCommitDetailReportsUnavailableAuthoredDate(t *testing.T) {
	group := ui.NewEditorGroupWidget(nil, 4, true, "relative")
	ref := strings.Repeat("a", 40)
	detail := ui.NewCommitDetailWidget("/repo", ref, "aaaaaaa", false)
	group.OpenPluginTab(commitDetailTabID("/repo", ref), "Commit aaaaaaa", detail)
	app := &App{EditorGroup: group}

	app.ApplyCommitDetail(&CommitDetailResult{
		Dir: "/repo", Ref: ref, Message: "subject", AuthoredAtUnavailable: true,
	})

	if detail.Metadata != "Authored date unavailable" {
		t.Fatalf("metadata = %q", detail.Metadata)
	}
}

func TestApplyCommitDetailCancellationDoesNotPublishAuthoredDateFallback(t *testing.T) {
	group := ui.NewEditorGroupWidget(nil, 4, true, "relative")
	ref := strings.Repeat("b", 40)
	detail := ui.NewCommitDetailWidget("/repo", ref, "bbbbbbb", false)
	group.OpenPluginTab(commitDetailTabID("/repo", ref), "Commit bbbbbbb", detail)
	app := &App{EditorGroup: group}

	app.ApplyCommitDetail(&CommitDetailResult{
		Dir: "/repo", Ref: ref, AuthoredAtUnavailable: true, Canceled: true,
	})

	if detail.Metadata != "" || !detail.Loading {
		t.Fatalf("canceled result changed detail: metadata=%q loading=%v", detail.Metadata, detail.Loading)
	}
}

func TestApplyCommitLogReleasesCurrentTimeoutOnCompletion(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "success"},
		{name: "failure", err: errors.New("git failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cp := NewChangesPanel()
			cp.logGen = 1
			cp.lastLogDir = "/repo"
			ctx, cancel := context.WithCancel(context.Background())
			cp.logCancel = cancel

			cp.ApplyCommitLog(&CommitLogResult{Gen: 1, Dir: "/repo", Err: tc.err})

			select {
			case <-ctx.Done():
			default:
				t.Fatal("completed history read retained its timeout")
			}
			if cp.logCancel != nil {
				t.Fatal("completed history read retained its cancel function")
			}
		})
	}
}

func TestStaleCommitLogCompletionDoesNotCancelNewerRead(t *testing.T) {
	cp := NewChangesPanel()
	cp.logGen = 2
	cp.lastLogDir = "/new"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cp.logCancel = cancel

	cp.ApplyCommitLog(&CommitLogResult{Gen: 1, Dir: "/old"})

	select {
	case <-ctx.Done():
		t.Fatal("stale history completion canceled the newer read")
	default:
	}
	if cp.logCancel == nil {
		t.Fatal("stale history completion cleared the newer cancel function")
	}
}

func TestRecordCommitFilesReleasesMatchingTimeoutOnCompletion(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "success"},
		{name: "failure", err: errors.New("git failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cp := NewChangesPanel()
			key := "/repo\x00ref"
			ctx, cancel := context.WithCancel(context.Background())
			cp.commitFilesPending[key] = commitFilesRequest{ID: 1, Cancel: cancel}

			if !cp.recordCommitFiles(&CommitFilesResult{Request: 1, Dir: "/repo", Ref: "ref", Err: tc.err}) {
				t.Fatal("matching completion was rejected")
			}
			select {
			case <-ctx.Done():
			default:
				t.Fatal("completed commit-file read retained its timeout")
			}
			if _, ok := cp.commitFilesPending[key]; ok {
				t.Fatal("completed commit-file read remained pending")
			}
		})
	}
}

func TestStaleCommitFilesCompletionDoesNotCancelNewerRead(t *testing.T) {
	cp := NewChangesPanel()
	key := "/repo\x00ref"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cp.commitFilesPending[key] = commitFilesRequest{ID: 2, Cancel: cancel}

	if cp.recordCommitFiles(&CommitFilesResult{Request: 1, Dir: "/repo", Ref: "ref"}) {
		t.Fatal("stale commit-file completion was accepted")
	}
	select {
	case <-ctx.Done():
		t.Fatal("stale commit-file completion canceled the newer read")
	default:
	}
	if request := cp.commitFilesPending[key]; request.ID != 2 {
		t.Fatalf("newer pending request = %#v", request)
	}
}

func TestCommitHistoryFileKeysActivateCommitAndFileRows(t *testing.T) {
	cp := NewChangesPanel()
	ref := strings.Repeat("c", 40)
	status := git.FileStatus{Status: "M", Path: "file.go"}
	commit := commitFileRef{Dir: "/repo", Ref: ref, Short: "ccccccc", Status: status}
	commitNode := &widgets.TreeNode{ID: "commit:" + ref}
	fileNode := &widgets.TreeNode{ID: "cfile:commit:" + ref + ":file.go"}
	cp.logCommits[commitNode.ID] = commit
	cp.logFiles[fileNode.ID] = commit

	var calls []commitFileRef
	cp.OnOpenCommit = func(dir, gotRef, short string) {
		calls = append(calls, commitFileRef{Dir: dir, Ref: gotRef, Short: short})
	}
	var diffCalls []bool
	cp.OnOpenCommitDiff = func(dir, gotRef, short string, gotStatus git.FileStatus, extended bool) {
		if dir != commit.Dir || gotRef != commit.Ref || short != commit.Short || gotStatus != status {
			t.Fatalf("diff call = %q %q %q %#v", dir, gotRef, short, gotStatus)
		}
		diffCalls = append(diffCalls, extended)
	}
	for _, node := range []*widgets.TreeNode{commitNode, fileNode} {
		for _, key := range []rune{'c', 'o', 'v', 'e'} {
			if !cp.handleCommitLogKey(tcell.NewEventKey(tcell.KeyRune, string(key), tcell.ModNone), node) {
				t.Fatalf("key %q on %s was not handled", key, node.ID)
			}
		}
	}

	if len(calls) != 4 {
		t.Fatalf("commit open calls = %d, want 4", len(calls))
	}
	commitOnly := commit
	commitOnly.Status = git.FileStatus{}
	for i, call := range calls {
		if call != commitOnly {
			t.Fatalf("open call %d = %#v, want %#v", i, call, commitOnly)
		}
	}
	if len(diffCalls) != 4 || diffCalls[0] || diffCalls[1] || diffCalls[2] || !diffCalls[3] {
		t.Fatalf("file diff calls = %v, want compact, compact, compact, extended", diffCalls)
	}
}
