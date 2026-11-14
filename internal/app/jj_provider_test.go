package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/jj"
	"github.com/eugenioenko/ttt/internal/jjtest"
	"github.com/eugenioenko/ttt/internal/scm"
)

func jjChangesRepo(t *testing.T, initArgs ...string) string {
	t.Helper()
	dir := jjtest.Init(t)
	if len(initArgs) > 0 {
		dir = t.TempDir()
		jjtest.Run(t, dir, append([]string{"git", "init"}, initArgs...)...)
	}
	jjtest.Write(t, dir, "src/a.go", "a\n")
	jjtest.Run(t, dir, "commit", "-m", "seed")
	jjtest.Write(t, dir, "src/a.go", "a changed\n")
	jjtest.Write(t, dir, "new.txt", "new\n")
	return dir
}

func TestChangesPanelPresentsJJRepositories(t *testing.T) {
	for name, args := range map[string][]string{"native": nil, "colocated": {"--colocate"}} {
		t.Run(name, func(t *testing.T) {
			dir := jjChangesRepo(t, args...)
			cp := NewChangesPanel(dir)
			cp.Refresh()

			if len(cp.groups) != 1 || !jj.IsRepository(cp.groups[0].Repository) {
				t.Fatalf("groups = %#v", cp.groups)
			}
			section := nodeWithLabel(cp.Tree.Config.Items, i18n.Format("changes.changes", i18n.Args{"unstagedCount": 2}))
			if section == nil {
				t.Fatalf("missing changes section in %#v", cp.Tree.Config.Items)
			}
			if staged := nodeWithLabel(cp.Tree.Config.Items, i18n.Format("changes.stagedHeading", i18n.Args{"stagedCount": 0})); staged != nil {
				t.Fatalf("jj rendered a staged section: %+v", staged)
			}
			for _, action := range section.Actions {
				if action.Command == "stageAll" {
					t.Fatalf("jj section offers staging: %+v", section.Actions)
				}
			}
			for _, child := range section.Children {
				for _, action := range child.Actions {
					if action.Command == "stage" {
						t.Fatalf("jj file offers staging: %+v", child)
					}
				}
			}

			var messages []string
			for _, node := range cp.CommitLog.Config.Items {
				messages = append(messages, node.Label)
			}
			if !strings.Contains(strings.Join(messages, "\n"), "seed") {
				t.Fatalf("history = %q", messages)
			}
		})
	}
}

func TestChangesPanelKeepsGitRepositoriesOnGit(t *testing.T) {
	dir := folderActionRepo(t)
	cp := NewChangesPanel(dir)
	cp.Refresh()
	if len(cp.groups) != 1 || cp.groups[0].Repository == nil || jj.IsRepository(cp.groups[0].Repository) {
		t.Fatalf("git repository discovered as %#v", cp.groups)
	}
}

func TestChangesPanelCommitsAndDiscardsThroughJJ(t *testing.T) {
	dir := jjChangesRepo(t)
	cp := NewChangesPanel(dir)
	cp.Refresh()

	cp.applied(cp.discardChanges(dir, []scm.Change{{Path: "new.txt", Kind: scm.Added, Group: scm.WorkingTree}}))
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("discard kept the added file: %v", err)
	}
	if got := cp.TotalChanges(); got != 1 {
		t.Fatalf("changes after discard = %d", got)
	}

	if err := cp.commitOp(dir, "from the box").Run(dir); err != nil {
		t.Fatal(err)
	}
	cp.Refresh()
	if got := cp.TotalChanges(); got != 0 {
		t.Fatalf("changes after commit = %d", got)
	}
	if first := cp.CommitLog.Config.Items[1].Label; !strings.Contains(first, "from the box") {
		t.Fatalf("latest history entry = %q", first)
	}
}

func TestChangesPanelDiscardsGitChangesThroughGit(t *testing.T) {
	dir := folderActionRepo(t)
	cp := NewChangesPanel(dir)
	cp.Refresh()
	err := cp.discardChanges(dir, []scm.Change{
		{Path: "src/a.go", Kind: scm.Modified, Group: scm.WorkingTree},
		{Path: "src/untracked.go", Kind: scm.Untracked, Group: scm.WorkingTree},
	})
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "src/a.go")); string(data) != "a\n" {
		t.Fatalf("tracked file = %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "src/untracked.go")); !os.IsNotExist(err) {
		t.Fatalf("untracked file kept: %v", err)
	}
	if op := cp.commitOp(dir, "msg"); op.Name != "commit" {
		t.Fatalf("git commit op = %+v", op)
	}
}

func TestJJDiscardOfCopyKeepsSourceEdits(t *testing.T) {
	dir := jjtest.Init(t)
	jjtest.Write(t, dir, "src.txt", "source\n")
	jjtest.Run(t, dir, "commit", "-m", "base")
	jjtest.Write(t, dir, "src.txt", "source edited\n")
	jjtest.Write(t, dir, "copy.txt", "source\n")
	cp := NewChangesPanel(dir)
	cp.Refresh()
	err := cp.discardChanges(dir, []scm.Change{{Path: "copy.txt", OldPath: "src.txt", Kind: scm.Copied, Group: scm.WorkingTree}})
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "src.txt")); string(data) != "source edited\n" {
		t.Fatalf("copy discard reverted its source: %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "copy.txt")); !os.IsNotExist(err) {
		t.Fatalf("copy kept: %v", err)
	}
}

func TestGitOnlyRepoOpsSkipJJWorkspaces(t *testing.T) {
	jjDir := jjChangesRepo(t, "--colocate")
	gitDir := folderActionRepo(t)
	a := &App{Changes: NewChangesPanel(jjDir, gitDir)}
	a.Changes.Refresh()

	for _, op := range []RepoOp{OpPull, OpPush, OpCommit("m"), OpCreateBranch("b")} {
		task := a.withoutJJDirs(RepoTask{Dirs: []string{jjDir, gitDir}, Ops: []RepoOp{op}})
		if len(task.Dirs) != 1 || task.Dirs[0] != gitDir {
			t.Fatalf("%s dirs = %q", op.Name, task.Dirs)
		}
	}
	if task := a.withoutJJDirs(RepoTask{Dirs: []string{jjDir}, Ops: []RepoOp{a.Changes.commitOp(jjDir, "m")}}); len(task.Dirs) != 1 {
		t.Fatalf("jj commit was filtered: %q", task.Dirs)
	}
	if !a.isJJDir(filepath.Join(jjDir, "src")) {
		t.Fatal("subdirectory of an undiscovered jj workspace is not jj")
	}
}

func TestGitOnlyGuardSeesWorkspacesInitializedAfterRefresh(t *testing.T) {
	dir := folderActionRepo(t)
	a := &App{Changes: NewChangesPanel(dir)}
	a.Changes.Refresh()
	if a.isJJDir(dir) {
		t.Fatal("git repository reported as jj")
	}
	jjtest.Require(t)
	jjtest.Isolate(t)
	jjtest.Run(t, dir, "git", "init", "--colocate")
	if !a.isJJDir(dir) {
		t.Fatal("cached git repository hid a newly colocated jj workspace")
	}
}

func TestStagingKeysExplainJJHasNoStagingArea(t *testing.T) {
	dir := jjChangesRepo(t)
	cp := NewChangesPanel(dir)
	cp.Refresh()
	var errs []string
	cp.OnError = func(msg string) { errs = append(errs, msg) }
	section := nodeWithLabel(cp.Tree.Config.Items, i18n.Format("changes.changes", i18n.Args{"unstagedCount": 2}))
	if section == nil || !revealTreeSelection(cp.Tree, section.Children[0].ID) {
		t.Fatal("no jj file row to select")
	}
	cp.ToggleStageSelected()
	if want := i18n.Text("status.notAvailableInJJRepositories"); len(errs) != 1 || errs[0] != want {
		t.Fatalf("errors = %q, want %q", errs, want)
	}
	if got := cp.TotalChanges(); got != 2 {
		t.Fatalf("changes after staging key = %d", got)
	}
}
