package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

// testPoster stands in for the screen, so a test can hold a finished read and
// decide when it is applied.
type testPoster struct {
	events chan tcell.Event
}

func newTestPoster() *testPoster {
	return &testPoster{events: make(chan tcell.Event, 8)}
}

func (p *testPoster) PostEvent(ev tcell.Event) error {
	p.events <- ev
	return nil
}

func (p *testPoster) await(t *testing.T) any {
	t.Helper()
	select {
	case ev := <-p.events:
		return ev.(*tcell.EventInterrupt).Data()
	case <-time.After(5 * time.Second):
		t.Fatal("no result was posted")
		return nil
	}
}

func dirtyRepo(t *testing.T) string {
	t.Helper()
	dir := commitLogRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func asyncPanel(t *testing.T, dirs ...string) (*ChangesPanel, *testPoster) {
	t.Helper()
	cp := NewChangesPanel(dirs...)
	p := newTestPoster()
	cp.Screen = p
	return cp, p
}

// The whole point of the sweep: Refresh must not run git before it returns. A
// panel that has read the working tree by the time Refresh returns is a panel
// that froze the event loop to do it.
func TestRefreshDoesNotReadGitBeforeReturning(t *testing.T) {
	cp, p := asyncPanel(t, dirtyRepo(t))

	cp.Refresh()
	if n := cp.TotalChanges(); n != 0 {
		t.Fatalf("Refresh returned with %d changes already read; it ran git inline", n)
	}

	cp.ApplyStatus(p.await(t).(*ChangesStatusResult))
	if cp.TotalChanges() == 0 {
		t.Error("the scan produced nothing once applied")
	}
}

// Several scans can be in flight after a burst of file changes. Only the last
// one started describes the tree as it is now.
func TestApplyStatusDropsASupersededScan(t *testing.T) {
	cp, p := asyncPanel(t, dirtyRepo(t))

	cp.Refresh()
	stale := p.await(t).(*ChangesStatusResult)
	cp.Refresh() // a newer scan starts, so the first one is now out of date
	fresh := p.await(t).(*ChangesStatusResult)

	cp.ApplyStatus(stale)
	if cp.TotalChanges() != 0 {
		t.Error("a superseded scan was applied")
	}
	cp.ApplyStatus(fresh)
	if cp.TotalChanges() == 0 {
		t.Error("the current scan was dropped")
	}
}

func TestCommitLogFailurePreservesHistoryAndRemainsRetryable(t *testing.T) {
	cp := NewChangesPanel("/repo")
	cp.lastLogDir = "/repo"
	cp.logDir = "/repo"
	cp.logGen = 4
	existing := &widgets.TreeNode{ID: "commit:good", Label: "last good commit"}
	cp.CommitLog.SetItems([]*widgets.TreeNode{existing})
	var callbackErr error
	cp.OnHistoryResult = func(err error) { callbackErr = err }

	cp.ApplyCommitLog(&CommitLogResult{
		Gen: 4, Dir: "/repo", Err: errors.New("temporary log failure"),
	})
	if len(cp.CommitLog.Config.Items) != 1 || cp.CommitLog.Config.Items[0] != existing {
		t.Fatalf("history failure replaced last good items: %+v", cp.CommitLog.Config.Items)
	}
	if cp.lastLogDir != "" {
		t.Fatalf("failed history read left retry guard at %q", cp.lastLogDir)
	}
	if callbackErr == nil {
		t.Fatal("history failure was not reported to the freshness coordinator")
	}
}

func TestCancelHistoryReadCancelsContextAndSupersedesResult(t *testing.T) {
	cp := NewChangesPanel("/repo")
	ctx, cancel := context.WithCancel(context.Background())
	cp.logCancel = cancel
	cp.logGen = 7

	cp.CancelHistoryRead()
	select {
	case <-ctx.Done():
	default:
		t.Fatal("CancelHistoryRead did not cancel the active history context")
	}
	if cp.logGen != 8 {
		t.Fatalf("history generation = %d, want 8", cp.logGen)
	}
}

func TestCommitHistoryRefreshKeyUsesCoordinatorCallback(t *testing.T) {
	cp := NewChangesPanel("/repo")
	called := 0
	cp.OnRefresh = func() { called++ }

	if !cp.handleCommitLogKey(tcell.NewEventKey(tcell.KeyRune, "r", tcell.ModNone), nil) {
		t.Fatal("commit-history refresh key was not handled")
	}
	if called != 1 {
		t.Fatalf("coordinator refresh callback called %d times, want 1", called)
	}
}

func TestCommitHistoryLoadsOlderCommitsOnDemand(t *testing.T) {
	dir := commitLogRepo(t)
	for i := 0; i < 51; i++ {
		cmd := exec.Command("git", "commit", "--allow-empty", "-qm", fmt.Sprintf("older page %02d", i))
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("create history commit %d: %v\n%s", i, err, out)
		}
	}
	cp, poster := asyncPanel(t, dir)
	cp.Refresh()
	cp.ApplyStatus(poster.await(t).(*ChangesStatusResult))
	cp.ApplyCommitLog(poster.await(t).(*CommitLogResult))
	items := cp.CommitLog.Config.Items
	if cp.logLoaded != commitLogPageSize || !cp.logHasMore {
		t.Fatalf("initial page state = loaded %d hasMore %v", cp.logLoaded, cp.logHasMore)
	}
	if len(items) != commitLogPageSize+2 || items[len(items)-1].ID != historyLoadOlderID {
		t.Fatalf("initial history rows = %d, tail=%+v", len(items), items[len(items)-1])
	}

	cp.openCommitLogNode(items[len(items)-1])
	if tail := cp.CommitLog.Config.Items[len(items)-1]; tail.Label != "Loading older commits…" {
		t.Fatalf("loading sentinel = %q", tail.Label)
	}
	// A repeated activation while the page is in flight is idempotent.
	cp.openCommitLogNode(cp.CommitLog.Config.Items[len(items)-1])
	cp.ApplyCommitLog(poster.await(t).(*CommitLogResult))
	items = cp.CommitLog.Config.Items
	if cp.logLoaded != 52 || cp.logHasMore {
		t.Fatalf("completed page state = loaded %d hasMore %v", cp.logLoaded, cp.logHasMore)
	}
	if len(items) != 53 { // branch header plus all 52 commits
		t.Fatalf("completed history rows = %d, want 53", len(items))
	}
	if items[len(items)-1].ID == historyLoadOlderID {
		t.Fatal("load-older sentinel remained after the final page")
	}
	select {
	case duplicate := <-poster.events:
		t.Fatalf("repeated sentinel activation started another page: %#v", duplicate)
	case <-time.After(100 * time.Millisecond):
	}
}

// Expanding a commit shows a placeholder immediately rather than waiting for
// git, and the placeholder is replaced when the read lands.
func TestExpandingACommitPostsInsteadOfBlocking(t *testing.T) {
	cp, p := asyncPanel(t, dirtyRepo(t))
	cp.Refresh()
	cp.ApplyStatus(p.await(t).(*ChangesStatusResult))
	cp.ApplyCommitLog(p.await(t).(*CommitLogResult))

	node := cp.CommitLog.Config.Items[1]
	if _, ok := cp.logCommits[node.ID]; !ok {
		t.Fatalf("expected a commit node, got %q", node.ID)
	}
	node.Expanded = true
	cp.loadCommitFiles(node)

	if len(node.Children) != 1 || node.Children[0].ID != node.ID+loadingSuffix {
		t.Fatalf("expected a loading placeholder, got %+v", node.Children)
	}

	cp.ApplyCommitFiles(p.await(t).(*CommitFilesResult))
	if len(node.Children) != 1 || node.Children[0].Label != "a.txt" {
		t.Fatalf("expected the commit's file, got %+v", node.Children[0])
	}
}

// Holding the expand key must not start a run of identical git processes.
func TestExpandingTwiceStartsOneRead(t *testing.T) {
	cp, p := asyncPanel(t, dirtyRepo(t))
	cp.Refresh()
	cp.ApplyStatus(p.await(t).(*ChangesStatusResult))
	cp.ApplyCommitLog(p.await(t).(*CommitLogResult))

	node := cp.CommitLog.Config.Items[1]
	commit := cp.logCommits[node.ID]
	cp.fetchCommitFiles(commit.Dir, commit.Ref, commit.Short, node.ID)
	cp.fetchCommitFiles(commit.Dir, commit.Ref, commit.Short, node.ID)

	cp.ApplyCommitFiles(p.await(t).(*CommitFilesResult))
	select {
	case ev := <-p.events:
		t.Fatalf("a second read was started: %#v", ev)
	case <-time.After(200 * time.Millisecond):
	}
}

// A read that finishes after the log has moved to another repository must not
// be pushed into whatever node now holds that ID.
func TestApplyCommitFilesIgnoresAResultFromAnotherRepo(t *testing.T) {
	cp, p := asyncPanel(t, dirtyRepo(t))
	cp.Refresh()
	cp.ApplyStatus(p.await(t).(*ChangesStatusResult))
	cp.ApplyCommitLog(p.await(t).(*CommitLogResult))

	node := cp.CommitLog.Config.Items[1]
	node.Children = nil
	cp.ApplyCommitFiles(&CommitFilesResult{
		Dir:    cp.logDir + "-somewhere-else",
		Ref:    "deadbeef",
		NodeID: node.ID,
		Files:  []git.FileStatus{{Status: "M", Path: "wrong.txt"}},
	})
	if len(node.Children) != 0 {
		t.Errorf("a result from another repo was applied: %+v", node.Children)
	}
}

// Clicking through files faster than git can answer should land on the file
// clicked last, not on whichever read happened to finish last.
func TestApplyDiffOpenDropsASupersededRead(t *testing.T) {
	a := &App{}
	a.diffOpenGen = 7
	before := a.EditorGroup
	a.ApplyDiffOpen(&DiffOpenResult{Gen: 3, Warn: "should not be shown"})
	if a.EditorGroup != before {
		t.Error("a superseded diff read was applied")
	}
}

func TestReadCommitDetailIncludesBodyAndEveryFileInGitOrder(t *testing.T) {
	dir := commitLogRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("new file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "-qm", "Detail subject", "-m", "Body line one.\n\nBody line two."},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	refCommand := exec.Command("git", "rev-parse", "HEAD")
	refCommand.Dir = dir
	refOutput, err := refCommand.Output()
	if err != nil {
		t.Fatal(err)
	}
	ref := strings.TrimSpace(string(refOutput))

	result := readCommitDetail(dir, ref, ref[:7])
	if result.Err != "" {
		t.Fatal(result.Err)
	}
	wantMessage := "Detail subject\n\nBody line one.\n\nBody line two."
	if result.Message != wantMessage {
		t.Fatalf("message = %q, want %q", result.Message, wantMessage)
	}
	if result.AuthoredAt.IsZero() {
		t.Fatal("commit detail omitted the authored timestamp")
	}
	statuses, err := git.CommitFiles(dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != len(statuses) || len(result.Files) != 2 {
		t.Fatalf("detail files = %d, Git files = %d", len(result.Files), len(statuses))
	}
	for index, status := range statuses {
		got := result.Files[index]
		if got.Path != status.Path {
			t.Errorf("file %d = %q, want Git-order path %q", index, got.Path, status.Path)
		}
		if len(got.Diff.Hunks) == 0 {
			t.Errorf("file %q has no parsed diff hunks", got.Path)
		}
	}
}

func TestApplyCommitDetailUsesOpenTabRepositoryAndHashIdentity(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	const (
		dir   = "/repo"
		ref   = "0123456789012345678901234567890123456789"
		short = "0123456"
	)
	tabID := commitDetailTabID(ref)
	detail := ui.NewCommitDetailWidget(dir, ref, short, false)
	a.EditorGroup.OpenPluginTab(tabID, "Commit "+short, detail)

	a.ApplyCommitDetail(&CommitDetailResult{Dir: "/other", Ref: ref, Short: short, Message: "wrong repo"})
	if !detail.Loading {
		t.Fatal("a result from another repository filled the loading tab")
	}
	authoredAt := time.Date(2026, time.August, 21, 17, 42, 0, 0, time.FixedZone("EDT", -4*60*60))
	a.ApplyCommitDetail(&CommitDetailResult{
		Dir: dir, Ref: ref, Short: short, Message: "subject\n\nbody", AuthoredAt: authoredAt,
	})
	if detail.Loading || detail.Message != "subject\n\nbody" {
		t.Fatalf("matching result was not applied: loading=%v message=%q", detail.Loading, detail.Message)
	}
	if detail.Metadata != "Authored Aug 21, 2026 at 5:42 PM -0400" {
		t.Fatalf("commit metadata = %q", detail.Metadata)
	}

	a.EditorGroup.ClosePluginTab(tabID)
	a.ApplyCommitDetail(&CommitDetailResult{Dir: dir, Ref: ref, Short: short, Message: "late"})
	if a.EditorGroup.CommitDetailWidgetByTab(tabID) != nil {
		t.Fatal("a result reopened a detail tab that was closed while Git ran")
	}
}

func TestOpenCommitDetailKeysTabsOnFullHashAndReusesEachCommit(t *testing.T) {
	dir := commitLogRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "second"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	logCommand := exec.Command("git", "log", "-2", "--format=%H %h")
	logCommand.Dir = dir
	output, err := logCommand.Output()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 2 {
		t.Fatalf("log returned %d commits", len(lines))
	}
	type identity struct{ ref, short string }
	ids := make([]identity, 0, 2)
	for _, line := range lines {
		ref, short, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("malformed log identity %q", line)
		}
		ids = append(ids, identity{ref: ref, short: short})
	}

	a := buildTestApp(t, config.DefaultSettings())
	a.OpenCommitDetail(dir, ids[0].ref, ids[0].short)
	firstTabID := "commit:" + ids[0].ref
	secondTabID := "commit:" + ids[1].ref
	first := a.EditorGroup.CommitDetailWidgetByTab(firstTabID)
	a.OpenCommitDetail(dir, ids[1].ref, ids[1].short)
	second := a.EditorGroup.CommitDetailWidgetByTab(secondTabID)
	if first == nil || second == nil || first == second {
		t.Fatalf("full hashes did not create distinct detail tabs: first=%p second=%p", first, second)
	}
	if got := a.EditorGroup.ActiveFilePath(); got != secondTabID {
		t.Fatalf("active tab = %q, want second full-hash tab", got)
	}

	a.OpenCommitDetail(dir, ids[0].ref, ids[0].short)
	if got := a.EditorGroup.CommitDetailWidgetByTab(firstTabID); got != first {
		t.Fatal("reopening an immutable commit replaced its existing tab")
	}
	if got := a.EditorGroup.ActiveFilePath(); got != firstTabID {
		t.Fatalf("reopened tab = %q, want first full-hash tab", got)
	}
}

func TestOpenCommitDetailUsesDiffDefaults(t *testing.T) {
	dir := commitLogRepo(t)
	cmd := exec.Command("git", "log", "-1", "--format=%H %h")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	ref, short, ok := strings.Cut(strings.TrimSpace(string(output)), " ")
	if !ok {
		t.Fatalf("malformed commit identity %q", output)
	}

	settings := config.DefaultSettings()
	settings.Editor.DiffMode = config.DiffModeUnified
	settings.Editor.DiffWordWrap = true
	settings.Editor.DiffHighContrast = true
	a := buildTestApp(t, settings)
	a.OpenCommitDetail(dir, ref, short)

	detail := a.EditorGroup.CommitDetailWidgetByTab(commitDetailTabID(ref))
	if detail == nil {
		t.Fatal("commit detail did not open")
	}
	if detail.Mode() != ui.DiffModeUnified || detail.WrapMode() != ui.DiffWrapOn {
		t.Fatalf("commit detail defaults = mode %v wrap %v, want unified/on",
			detail.Mode(), detail.WrapMode())
	}
	if !detail.DiffHighContrast() {
		t.Fatal("commit detail did not inherit the high-contrast diff setting")
	}
}

func TestCommitLogRefreshPostsInsteadOfBlocking(t *testing.T) {
	cp, p := asyncPanel(t, dirtyRepo(t))
	cp.Refresh()
	cp.ApplyStatus(p.await(t).(*ChangesStatusResult))

	// ApplyStatus asks for the log; it must not have read it inline.
	if cp.logDir != "" {
		t.Fatal("the commit log was read on the event path")
	}
	cp.ApplyCommitLog(p.await(t).(*CommitLogResult))
	if cp.logDir == "" || cp.CommitLog.ItemCount() < 2 {
		t.Error("the commit log was not populated once applied")
	}
}

// Every foreground content change goes through the editor-group hook wired by
// RegisterCommands. These are deliberately routes that never call an App open
// helper: a tab switch and the same direct OpenDiff call plugins use.
func TestForegroundNavigationCancelsAPendingDiff(t *testing.T) {
	for _, tc := range []struct {
		name     string
		prepare  func(*App)
		navigate func(*App) string
	}{
		{
			name: "tab switch",
			prepare: func(a *App) {
				a.EditorGroup.NewFile()
			},
			navigate: func(a *App) string {
				a.EditorGroup.SwitchTab(0)
				return a.EditorGroup.ActiveFilePath()
			},
		},
		{
			name: "plugin-style diff open",
			navigate: func(a *App) string {
				a.EditorGroup.OpenDiff("plugin.go", diff.FileDiff{}, []string{"old"}, []string{"new"}, false)
				return a.EditorGroup.ActiveFilePath()
			},
		},
		{
			name: "active tab close",
			prepare: func(a *App) {
				a.EditorGroup.NewFile()
			},
			navigate: func(a *App) string {
				a.EditorGroup.CloseTab()
				return a.EditorGroup.ActiveFilePath()
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := buildTestApp(t, config.DefaultSettings())
			a.Reg = command.NewRegistry()
			RegisterCommands(a)
			if tc.prepare != nil {
				tc.prepare(a)
			}
			a.diffOpenGen = 4
			a.setDiffOpenSegment("Opening diff…")

			want := tc.navigate(a)
			a.ApplyDiffOpen(&DiffOpenResult{
				Gen: 4, TabName: "superseded (diff)", Path: "superseded.txt", Diff: diff.FileDiff{},
			})
			if got := a.EditorGroup.ActiveFilePath(); got != want {
				t.Fatalf("late diff replaced foreground navigation: got %q, want %q", got, want)
			}
			if text := segmentText(a.Status, "diffopen"); text != "" {
				t.Errorf("the in-progress message was left behind: %q", text)
			}
		})
	}
}

// Applying the current result changes active content too, but only after its
// generation has been accepted. The common hook must not prevent that result
// from opening normally.
func TestCurrentDiffOpensThroughActiveContentHook(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.Reg = command.NewRegistry()
	RegisterCommands(a)
	a.diffOpenGen = 4
	a.ApplyDiffOpen(&DiffOpenResult{
		Gen: 4, TabName: "current (diff)", Path: "current.txt", Diff: diff.FileDiff{},
	})
	if got := a.EditorGroup.ActiveFilePath(); got != "current (diff)" {
		t.Fatalf("current diff did not open: got %q", got)
	}
}

// A watcher reload changes bytes underneath the reader; it is not a choice to
// look at something else and must not supersede the diff they requested.
func TestExternalReloadDoesNotCancelPendingDiff(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.Reg = command.NewRegistry()
	RegisterCommands(a)
	path := filepath.Join(t.TempDir(), "watched.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.EditorGroup.OpenFile(path)
	a.diffOpenGen = 4
	a.setDiffOpenSegment("Opening diff…")

	if err := os.WriteFile(path, []byte("after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.EditorGroup.ReloadFile(path)
	a.ApplyDiffOpen(&DiffOpenResult{
		Gen: 4, TabName: "requested (diff)", Path: path, Diff: diff.FileDiff{},
	})
	if got := a.EditorGroup.ActiveFilePath(); got != "requested (diff)" {
		t.Fatalf("external reload cancelled requested diff: active=%q", got)
	}
}

// Reopening the active file is foreground navigation even though its tab index
// does not move. Unlike ReloadFile above, this is the reader choosing what to
// show, so a diff requested earlier must not reclaim the editor afterward.
func TestForegroundReopenOfActiveFileCancelsPendingDiff(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	a.Reg = command.NewRegistry()
	RegisterCommands(a)
	path := filepath.Join(t.TempDir(), "active.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.EditorGroup.OpenFile(path)
	a.diffOpenGen = 4
	a.setDiffOpenSegment("Opening diff…")

	if err := os.WriteFile(path, []byte("reopened\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.EditorGroup.OpenFile(path)
	if got := a.EditorGroup.ActiveBuffer().Lines[0]; got != "reopened" {
		t.Fatalf("precondition: foreground reopen did not reload the buffer: %q", got)
	}

	a.ApplyDiffOpen(&DiffOpenResult{
		Gen: 4, TabName: "superseded (diff)", Path: "superseded.txt", Diff: diff.FileDiff{},
	})
	if got := a.EditorGroup.ActiveFilePath(); got != path {
		t.Fatalf("late diff replaced explicitly reopened active file: got %q, want %q", got, path)
	}
	if text := segmentText(a.Status, "diffopen"); text != "" {
		t.Errorf("the in-progress message was left behind: %q", text)
	}
}

// Tab-cleanup commands are navigation only when they change the active view.
// Keeping the same pinned or dirty tab must not discard a requested diff.
func TestTabCleanupWithoutNavigationDoesNotCancelPendingDiff(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, *App)
		cleanup func(*App)
	}{
		{
			name: "close all keeps active pinned tab",
			prepare: func(_ *testing.T, a *App) {
				a.EditorGroup.TogglePinTab()
				a.EditorGroup.NewFile()
				a.EditorGroup.SwitchTab(0)
			},
			cleanup: func(a *App) { a.EditorGroup.CloseAllTabs() },
		},
		{
			name: "close all saved keeps active dirty tab",
			prepare: func(t *testing.T, a *App) {
				dir := t.TempDir()
				first := filepath.Join(dir, "first.txt")
				second := filepath.Join(dir, "second.txt")
				for _, path := range []string{first, second} {
					if err := os.WriteFile(path, []byte(path+"\n"), 0o644); err != nil {
						t.Fatal(err)
					}
					a.EditorGroup.OpenFile(path)
					a.EditorGroup.CommitActiveTab()
				}
				a.EditorGroup.SwitchToTabByPath(first)
				a.EditorGroup.ActiveBuffer().Dirty = true
			},
			cleanup: func(a *App) { a.EditorGroup.CloseAllSaved() },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := buildTestApp(t, config.DefaultSettings())
			a.Reg = command.NewRegistry()
			RegisterCommands(a)
			tc.prepare(t, a)
			wantBefore := a.EditorGroup.ActiveFilePath()
			a.diffOpenGen = 4
			a.setDiffOpenSegment("Opening diff…")

			tc.cleanup(a)
			if got := a.EditorGroup.ActiveFilePath(); got != wantBefore {
				t.Fatalf("precondition: cleanup changed active tab: got %q, want %q", got, wantBefore)
			}
			a.ApplyDiffOpen(&DiffOpenResult{
				Gen: 4, TabName: "requested (diff)", Path: "requested.txt", Diff: diff.FileDiff{},
			})
			if got := a.EditorGroup.ActiveFilePath(); got != "requested (diff)" {
				t.Fatalf("cleanup without navigation cancelled requested diff: active=%q", got)
			}
		})
	}
}

func segmentText(bar *view.StatusBar, id string) string {
	for _, seg := range bar.LeftSegments() {
		if seg.ID == id {
			return seg.Text
		}
	}
	return ""
}

// Emptying the log is a desired state like any other, so a read still running
// must not arrive and put the cleared repository back.
func TestClearingTheLogInvalidatesAReadInFlight(t *testing.T) {
	cp, p := asyncPanel(t, dirtyRepo(t))
	cp.Refresh()
	cp.ApplyStatus(p.await(t).(*ChangesStatusResult))
	inFlight := p.await(t).(*CommitLogResult)

	// Everything deselected and no groups left: the log has nothing to show.
	cp.groups = nil
	cp.refreshCommitLog()

	cp.ApplyCommitLog(inFlight)
	if cp.logDir != "" || cp.CommitLog.ItemCount() != 0 {
		t.Errorf("a stale read resurrected the cleared repo: logDir=%q items=%d",
			cp.logDir, cp.CommitLog.ItemCount())
	}
}

// A selected file under a commit cannot be restored at rebuild time, because
// its siblings are still being read. Dropping it puts the reader on the commit
// row instead of the file they were on.
func TestSelectionOfACommitChildSurvivesAnAsyncRebuild(t *testing.T) {
	dir := dirtyRepo(t)
	// Three files in the commit, so the single "Loading…" placeholder does not
	// happen to sit at the same index as the child being restored. With a
	// one-file commit the indices coincide and the restore looks to work even
	// when it does not happen.
	for _, name := range []string{"one.txt", "two.txt", "three.txt"} {
		addCommitFile(t, dir, name)
	}
	commitAll(t, dir, "three more")

	cp, p := asyncPanel(t, dir)
	cp.Refresh()
	cp.ApplyStatus(p.await(t).(*ChangesStatusResult))
	cp.ApplyCommitLog(p.await(t).(*CommitLogResult))

	node := cp.CommitLog.Config.Items[1]
	node.Expanded = true
	cp.loadCommitFiles(node)
	cp.ApplyCommitFiles(p.await(t).(*CommitFilesResult))
	cp.CommitLog.SetItems(cp.CommitLog.Config.Items)

	child := node.Children[len(node.Children)-1].ID
	if !cp.selectLogNode(child) {
		t.Fatalf("could not select %q to begin with", child)
	}

	// Rebuild with the cache emptied, as an eviction would leave it, so the
	// children have to be read again.
	cp.commitFiles = map[string][]git.FileStatus{}
	cp.commitFilesOrder = nil
	cp.lastLogDir = ""
	cp.refreshCommitLog()
	cp.ApplyCommitLog(p.await(t).(*CommitLogResult))
	if cp.pendingLogSelection != child {
		t.Fatalf("pending selection = %q, want %q", cp.pendingLogSelection, child)
	}
	if got := cp.CommitLog.Selected(); got == nil || got.ID != node.ID {
		id := "<nil>"
		if got != nil {
			id = got.ID
		}
		t.Fatalf("deferred child rests on %q, want parent %q", id, node.ID)
	}

	// Another rebuild before the file list arrives must preserve the deferred
	// child, rather than replacing it with the temporary parent selection.
	files := p.await(t).(*CommitFilesResult)
	cp.lastLogDir = ""
	cp.refreshCommitLog()
	cp.ApplyCommitLog(p.await(t).(*CommitLogResult))
	if cp.pendingLogSelection != child {
		t.Fatalf("second rebuild replaced pending child with %q", cp.pendingLogSelection)
	}
	cp.ApplyCommitFiles(files)

	if got := cp.CommitLog.Selected(); got == nil || got.ID != child {
		id := "<nil>"
		if got != nil {
			id = got.ID
		}
		t.Errorf("selection landed on %q, want %q", id, child)
	}
}

func TestFailedCommitFileReadStopsDeferringItsSelection(t *testing.T) {
	dir := dirtyRepo(t)
	addCommitFile(t, dir, "one.txt")
	commitAll(t, dir, "one more")

	cp, p := asyncPanel(t, dir)
	cp.Refresh()
	cp.ApplyStatus(p.await(t).(*ChangesStatusResult))
	cp.ApplyCommitLog(p.await(t).(*CommitLogResult))

	node := cp.CommitLog.Config.Items[1]
	node.Expanded = true
	cp.loadCommitFiles(node)
	cp.ApplyCommitFiles(p.await(t).(*CommitFilesResult))
	cp.CommitLog.SetItems(cp.CommitLog.Config.Items)
	child := node.Children[0].ID
	if !cp.selectLogNode(child) {
		t.Fatalf("could not select %q", child)
	}

	cp.commitFiles = map[string][]git.FileStatus{}
	cp.commitFilesOrder = nil
	cp.lastLogDir = ""
	cp.refreshCommitLog()
	cp.ApplyCommitLog(p.await(t).(*CommitLogResult))
	if cp.pendingLogSelection != child {
		t.Fatalf("precondition: pending selection = %q, want %q", cp.pendingLogSelection, child)
	}

	failed := p.await(t).(*CommitFilesResult)
	failed.Files = nil
	failed.Err = errors.New("read failed")
	cp.ApplyCommitFiles(failed)
	if cp.pendingLogSelection != "" {
		t.Fatalf("failed read left selection %q deferred", cp.pendingLogSelection)
	}
	if got := cp.CommitLog.Selected(); got == nil || got.ID != node.ID {
		id := "<nil>"
		if got != nil {
			id = got.ID
		}
		t.Fatalf("failed child rests on %q, want parent %q", id, node.ID)
	}
}

// A missing identity is only deferred when its parent commit is still in the
// log and its children can therefore still arrive. If the commit itself was
// removed by a reset, rebase, amend, or branch switch, preserving the numeric
// row would silently retarget the selection to another commit's file.
func TestRemovedCommitSelectionRestsOnBranchHeader(t *testing.T) {
	dir := commitLogRepo(t)
	addCommitFile(t, dir, "newest.txt")
	commitAll(t, dir, "newest")

	cp := newRefreshedPanel(dir)
	if len(cp.CommitLog.Config.Items) < 3 {
		t.Fatalf("need branch header and two commits, got %d items", len(cp.CommitLog.Config.Items))
	}
	newest := cp.CommitLog.Config.Items[1]
	older := cp.CommitLog.Config.Items[2]
	for _, node := range []*widgets.TreeNode{newest, older} {
		node.Expanded = true
		cp.loadCommitFiles(node)
		if len(node.Children) == 0 {
			t.Fatalf("expanded commit %q has no children", node.ID)
		}
	}
	cp.CommitLog.SetItems(cp.CommitLog.Config.Items)
	removedChild := newest.Children[0].ID
	if !cp.selectLogNode(removedChild) {
		t.Fatalf("could not select %q", removedChild)
	}
	cp.CommitLog.SetFocused(true)

	cmd := exec.Command("git", "reset", "--hard", "HEAD^")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git reset: %v\n%s", err, out)
	}
	cp.Refresh()

	if got := cp.CommitLog.Selected(); got == nil || got.ID != "branch" {
		id := "<nil>"
		if got != nil {
			id = got.ID
		}
		t.Fatalf("removed commit selection retargeted %q, want branch header", id)
	}
	if cp.pendingLogSelection != "" {
		t.Fatalf("removed selection was deferred as %q", cp.pendingLogSelection)
	}
	if _, remembered := cp.logSelected[dir]; remembered {
		t.Fatalf("removed selection remains remembered as %q", cp.logSelected[dir])
	}

	opened := false
	cp.OnOpenCommitDiff = func(string, string, string, git.FileStatus, bool) { opened = true }
	cp.OpenSelectedDiff(false)
	if opened {
		t.Fatal("branch-header resting selection opened an unrelated commit file")
	}
}

func TestRepoWithoutSavedSelectionDoesNotInheritAnotherReposIndex(t *testing.T) {
	first := commitLogRepo(t)
	addCommitFile(t, first, "newest.txt")
	commitAll(t, first, "newest")
	second := commitLogRepo(t)

	cp := newRefreshedPanel(first)
	newest := cp.CommitLog.Config.Items[1]
	newest.Expanded = true
	cp.loadCommitFiles(newest)
	cp.CommitLog.SetItems(cp.CommitLog.Config.Items)
	if len(newest.Children) == 0 || !cp.selectLogNode(newest.Children[0].ID) {
		t.Fatal("could not select a commit child in the first repo")
	}

	cp.SetDirs([]string{second})
	if got := cp.CommitLog.Selected(); got == nil || got.ID != "branch" {
		id := "<nil>"
		if got != nil {
			id = got.ID
		}
		t.Fatalf("new repo inherited selection index %q, want branch header", id)
	}
}

// A deferred restore is only valid while the reader has not made a newer
// choice. Moving to another row while the children load must transfer ownership
// of the selection to that deliberate navigation.
func TestDeliberateCommitLogMoveCancelsDeferredSelection(t *testing.T) {
	dir := dirtyRepo(t)
	for _, name := range []string{"one.txt", "two.txt", "three.txt"} {
		addCommitFile(t, dir, name)
	}
	commitAll(t, dir, "three more")

	cp, p := asyncPanel(t, dir)
	cp.Refresh()
	cp.ApplyStatus(p.await(t).(*ChangesStatusResult))
	cp.ApplyCommitLog(p.await(t).(*CommitLogResult))

	node := cp.CommitLog.Config.Items[1]
	node.Expanded = true
	cp.loadCommitFiles(node)
	cp.ApplyCommitFiles(p.await(t).(*CommitFilesResult))
	cp.CommitLog.SetItems(cp.CommitLog.Config.Items)
	child := node.Children[len(node.Children)-1].ID
	if !cp.selectLogNode(child) {
		t.Fatalf("could not select %q to begin with", child)
	}

	cp.commitFiles = map[string][]git.FileStatus{}
	cp.commitFilesOrder = nil
	cp.lastLogDir = ""
	cp.refreshCommitLog()
	cp.ApplyCommitLog(p.await(t).(*CommitLogResult))
	if cp.pendingLogSelection != child {
		t.Fatalf("precondition: pending selection = %q, want %q", cp.pendingLogSelection, child)
	}

	// Set up a one-step real navigation rather than assigning the final choice:
	// TreeWidget invokes OnSelect only for keyboard/mouse movement.
	cp.CommitLog.SetSelectedIndex(0)
	cp.CommitLog.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	want := cp.CommitLog.Selected()
	if want == nil || want.ID == child {
		t.Fatalf("failed to move away from deferred child: got %#v", want)
	}
	if cp.pendingLogSelection != "" {
		t.Fatalf("deliberate move left deferred selection %q armed", cp.pendingLogSelection)
	}

	cp.ApplyCommitFiles(p.await(t).(*CommitFilesResult))
	if got := cp.CommitLog.Selected(); got == nil || got.ID != want.ID {
		id := "<nil>"
		if got != nil {
			id = got.ID
		}
		t.Fatalf("late children overrode deliberate move: got %q, want %q", id, want.ID)
	}
}

func addCommitFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, dir, message string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", message}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}
