package git

import (
	"context"

	"github.com/eugenioenko/ttt/internal/scm"
)

type Provider struct{}

type repository struct{ root string }

var _ scm.Provider = Provider{}
var _ scm.Repository = repository{}
var _ scm.Staging = repository{}

func (Provider) Discover(ctx context.Context, dir string) (scm.Repository, error) {
	root, err := RepoRootWithErrorContext(ctx, dir)
	if err != nil {
		return nil, err
	}
	return repository{root: root}, nil
}

func (r repository) Root() string { return r.root }

func (r repository) Identity(ctx context.Context) (scm.Identity, error) {
	branch, err := BranchNameContext(ctx, r.root)
	return scm.Identity{Root: r.root, Label: branch}, err
}

func (r repository) Changes(ctx context.Context) ([]scm.Change, error) {
	files, err := StatusFilesContext(ctx, r.root)
	if err != nil {
		return nil, err
	}
	changes := make([]scm.Change, 0, len(files))
	for _, file := range files {
		group := scm.WorkingTree
		if file.Staged {
			group = scm.Staged
		}
		changes = append(changes, scm.Change{Path: file.Path, OldPath: file.OldPath, Status: file.Status, Group: group})
	}
	return changes, nil
}

func (r repository) Revision(ctx context.Context) (string, error) {
	return RevisionIdentityContext(ctx, r.root)
}

func (r repository) Stage(ctx context.Context, paths []string) error {
	return runPathsContext(ctx, r.root, []string{"add", "--"}, paths)
}

func (r repository) Unstage(ctx context.Context, paths []string) error {
	return runPathsContext(ctx, r.root, []string{"reset", "HEAD", "--"}, paths)
}
