package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/workspace"
)

var githubShorthand = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9_.-]+$`)

// owner/repo goes through gh when it is installed, so private repos use the
// credentials gh already has.
func cloneSource(input string, haveGH bool) (repo, name string, viaGH bool) {
	repo = strings.TrimSpace(input)
	shorthand := githubShorthand.MatchString(repo)
	if shorthand && !haveGH {
		repo = "https://github.com/" + repo
	}
	name = strings.TrimSuffix(strings.TrimRight(repo, "/"), ".git")
	if i := strings.LastIndexAny(name, "/:"); i >= 0 {
		name = name[i+1:]
	}
	return repo, name, shorthand && haveGH
}

func (a *App) CloneRepository() {
	a.ShowInputDialogEx(i18n.Text("clone.cloneRepository"), i18n.Text("clone.uRLOrOwnerRepo"), "", i18n.Text("general.next"), func(input string) {
		if strings.TrimSpace(input) == "" {
			return
		}
		_, ghErr := exec.LookPath("gh")
		repo, name, viaGH := cloneSource(input, ghErr == nil)
		if name == "" {
			a.StatusError(i18n.Format("status.cannotTellTheFolderNameFrom", i18n.Args{"input": input}))
			return
		}
		a.ShowFolderPicker(i18n.Text("clone.cloneInto"), i18n.Text("clone.clone"), "", func(parent string) {
			parent, err := filepath.Abs(workspace.ExpandPath(parent))
			if err != nil {
				a.StatusError(i18n.Format("status.error", i18n.Args{"error": err.Error()}))
				return
			}
			dest := filepath.Join(parent, name)
			if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
				a.StatusError(i18n.Format("status.pathAlreadyExists", i18n.Args{"dest": dest}))
				return
			}
			clone := func(string) error { return git.Clone(repo, dest) }
			if viaGH {
				clone = func(string) error { return github.CloneRepo(repo, dest) }
			}
			a.RunRepoTask(RepoTask{
				Progress: i18n.Format("clone.cloning", i18n.Args{"name": name}),
				Done:     i18n.Format("clone.cloned", i18n.Args{"name": name}),
				Dirs:     []string{parent},
				Ops:      []RepoOp{{Name: "clone", Run: clone}},
				OnDone:   func() { a.openFolderPath(dest) },
			})
		})
	})
}
