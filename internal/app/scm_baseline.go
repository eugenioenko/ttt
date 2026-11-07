package app

import (
	"context"
	"errors"

	"github.com/eugenioenko/ttt/internal/scm"
)

type baselineSource struct {
	repository scm.Repository
	provider   scm.Provider
	root       string
}

// Snapshot service references on the main thread; workers must not read panel state.
func (s *RepositoryState) baselineSource(root string) baselineSource {
	source := baselineSource{root: root}
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

func (s baselineSource) read(ctx context.Context, path string) (scm.Baseline, error) {
	if ctx.Err() != nil {
		return scm.Baseline{}, ctx.Err()
	}
	repo := s.repository
	if repo == nil {
		if s.provider == nil {
			return scm.Baseline{}, errors.ErrUnsupported
		}
		var err error
		repo, err = s.provider.Discover(ctx, s.root)
		if err != nil {
			return scm.Baseline{}, err
		}
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
