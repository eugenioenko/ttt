package app

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/jj"
	"github.com/eugenioenko/ttt/internal/jjtest"
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
