package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

// newRefreshedPanel builds a panel and completes its first scan. With no screen
// the panel reads git inline, so the data is there when Refresh returns.
func newRefreshedPanel(dirs ...string) *ChangesPanel {
	cp := NewChangesPanel(dirs...)
	cp.Refresh()
	return cp
}

func commitLogRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test User"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The message carries the temp dir so two fixtures never produce the same
	// commit hash: identical content, message, author and second would.
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "only " + dir}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// A failed read is a fact about the moment, not about the commit, so it must not
// land in the cache that exists because commits are immutable. Caching it would
// leave that commit showing an empty file list for the rest of the session.
func TestCommitFileNodesDoesNotCacheFailures(t *testing.T) {
	dir := commitLogRepo(t)
	cp := newRefreshedPanel(dir)

	const missing = "0000000000000000000000000000000000000000"
	nodes := cp.commitChildren(dir, missing, "0000000", "commit:"+missing)
	if len(nodes) != 1 || nodes[0].ID != "commit:"+missing+errorSuffix {
		t.Fatalf("expected a single error placeholder, got %+v", nodes)
	}
	if _, cached := cp.commitFiles[dir+"\x00"+missing]; cached {
		t.Error("a failed read was cached")
	}
}

// The empty result of a merge that changed nothing IS a fact about the commit,
// so it is cached — and it still renders a placeholder rather than an
// expandable node that opens onto nothing.
func TestCommitFileNodesCachesSuccess(t *testing.T) {
	dir := commitLogRepo(t)
	cp := newRefreshedPanel(dir)

	var commit commitFileRef
	for _, c := range cp.logCommits {
		commit = c
	}
	if commit.Ref == "" {
		t.Fatal("no commit in the log")
	}
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	full, err := cmd.Output()
	if err != nil {
		t.Fatalf("resolve full hash: %v", err)
	}
	if want := strings.TrimSpace(string(full)); commit.Ref != want {
		t.Errorf("log entry ref = %q, want full hash %q", commit.Ref, want)
	}
	nodes := cp.commitChildren(dir, commit.Ref, commit.Short, "commit:"+commit.Ref)
	if len(nodes) != 1 || nodes[0].Label != "a.txt" {
		t.Fatalf("expected the commit's one file, got %+v", nodes[0])
	}
	if _, cached := cp.commitFiles[dir+"\x00"+commit.Ref]; !cached {
		t.Error("a successful read was not cached")
	}
	// Through the bounded helper, not straight into the map — otherwise the
	// cache has no eviction order and the bound never fires.
	if len(cp.commitFilesOrder) != 1 {
		t.Errorf("insertion order holds %d entries, want 1", len(cp.commitFilesOrder))
	}
}

func TestCommitRowActivationOpensDetailWithoutExpanding(t *testing.T) {
	dir := commitLogRepo(t)
	cp := newRefreshedPanel(dir)
	var node *widgets.TreeNode
	for index, candidate := range cp.CommitLog.FlatList() {
		if _, ok := cp.logCommits[candidate.ID]; ok {
			node = candidate
			cp.CommitLog.SetSelectedIndex(index)
			break
		}
	}
	if node == nil {
		t.Fatal("no commit row")
	}

	var opened commitFileRef
	cp.OnOpenCommit = func(dir, ref, short string) {
		opened = commitFileRef{Dir: dir, Ref: ref, Short: short}
	}
	cp.CommitLog.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	want := cp.logCommits[node.ID]
	if opened.Dir != want.Dir || opened.Ref != want.Ref || opened.Short != want.Short {
		t.Fatalf("activated commit = %+v, want %+v", opened, want)
	}
	if node.Expanded {
		t.Fatal("Enter expanded the commit instead of only activating it")
	}

	cp.CommitLog.HandleEvent(tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone))
	if !node.Expanded {
		t.Fatal("explicit disclosure no longer expands the commit")
	}
}

// Expanding a commit whose previous read failed must retry rather than keep the
// placeholder forever, which is what a plain len(Children) > 0 guard would do.
func TestLoadCommitFilesRetriesAfterFailure(t *testing.T) {
	dir := commitLogRepo(t)
	cp := newRefreshedPanel(dir)

	var node *widgets.TreeNode
	for _, n := range cp.CommitLog.Config.Items {
		if _, ok := cp.logCommits[n.ID]; ok {
			node = n
			break
		}
	}
	if node == nil {
		t.Fatal("no commit node in the log")
	}

	node.Children = []*widgets.TreeNode{errorNode(node.ID)}
	cp.loadCommitFiles(node)
	if len(node.Children) != 1 || node.Children[0].Label != "a.txt" {
		t.Fatalf("expected a retry to load the file, got %+v", node.Children[0])
	}

	// A loaded list is not re-read.
	node.Children[0].Label = "sentinel"
	cp.loadCommitFiles(node)
	if node.Children[0].Label != "sentinel" {
		t.Error("an already-loaded commit was re-read")
	}
}

// git's abbreviated hash is not stable: its length follows core.abbrev and the
// repo's object count. Keying node IDs or the file cache on it means both
// silently orphan the first time the abbreviation grows, collapsing every
// expanded commit. Node IDs must be built from the full hash.
func TestCommitLogNodeIDsSurviveAbbreviationChange(t *testing.T) {
	dir := commitLogRepo(t)

	cp := newRefreshedPanel(dir)
	var before []string
	for _, n := range cp.CommitLog.Config.Items {
		if _, ok := cp.logCommits[n.ID]; ok {
			before = append(before, n.ID)
		}
	}
	if len(before) == 0 {
		t.Fatal("no commit nodes in the log")
	}

	// Widen the abbreviation the way a growing repo would.
	cmd := exec.Command("git", "config", "core.abbrev", "20")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config: %v\n%s", err, out)
	}

	cp2 := newRefreshedPanel(dir)
	var after []string
	for _, n := range cp2.CommitLog.Config.Items {
		if _, ok := cp2.logCommits[n.ID]; ok {
			after = append(after, n.ID)
		}
	}
	if len(after) != len(before) {
		t.Fatalf("commit count changed: %d -> %d", len(before), len(after))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("node ID moved with the abbreviation: %q -> %q", before[i], after[i])
		}
	}

	// The badge, which is display only, is expected to have grown.
	for _, n := range cp2.CommitLog.Config.Items {
		if _, ok := cp2.logCommits[n.ID]; ok && len(n.Badge) != 20 {
			t.Errorf("badge should follow core.abbrev, got %q", n.Badge)
		}
	}
}

// A new commit prepends a row to the log. Keeping the selected *index* across
// that rebuild moves the selection onto a different commit, so the next Enter
// expands the wrong one. Selection is restored by node ID instead.
func TestCommitLogSelectionSurvivesANewCommit(t *testing.T) {
	dir := commitLogRepo(t)
	cp := newRefreshedPanel(dir)

	// Select the one commit in the log (index 0 is the branch header).
	var target string
	for i, n := range cp.CommitLog.FlatList() {
		if _, ok := cp.logCommits[n.ID]; ok {
			cp.CommitLog.SetSelectedIndex(i)
			target = n.ID
			break
		}
	}
	if target == "" {
		t.Fatal("no commit node in the log")
	}

	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "newer"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	cp.Refresh()

	got := cp.CommitLog.Selected()
	if got == nil || got.ID != target {
		id := "<nil>"
		if got != nil {
			id = got.ID
		}
		t.Errorf("selection moved: want %q, got %q", target, id)
	}
}

func addCommit(t *testing.T, dir, name, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", message}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func expandFirstCommit(t *testing.T, cp *ChangesPanel) string {
	t.Helper()
	for _, n := range cp.CommitLog.Config.Items {
		if _, ok := cp.logCommits[n.ID]; ok {
			n.Expanded = true
			cp.loadCommitFiles(n)
			return n.ID
		}
	}
	t.Fatal("no commit node in the log")
	return ""
}

func expandedIDs(cp *ChangesPanel) map[string]bool {
	out := map[string]bool{}
	for _, n := range cp.CommitLog.Config.Items {
		if _, ok := cp.logCommits[n.ID]; ok && n.Expanded {
			out[n.ID] = true
		}
	}
	return out
}

// Switching to another root in a multi-root workspace and back should return to
// what was open, the way the changes tree above it already does.
func TestCommitLogExpansionSurvivesARepoSwitch(t *testing.T) {
	a := commitLogRepo(t)
	b := commitLogRepo(t)

	cp := newRefreshedPanel(a, b)
	opened := expandFirstCommit(t, cp)
	cp.saveCommitLogState()

	// Move the log to the other repo, then back.
	forceLogDir(t, cp, b)
	if expandedIDs(cp)[opened] {
		t.Fatal("repo B's log should not carry repo A's expansion")
	}
	forceLogDir(t, cp, a)

	if !expandedIDs(cp)[opened] {
		t.Error("expansion was lost across a repo switch")
	}
}

// forceLogDir rebuilds the commit log for dir, standing in for the selection
// change that normally drives it.
func forceLogDir(t *testing.T, cp *ChangesPanel, dir string) {
	t.Helper()
	saved := cp.Dirs
	cp.Dirs = []string{dir}
	cp.lastLogDir = ""
	cp.Refresh()
	cp.Dirs = saved
}

// Entries are small and never stale, so the cache exists to be kept — but a
// long session browsing history must not grow it without limit.
func TestCommitFilesCacheIsBounded(t *testing.T) {
	dir := commitLogRepo(t)
	cp := newRefreshedPanel(dir)

	for i := range commitFilesCacheMax + 20 {
		cp.cacheCommitFiles(fmt.Sprintf("%s\x00key%d", dir, i), nil)
	}
	if len(cp.commitFiles) != commitFilesCacheMax {
		t.Errorf("cache holds %d entries, want the bound of %d", len(cp.commitFiles), commitFilesCacheMax)
	}
	if len(cp.commitFilesOrder) != commitFilesCacheMax {
		t.Errorf("insertion order holds %d, want %d", len(cp.commitFilesOrder), commitFilesCacheMax)
	}
	// The oldest went first, the newest is still there.
	if _, ok := cp.commitFiles[fmt.Sprintf("%s\x00key0", dir)]; ok {
		t.Error("the oldest entry should have been evicted")
	}
	if _, ok := cp.commitFiles[fmt.Sprintf("%s\x00key%d", dir, commitFilesCacheMax+19)]; !ok {
		t.Error("the newest entry should still be cached")
	}
}

// Re-caching a key already present must not add a second order entry, or the
// bound would evict live entries while the map stayed under it.
func TestCommitFilesCacheOrderHasNoDuplicates(t *testing.T) {
	cp := newRefreshedPanel(commitLogRepo(t))
	for range 5 {
		cp.cacheCommitFiles("same", nil)
	}
	if len(cp.commitFilesOrder) != 1 {
		t.Errorf("order holds %d entries for one key, want 1", len(cp.commitFilesOrder))
	}
}
