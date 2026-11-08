package git

import (
	"context"
	"fmt"
	"strings"

	"github.com/eugenioenko/ttt/internal/scm"
)

var _ scm.RevisionReader = repository{}

func (r repository) ReadRevision(ctx context.Context, ref string) (scm.Revision, error) {
	message, err := CommitMessageContext(ctx, r.root, ref)
	if err != nil {
		return scm.Revision{}, err
	}
	date, err := CommitAuthoredAtContext(ctx, r.root, ref)
	if ctx.Err() != nil {
		return scm.Revision{}, ctx.Err()
	}
	return scm.Revision{Message: message, AuthoredAt: date, AuthoredAtUnavailable: err != nil}, nil
}
func (r repository) RevisionChanges(ctx context.Context, ref string) ([]scm.Change, error) {
	files, err := CommitFilesContext(ctx, r.root, ref)
	return ToChanges(files), err
}
func (r repository) ReadRevisionPatch(ctx context.Context, ref string, change scm.Change) (string, error) {
	return CommitFileDiffContext(ctx, r.root, ref, FromChange(change))
}
func (r repository) ReadRevisionContent(ctx context.Context, ref, path, oldPath string) (scm.RevisionContent, error) {
	out, err := gitCommandContext(ctx, "-C", r.root, "rev-list", "--parents", "-n", "1", ref, "--").Output()
	if ctx.Err() != nil {
		return scm.RevisionContent{}, ctx.Err()
	}
	if err != nil {
		return scm.RevisionContent{}, err
	}
	refs := strings.Fields(string(out))
	if len(refs) == 0 {
		return scm.RevisionContent{}, fmt.Errorf("revision has no identity")
	}
	parent := ""
	if len(refs) > 1 {
		parent = refs[1]
	}
	if oldPath == "" {
		oldPath = path
	}
	before, err := r.revisionContentAt(ctx, parent, oldPath)
	if err != nil {
		return scm.RevisionContent{}, err
	}
	after, err := r.revisionContentAt(ctx, refs[0], path)
	if err != nil {
		return scm.RevisionContent{}, err
	}
	if !before.Exists && !after.Exists {
		return scm.RevisionContent{}, fmt.Errorf("revision path %q does not exist", path)
	}
	return scm.RevisionContent{Before: before, After: after}, ctx.Err()
}
func (r repository) revisionContentAt(ctx context.Context, ref, path string) (scm.FileContent, error) {
	if ref == "" || path == "" {
		return scm.FileContent{}, nil
	}
	entry, exists, err := TreeEntryContext(ctx, r.root, ref, path)
	if err != nil || !exists {
		return scm.FileContent{}, err
	}
	return readChangeContent(entry.Mode, entry.Object, func() (string, error) { return ShowFileContext(ctx, r.root, path, ref) })
}
