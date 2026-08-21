package app

import (
	"fmt"
	"path/filepath"
	"strings"

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
	// commitFiles caches a commit's file list. A commit's contents never
	// change, so an entry can never go stale — only numerous, hence the bound.
	commitFiles      map[string][]git.FileStatus
	commitFilesOrder []string
	// commitFilesPending marks reads already in flight, so repeated expands of
	// one commit do not each start their own git process.
	commitFilesPending map[string]bool
	logCommits         map[string]commitFileRef
	logFiles           map[string]commitFileRef
	// statusGen and logGen let a finished read tell whether a newer one has
	// already superseded it.
	statusGen int
	logGen    int
	// pendingLogSelection is a selection that could not be restored yet because
	// the node it names is a commit's child and those children are still being
	// read.
	pendingLogSelection string
	// logExpanded and logSelected outlive any one repo's log, so switching
	// between roots in a workspace and back returns to what was open. Keys are
	// full hashes, which do not collide across repos.
	logExpanded map[string]bool
	logSelected map[string]string

	OnOpenDiff       func(dir string, status git.FileStatus, extended bool)
	OnOpenCommitDiff func(dir, ref, short string, status git.FileStatus, extended bool)
	OnOpenCommit     func(dir, ref, short string)
	OnOpenPRDiff     func(group *ui.ChangesGroup, status git.FileStatus, extended bool)
	OnOpenFile       func(path string)
	OnRightClick     func(dir string, status git.FileStatus, screenX, screenY int)
	OnCommit         func(dir string, message string)
	OnGroupMenu      func(dir string, screenX, screenY int)
	OnPRGroupMenu    func(group *ui.ChangesGroup, screenX, screenY int)
	OnRefreshPR      func(url string)
	OnConfirmDiscard func(message string, onConfirm func())
	OnError          func(message string)
	OnRefreshed      func()

	PRGroups []prGroup
}

type changesCommandContext uint8

const (
	changesWorkingTree changesCommandContext = iota
	changesCommitLog

	changesHistoryDefaultHeight = 8 // title plus the log's former seven rows
	changesHistoryMinHeight     = 4 // title plus three usable log rows
	changesWorkingTreeMinHeight = 5 // input, divider, and three tree rows
)

type changesGroup struct {
	Dir      string
	Name     string
	Staged   []git.FileStatus
	Unstaged []git.FileStatus
}

// commitFileRef ties a commit-log node back to the file it stands for. The
// panel keeps a map rather than encoding dir, hash and path into the node ID:
// all three can contain a colon, and parseFileNode's reverse scan is already at
// its limit with two fields.
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

func NewChangesPanel(dirs ...string) *ChangesPanel {
	cp := &ChangesPanel{
		Dirs:        dirs,
		multiRoot:   len(dirs) > 1,
		expanded:    make(map[string]bool),
		commitFiles: make(map[string][]git.FileStatus),
		logCommits:  make(map[string]commitFileRef),
		logFiles:    make(map[string]commitFileRef),
		logExpanded: make(map[string]bool),
		logSelected: make(map[string]string),

		commitFilesPending: make(map[string]bool),
	}

	cp.Input = widgets.NewInputWidget(widgets.InputConfig{
		Placeholder: "Message",
		Bordered:    false,
		OnSubmit: func(text string) {
			cp.commitFocusedGroup()
		},
	})

	cp.Tree = widgets.NewTreeWidget(widgets.TreeConfig{
		Indent:       1,
		EmptyText:    "No changes",
		TruncateLeft: true,
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
				cp.openCommitLogNode(node)
			}
		},
		OnKey: func(ev *tcell.EventKey, node *widgets.TreeNode) bool {
			return cp.handleCommitLogKey(ev, node)
		},
	})

	logTitle := widgets.NewTitleWidget(widgets.TitleConfig{Title: "Commit History"})

	logBox := &widgets.BoxWidget{}
	logBox.Child = cp.CommitLog

	divTop := widgets.NewDividerWidget(widgets.DividerConfig{})

	top := widgets.NewVStackWidget(cp.Input, divTop, cp.Tree)
	bottom := widgets.NewVStackWidget(logTitle, logBox)

	cp.Split = ui.NewContentSplitWidget()
	cp.Split.Top = top
	cp.Split.Bottom = bottom
	cp.Split.ShowBottom = true
	cp.Split.BottomH = changesHistoryDefaultHeight
	cp.Split.MinBottomH = changesHistoryMinHeight
	cp.Split.MinTopH = changesWorkingTreeMinHeight
	cp.Split.OnResize = func(height int) {
		// Like the sidebar width, this value lives for the panel's lifetime but is
		// not written to settings. The split itself applies both child minimums.
		cp.Split.BottomH = height
	}

	cp.Adapter = ui.NewWidgetAdapter(cp.Split)

	// No refresh here. At construction there is no screen to come back through,
	// so the scan would run inline and hold up startup by however long `git
	// status` takes. App.Init hands the panel a screen and refreshes then.
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
	cp.Refresh()
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
	cp.statusGen++
	gen := cp.statusGen
	dirs := append([]string(nil), cp.Dirs...)
	read := func() *ChangesStatusResult {
		return &ChangesStatusResult{Gen: gen, Groups: readChangesGroups(dirs)}
	}
	if cp.Screen == nil {
		cp.ApplyStatus(read())
		return
	}
	screen := cp.Screen
	go func() {
		screen.PostEvent(tcell.NewEventInterrupt(read()))
	}()
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
		cp.logGen++
		cp.saveCommitLogState()
		cp.lastLogDir = ""
		cp.logDir = ""
		cp.CommitLog.SetItems(nil)
		return
	}
	if dir == cp.lastLogDir {
		return
	}
	cp.lastLogDir = dir
	cp.logGen++
	gen := cp.logGen
	if cp.Screen == nil {
		cp.ApplyCommitLog(readCommitLog(dir, gen))
		return
	}
	screen := cp.Screen
	go func() {
		screen.PostEvent(tcell.NewEventInterrupt(readCommitLog(dir, gen)))
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
		if node.Expanded {
			cp.logExpanded[node.ID] = true
		} else {
			delete(cp.logExpanded, node.ID)
		}
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
		r := readCommitFiles(dir, ref, short, parentID)
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
	nodes := make([]*widgets.TreeNode, 0, len(files))
	for _, f := range files {
		id := fmt.Sprintf("cfile:%s:%s", parentID, f.Path)
		cp.logFiles[id] = commitFileRef{Dir: dir, Ref: ref, Short: short, Status: f}
		nodes = append(nodes, &widgets.TreeNode{
			ID:           id,
			Label:        f.Path,
			Icon:         ui.StatusBadge(f.Status),
			IconStyle:    ui.StatusStyle(f.Status),
			TruncateLeft: true,
		})
	}
	return nodes
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
		cp.Refresh()
		return true
	case 'c', 'o', 'v':
		cp.openCommitFile(node, false)
		return true
	case 'e':
		cp.openCommitFile(node, true)
		return true
	}
	return false
}

func (cp *ChangesPanel) saveExpanded() {
	for _, node := range cp.Tree.FlatList() {
		if node.Expandable || len(node.Children) > 0 {
			cp.expanded[node.ID] = node.Expanded
		}
	}
}

func (cp *ChangesPanel) restoreExpanded(node *widgets.TreeNode) {
	if exp, ok := cp.expanded[node.ID]; ok {
		node.Expanded = exp
	}
	for _, child := range node.Children {
		cp.restoreExpanded(child)
	}
}

func (cp *ChangesPanel) buildTree() {
	var roots []*widgets.TreeNode

	for gi, g := range cp.groups {
		var sectionNodes []*widgets.TreeNode

		if len(g.Staged) > 0 {
			stagedNode := &widgets.TreeNode{
				ID:         fmt.Sprintf("staged:%d", gi),
				Label:      fmt.Sprintf("Staged (%d)", len(g.Staged)),
				Expandable: true,
				Expanded:   true,
				Muted:      true,
				Actions: []widgets.Action{
					{Icon: "−", Command: "unstageAll"},
				},
			}
			for _, f := range g.Staged {
				child := cp.fileNode(g.Dir, f, true)
				stagedNode.Children = append(stagedNode.Children, child)
			}
			sectionNodes = append(sectionNodes, stagedNode)
		}

		if len(g.Unstaged) > 0 {
			changesNode := &widgets.TreeNode{
				ID:         fmt.Sprintf("changes:%d", gi),
				Label:      fmt.Sprintf("Changes (%d)", len(g.Unstaged)),
				Expandable: true,
				Expanded:   true,
				Muted:      true,
				Actions: []widgets.Action{
					{Icon: "✕", Command: "discardAll"},
					{Icon: "+", Command: "stageAll"},
				},
			}
			for _, f := range g.Unstaged {
				child := cp.fileNode(g.Dir, f, false)
				changesNode.Children = append(changesNode.Children, child)
			}
			sectionNodes = append(sectionNodes, changesNode)
		}

		if cp.multiRoot {
			root := &widgets.TreeNode{
				ID:         fmt.Sprintf("root:%d", gi),
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
		prRoot := &widgets.TreeNode{
			ID:         fmt.Sprintf("pr:%d", pi),
			Label:      pg.Name,
			Expandable: true,
			Expanded:   true,
			Actions: []widgets.Action{
				{Icon: "⋮", Command: "prGroupMenu"},
			},
		}
		for _, f := range pg.Files {
			child := cp.fileNode(pg.Dir, f, false)
			child.Actions = nil
			prRoot.Children = append(prRoot.Children, child)
		}
		roots = append(roots, prRoot)
	}

	for _, root := range roots {
		cp.restoreExpanded(root)
	}

	cp.Tree.SetItems(roots)
}

func (cp *ChangesPanel) fileNode(dir string, f git.FileStatus, staged bool) *widgets.TreeNode {
	icon := ui.StatusBadge(f.Status)
	iconStyle := ui.StatusStyle(f.Status)
	actionIcon := "+"
	actionCmd := "stage"
	if staged {
		actionIcon = "−"
		actionCmd = "unstage"
	}
	return &widgets.TreeNode{
		ID:        fmt.Sprintf("file:%s:%s:%v", dir, f.Path, staged),
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
	gi := cp.groupIndexFromNode(node)
	if gi >= 0 && gi < len(cp.groups) {
		return cp.groups[gi].Dir
	}
	if strings.HasPrefix(node.ID, "root:") {
		var idx int
		if _, err := fmt.Sscanf(node.ID, "root:%d", &idx); err == nil && idx < len(cp.groups) {
			return cp.groups[idx].Dir
		}
	}
	return ""
}

func (cp *ChangesPanel) selectedInPR() bool {
	node := cp.Tree.Selected()
	if node == nil {
		return false
	}
	dir, _, _, ok := cp.parseFileNode(node)
	if ok {
		for _, pg := range cp.PRGroups {
			if pg.Dir == dir {
				return true
			}
		}
		return false
	}
	return strings.HasPrefix(node.ID, "pr:")
}

func (cp *ChangesPanel) handleCommand(cmd string, node *widgets.TreeNode) {
	dir, status, staged, ok := cp.parseFileNode(node)
	switch cmd {
	case "activate":
		if ok {
			cp.openDiff(dir, status, staged, false)
		}
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
	for _, pg := range cp.PRGroups {
		if node.Label == pg.Name {
			if cp.OnPRGroupMenu != nil {
				uiGroup := cp.toUIChangesGroup(&pg)
				cp.OnPRGroupMenu(uiGroup, sx, sy)
			}
			return
		}
	}
	for gi, g := range cp.groups {
		if node.ID == fmt.Sprintf("root:%d", gi) {
			if cp.OnGroupMenu != nil {
				cp.OnGroupMenu(g.Dir, sx, sy)
			}
			return
		}
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
	id := node.ID
	if len(id) < 6 || id[:5] != "file:" {
		return
	}
	rest := id[5:]
	lastColon := -1
	for i := len(rest) - 1; i >= 0; i-- {
		if rest[i] == ':' {
			lastColon = i
			break
		}
	}
	if lastColon < 0 {
		return
	}
	s := rest[lastColon+1:] == "true"
	rest = rest[:lastColon]

	secondLastColon := -1
	for i := len(rest) - 1; i >= 0; i-- {
		if rest[i] == ':' {
			secondLastColon = i
			break
		}
	}
	if secondLastColon < 0 {
		return
	}
	d := rest[:secondLastColon]
	path := rest[secondLastColon+1:]

	for _, g := range cp.groups {
		if g.Dir == d {
			files := g.Unstaged
			if s {
				files = g.Staged
			}
			for _, f := range files {
				if f.Path == path {
					return d, f, s, true
				}
			}
		}
	}
	for _, pg := range cp.PRGroups {
		if pg.Dir == d {
			for _, f := range pg.Files {
				if f.Path == path {
					return d, f, false, true
				}
			}
		}
	}
	return
}

func (cp *ChangesPanel) groupIndexFromNode(node *widgets.TreeNode) int {
	var gi int
	if _, err := fmt.Sscanf(node.ID, "changes:%d", &gi); err == nil {
		return gi
	}
	if _, err := fmt.Sscanf(node.ID, "staged:%d", &gi); err == nil {
		return gi
	}
	if _, err := fmt.Sscanf(node.ID, "root:%d", &gi); err == nil {
		return gi
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
