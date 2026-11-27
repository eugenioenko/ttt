package git

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/eugenioenko/ttt/internal/scm"
)

var _ scm.BaselineReader = repository{}

func (r repository) ReadBaseline(ctx context.Context, path string) (scm.Baseline, error) {
	content, readErr := ShowIndexFileContext(ctx, r.root, path)
	if ctx.Err() != nil {
		return scm.Baseline{}, ctx.Err()
	}
	if readErr == nil {
		return scm.Baseline{Content: content, Exists: true}, nil
	}
	entries, err := IndexEntriesContext(ctx, r.root, path)
	if err != nil {
		return scm.Baseline{}, err
	}
	if len(entries) == 0 {
		ignored, err := r.baselineIgnored(ctx, path)
		return scm.Baseline{Ignored: ignored}, err
	}
	for _, entry := range entries {
		if entry.Stage != 0 {
			return r.conflictBaseline(ctx, path)
		}
	}
	return scm.Baseline{}, readErr
}

func (r repository) conflictBaseline(ctx context.Context, path string) (scm.Baseline, error) {
	base := scm.Baseline{Conflicted: true}
	revision, err := RevisionIdentityContext(ctx, r.root)
	if err != nil || revision == "" {
		return base, err
	}
	_, exists, err := TreeEntryContext(ctx, r.root, revision, path)
	if err != nil || !exists {
		return base, err
	}
	base.Content, err = ShowFileContext(ctx, r.root, path, revision)
	base.Exists = err == nil
	return base, err
}

func (r repository) baselineIgnored(ctx context.Context, path string) (bool, error) {
	cmd := gitCommandContext(ctx, "-C", r.root, "check-ignore", "--stdin", "-z")
	cmd.Stdin = strings.NewReader(path + "\x00")
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSuffix(string(out), "\x00") == path, nil
}
