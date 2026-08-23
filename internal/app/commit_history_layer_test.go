package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func TestReadCommitDiffBuildsAnImmutableFileView(t *testing.T) {
	dir := testAppRepository(t)
	path := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(path, []byte("new content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testAppGit(t, dir, "add", "tracked.txt")
	testAppGit(t, dir, "commit", "-qm", "change tracked")
	ref := git.HeadSHA(dir)
	files, err := git.CommitFiles(dir, ref)
	if err != nil || len(files) != 1 {
		t.Fatalf("commit files = %+v, err = %v", files, err)
	}

	result := readCommitDiff(context.Background(), dir, ref, ref[:7], files[0], false)
	if result.Canceled || result.Warn != "" || len(result.Diff.Hunks) == 0 {
		t.Fatalf("commit diff result = %+v", result)
	}
	if result.Title != "tracked.txt @ "+ref[:7] || result.TabName != ref+":tracked.txt (diff)" {
		t.Fatalf("commit diff identity = title %q tab %q", result.Title, result.TabName)
	}
	if strings.Join(result.NewLines, "\n") != "new content" {
		t.Fatalf("new lines = %q", result.NewLines)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if result := readCommitDiff(canceled, dir, ref, ref[:7], files[0], false); !result.Canceled {
		t.Fatalf("canceled commit diff result = %+v", result)
	}
}

func TestApplyDiffOpenDoesNotStealFocusAfterNavigation(t *testing.T) {
	group := ui.NewEditorGroupWidget(nil, 4, true, "relative")
	origin := group.ActiveFilePath()
	group.NewFile()
	app := &App{EditorGroup: group, Status: view.NewStatusBar(), diffOpenGen: 3}

	app.ApplyDiffOpen(&DiffOpenResult{Gen: 3, Origin: origin, TabName: "late (diff)", Path: "late.txt"})
	if group.DiffWidgetByTab("late (diff)") != nil {
		t.Fatal("late diff opened after foreground navigation")
	}
}

func historyEntries(prefix string, start, count int) []git.LogEntry {
	entries := make([]git.LogEntry, 0, count)
	for index := start; index < start+count; index++ {
		ref := fmt.Sprintf("%040x", index+1)
		entries = append(entries, git.LogEntry{Ref: ref, Hash: ref[:7], Message: fmt.Sprintf("%s %d", prefix, index)})
	}
	return entries
}

func TestReadCommitLogAllowsOnlyVerifiedUnbornEmptyHistory(t *testing.T) {
	dir := t.TempDir()
	testAppGit(t, dir, "init", "-q", "-b", "main")
	result := readCommitLog(context.Background(), dir, 1)
	if result.Err != nil || result.Canceled || result.Branch != "main" || len(result.Entries) != 0 {
		t.Fatalf("unborn history result = %+v, want empty successful main history", result)
	}
}

func TestReadCommitLogMarksNonRepositoryUnavailable(t *testing.T) {
	dir := t.TempDir()
	result := readCommitLog(context.Background(), dir, 7)
	if result.Gen != 7 || result.Dir != dir || !result.Unavailable || result.Err != nil || result.Canceled {
		t.Fatalf("non-repository history result = %+v, want unavailable", result)
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

func TestCommitHistoryPaginationAppendsOnceAndPreservesReaderState(t *testing.T) {
	cp := NewChangesPanel()
	dir, anchor := "/repo", git.ObjectID(strings.Repeat("f", 40))
	cp.logGen, cp.lastLogDir = 1, dir
	cp.ApplyCommitLog(&CommitLogResult{
		Gen: 1, Dir: dir, Branch: "main", Anchor: anchor, Entries: historyEntries("initial", 0, 10), HasMore: true,
	})
	if got := cp.CommitLog.Config.Items[len(cp.CommitLog.Config.Items)-1]; got.ID != historyLoadOlderID || got.Label != "Load older commits…" {
		t.Fatalf("initial pagination sentinel = %#v", got)
	}
	firstID := "commit:" + historyEntries("initial", 0, 1)[0].Ref
	first := cp.commitNode(firstID)
	first.Expanded = true
	cp.CommitLog.SetItems(cp.CommitLog.Config.Items)
	cp.selectLogNode(historyLoadOlderID)
	cp.logPagePending = true
	cp.replaceHistoryLoadNode(true, false)
	cp.ApplyCommitLog(&CommitLogResult{
		Gen: 1, Dir: dir, Anchor: anchor, Offset: 10, Append: true, Entries: historyEntries("older", 10, 50), HasMore: true,
	})
	if !first.Expanded {
		t.Fatal("pagination collapsed an expanded commit")
	}
	if cp.logOffset != 60 || !cp.logHasMore || cp.CommitLog.ItemCount() != 62 {
		t.Fatalf("appended state offset=%d hasMore=%v rows=%d", cp.logOffset, cp.logHasMore, cp.CommitLog.ItemCount())
	}
	if selected := cp.CommitLog.Selected(); selected == nil || selected.ID != "commit:"+historyEntries("older", 10, 1)[0].Ref {
		t.Fatalf("selection after sentinel activation = %#v", selected)
	}
	seen := make(map[string]bool)
	for _, node := range cp.CommitLog.Config.Items {
		if !strings.HasPrefix(node.ID, "commit:") {
			continue
		}
		if seen[node.ID] {
			t.Fatalf("duplicate appended row %s", node.ID)
		}
		seen[node.ID] = true
	}

	cp.logPagePending = true
	cp.ApplyCommitLog(&CommitLogResult{
		Gen: 1, Dir: dir, Anchor: anchor, Offset: 60, Append: true, HasMore: true,
	})
	if cp.logHasMore || cp.CommitLog.Config.Items[len(cp.CommitLog.Config.Items)-1].ID == historyLoadOlderID {
		t.Fatal("empty terminal page left a load-older sentinel")
	}
}

func TestCommitHistoryPageFailureRetainsRowsAndRetries(t *testing.T) {
	cp := NewChangesPanel()
	dir, anchor := "/repo", git.ObjectID(strings.Repeat("a", 40))
	cp.logGen, cp.lastLogDir = 4, dir
	cp.ApplyCommitLog(&CommitLogResult{Gen: 4, Dir: dir, Anchor: anchor, Entries: historyEntries("good", 0, 10), HasMore: true})
	good := append([]*widgets.TreeNode(nil), cp.CommitLog.Config.Items[:len(cp.CommitLog.Config.Items)-1]...)
	var reported string
	cp.OnError = func(message string) { reported = message }
	cp.logPagePending = true
	cp.ApplyCommitLog(&CommitLogResult{Gen: 4, Dir: dir, Anchor: anchor, Offset: 10, Append: true, Err: errors.New("temporary failure")})
	if !strings.Contains(reported, "temporary failure") {
		t.Fatalf("page error was not reported: %q", reported)
	}
	for index, node := range good {
		if cp.CommitLog.Config.Items[index] != node {
			t.Fatal("page error replaced a last-good history row")
		}
	}
	if sentinel := cp.CommitLog.Config.Items[len(cp.CommitLog.Config.Items)-1]; sentinel.Label != "Retry loading older commits…" {
		t.Fatalf("failed page sentinel = %#v", sentinel)
	}

	cp.logPagePending = true
	cp.ApplyCommitLog(&CommitLogResult{
		Gen: 4, Dir: dir, Anchor: anchor, Offset: 10, Append: true, Entries: historyEntries("retry", 10, 2), HasMore: false,
	})
	if cp.logOffset != 12 || cp.logHasMore || cp.CommitLog.ItemCount() != 13 {
		t.Fatalf("retry state offset=%d hasMore=%v rows=%d", cp.logOffset, cp.logHasMore, cp.CommitLog.ItemCount())
	}
}

func TestCommitHistoryPageDropsStaleRewriteAndRootResults(t *testing.T) {
	cp := NewChangesPanel()
	dir, anchor := "/one", git.ObjectID(strings.Repeat("1", 40))
	cp.logGen, cp.lastLogDir = 7, dir
	cp.ApplyCommitLog(&CommitLogResult{Gen: 7, Dir: dir, Anchor: anchor, Entries: historyEntries("one", 0, 10), HasMore: true})
	cp.logPagePending = true
	cp.ApplyCommitLog(&CommitLogResult{
		Gen: 7, Dir: dir, Anchor: git.ObjectID(strings.Repeat("2", 40)), Offset: 10, Append: true, Entries: historyEntries("rewrite", 10, 1),
	})
	if cp.logOffset != 10 || !cp.logPagePending {
		t.Fatal("mismatched anchor changed the active page request")
	}
	cp.ApplyCommitLog(&CommitLogResult{
		Gen: 6, Dir: dir, Anchor: anchor, Offset: 10, Append: true, Entries: historyEntries("stale", 10, 1),
	})
	if cp.logOffset != 10 || cp.CommitLog.ItemCount() != 12 {
		t.Fatal("stale generation changed history")
	}
	cp.CancelHistoryRead()
	if cp.logPagePending || cp.logGen != 8 {
		t.Fatalf("cancel state pending=%v gen=%d", cp.logPagePending, cp.logGen)
	}

	cp.lastLogDir = "/two"
	cp.ApplyCommitLog(&CommitLogResult{
		Gen: 8, Dir: "/two", Anchor: git.ObjectID(strings.Repeat("3", 40)), Entries: historyEntries("two", 100, 1),
	})
	if cp.logDir != "/two" || cp.logAnchor == anchor || cp.commitNode("commit:"+historyEntries("one", 0, 1)[0].Ref) != nil {
		t.Fatal("second root inherited first-root pagination state")
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
