package app

import (
	"context"
	"errors"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/jj"
	"github.com/eugenioenko/ttt/internal/scm"
)

// jj comes first so a colocated workspace, which also has a .git directory,
// is presented through jj.
func defaultSCMProvider() scm.Provider {
	return scm.Providers{jj.Provider{}, git.Provider{}}
}

func (cp *ChangesPanel) repository(dir string) scm.Repository {
	for _, group := range cp.groups {
		if group.Dir == dir {
			return group.Repository
		}
	}
	return nil
}

func (cp *ChangesPanel) supportsStaging(dir string) bool {
	repo := cp.repository(dir)
	if repo == nil {
		return true
	}
	_, ok := repo.(scm.Staging)
	return ok
}

// discardChanges reverts working-copy changes through the repository's
// provider when it supports discarding, otherwise through Git.
func (cp *ChangesPanel) discardChanges(dir string, files []scm.Change) error {
	if len(files) == 0 {
		return nil
	}
	if discarder, ok := cp.repository(dir).(scm.Discarder); ok {
		var paths []string
		for _, f := range files {
			// A copy's source is a separate file whose own edits must survive.
			if f.Kind == scm.Renamed && f.OldPath != "" {
				paths = append(paths, f.OldPath)
			}
			paths = append(paths, f.Path)
		}
		ctx, cancel := context.WithTimeout(context.Background(), repositoryStatusTimeout)
		defer cancel()
		return discarder.Discard(ctx, paths)
	}
	untracked, tracked := discardPaths(files)
	err := git.DiscardUntracked(dir, untracked...)
	if e := git.Discard(dir, tracked...); e != nil && err == nil {
		err = e
	}
	return err
}

func (cp *ChangesPanel) commitOp(dir, message string) RepoOp {
	if committer, ok := cp.repository(dir).(scm.Committer); ok {
		return RepoOp{Name: "commit", Run: func(string) error {
			return committer.Commit(context.Background(), message)
		}}
	}
	return OpCommit(message)
}

func (cp *ChangesPanel) stagePaths(dir string, unstage bool, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), repositoryStatusTimeout)
	defer cancel()
	repo := cp.repository(dir)
	if repo == nil {
		var err error
		repo, err = cp.provider.Discover(ctx, dir)
		if err != nil {
			return err
		}
	}
	staging, ok := repo.(scm.Staging)
	if !ok {
		if jj.IsRepository(repo) {
			return errors.New(i18n.Text("status.notAvailableInJJRepositories"))
		}
		return errors.ErrUnsupported
	}
	if unstage {
		return staging.Unstage(ctx, paths)
	}
	return staging.Stage(ctx, paths)
}

func (g changesGroup) filesIn(group scm.ChangeGroup) []scm.Change {
	// Preserve the existing two-section projection while retaining provider
	// group identity in each change for consumers that do not use an index.
	var files []scm.Change
	for _, change := range g.Changes {
		if change.Group == group || group == scm.WorkingTree && change.Group != scm.Staged {
			files = append(files, change)
		}
	}
	return files
}
