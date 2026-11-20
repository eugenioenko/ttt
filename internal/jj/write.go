package jj

import (
	"context"

	"github.com/eugenioenko/ttt/internal/scm"
)

var _ scm.Committer = repository{}
var _ scm.Discarder = repository{}

// Commit describes the working-copy commit and starts a new empty one on top,
// so every working-copy change is committed; jj has no staging area. Without a
// configured user.name and user.email, jj commits with an empty identity.
func (r repository) Commit(ctx context.Context, message string) error {
	_, err := r.run(ctx, "commit", "--message="+message)
	return err
}

// Discard restores paths from the working-copy parent; jj undo reverts it.
func (r repository) Discard(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	_, err := r.run(ctx, "restore", "--", fileset(paths...))
	return err
}
