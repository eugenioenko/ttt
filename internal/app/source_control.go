package app

import (
	"context"
	"errors"

	"github.com/eugenioenko/ttt/internal/scm"
)

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
