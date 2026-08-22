package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestReadCommitLogAllowsOnlyVerifiedUnbornEmptyHistory(t *testing.T) {
	dir := t.TempDir()
	testAppGit(t, dir, "init", "-q", "-b", "main")
	result := readCommitLog(context.Background(), dir, 1)
	if result.Err != nil || result.Canceled || result.Branch != "main" || len(result.Entries) != 0 {
		t.Fatalf("unborn history result = %+v, want empty successful main history", result)
	}
}

func TestCommitLogRestoresSelectionByFullHashAfterAsyncRebuild(t *testing.T) {
	cp := NewChangesPanel()
	dir := "/repo"
	cp.logGen = 1
	cp.lastLogDir = dir
	cp.ApplyCommitLog(&CommitLogResult{Gen: 1, Dir: dir, Branch: "main", Entries: []git.LogEntry{
		{Ref: strings.Repeat("a", 40), Hash: "aaaaaaa", Message: "selected"},
		{Ref: strings.Repeat("b", 40), Hash: "bbbbbbb", Message: "older"},
	}})
	selectedID := "commit:" + strings.Repeat("a", 40)
	if !cp.selectLogNode(selectedID) {
		t.Fatal("initial full-hash row missing")
	}

	cp.logGen = 2
	cp.lastLogDir = dir
	cp.ApplyCommitLog(&CommitLogResult{Gen: 2, Dir: dir, Branch: "main", Entries: []git.LogEntry{
		{Ref: strings.Repeat("c", 40), Hash: "ccccccc", Message: "newest"},
		{Ref: strings.Repeat("a", 40), Hash: "aaaaaaa", Message: "selected"},
		{Ref: strings.Repeat("b", 40), Hash: "bbbbbbb", Message: "older"},
	}})
	if got := cp.CommitLog.Selected(); got == nil || got.ID != selectedID {
		t.Fatalf("selection after prepend = %#v, want %s", got, selectedID)
	}
}

func TestCommitLogExpansionIdentityIncludesRepository(t *testing.T) {
	cp := NewChangesPanel()
	ref := strings.Repeat("a", 40)
	cp.logExpanded[commitLogStateKey("/first", "commit:"+ref)] = true
	cp.logGen = 1
	cp.lastLogDir = "/second"
	cp.ApplyCommitLog(&CommitLogResult{Gen: 1, Dir: "/second", Entries: []git.LogEntry{{Ref: ref, Hash: "aaaaaaa", Message: "same object"}}})
	if node := cp.commitNode("commit:" + ref); node == nil || node.Expanded {
		t.Fatalf("second repository inherited expansion: %#v", node)
	}
}

func TestCommitLogDropsStaleGenerationAndShowsFirstLoadError(t *testing.T) {
	cp := NewChangesPanel()
	cp.logGen = 2
	cp.lastLogDir = "/new"
	cp.CommitLog.SetItems(nil)
	cp.ApplyCommitLog(&CommitLogResult{Gen: 1, Dir: "/old", Entries: []git.LogEntry{{Ref: strings.Repeat("a", 40), Hash: "aaaaaaa", Message: "stale"}}})
	if cp.CommitLog.ItemCount() != 0 {
		t.Fatal("stale history result changed the tree")
	}

	cp.ApplyCommitLog(&CommitLogResult{Gen: 2, Dir: "/new", Err: errors.New("git unavailable")})
	if got := cp.CommitLog.Selected(); got == nil || got.Label != "Could not read history" {
		t.Fatalf("first-load error row = %#v", got)
	}
	if cp.lastLogDir != "" {
		t.Fatalf("failed history remained non-retryable: %q", cp.lastLogDir)
	}
}

func TestApplyCommitDetailUsesRepositoryHashIdentityAndPreciseTimestamp(t *testing.T) {
	group := ui.NewEditorGroupWidget(nil, 4, true, "relative")
	ref := strings.Repeat("d", 40)
	detail := ui.NewCommitDetailWidget("/repo", ref, "ddddddd", false)
	other := ui.NewCommitDetailWidget("/other", ref, "ddddddd", false)
	group.OpenPluginTab(commitDetailTabID("/repo", ref), "Commit ddddddd", detail)
	group.OpenPluginTab(commitDetailTabID("/other", ref), "Commit ddddddd", other)
	app := &App{EditorGroup: group}

	app.ApplyCommitDetail(&CommitDetailResult{
		Dir: "/other", Ref: ref, Message: "other repository",
	})
	if !detail.Loading {
		t.Fatal("mismatched repository populated loading detail")
	}
	if other.Loading || other.Message != "other repository" {
		t.Fatal("same-hash repository detail did not populate its exact tab")
	}

	authored := time.Date(2026, time.August, 22, 3, 14, 15, 0, time.FixedZone("EDT", -4*60*60))
	app.ApplyCommitDetail(&CommitDetailResult{
		Dir: "/repo", Ref: ref, Message: "subject", AuthoredAt: authored,
	})
	if detail.Loading || detail.Metadata != "Authored Aug 22, 2026 at 3:14:15 AM -0400" {
		t.Fatalf("applied detail loading=%v metadata=%q", detail.Loading, detail.Metadata)
	}
}
