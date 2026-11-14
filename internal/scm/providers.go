package scm

import (
	"context"
	"errors"
)

// Providers discovers with the first provider that claims a directory. Order is
// precedence: a directory both providers claim belongs to the earlier one.
type Providers []Provider

func (p Providers) Discover(ctx context.Context, dir string) (Repository, error) {
	var missing error = ErrNotRepository
	for _, provider := range p {
		repo, err := provider.Discover(ctx, dir)
		if err == nil {
			return repo, nil
		}
		if !errors.Is(err, ErrNotRepository) {
			return nil, err
		}
		missing = err
	}
	return nil, missing
}
