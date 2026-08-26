package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

type ChangesPanel struct {
	Tree      *widgets.TreeWidget
	Input     *widgets.InputWidget
	CommitLog *widgets.TreeWidget
	Adapter   *ui.WidgetAdapter
	Split     *ui.ContentSplitWidget
	Dirs      []string
	// Screen is how a finished background read gets back onto the event loop.
	// With no screen the panel reads git inline instead — see changes_async.go.
	Screen eventPoster

	groups     []changesGroup
	multiRoot  bool
	expanded   map[string]bool
	lastLogDir string
	// commandContext remembers which tree the reader last acted in. Modal
	// widgets temporarily take focus before running a command, so live widget
	// focus cannot reliably identify the selection the command should use.
	commandContext changesCommandContext

	// logDir is the repo the commit log currently displays, as opposed to
	// lastLogDir which only guards redundant rebuilds. They differ because
	// Refresh clears the guard while the rendered log stays put.
	logDir string
	// Every appended page is tied to one immutable full HEAD snapshot.
	logAnchor      git.ObjectID
	logOffset      int
	logHasMore     bool
	logPagePending bool
	// commitFiles caches a commit's file list. A commit's contents never
	// change, so an entry can never go stale — only numerous, hence the bound.
	commitFiles      map[string][]git.FileStatus
	commitFilesOrder []string
	// commitFilesPending marks reads already in flight, so repeated expands of
	// one commit do not each start their own git process.
	commitFilesPending map[string]commitFilesRequest
	commitFilesNext    uint64
	logCancel          context.CancelFunc
	logCommits         map[string]commitFileRef
	logFiles           map[string]commitFileRef
	// logGen lets a finished read tell whether a newer one has superseded it.
	logGen int
	// pendingLogSelection is a selection that could not be restored yet because
	// the node it names is a commit's child and those children are still being
	// read.
	pendingLogSelection string
	// logExpanded and logSelected outlive any one repo's log, so switching
	// between roots in a workspace and back returns to what was open.
	logExpanded        map[string]bool
	logSelected        map[string]string
	logFolderExpanded  map[string]bool
	workFolderExpanded map[workFolderStateKey]bool
	workNodes          map[string]workNodeRef
	workFiles          map[string]workFileRef
	fileView           string

	OnOpenDiff       func(dir string, status git.FileStatus, extended bool)
	OnOpenCommitDiff func(dir, ref, short string, status git.FileStatus, extended bool)
	OnOpenCommit     func(dir, ref, short string)
	OnOpenPRDiff     func(group *ui.ChangesGroup, status git.FileStatus, extended bool)
	OnOpenPRDetail   func(group *ui.ChangesGroup)
	OnOpenFile       func(path string)
	OnRightClick     func(dir string, status git.FileStatus, screenX, screenY int)
	OnPanelMenu      func(screenX, screenY int)
	OnCommit         func(dir string, message string)
	OnGroupMenu      func(dir string, screenX, screenY int)
	OnPRGroupMenu    func(group *ui.ChangesGroup, screenX, screenY int)
	OnRefreshPR      func(url string)
	OnConfirmDiscard func(message string, onConfirm func())
	OnError          func(message string)
	OnRefreshed      func()
	OnRefresh        func()
	OnStatusChanged  func()
	OnHistoryResult  func(error)

	PRGroups []prGroup
}

type changesCommandContext uint8

const (
	changesWorkingTree changesCommandContext = iota
	changesCommitLog
)

const (
	changesHistoryMinHeight     = 4 // title plus three usable log rows
	changesWorkingTreeMinHeight = 5 // input, divider, and three tree rows
)

type changesGroup struct {
	Dir      string
	Name     string
	Staged   []git.FileStatus
	Unstaged []git.FileStatus
}

type workFileRef struct {
	Dir    string
	Status git.FileStatus
	Staged bool
	Kind   workNodeKind
}

type workNodeRef struct {
	Dir    string
	Path   string
	Staged bool
	Kind   workNodeKind
	Group  int
	PR     bool
}

type workFolderStateKey struct {
	Dir  string
	Path string
	PR   bool
}

// commitFileRef ties a commit-log node back to the immutable commit it belongs
// to without parsing path content out of the node ID.
type commitFileRef struct {
	Dir string
	// Ref is the full hash, which is what git is asked with and what the tab
	// key is built from. Short is only ever shown to the reader.
	Ref    string
	Short  string
	Status git.FileStatus
}

type prGroup struct {
	Dir       string
	Name      string
	Files     []git.FileStatus
	PRURL     string
	PRDiffs   map[string]string
	PROwner   string
	PRRepo    string
	PRBaseSHA string
	PRHeadSHA string
}

func (a *App) persistCommitHistoryHeight(height int) {
	a.Changes.Split.BottomH = height
	a.Settings.Sidebar.CommitHistoryHeight = height
	if err := config.SaveSettings(*a.Settings); err != nil {
		a.StatusError("Failed to save commit history height: " + err.Error())
	}
}

func NewChangesPanel(dirs ...string) *ChangesPanel {
	cp := &ChangesPanel{
		Dirs:               dirs,
		multiRoot:          len(dirs) > 1,
		expanded:           make(map[string]bool),
		commitFiles:        make(map[string][]git.FileStatus),
		logCommits:         make(map[string]commitFileRef),
		logFiles:           make(map[string]commitFileRef),
		logExpanded:        make(map[string]bool),
		logSelected:        make(map[string]string),
		logFolderExpanded:  make(map[string]bool),
		workFolderExpanded: make(map[workFolderStateKey]bool),
		workNodes:          make(map[string]workNodeRef),
		workFiles:          make(map[string]workFileRef),
		fileView:           config.GitFileViewList,

		commitFilesPending: make(map[string]commitFilesRequest),
	}

	cp.Input = widgets.NewInputWidget(widgets.InputConfig{
		Placeholder: "Message",
		Bordered:    false,
		OnSubmit: func(text string) {
			cp.commitFocusedGroup()
		},
	})

	cp.Tree = widgets.NewTreeWidget(widgets.TreeConfig{
		Indent:             1,
		EmptyText:          "No changes",
		TruncateLeft:       true,
		ActivateExpandable: true,
		OnCommand: func(cmd string, node *widgets.TreeNode) {
			cp.handleCommand(cmd, node)
		},
		OnMenu: func(_ []widgets.MenuEntry, node *widgets.TreeNode, sx, sy int) {
			cp.handleMenu(node, sx, sy)
		},
		OnSelect: func(node *widgets.TreeNode) {
			cp.refreshCommitLog()
		},
		OnFocus: func() {
			cp.commandContext = changesWorkingTree
		},
		OnKey: func(ev *tcell.EventKey, node *widgets.TreeNode) bool {
			return cp.handleKey(ev)
		},
	})

	cp.CommitLog = widgets.NewTreeWidget(widgets.TreeConfig{
		Indent:             1,
		EmptyText:          "No commits",
		ActivateExpandable: true,
		OnSelect: func(_ *widgets.TreeNode) {
			// A deferred restore belongs to the selection that existed before the
			// rebuild. Once the reader moves, their newer choice owns the cursor.
			cp.pendingLogSelection = ""
		},
		OnFocus: func() {
			cp.commandContext = changesCommitLog
		},
		OnExpand: func(node *widgets.TreeNode) {
			cp.loadCommitFiles(node)
		},
		OnCommand: func(cmd string, node *widgets.TreeNode) {
			if cmd == "activate" {
				if isCommitFolderNode(node) {
					node.Expanded = !node.Expanded
					cp.CommitLog.SetItems(cp.CommitLog.Config.Items)
					return
				}
				cp.openCommitLogNode(node)
			}
		},
		OnMenu: func(_ []widgets.MenuEntry, _ *widgets.TreeNode, sx, sy int) {
			cp.showPanelMenu(sx, sy)
		},
		OnKey: func(ev *tcell.EventKey, node *widgets.TreeNode) bool {
			return cp.handleCommitLogKey(ev, node)
		},
	})

	logTitle := widgets.NewTitleWidget(widgets.TitleConfig{Title: "Commit History"})

	logBox := &widgets.BoxWidget{}
	logBox.Child = cp.CommitLog

	divTop := widgets.NewDividerWidget(widgets.DividerConfig{})

	top := widgets.NewVStackWidget(cp.Tree, divTop, cp.Input)
	bottom := widgets.NewVStackWidget(logTitle, logBox)

	cp.Split = ui.NewContentSplitWidget()
	cp.Split.Top = top
	cp.Split.Bottom = bottom
	cp.Split.ShowBottom = true
	cp.Split.BottomH = 0
	cp.Split.BottomRatio = 0.5
	cp.Split.MinBottomH = changesHistoryMinHeight
	cp.Split.MinTopH = changesWorkingTreeMinHeight
	cp.Split.OnResize = func(height int) {
		// Default for standalone/test use; App wires this to persistCommitHistoryHeight
		// (see registerWidgetCallbacks) once the panel is attached to Settings.
		cp.Split.BottomH = height
	}

	cp.Adapter = ui.NewWidgetAdapter(cp.Split)

	// App.Init installs the event poster before the first refresh so history can
	// load asynchronously. Tests without an event loop can call Refresh inline.
	return cp
}

func (cp *ChangesPanel) SetDirs(dirs []string) {
	cp.Dirs = dirs
	cp.multiRoot = len(dirs) > 1
	cp.Refresh()
}

// applied reports a git failure and refreshes either way, so the panel never
// silently redraws unchanged after an operation that did not happen.
func (cp *ChangesPanel) applied(err error) {
	if err != nil && cp.OnError != nil {
		cp.OnError(err.Error())
	}
	if cp.OnStatusChanged != nil {
		cp.OnStatusChanged()
	} else {
		cp.Refresh()
	}
}

// paths splits a file list into the untracked ones, which are deleted outright,
// and the rest, which are checked out from HEAD.
func discardPaths(files []git.FileStatus) (untracked, tracked []string) {
	for _, f := range files {
		if f.Status == "?" {
			untracked = append(untracked, f.Path)
		} else {
			tracked = append(tracked, f.Path)
		}
	}
	return untracked, tracked
}

func filePaths(files []git.FileStatus) []string {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	return paths
}

func (cp *ChangesPanel) Refresh() {
	cp.cancelHistoryReads()
	dirs := append([]string(nil), cp.Dirs...)
	cp.applyWorkingTree(readChangesGroups(dirs))
	cp.lastLogDir = ""
	cp.refreshCommitLog()
}

func (cp *ChangesPanel) applyWorkingTree(groups []changesGroup) {
	cp.saveExpanded()
	cp.groups = groups
	cp.multiRoot = len(cp.groups)+len(cp.PRGroups) > 1
	cp.buildTree()
	if cp.OnRefreshed != nil {
		cp.OnRefreshed()
	}
}

func (cp *ChangesPanel) refreshCommitLog() {
	dir := cp.selectedGroupDir()
	if dir == "" && len(cp.groups) > 0 {
		dir = cp.groups[0].Dir
	}
	if dir == "" {
		// Emptying the log is a desired state too, so it has to invalidate a
		// read still running — otherwise that read arrives and resurrects the
		// repository that was just cleared.
		cp.cancelHistoryReads()
		cp.logGen++
		cp.saveCommitLogState()
		cp.lastLogDir = ""
		cp.logDir = ""
		cp.resetHistoryPaging()
		cp.CommitLog.SetItems(nil)
		return
	}
	if dir == cp.lastLogDir {
		return
	}
	if dir != cp.logDir {
		cp.cancelCommitFileReads()
		cp.saveCommitLogState()
		cp.logDir = ""
		cp.resetHistoryPaging()
		cp.logCommits = make(map[string]commitFileRef)
		cp.logFiles = make(map[string]commitFileRef)
		cp.CommitLog.SetItems([]*widgets.TreeNode{{ID: "history:loading", Label: "Loading…", Muted: true}})
	}
	cp.lastLogDir = dir
	cp.logGen++
	gen := cp.logGen
	cp.cancelLogRead()
	ctx, cancel := context.WithTimeout(context.Background(), commitHistoryTimeout)
	cp.logCancel = cancel
	if cp.Screen == nil {
		result := readCommitLog(ctx, dir, gen)
		cancel()
		cp.ApplyCommitLog(result)
		return
	}
	screen := cp.Screen
	go func() {
		result := readCommitLog(ctx, dir, gen)
		cancel()
		_ = screen.PostEvent(tcell.NewEventInterrupt(result))
	}()
}

func (cp *ChangesPanel) cancelLogRead() {
	if cp.logCancel != nil {
		cp.logCancel()
		cp.logCancel = nil
	}
}

func (cp *ChangesPanel) cancelCommitFileReads() {
	for _, request := range cp.commitFilesPending {
		request.Cancel()
	}
	cp.commitFilesPending = make(map[string]commitFilesRequest)
}

func (cp *ChangesPanel) cancelHistoryReads() {
	cp.cancelLogRead()
	cp.cancelCommitFileReads()
}

func (cp *ChangesPanel) resetHistoryPaging() {
	cp.logAnchor = ""
	cp.logOffset = 0
	cp.logHasMore = false
	cp.logPagePending = false
}

func (cp *ChangesPanel) RefreshHistory() {
	cp.lastLogDir = ""
	cp.refreshCommitLog()
}

func (cp *ChangesPanel) CancelHistoryRead() {
	cp.cancelHistoryReads()
	cp.logGen++
	cp.logPagePending = false
}

func (cp *ChangesPanel) Shutdown() {
	cp.CancelHistoryRead()
}

func (cp *ChangesPanel) loadOlderHistory() {
	if cp.logPagePending || cp.logCancel != nil || !cp.logHasMore || cp.logDir == "" || cp.logAnchor == "" {
		return
	}
	cp.logPagePending = true
	cp.replaceHistoryLoadNode(true, false)
	dir, anchor, offset, gen := cp.logDir, cp.logAnchor, cp.logOffset, cp.logGen
	ctx, cancel := context.WithTimeout(context.Background(), commitHistoryTimeout)
	cp.logCancel = cancel
	if cp.Screen == nil {
		result := readCommitLogPage(ctx, dir, anchor, offset, gen)
		cancel()
		cp.ApplyCommitLog(result)
		return
	}
	screen := cp.Screen
	go func() {
		result := readCommitLogPage(ctx, dir, anchor, offset, gen)
		cancel()
		_ = screen.PostEvent(tcell.NewEventInterrupt(result))
	}()
}

// saveCommitLogState records the currently rendered log's expansion and
// selection before it is thrown away. Collapsed entries are dropped rather than
// stored as false: absent already means collapsed, and that keeps the map the
// size of what is open rather than of everything ever opened.
func (cp *ChangesPanel) saveCommitLogState() {
	if cp.logDir == "" {
		return
	}
	for _, node := range cp.CommitLog.Config.Items {
		if _, ok := cp.logCommits[node.ID]; !ok {
			continue
		}
		key := commitLogStateKey(cp.logDir, node.ID)
		if node.Expanded {
			cp.logExpanded[key] = true
		} else {
			delete(cp.logExpanded, key)
		}
		cp.saveCommitFolderState(node.Children)
	}
	if cp.pendingLogSelection != "" {
		// A second rebuild can land while the selected child's read is still in
		// flight. Preserve the identity that can still arrive, not the parent row
		// where the cursor is resting temporarily.
		cp.logSelected[cp.logDir] = cp.pendingLogSelection
	} else if node := cp.CommitLog.Selected(); node != nil {
		cp.logSelected[cp.logDir] = node.ID
	}
}

func (cp *ChangesPanel) saveCommitFolderState(nodes []*widgets.TreeNode) {
	for _, node := range nodes {
		if isCommitFolderNode(node) {
			cp.logFolderExpanded[node.ID] = node.Expanded
		}
		cp.saveCommitFolderState(node.Children)
	}
}

func commitLogStateKey(dir, nodeID string) string {
	return dir + "\x00" + nodeID
}

// commitFilesCacheMax bounds the file-list cache. Entries are small and never
// go stale, so the only reason to evict is that a long session browsing history
// would otherwise grow one entry per commit ever opened, without limit.
const commitFilesCacheMax = 256

func (cp *ChangesPanel) cacheCommitFiles(key string, files []git.FileStatus) {
	if _, exists := cp.commitFiles[key]; !exists {
		cp.commitFilesOrder = append(cp.commitFilesOrder, key)
	}
	cp.commitFiles[key] = files
	for len(cp.commitFilesOrder) > commitFilesCacheMax {
		oldest := cp.commitFilesOrder[0]
		cp.commitFilesOrder = cp.commitFilesOrder[1:]
		delete(cp.commitFiles, oldest)
	}
}

// loadCommitFiles fills in a commit's children when it is expanded. TreeWidget
// calls this before it re-flattens, so mutating Children here is enough — no
// second SetItems.
func (cp *ChangesPanel) loadCommitFiles(node *widgets.TreeNode) {
	commit, ok := cp.logCommits[node.ID]
	if !ok {
		return
	}
	// A read still running keeps its placeholder; one that failed is retried,
	// since the failure was never cached.
	if len(node.Children) > 0 {
		first := node.Children[0].ID
		if first != node.ID+errorSuffix {
			return
		}
	}
	node.Children = cp.commitChildren(commit.Dir, commit.Ref, commit.Short, node.ID)
}

// commitChildren returns what to render under a commit. A cached list renders
// straight away; anything else has to be read, and git must not run on the
// event path — so the read is started and a placeholder stands in until it
// lands. A panel with no screen has no event loop to come back through and
// reads inline instead.
func (cp *ChangesPanel) commitChildren(dir, ref, short, parentID string) []*widgets.TreeNode {
	if files, cached := cp.commitFiles[dir+"\x00"+ref]; cached {
		return cp.commitFileNodes(dir, ref, short, parentID, files)
	}
	if cp.Screen == nil {
		ctx, cancel := context.WithTimeout(context.Background(), commitFilesTimeout)
		defer cancel()
		r := readCommitFiles(ctx, 0, dir, ref, short, parentID)
		cp.recordCommitFiles(r)
		return cp.childrenFor(r)
	}
	cp.fetchCommitFiles(dir, ref, short, parentID)
	return []*widgets.TreeNode{{
		ID:    parentID + loadingSuffix,
		Label: "Loading…",
		Muted: true,
	}}
}

const (
	loadingSuffix = ":loading"
	errorSuffix   = ":error"
	emptySuffix   = ":empty"
)

func errorNode(parentID string) *widgets.TreeNode {
	return &widgets.TreeNode{ID: parentID + errorSuffix, Label: "Could not read commit", Muted: true}
}

func (cp *ChangesPanel) commitFileNodes(dir, ref, short, parentID string, files []git.FileStatus) []*widgets.TreeNode {
	if len(files) == 0 {
		// An expandable node that opens onto nothing reads as broken. A merge
		// that changed nothing against its first parent is the usual cause.
		return []*widgets.TreeNode{{ID: parentID + emptySuffix, Label: "No files", Muted: true}}
	}
	makeLeaf := func(f git.FileStatus) *widgets.TreeNode {
		id := fmt.Sprintf("cfile:%s:%s", parentID, f.Path)
		cp.logFiles[id] = commitFileRef{Dir: dir, Ref: ref, Short: short, Status: f}
		return &widgets.TreeNode{
			ID:           id,
			Label:        f.Path,
			Icon:         ui.StatusBadge(f.Status),
			IconStyle:    ui.StatusStyle(f.Status),
			TruncateLeft: true,
		}
	}
	if cp.fileView == config.GitFileViewList {
		nodes := make([]*widgets.TreeNode, 0, len(files))
		for _, f := range files {
			nodes = append(nodes, makeLeaf(f))
		}
		return nodes
	}
	return compactFileTree("history:"+parentID, files, makeLeaf, cp.logFolderExpanded)
}

func (cp *ChangesPanel) openCommitFile(node *widgets.TreeNode, extended bool) {
	if node == nil || cp.OnOpenCommitDiff == nil {
		return
	}
	ref, ok := cp.logFiles[node.ID]
	if !ok {
		return
	}
	cp.OnOpenCommitDiff(ref.Dir, ref.Ref, ref.Short, ref.Status, extended)
}

func (cp *ChangesPanel) openCommitLogNode(node *widgets.TreeNode) {
	if node == nil {
		return
	}
	if node.ID == historyLoadOlderID {
		cp.loadOlderHistory()
		return
	}
	if commit, ok := cp.logCommits[node.ID]; ok {
		if cp.OnOpenCommit != nil {
			cp.OnOpenCommit(commit.Dir, commit.Ref, commit.Short)
		}
		return
	}
	cp.openCommitFile(node, false)
}

func (cp *ChangesPanel) handleCommitLogKey(ev *tcell.EventKey, node *widgets.TreeNode) bool {
	if ev.Key() != tcell.KeyRune {
		return false
	}
	switch term.KeyRune(ev) {
	case 'r', 'R':
		if cp.OnRefresh != nil {
			cp.OnRefresh()
		} else {
			cp.Refresh()
		}
		return true
	case 'c', 'o', 'v':
		cp.openCommitLogNode(node)
		return true
	case 'e':
		if node == nil {
			return false
		}
		if _, isCommit := cp.logCommits[node.ID]; isCommit {
			cp.openCommitLogNode(node)
		} else {
			cp.openCommitFile(node, true)
		}
		return true
	}
	return false
}

func (cp *ChangesPanel) saveExpanded() {
	var visit func([]*widgets.TreeNode)
	visit = func(nodes []*widgets.TreeNode) {
		for _, node := range nodes {
			if node.Expandable || len(node.Children) > 0 {
				cp.expanded[node.ID] = node.Expanded
			}
			if ref, ok := cp.workNodes[node.ID]; ok && (ref.Kind == workNodeFolder || ref.Kind == workNodePRFolder) {
				cp.workFolderExpanded[workFolderStateKey{Dir: ref.Dir, Path: ref.Path, PR: ref.PR}] = node.Expanded
			}
			visit(node.Children)
		}
	}
	visit(cp.Tree.Config.Items)
}

// ExpandAll opens the working-tree hierarchy and every already-loaded folder
// in commit file trees. Commit rows remain untouched so this never starts a
// history-wide burst of Git reads.
func (cp *ChangesPanel) ExpandAll() {
	cp.Tree.ExpandAll()
	cp.setCommitFoldersExpanded(true)
	cp.saveExpanded()
}

func (cp *ChangesPanel) CollapseAll() {
	cp.Tree.CollapseAll()
	cp.setCommitFoldersExpanded(false)
	cp.saveExpanded()
}

func (cp *ChangesPanel) setCommitFoldersExpanded(expanded bool) {
	var visit func([]*widgets.TreeNode)
	visit = func(nodes []*widgets.TreeNode) {
		for _, node := range nodes {
			if isCommitFolderNode(node) {
				node.Expanded = expanded
				cp.logFolderExpanded[node.ID] = expanded
			}
			visit(node.Children)
		}
	}
	visit(cp.CommitLog.Config.Items)
	cp.CommitLog.SetItems(cp.CommitLog.Config.Items)
}

func (cp *ChangesPanel) restoreExpanded(node *widgets.TreeNode) {
	if ref, ok := cp.workNodes[node.ID]; ok && (ref.Kind == workNodeFolder || ref.Kind == workNodePRFolder) {
		if exp, ok := cp.workFolderExpanded[workFolderStateKey{Dir: ref.Dir, Path: ref.Path, PR: ref.PR}]; ok {
			node.Expanded = exp
		}
	} else if exp, ok := cp.expanded[node.ID]; ok {
		node.Expanded = exp
	}
	for _, child := range node.Children {
		cp.restoreExpanded(child)
	}
}

func (cp *ChangesPanel) buildTree() {
	selected := ""
	var selectedFile workFileRef
	selectedWasFile := false
	if node := cp.Tree.Selected(); node != nil {
		selected = node.ID
		selectedFile, selectedWasFile = cp.workFiles[node.ID]
	}
	cp.workNodes = make(map[string]workNodeRef)
	cp.workFiles = make(map[string]workFileRef)
	var roots []*widgets.TreeNode

	for gi, g := range cp.groups {
		var sectionNodes []*widgets.TreeNode

		if len(g.Staged) > 0 {
			id := workingNodeID(workNodeSection, g.Dir, "", true)
			cp.workNodes[id] = workNodeRef{Dir: g.Dir, Staged: true, Kind: workNodeSection, Group: gi}
			stagedNode := &widgets.TreeNode{
				ID:         id,
				Label:      fmt.Sprintf("Staged (%d)", len(g.Staged)),
				Expandable: true,
				Expanded:   true,
				Muted:      true,
				Actions: []widgets.Action{
					{Icon: "−", Command: "unstageAll"},
				},
			}
			stagedNode.Children = cp.fileNodes(g.Dir, g.Staged, true, gi, false)
			sectionNodes = append(sectionNodes, stagedNode)
		}

		if len(g.Unstaged) > 0 {
			id := workingNodeID(workNodeSection, g.Dir, "", false)
			cp.workNodes[id] = workNodeRef{Dir: g.Dir, Kind: workNodeSection, Group: gi}
			changesNode := &widgets.TreeNode{
				ID:         id,
				Label:      fmt.Sprintf("Changes (%d)", len(g.Unstaged)),
				Expandable: true,
				Expanded:   true,
				Muted:      true,
				Actions: []widgets.Action{
					{Icon: "✕", Command: "discardAll"},
					{Icon: "+", Command: "stageAll"},
				},
			}
			changesNode.Children = cp.fileNodes(g.Dir, g.Unstaged, false, gi, false)
			sectionNodes = append(sectionNodes, changesNode)
		}

		if cp.multiRoot {
			id := workingNodeID(workNodeRoot, g.Dir, "", false)
			cp.workNodes[id] = workNodeRef{Dir: g.Dir, Kind: workNodeRoot, Group: gi}
			root := &widgets.TreeNode{
				ID:         id,
				Label:      g.Name,
				Expandable: true,
				Expanded:   true,
				Children:   sectionNodes,
				Actions: []widgets.Action{
					{Icon: "⋮", Command: "groupMenu"},
				},
			}
			roots = append(roots, root)
		} else {
			roots = append(roots, sectionNodes...)
		}
	}

	for pi, pg := range cp.PRGroups {
		id := workingNodeID(workNodePRRoot, pg.Dir, pg.Name, false)
		cp.workNodes[id] = workNodeRef{Dir: pg.Dir, Path: pg.Name, Kind: workNodePRRoot, Group: pi, PR: true}
		prRoot := &widgets.TreeNode{
			ID:         id,
			Label:      pg.Name,
			Expandable: true,
			Expanded:   true,
			Actions: []widgets.Action{
				{Icon: "⋮", Command: "prGroupMenu"},
			},
		}
		prRoot.Children = cp.fileNodes(pg.Dir, pg.Files, false, pi, true)
		clearTreeActions(prRoot.Children)
		roots = append(roots, prRoot)
	}

	for _, root := range roots {
		cp.restoreExpanded(root)
	}

	cp.Tree.SetItems(roots)
	if revealTreeSelection(cp.Tree, selected) || !selectedWasFile {
		return
	}
	fallbackID := workingNodeID(selectedFile.Kind, selectedFile.Dir, selectedFile.Status.Path, !selectedFile.Staged)
	revealTreeSelection(cp.Tree, fallbackID)
}

func clearTreeActions(nodes []*widgets.TreeNode) {
	for _, node := range nodes {
		node.Actions = nil
		clearTreeActions(node.Children)
	}
}

func (cp *ChangesPanel) fileNode(dir string, f git.FileStatus, staged bool, kind workNodeKind, group int, pr bool) *widgets.TreeNode {
	icon := ui.StatusBadge(f.Status)
	iconStyle := ui.StatusStyle(f.Status)
	actionIcon := "+"
	actionCmd := "stage"
	if staged {
		actionIcon = "−"
		actionCmd = "unstage"
	}
	id := workingNodeID(kind, dir, f.Path, staged)
	cp.workFiles[id] = workFileRef{Dir: dir, Status: f, Staged: staged, Kind: kind}
	cp.workNodes[id] = workNodeRef{Dir: dir, Path: f.Path, Staged: staged, Kind: kind, Group: group, PR: pr}
	return &widgets.TreeNode{
		ID:        id,
		Label:     f.Path,
		Icon:      icon,
		IconStyle: iconStyle,
		Actions: []widgets.Action{
			{Icon: actionIcon, Command: actionCmd},
		},
	}
}

func (cp *ChangesPanel) TotalChanges() int {
	n := 0
	for _, g := range cp.groups {
		n += len(g.Staged) + len(g.Unstaged)
	}
	return n
}

func (cp *ChangesPanel) commitFocusedGroup() {
	msg := cp.Input.Text()
	if msg == "" {
		return
	}
	dir := cp.selectedGroupDir()
	if dir == "" {
		for _, g := range cp.groups {
			if len(g.Staged) > 0 {
				dir = g.Dir
				break
			}
		}
	}
	if dir != "" && cp.OnCommit != nil {
		cp.OnCommit(dir, msg)
		cp.Input.Clear()
	}
}

func (cp *ChangesPanel) selectedGroupDir() string {
	node := cp.Tree.Selected()
	if node == nil {
		return ""
	}
	dir, _, _, ok := cp.parseFileNode(node)
	if ok {
		return dir
	}
	if ref, found := cp.workNodes[node.ID]; found && !ref.PR {
		return ref.Dir
	}
	return ""
}

func (cp *ChangesPanel) selectedInPR() bool {
	node := cp.Tree.Selected()
	if node == nil {
		return false
	}
	ref, found := cp.workNodes[node.ID]
	return found && ref.PR
}

func (cp *ChangesPanel) handleCommand(cmd string, node *widgets.TreeNode) {
	dir, status, staged, ok := cp.parseFileNode(node)
	switch cmd {
	case "activate":
		if ok {
			cp.openDiff(dir, status, staged, false)
			return
		}
		ref, found := cp.workNodes[node.ID]
		if found && ref.Kind == workNodePRRoot && ref.Group >= 0 && ref.Group < len(cp.PRGroups) && cp.OnOpenPRDetail != nil {
			cp.OnOpenPRDetail(cp.toUIChangesGroup(&cp.PRGroups[ref.Group]))
			return
		}
		node.Expanded = !node.Expanded
		cp.Tree.SetItems(cp.Tree.Config.Items)
	case "stage":
		if ok && !staged {
			cp.applied(git.Stage(dir, status.Path))
		}
	case "unstage":
		if ok && staged {
			cp.applied(git.Unstage(dir, status.Path))
		}
	case "stageAll":
		gi := cp.groupIndexFromNode(node)
		if gi >= 0 {
			cp.stageAllInGroup(gi)
		}
	case "unstageAll":
		gi := cp.groupIndexFromNode(node)
		if gi >= 0 {
			cp.unstageAllInGroup(gi)
		}
	case "discardAll":
		gi := cp.groupIndexFromNode(node)
		if gi >= 0 {
			cp.confirmDiscardAll(gi)
		}
	case "groupMenu":
		r := cp.Tree.GetRect()
		cp.handleMenu(node, r.X+r.W-2, r.Y+cp.Tree.SelectedIndex()-cp.Tree.ScrollTop())
	case "prGroupMenu":
		r := cp.Tree.GetRect()
		cp.handleMenu(node, r.X+r.W-2, r.Y+cp.Tree.SelectedIndex()-cp.Tree.ScrollTop())
	}
}

func (cp *ChangesPanel) handleKey(ev *tcell.EventKey) bool {
	if ev.Key() != tcell.KeyRune {
		return false
	}
	inPR := cp.selectedInPR()
	switch term.KeyRune(ev) {
	case 'r', 'R':
		if inPR {
			cp.refreshSelectedPR()
		} else if cp.OnRefresh != nil {
			cp.OnRefresh()
		} else {
			cp.Refresh()
		}
		return true
	case ' ', 's':
		if !inPR {
			cp.ToggleStageSelected()
		}
		return true
	case 'a', 'A':
		if !inPR {
			cp.stageAll()
		}
		return true
	case 'u', 'U':
		if !inPR {
			cp.unstageAll()
		}
		return true
	case 'd':
		if !inPR {
			cp.DiscardSelected()
		}
		return true
	case 'D':
		if !inPR {
			node := cp.Tree.Selected()
			if node != nil {
				gi := cp.groupIndexFromNode(node)
				if gi >= 0 {
					cp.confirmDiscardAll(gi)
				}
			}
		}
		return true
	case 'o', 'v':
		cp.ActivateSelected()
		return true
	case 'c':
		cp.OpenSelectedDiff(false)
		return true
	case 'e':
		cp.OpenSelectedDiff(true)
		return true
	}
	return false
}

func (cp *ChangesPanel) refreshSelectedPR() {
	node := cp.Tree.Selected()
	if node == nil {
		return
	}
	for _, pg := range cp.PRGroups {
		if strings.HasPrefix(node.ID, "pr:") && node.Label == pg.Name {
			if cp.OnRefreshPR != nil && pg.PRURL != "" {
				cp.RemovePRGroup(pg.Name)
				cp.OnRefreshPR(pg.PRURL)
			}
			return
		}
	}
	dir, _, _, ok := cp.parseFileNode(node)
	if ok {
		for _, pg := range cp.PRGroups {
			if pg.Dir == dir {
				if cp.OnRefreshPR != nil && pg.PRURL != "" {
					cp.RemovePRGroup(pg.Name)
					cp.OnRefreshPR(pg.PRURL)
				}
				return
			}
		}
	}
}

func (cp *ChangesPanel) handleMenu(node *widgets.TreeNode, sx, sy int) {
	dir, status, _, ok := cp.parseFileNode(node)
	if ok && cp.OnRightClick != nil {
		cp.OnRightClick(dir, status, sx, sy)
		return
	}
	if ref, found := cp.workNodes[node.ID]; found && ref.Kind == workNodePRRoot && ref.Group >= 0 && ref.Group < len(cp.PRGroups) {
		if cp.OnPRGroupMenu != nil {
			uiGroup := cp.toUIChangesGroup(&cp.PRGroups[ref.Group])
			cp.OnPRGroupMenu(uiGroup, sx, sy)
		}
		return
	}
	if ref, found := cp.workNodes[node.ID]; found && ref.Kind == workNodeRoot {
		if cp.OnGroupMenu != nil {
			cp.OnGroupMenu(ref.Dir, sx, sy)
		}
		return
	}
	cp.showPanelMenu(sx, sy)
}

func (cp *ChangesPanel) showPanelMenu(sx, sy int) {
	if cp.OnPanelMenu != nil {
		cp.OnPanelMenu(sx, sy)
	}
}

func (cp *ChangesPanel) openDiff(dir string, status git.FileStatus, staged bool, extended bool) {
	for _, pg := range cp.PRGroups {
		if pg.Dir == dir {
			if cp.OnOpenPRDiff != nil {
				uiGroup := cp.toUIChangesGroup(&pg)
				cp.OnOpenPRDiff(uiGroup, status, extended)
			}
			return
		}
	}
	if cp.OnOpenDiff != nil {
		cp.OnOpenDiff(dir, status, extended)
	}
}

func (cp *ChangesPanel) parseFileNode(node *widgets.TreeNode) (dir string, status git.FileStatus, staged bool, ok bool) {
	if node == nil {
		return
	}
	ref, found := cp.workFiles[node.ID]
	if !found {
		return "", git.FileStatus{}, false, false
	}
	return ref.Dir, ref.Status, ref.Staged, true
}

func (cp *ChangesPanel) groupIndexFromNode(node *widgets.TreeNode) int {
	if node != nil {
		if ref, ok := cp.workNodes[node.ID]; ok && !ref.PR {
			return ref.Group
		}
	}
	return -1
}

func (cp *ChangesPanel) stageAll() {
	var err error
	for _, g := range cp.groups {
		if e := git.Stage(g.Dir, filePaths(g.Unstaged)...); e != nil && err == nil {
			err = e
		}
	}
	cp.applied(err)
}

func (cp *ChangesPanel) unstageAll() {
	var err error
	for _, g := range cp.groups {
		if e := git.Unstage(g.Dir, filePaths(g.Staged)...); e != nil && err == nil {
			err = e
		}
	}
	cp.applied(err)
}

func (cp *ChangesPanel) stageAllInGroup(gi int) {
	if gi < 0 || gi >= len(cp.groups) {
		return
	}
	g := cp.groups[gi]
	cp.applied(git.Stage(g.Dir, filePaths(g.Unstaged)...))
}

func (cp *ChangesPanel) unstageAllInGroup(gi int) {
	if gi < 0 || gi >= len(cp.groups) {
		return
	}
	g := cp.groups[gi]
	cp.applied(git.Unstage(g.Dir, filePaths(g.Staged)...))
}

func (cp *ChangesPanel) confirmDiscard(dir string, f git.FileStatus) {
	if cp.OnConfirmDiscard == nil {
		return
	}
	msg := fmt.Sprintf("Discard changes to %s? This is irreversible.", f.Path)
	if f.Status == "?" {
		msg = fmt.Sprintf("Delete untracked file %s? This is irreversible.", f.Path)
	}
	cp.OnConfirmDiscard(msg, func() {
		if f.Status == "?" {
			cp.applied(git.DiscardUntracked(dir, f.Path))
		} else {
			cp.applied(git.Discard(dir, f.Path))
		}
	})
}

func (cp *ChangesPanel) confirmDiscardAll(gi int) {
	if cp.OnConfirmDiscard == nil || gi < 0 || gi >= len(cp.groups) {
		return
	}
	g := cp.groups[gi]
	msg := fmt.Sprintf("Discard all %d changes? This is irreversible.", len(g.Unstaged))
	cp.OnConfirmDiscard(msg, func() {
		untracked, tracked := discardPaths(g.Unstaged)
		err := git.DiscardUntracked(g.Dir, untracked...)
		if e := git.Discard(g.Dir, tracked...); e != nil && err == nil {
			err = e
		}
		cp.applied(err)
	})
}

func (cp *ChangesPanel) SelectedFile() (dir string, status git.FileStatus, ok bool) {
	if cp.commandContext != changesWorkingTree {
		return
	}
	node := cp.Tree.Selected()
	if node == nil {
		return
	}
	dir, status, _, ok = cp.parseFileNode(node)
	return
}

func (cp *ChangesPanel) SelectedFullPath() string {
	dir, status, ok := cp.SelectedFile()
	if !ok {
		return ""
	}
	return filepath.Join(dir, status.Path)
}

func (cp *ChangesPanel) SelectedGroup() *ui.ChangesGroup {
	node := cp.Tree.Selected()
	if node == nil {
		return nil
	}
	for _, pg := range cp.PRGroups {
		if node.Label == pg.Name {
			return cp.toUIChangesGroup(&pg)
		}
	}
	_, _, _, ok := cp.parseFileNode(node)
	if ok {
		dir, _, _ := cp.SelectedFile()
		for _, g := range cp.groups {
			if g.Dir == dir {
				return &ui.ChangesGroup{
					Dir:      g.Dir,
					Name:     g.Name,
					Staged:   g.Staged,
					Unstaged: g.Unstaged,
				}
			}
		}
		for _, pg := range cp.PRGroups {
			if pg.Dir == dir {
				return cp.toUIChangesGroup(&pg)
			}
		}
	}
	return nil
}

func (cp *ChangesPanel) toUIChangesGroup(pg *prGroup) *ui.ChangesGroup {
	return &ui.ChangesGroup{
		Dir:       pg.Dir,
		Name:      pg.Name,
		Unstaged:  pg.Files,
		IsPR:      true,
		PRURL:     pg.PRURL,
		PRDiffs:   pg.PRDiffs,
		PROwner:   pg.PROwner,
		PRRepo:    pg.PRRepo,
		PRBaseSHA: pg.PRBaseSHA,
		PRHeadSHA: pg.PRHeadSHA,
	}
}

func (cp *ChangesPanel) AddPRGroup(name, url, owner, repo, baseSHA, headSHA string, files []git.FileStatus, diffs map[string]string) {
	cp.PRGroups = append(cp.PRGroups, prGroup{
		Dir:       "pr://" + name,
		Name:      name,
		Files:     files,
		PRURL:     url,
		PRDiffs:   diffs,
		PROwner:   owner,
		PRRepo:    repo,
		PRBaseSHA: baseSHA,
		PRHeadSHA: headSHA,
	})
	cp.multiRoot = len(cp.groups)+len(cp.PRGroups) > 1
	cp.buildTree()
}

func (cp *ChangesPanel) RemovePRGroup(name string) {
	var kept []prGroup
	for _, pg := range cp.PRGroups {
		if pg.Name != name {
			kept = append(kept, pg)
		}
	}
	cp.PRGroups = kept
	cp.multiRoot = len(cp.groups)+len(cp.PRGroups) > 1
	cp.buildTree()
}

func (cp *ChangesPanel) RemovePRGroups() {
	cp.PRGroups = nil
	cp.multiRoot = len(cp.groups)+len(cp.PRGroups) > 1
	cp.buildTree()
}

func (cp *ChangesPanel) DiscardSelected() {
	if cp.commandContext != changesWorkingTree {
		return
	}
	dir, status, _, ok := cp.parseFileNode(cp.Tree.Selected())
	if !ok || status.Staged {
		return
	}
	cp.confirmDiscard(dir, status)
}

func (cp *ChangesPanel) ToggleStageSelected() {
	if cp.commandContext != changesWorkingTree {
		return
	}
	node := cp.Tree.Selected()
	if node == nil {
		return
	}
	dir, status, staged, ok := cp.parseFileNode(node)
	if !ok {
		return
	}
	if staged {
		cp.applied(git.Unstage(dir, status.Path))
	} else {
		cp.applied(git.Stage(dir, status.Path))
	}
}

func (cp *ChangesPanel) OpenSelectedDiff(extended bool) {
	if cp.commandContext == changesCommitLog {
		cp.openCommitFile(cp.CommitLog.Selected(), extended)
		return
	}
	node := cp.Tree.Selected()
	if node == nil {
		return
	}
	dir, status, staged, ok := cp.parseFileNode(node)
	if !ok {
		return
	}
	cp.openDiff(dir, status, staged, extended)
}

func (cp *ChangesPanel) ActivateSelected() {
	if cp.commandContext == changesCommitLog {
		cp.openCommitLogNode(cp.CommitLog.Selected())
		return
	}
	if cp.selectedInPR() {
		cp.OpenSelectedDiff(false)
	} else {
		cp.OpenSelectedFile()
	}
}

func (cp *ChangesPanel) OpenSelectedFile() {
	if cp.OnOpenFile != nil {
		if path := cp.SelectedFullPath(); path != "" {
			cp.OnOpenFile(path)
		}
	}
}

func (cp *ChangesPanel) Groups() []ui.ChangesGroup {
	var result []ui.ChangesGroup
	for _, g := range cp.groups {
		result = append(result, ui.ChangesGroup{
			Dir:      g.Dir,
			Name:     g.Name,
			Staged:   g.Staged,
			Unstaged: g.Unstaged,
		})
	}
	for _, pg := range cp.PRGroups {
		result = append(result, *cp.toUIChangesGroup(&pg))
	}
	return result
}

func (cp *ChangesPanel) ClearInput(dir string) {
	for _, g := range cp.groups {
		if g.Dir == dir {
			cp.Input.Clear()
			return
		}
	}
}
