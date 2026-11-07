package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func writeFolderRepoFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// folderActionRepo seeds a repo with two changed folders plus one untracked
// file, which is the smallest shape that can tell folder scoping apart from
// staging everything.
func folderActionRepo(t *testing.T) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, "repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	runTreeGit(t, dir, "init", "-q")
	runTreeGit(t, dir, "config", "user.email", "alice@example.com")
	runTreeGit(t, dir, "config", "user.name", "alice")
	writeFolderRepoFile(t, dir, "src/a.go", "a\n")
	writeFolderRepoFile(t, dir, "src/deep/b.go", "b\n")
	writeFolderRepoFile(t, dir, "docs/c.md", "c\n")
	runTreeGit(t, dir, "add", "-A")
	runTreeGit(t, dir, "commit", "-qm", "seed")
	writeFolderRepoFile(t, dir, "src/a.go", "a changed\n")
	writeFolderRepoFile(t, dir, "src/deep/b.go", "b changed\n")
	writeFolderRepoFile(t, dir, "docs/c.md", "c changed\n")
	writeFolderRepoFile(t, dir, "src/untracked.go", "new\n")
	return dir
}

func stagedPaths(t *testing.T, dir string) []string {
	t.Helper()
	out := runTreeGit(t, dir, "diff", "--cached", "--name-only", "-z")
	return strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
}

func TestWorkingTreeFolderRowsCarryStageAndUnstageActions(t *testing.T) {
	cp := NewChangesPanel("/repo")
	cp.SetFileView(config.GitFileViewTree)
	cp.groups = []changesGroup{{
		Dir:      "/repo",
		Name:     "repo",
		Staged:   []git.FileStatus{{Status: "M", Path: "docs/c.md"}},
		Unstaged: []git.FileStatus{{Status: "M", Path: "src/a.go"}, {Status: "M", Path: "src/deep/b.go"}},
	}}
	cp.buildTree()

	unstaged := nodeWithLabel(cp.Tree.Config.Items, "src")
	if unstaged == nil || len(unstaged.Actions) != 1 {
		t.Fatalf("unstaged folder actions = %+v", unstaged)
	}
	if got := unstaged.Actions[0]; got.Command != "stageFolder" || got.Icon != string(icons.Rune(icons.Add)) {
		t.Fatalf("unstaged folder action = %+v", got)
	}

	staged := nodeWithLabel(cp.Tree.Config.Items, "docs")
	if staged == nil || len(staged.Actions) != 1 {
		t.Fatalf("staged folder actions = %+v", staged)
	}
	if got := staged.Actions[0]; got.Command != "unstageFolder" || got.Icon != string(icons.Rune(icons.Remove)) {
		t.Fatalf("staged folder action = %+v", got)
	}

	cp.SetFileView(config.GitFileViewList)
	if folder := nodeWithLabel(cp.Tree.Config.Items, "src"); folder != nil {
		t.Fatalf("list view still rendered a folder row: %+v", folder)
	}
}

func TestFolderRowStagesEveryFileBelowItIncludingUntracked(t *testing.T) {
	dir := folderActionRepo(t)
	cp := NewChangesPanel(dir)
	cp.SetFileView(config.GitFileViewTree)
	cp.Refresh()

	folder := nodeWithLabel(cp.Tree.Config.Items, "src")
	if folder == nil || !revealTreeSelection(cp.Tree, folder.ID) {
		t.Fatalf("src folder is not selectable: %+v", folder)
	}
	// Collapsed folders must stage the same set: the tree builds children eagerly.
	folder.Expanded = false
	cp.Tree.SetItems(cp.Tree.Config.Items)

	cp.ToggleStageSelected()

	if got, want := stagedPaths(t, dir), []string{"src/a.go", "src/deep/b.go", "src/untracked.go"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("staged paths = %q, want %q", got, want)
	}
}

func TestStagedFolderRowUnstagesEveryFileBelowIt(t *testing.T) {
	dir := folderActionRepo(t)
	runTreeGit(t, dir, "add", "-A")
	cp := NewChangesPanel(dir)
	cp.SetFileView(config.GitFileViewTree)
	cp.Refresh()

	folder := nodeWithLabel(cp.Tree.Config.Items, "src")
	if folder == nil || !revealTreeSelection(cp.Tree, folder.ID) {
		t.Fatalf("staged src folder is not selectable: %+v", folder)
	}
	cp.ToggleStageSelected()

	if got, want := stagedPaths(t, dir), []string{"docs/c.md"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("staged paths after folder unstage = %q, want %q", got, want)
	}
}

func TestFolderDiscardConfirmsAndTouchesOnlyThatFolder(t *testing.T) {
	dir := folderActionRepo(t)
	cp := NewChangesPanel(dir)
	cp.SetFileView(config.GitFileViewTree)
	cp.Refresh()

	var messages []string
	cp.OnConfirmDiscard = func(message string, onConfirm func()) {
		messages = append(messages, message)
		onConfirm()
	}
	folder := nodeWithLabel(cp.Tree.Config.Items, "src")
	if folder == nil || !revealTreeSelection(cp.Tree, folder.ID) {
		t.Fatalf("src folder is not selectable: %+v", folder)
	}
	cp.confirmDiscardFolder(folder)

	if len(messages) != 1 || !strings.Contains(messages[0], "src") || !strings.Contains(messages[0], "irreversible") {
		t.Fatalf("confirm messages = %q", messages)
	}
	if body, err := os.ReadFile(filepath.Join(dir, "src/a.go")); err != nil || string(body) != "a\n" {
		t.Fatalf("src/a.go = %q, %v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(dir, "src/deep/b.go")); err != nil || string(body) != "b\n" {
		t.Fatalf("src/deep/b.go = %q, %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "src/untracked.go")); !os.IsNotExist(err) {
		t.Fatalf("untracked file survived a folder discard: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(dir, "docs/c.md")); err != nil || string(body) != "c changed\n" {
		t.Fatalf("docs/c.md outside the folder was touched: %q, %v", body, err)
	}
}

func TestFolderActionsIgnoreFileAndPullRequestRows(t *testing.T) {
	dir := folderActionRepo(t)
	cp := NewChangesPanel(dir)
	cp.SetFileView(config.GitFileViewTree)
	cp.PRGroups = []prGroup{{Dir: dir, Name: "pull request", Files: []git.FileStatus{{Status: "M", Path: "src/a.go"}}}}
	cp.Refresh()

	file := nodeWithLabel(cp.Tree.Config.Items, "a.go")
	if file == nil {
		t.Fatal("missing working file row")
	}
	if _, _, _, ok := cp.SelectedFolder(); ok {
		t.Fatal("file row reported as a folder")
	}
	cp.stageFolder(file)

	prFolder := findTreeNode(cp.Tree.Config.Items, func(node *widgets.TreeNode) bool {
		ref, ok := cp.workNodes[node.ID]
		return ok && ref.Kind == workNodePRFolder
	})
	if prFolder == nil {
		t.Fatal("missing pull-request folder row")
	}
	if len(prFolder.Actions) != 0 {
		t.Fatalf("pull-request folder gained stage actions: %+v", prFolder.Actions)
	}
	cp.stageFolder(prFolder)

	if got := stagedPaths(t, dir); strings.Join(got, ",") != "" {
		t.Fatalf("folder actions staged paths from a non-folder row: %q", got)
	}
}

func TestFolderStagingRoutesThroughTheRepositoryProvider(t *testing.T) {
	root := t.TempDir()
	repo := &testSCMStaging{testSCMRepository: testSCMRepository{root: root}}
	cp := NewChangesPanelWithProvider(&testSCMProvider{repo: repo}, root)
	cp.OnStatusChanged = func() {}
	cp.Tree.Config.OnSelect = nil
	cp.SetFileView(config.GitFileViewTree)
	cp.groups = []changesGroup{{
		Repository: repo,
		Dir:        root,
		Name:       "repo",
		Unstaged:   []git.FileStatus{{Status: "M", Path: "src/a.go"}, {Status: "M", Path: "src/deep/b.go"}},
	}}
	cp.buildTree()

	folder := nodeWithLabel(cp.Tree.Config.Items, "src")
	if folder == nil || !revealTreeSelection(cp.Tree, folder.ID) {
		t.Fatalf("src folder is not selectable: %+v", folder)
	}
	cp.stageFolder(folder)
	if len(repo.staged) != 1 {
		t.Fatalf("staged through provider = %v", repo.staged)
	}
	slices.Sort(repo.staged[0])
	if !reflect.DeepEqual(repo.staged[0], []string{"src/a.go", "src/deep/b.go"}) {
		t.Fatalf("staged paths through provider = %v", repo.staged)
	}

	cp.groups = []changesGroup{{
		Repository: repo,
		Dir:        root,
		Name:       "repo",
		Staged:     []git.FileStatus{{Status: "M", Path: "src/a.go"}, {Status: "M", Path: "src/deep/b.go"}},
	}}
	cp.buildTree()
	folder = nodeWithLabel(cp.Tree.Config.Items, "src")
	cp.unstageFolder(folder)
	if len(repo.unstaged) != 1 {
		t.Fatalf("unstaged through provider = %v", repo.unstaged)
	}
	slices.Sort(repo.unstaged[0])
	if !reflect.DeepEqual(repo.unstaged[0], []string{"src/a.go", "src/deep/b.go"}) {
		t.Fatalf("unstaged paths through provider = %v", repo.unstaged)
	}
}

func TestReadOnlyRepositoryKeepsFolderRowsActionless(t *testing.T) {
	root := t.TempDir()
	provider := &testSCMProvider{repo: testSCMRepository{root: root}}
	cp := NewChangesPanelWithProvider(provider, root)
	cp.Tree.Config.OnSelect = nil
	cp.SetFileView(config.GitFileViewTree)
	cp.groups = []changesGroup{{
		Repository: provider.repo,
		Dir:        root,
		Name:       "repo",
		Unstaged:   []git.FileStatus{{Status: "M", Path: "src/a.go"}, {Status: "M", Path: "src/deep/b.go"}},
	}}
	cp.buildTree()

	folder := nodeWithLabel(cp.Tree.Config.Items, "src")
	if folder == nil {
		t.Fatal("missing folder row")
	}
	if len(folder.Actions) != 0 {
		t.Fatalf("read-only folder gained actions: %+v", folder.Actions)
	}
	cp.stageFolder(folder)
	if err := cp.stagePaths(root, false, "src/deep/b.go"); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("unsupported staging: %v", err)
	}
}
