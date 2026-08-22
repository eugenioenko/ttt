package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

// Commit history and detail reads run off the event loop and apply their widget
// state after returning through it. Working-tree refresh remains synchronous so
// command-triggered Git mutations preserve their existing exec lifecycle.

// eventPoster is the one thing the panel needs from the screen: a way to hand a
// finished read back to the event loop. Narrow so a test can stand in for it.
type eventPoster interface {
	PostEvent(tcell.Event) error
}

// commitLogLimit is how many commits the log shows.
const commitLogLimit = 10

// CommitLogResult carries a finished read of one repository's recent commits.
type CommitLogResult struct {
	Gen         int
	Dir         string
	Branch      string
	Entries     []git.LogEntry
	Err         error
	Unavailable bool
}

// CommitFilesResult carries the file list of a single commit back to the node
// that asked for it.
type CommitFilesResult struct {
	Dir    string
	Ref    string
	Short  string
	NodeID string
	Files  []git.FileStatus
	Err    error
}

// CommitDetailResult carries the complete message and every per-file diff for
// one immutable commit. Dir and Ref are the result identity; ApplyCommitDetail
// only installs it into the loading tab created for that exact pair.
type CommitDetailResult struct {
	Dir        string
	Ref        string
	Short      string
	Message    string
	AuthoredAt time.Time
	Files      []ui.CommitDetailFile
	Err        string
}

func readChangesGroups(dirs []string) []changesGroup {
	var groups []changesGroup
	seen := make(map[string]bool)
	for _, dir := range dirs {
		if root := git.RepoRoot(dir); root != "" {
			dir = root
		}
		if seen[dir] {
			continue
		}
		seen[dir] = true
		files, err := git.StatusFiles(dir)
		if err != nil {
			files = nil
		}
		var staged, unstaged []git.FileStatus
		for _, f := range files {
			if f.Staged {
				staged = append(staged, f)
			} else {
				unstaged = append(unstaged, f)
			}
		}
		groups = append(groups, changesGroup{
			Dir:      dir,
			Name:     filepath.Base(dir),
			Staged:   staged,
			Unstaged: unstaged,
		})
	}
	return groups
}

func readCommitLog(dir string, gen int) *CommitLogResult {
	if !git.IsRepo(dir) {
		return &CommitLogResult{Gen: gen, Dir: dir, Unavailable: true}
	}
	entries, err := git.LogWithError(dir, commitLogLimit)
	return &CommitLogResult{
		Gen:     gen,
		Dir:     dir,
		Branch:  git.BranchName(dir),
		Entries: entries,
		Err:     err,
	}
}

// ApplyCommitLog rebuilds the commit log from a finished read.
func (cp *ChangesPanel) ApplyCommitLog(r *CommitLogResult) {
	// Both checks, not one: the counter catches a superseded read, and the
	// directory catches a counter that agrees for some reason it should not.
	if r.Gen != cp.logGen || r.Dir != cp.lastLogDir {
		return
	}
	if r.Unavailable {
		cp.logDir = ""
		cp.logCommits = make(map[string]commitFileRef)
		cp.logFiles = make(map[string]commitFileRef)
		cp.CommitLog.SetItems(nil)
		return
	}
	if r.Err != nil {
		cp.lastLogDir = ""
		if cp.logDir != r.Dir {
			cp.CommitLog.SetItems([]*widgets.TreeNode{{ID: "history:error", Label: "Could not read history", Muted: true}})
		}
		if cp.OnError != nil {
			cp.OnError("Could not refresh commit history: " + r.Err.Error())
		}
		return
	}
	name := filepath.Base(r.Dir)
	if r.Branch != "" {
		cp.Input.Config.Placeholder = fmt.Sprintf("Commit to %s (%s)", name, r.Branch)
	} else {
		cp.Input.Config.Placeholder = fmt.Sprintf("Commit to %s", name)
	}

	// Record what was open before the rebuild, so a Refresh triggered by
	// staging a file does not collapse the commit the reader was in the middle
	// of — and so does switching to another root and back.
	cp.saveCommitLogState()
	cp.logDir = r.Dir
	selected := cp.logSelected[r.Dir]
	cp.logCommits = make(map[string]commitFileRef)
	cp.logFiles = make(map[string]commitFileRef)

	branchLabel := name
	if r.Branch != "" {
		branchLabel = fmt.Sprintf("%s · %s", name, r.Branch)
	}
	nodes := make([]*widgets.TreeNode, 0, len(r.Entries)+1)
	nodes = append(nodes, &widgets.TreeNode{
		ID:    "branch",
		Label: branchLabel,
		Icon:  "⎇",
		Muted: true,
	})
	for _, e := range r.Entries {
		// Keyed on the full hash: the abbreviation git prints can change length
		// as the repo grows, which would silently orphan both the expansion
		// state below and the file cache.
		id := "commit:" + e.Ref
		cp.logCommits[id] = commitFileRef{Dir: r.Dir, Ref: e.Ref, Short: e.Hash}
		node := &widgets.TreeNode{
			ID:         id,
			Label:      e.Message,
			Icon:       "●",
			Badge:      e.Hash,
			Expandable: true,
		}
		if cp.logExpanded[commitLogStateKey(r.Dir, id)] {
			node.Expanded = true
			node.Children = cp.commitChildren(r.Dir, e.Ref, e.Hash, id)
		}
		nodes = append(nodes, node)
	}
	if len(r.Entries) == 0 {
		nodes = append(nodes, &widgets.TreeNode{ID: "history:empty", Label: "No commits", Muted: true})
	}
	cp.CommitLog.SetItems(nodes)

	// Restore by identity, never by the index SetItems happened to preserve.
	// Committing prepends rows, and rewriting history can remove them entirely;
	// in either case the old index now names something the reader did not pick.
	cp.pendingLogSelection = ""
	if selected == "" {
		cp.CommitLog.SetSelectedIndex(0)
		return
	}
	if cp.selectLogNode(selected) {
		return
	}

	// A commit file can be absent temporarily while its surviving parent is
	// expanded and the file list is in flight. Rest on that parent and restore
	// the child when it arrives. Every other missing identity is permanently
	// gone from this log, so the inert branch header is the safe resting place.
	if parentID, ref, isCommitFile := commitFileParent(selected); isCommitFile {
		key := r.Dir + "\x00" + ref
		if cp.commitNode(parentID) != nil && cp.commitFilesPending[key] {
			cp.selectLogNode(parentID)
			cp.pendingLogSelection = selected
			return
		}
	}
	delete(cp.logSelected, r.Dir)
	cp.CommitLog.SetSelectedIndex(0)
}

// commitFileParent extracts only the stable parent identity from a commit-file
// node ID. The path after the hash may itself contain colons, so it remains
// opaque; full Git object IDs cannot contain the separator.
func commitFileParent(id string) (parentID, ref string, ok bool) {
	rest, ok := strings.CutPrefix(id, "cfile:commit:")
	if !ok {
		return "", "", false
	}
	ref, _, ok = strings.Cut(rest, ":")
	if !ok || ref == "" {
		return "", "", false
	}
	return "commit:" + ref, ref, true
}

// selectLogNode selects a node by ID, reporting whether it was there to select.
func (cp *ChangesPanel) selectLogNode(id string) bool {
	for i, node := range cp.CommitLog.FlatList() {
		if node.ID == id {
			cp.CommitLog.SetSelectedIndex(i)
			return true
		}
	}
	return false
}

func readCommitFiles(dir, ref, short, nodeID string) *CommitFilesResult {
	files, err := git.CommitFiles(dir, ref)
	return &CommitFilesResult{Dir: dir, Ref: ref, Short: short, NodeID: nodeID, Files: files, Err: err}
}

// recordCommitFiles caches a finished read and clears its in-flight mark.
//
// A failure is deliberately not cached. A commit's contents are immutable,
// which is what makes the cache safe — but a failure is not a fact about the
// commit, it is a fact about the moment, and index.lock or a mid-rebase repo
// clears on its own. Caching one would leave that commit permanently empty.
func (cp *ChangesPanel) recordCommitFiles(r *CommitFilesResult) {
	key := r.Dir + "\x00" + r.Ref
	delete(cp.commitFilesPending, key)
	if r.Err == nil {
		cp.cacheCommitFiles(key, r.Files)
	}
}

func (cp *ChangesPanel) childrenFor(r *CommitFilesResult) []*widgets.TreeNode {
	if r.Err != nil {
		return []*widgets.TreeNode{errorNode(r.NodeID)}
	}
	return cp.commitFileNodes(r.Dir, r.Ref, r.Short, r.NodeID, r.Files)
}

// fetchCommitFiles reads one commit's file list in the background. A commit
// already being read is skipped rather than queued, so holding down the expand
// key cannot start a run of identical git processes.
func (cp *ChangesPanel) fetchCommitFiles(dir, ref, short, nodeID string) {
	key := dir + "\x00" + ref
	if cp.commitFilesPending[key] {
		return
	}
	cp.commitFilesPending[key] = true
	screen := cp.Screen
	go func() {
		screen.PostEvent(tcell.NewEventInterrupt(readCommitFiles(dir, ref, short, nodeID)))
	}()
}

// ApplyCommitFiles installs a commit's file list under the node that asked for
// it. The node is looked up by ID rather than held as a pointer: the log may
// have been rebuilt, or moved to another repository, while git was running.
func (cp *ChangesPanel) ApplyCommitFiles(r *CommitFilesResult) {
	cp.recordCommitFiles(r)
	if r.Dir != cp.logDir {
		return
	}
	node := cp.commitNode(r.NodeID)
	if node == nil {
		return
	}
	node.Children = cp.childrenFor(r)
	// Same slice, but the widget has to re-flatten now that it has children.
	cp.CommitLog.SetItems(cp.CommitLog.Config.Items)

	if cp.pendingLogSelection != "" {
		parentID, _, isCommitFile := commitFileParent(cp.pendingLogSelection)
		if !isCommitFile || parentID != r.NodeID {
			return
		}
		if cp.selectLogNode(cp.pendingLogSelection) {
			cp.pendingLogSelection = ""
			return
		}
		// This parent's read completed without the selected child, so that
		// identity can no longer arrive. Keep the cursor on the surviving parent
		// and stop carrying the dead selection into later rebuilds.
		cp.pendingLogSelection = ""
		cp.selectLogNode(r.NodeID)
		cp.logSelected[r.Dir] = r.NodeID
	}
}

func (cp *ChangesPanel) commitNode(id string) *widgets.TreeNode {
	for _, node := range cp.CommitLog.Config.Items {
		if node.ID == id {
			return node
		}
	}
	return nil
}

func commitDetailTabID(dir, ref string) string {
	return "commit:" + dir + "\x00" + ref
}

func readCommitDetail(dir, ref, short string) *CommitDetailResult {
	result := &CommitDetailResult{Dir: dir, Ref: ref, Short: short}
	message, err := git.CommitMessage(dir, ref)
	if err != nil {
		result.Err = fmt.Sprintf("Could not read commit %s", short)
		return result
	}
	result.Message = message
	result.AuthoredAt, _ = git.CommitAuthoredAt(dir, ref)

	files, err := git.CommitFiles(dir, ref)
	if err != nil {
		result.Err = fmt.Sprintf("Could not read files for commit %s", short)
		return result
	}
	result.Files = make([]ui.CommitDetailFile, 0, len(files))
	for _, file := range files {
		detail := ui.CommitDetailFile{Path: file.Path, OldPath: file.OldPath}
		diffText, err := git.CommitFileDiff(dir, ref, file)
		if err != nil {
			detail.Error = fmt.Sprintf("Could not read diff for %s", file.Path)
		} else {
			detail.Diff = diff.Parse(diffText)
		}
		result.Files = append(result.Files, detail)
	}
	return result
}

// OpenCommitDetail puts a loading tab in front of the reader immediately, then
// performs every Git read off the event loop. Reopening an immutable commit
// reuses its full-hash tab and never starts a duplicate read.
func (a *App) OpenCommitDetail(dir, ref, short string) {
	tabID := commitDetailTabID(dir, ref)
	if a.EditorGroup.CommitDetailWidgetByTab(tabID) != nil {
		a.EditorGroup.SwitchToTabByPath(tabID)
		a.FocusEditorIfEnabled()
		return
	}

	detail := ui.NewCommitDetailWidget(dir, ref, short, a.EditorGroup.SyntaxHighlight)
	a.wireCommitDetailContext(tabID, detail)
	a.EditorGroup.ApplyDiffDefaults(detail)
	a.EditorGroup.OpenPluginTab(tabID, "Commit "+short, detail)
	a.FocusEditorIfEnabled()
	if a.Screen == nil {
		a.ApplyCommitDetail(readCommitDetail(dir, ref, short))
		return
	}
	screen := a.Screen
	go func() {
		screen.PostEvent(tcell.NewEventInterrupt(readCommitDetail(dir, ref, short)))
	}()
}

// ApplyCommitDetail fills only the still-open tab whose repository and full
// hash match the result. Closing the tab while Git runs supersedes the request.
func (a *App) ApplyCommitDetail(result *CommitDetailResult) {
	detail := a.EditorGroup.CommitDetailWidgetByTab(commitDetailTabID(result.Dir, result.Ref))
	if detail == nil || detail.Dir != result.Dir || detail.Ref != result.Ref {
		return
	}
	if !result.AuthoredAt.IsZero() {
		detail.Metadata = "Authored " + result.AuthoredAt.Format("Jan 2, 2006 at 3:04 PM -0700")
	}
	detail.SetDetail(result.Message, result.Files, result.Err)
}
