package app

import (
	"context"
	"errors"

	"github.com/eugenioenko/ttt/internal/scm"
)

type repositorySource struct {
	repository scm.Repository
	provider   scm.Provider
	root       string
}

// Snapshot service references on the main thread; workers must not read panel state.
func (s *RepositoryState) repositorySource(root string) repositorySource {
	source := repositorySource{root: root}
	if s == nil || s.closed {
		return source
	}
	source.provider = s.provider
	source.repository = s.repositories[root]
	if s.changes != nil {
		if repo := s.changes.repository(root); repo != nil {
			source.repository = repo
		}
	}
	return source
}

func (s repositorySource) resolve(ctx context.Context) (scm.Repository, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	repo := s.repository
	if repo == nil {
		if s.provider == nil {
			return nil, errors.ErrUnsupported
		}
		var err error
		repo, err = s.provider.Discover(ctx, s.root)
		if err != nil {
			return nil, err
		}
	}
	return repo, ctx.Err()
}

func (s repositorySource) readBaseline(ctx context.Context, path string) (scm.Baseline, error) {
	repo, err := s.resolve(ctx)
	if err != nil {
		return scm.Baseline{}, err
	}
	reader, ok := repo.(scm.BaselineReader)
	if !ok {
		return scm.Baseline{}, errors.ErrUnsupported
	}
	base, err := reader.ReadBaseline(ctx, path)
	if ctx.Err() != nil {
		return scm.Baseline{}, ctx.Err()
	}
	return base, err
}

func (s repositorySource) readChange(ctx context.Context, change scm.Change) (scm.ChangeDiff, error) {
	repo, err := s.resolve(ctx)
	if err != nil {
		return scm.ChangeDiff{}, err
	}
	reader, ok := repo.(scm.ChangeReader)
	if !ok {
		return scm.ChangeDiff{}, errors.ErrUnsupported
	}
	result, err := reader.ReadChange(ctx, change)
	if ctx.Err() != nil {
		return scm.ChangeDiff{}, ctx.Err()
	}
	return result, err
}
