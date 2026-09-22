package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func TestExplorerGitStylesColorsFilesByStatus(t *testing.T) {
	dir := filepath.FromSlash("/repo")
	groups := []changesGroup{
		{
			Dir: dir,
			Unstaged: []git.FileStatus{
				{Path: "modified.go", Status: "M"},
				{Path: "untracked.go", Status: "?"},
				{Path: "src/deleted.go", Status: "D"},
			},
		},
	}

	styles := explorerGitStyles(groups)

	cases := map[string]term.Style{
		filepath.Join(dir, "modified.go"):    term.StyleWarning,
		filepath.Join(dir, "untracked.go"):   term.StyleSuccess,
		filepath.Join(dir, "src/deleted.go"): term.StyleDanger,
	}
	for path, want := range cases {
		if got := styles[path]; got != want {
			t.Errorf("styles[%q] = %v, want %v", path, got, want)
		}
	}
}

func TestExplorerGitStylesDimsStagedFiles(t *testing.T) {
	dir := filepath.FromSlash("/repo")
	groups := []changesGroup{
		{
			Dir: dir,
			Staged: []git.FileStatus{
				{Path: "staged.go", Status: "M", Staged: true},
			},
			Unstaged: []git.FileStatus{
				{Path: "pending.go", Status: "M"},
			},
		},
	}

	styles := explorerGitStyles(groups)

	if got := styles[filepath.Join(dir, "staged.go")]; got != term.StyleWarningStaged {
		t.Errorf("staged.go = %v, want dimmed %v", got, term.StyleWarningStaged)
	}
	if got := styles[filepath.Join(dir, "pending.go")]; got != term.StyleWarning {
		t.Errorf("pending.go = %v, want live %v", got, term.StyleWarning)
	}
}

func TestExplorerGitStylesFolderPrefersLiveOverStagedInSameCategory(t *testing.T) {
	dir := filepath.FromSlash("/repo")
	groups := []changesGroup{
		{
			Dir: dir,
			Staged: []git.FileStatus{
				{Path: "src/staged.go", Status: "M", Staged: true},
			},
			Unstaged: []git.FileStatus{
				{Path: "src/pending.go", Status: "M"},
			},
		},
	}

	styles := explorerGitStyles(groups)

	folder := filepath.Join(dir, "src")
	if got := styles[folder]; got != term.StyleWarning {
		t.Errorf("styles[%q] = %v, want the live (non-dimmed) %v to win", folder, got, term.StyleWarning)
	}
}

func TestExplorerGitStylesPropagatesToAncestorFolders(t *testing.T) {
	dir := filepath.FromSlash("/repo")
	groups := []changesGroup{
		{
			Dir: dir,
			Unstaged: []git.FileStatus{
				{Path: "src/pkg/modified.go", Status: "M"},
			},
		},
	}

	styles := explorerGitStyles(groups)

	for _, ancestor := range []string{
		filepath.Join(dir, "src/pkg"),
		filepath.Join(dir, "src"),
		dir,
	} {
		if got := styles[ancestor]; got != term.StyleWarning {
			t.Errorf("styles[%q] = %v, want %v", ancestor, got, term.StyleWarning)
		}
	}
}

func TestExplorerGitStylesFolderTakesHighestPriorityStatus(t *testing.T) {
	dir := filepath.FromSlash("/repo")
	groups := []changesGroup{
		{
			Dir: dir,
			Unstaged: []git.FileStatus{
				{Path: "src/added.go", Status: "?"},
				{Path: "src/conflicted.go", Status: "U"},
				{Path: "src/modified.go", Status: "M"},
			},
		},
	}

	styles := explorerGitStyles(groups)

	folder := filepath.Join(dir, "src")
	if got := styles[folder]; got != term.StyleGitConflict {
		t.Errorf("styles[%q] = %v, want %v (conflict should win)", folder, got, term.StyleGitConflict)
	}
}

func TestNavigationPanelApplyGitStatusColorsLoadedNodes(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootPath, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"src/changed.go", "src/plain.go"} {
		if err := os.WriteFile(filepath.Join(rootPath, filepath.FromSlash(path)), []byte(path), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runTreeGit(t, rootPath, "init", "-q")

	explorer := NewNavigationPanel(config.DefaultExplorerSettings(), config.IconsNone, rootPath)
	explorer.ExpandAll()

	srcDir := filepath.Join(rootPath, "src")
	changedFile := filepath.Join(srcDir, "changed.go")
	styles := map[string]term.Style{
		changedFile: term.StyleWarning,
		srcDir:      term.StyleWarning,
	}

	explorer.ApplyGitStatus(styles)

	changedNode := findTreeNode(explorer.Tree.Config.Items, func(n *widgets.TreeNode) bool { return n.ID == changedFile })
	if changedNode == nil || changedNode.LabelStyle != term.StyleWarning {
		t.Fatalf("expected changed.go to carry StyleWarning, got %+v", changedNode)
	}
	plainNode := findTreeNode(explorer.Tree.Config.Items, func(n *widgets.TreeNode) bool { return n.Label == "plain.go" })
	if plainNode == nil || plainNode.LabelStyle != term.StyleDefault {
		t.Fatalf("expected plain.go to stay uncolored, got %+v", plainNode)
	}
	srcNode := findTreeNode(explorer.Tree.Config.Items, func(n *widgets.TreeNode) bool { return n.ID == srcDir })
	if srcNode == nil || srcNode.LabelStyle != term.StyleWarning {
		t.Fatalf("expected src/ folder to carry StyleWarning, got %+v", srcNode)
	}

	explorer.Settings.GitStatusColors = false
	explorer.ApplyGitStatus(styles)
	if changedNode.LabelStyle != term.StyleDefault {
		t.Fatalf("disabling the setting should clear LabelStyle, got %v", changedNode.LabelStyle)
	}
}

// A `git rm --cached` leaves the file on disk, reported as both a staged
// delete and an untracked add, so the tree still has a row to color.
func TestExplorerGitStylesRanksStagedDeleteOverUntrackedOnSamePath(t *testing.T) {
	dir := filepath.FromSlash("/repo")
	groups := []changesGroup{
		{
			Dir:      dir,
			Staged:   []git.FileStatus{{Path: "cached.go", Status: "D", Staged: true}},
			Unstaged: []git.FileStatus{{Path: "cached.go", Status: "?"}},
		},
	}

	styles := explorerGitStyles(groups)

	if got := styles[filepath.Join(dir, "cached.go")]; got != term.StyleDangerStaged {
		t.Errorf("cached.go = %v, want %v", got, term.StyleDangerStaged)
	}
}

func TestExplorerGitStylesColorsFolderOfDeletedFile(t *testing.T) {
	dir := filepath.FromSlash("/repo")
	groups := []changesGroup{
		{
			Dir:      dir,
			Unstaged: []git.FileStatus{{Path: "src/gone.go", Status: "D"}},
		},
	}

	styles := explorerGitStyles(groups)

	// The file itself is off disk and has no row, but its folder still does.
	if got := styles[filepath.Join(dir, "src")]; got != term.StyleDanger {
		t.Errorf("src/ = %v, want %v", got, term.StyleDanger)
	}
}

func TestExplorerGitStylesStopsAtRepositoryRoot(t *testing.T) {
	dir := filepath.FromSlash("/repo/nested")
	groups := []changesGroup{
		{
			Dir:      dir,
			Unstaged: []git.FileStatus{{Path: "src/modified.go", Status: "M"}},
		},
	}

	styles := explorerGitStyles(groups)

	if _, ok := styles[filepath.FromSlash("/repo")]; ok {
		t.Error("walked past the repository root into its parent")
	}
	if _, ok := styles[string(filepath.Separator)]; ok {
		t.Error("walked all the way to the filesystem root")
	}
	if got := styles[dir]; got != term.StyleWarning {
		t.Errorf("styles[%q] = %v, want the root itself colored %v", dir, got, term.StyleWarning)
	}
}
