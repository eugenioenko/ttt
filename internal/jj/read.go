package jj

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/scm"
)

var _ scm.BaselineReader = repository{}
var _ scm.ChangeReader = repository{}

// The baseline of a merge working copy is its first parent, not the merged
// parents: jj has no revision that addresses the merged parent tree.
func (r repository) baselineRevision(ctx context.Context) (string, error) {
	parents, err := r.parents(ctx, "@")
	if err != nil {
		return "", err
	}
	if len(parents) == 0 {
		return "", nil
	}
	return parents[0], nil
}

func (r repository) ReadBaseline(ctx context.Context, path string) (scm.Baseline, error) {
	parent, err := r.baselineRevision(ctx)
	if err != nil {
		return scm.Baseline{}, err
	}
	content, err := r.fileAt(ctx, parent, path)
	if err != nil {
		return scm.Baseline{}, err
	}
	if !content.Exists && parent != "" {
		source, err := r.copySource(ctx, path)
		if err != nil {
			return scm.Baseline{}, err
		}
		if content, err = r.fileAt(ctx, parent, source); err != nil {
			return scm.Baseline{}, err
		}
	}
	conflicted, err := r.conflicted(ctx, "@", path)
	if err != nil {
		return scm.Baseline{}, err
	}
	// A parent's conflict is materialized as markers, which must never become
	// an editable diff base even when the working copy has resolved it.
	if !conflicted && parent != "" {
		if conflicted, err = r.conflicted(ctx, parent, path); err != nil {
			return scm.Baseline{}, err
		}
	}
	base := scm.Baseline{Content: content.Content, Exists: content.Exists, Conflicted: conflicted}
	if !base.Exists && !conflicted {
		base.Ignored, err = r.ignored(ctx, path)
	}
	return base, err
}

// copySource returns the parent path a renamed or copied working-copy file
// came from, so its baseline matches the rename the Changes list reports.
func (r repository) copySource(ctx context.Context, path string) (string, error) {
	changes, err := r.Changes(ctx)
	if err != nil {
		return "", err
	}
	for _, change := range changes {
		if change.Path == path && change.OldPath != "" {
			return change.OldPath, nil
		}
	}
	return "", nil
}

func (r repository) conflicted(ctx context.Context, rev, path string) (bool, error) {
	out, err := r.read(ctx, "log", "-r", rev, "--no-graph", "-T", `self.conflicted_files().map(|e| e.path() ++ "\0").join("")`)
	if err != nil {
		return false, err
	}
	for _, conflict := range splitFields(out) {
		if filepath.ToSlash(conflict) == path {
			return true, nil
		}
	}
	return false, nil
}

// ignored reports an on-disk file the working copy does not track. jj tracks
// new files automatically, so after a snapshot that means an ignore rule.
func (r repository) ignored(ctx context.Context, path string) (bool, error) {
	if found, err := exists(filepath.Join(r.root, filepath.FromSlash(path))); !found || err != nil {
		return false, err
	}
	out, err := r.run(ctx, "file", "list", "-r", "@", "--", fileset(path))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "", nil
}

func (r repository) ReadChange(ctx context.Context, change scm.Change) (scm.ChangeDiff, error) {
	if err := ctx.Err(); err != nil {
		return scm.ChangeDiff{}, err
	}
	if change.Group != "" && change.Group != scm.WorkingTree {
		return scm.ChangeDiff{}, errors.ErrUnsupported
	}
	parent, err := r.baselineRevision(ctx)
	if err != nil {
		return scm.ChangeDiff{}, err
	}
	oldPath := change.Path
	if change.OldPath != "" {
		oldPath = change.OldPath
	}
	result := scm.ChangeDiff{}
	if result.Before, err = r.fileAt(ctx, parent, oldPath); err != nil {
		return scm.ChangeDiff{}, err
	}
	if result.After, err = r.workingFile(change.Path); err != nil {
		return scm.ChangeDiff{}, err
	}
	if change.Kind == scm.Conflicted {
		result.Conflicted, result.ConflictDetail = true, change.Detail
		return result, ctx.Err()
	}
	out, err := r.run(ctx, append(diffRange(parent, "@"), "--", fileset(oldPath, change.Path))...)
	if err != nil {
		return scm.ChangeDiff{}, err
	}
	result.Patch = string(out)
	return result, ctx.Err()
}

func (r repository) workingFile(path string) (scm.FileContent, error) {
	full := filepath.Join(r.root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if errors.Is(err, fs.ErrNotExist) {
		return scm.FileContent{}, nil
	}
	if err != nil {
		return scm.FileContent{}, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		target, err := os.Readlink(full)
		return scm.FileContent{Content: target, Exists: err == nil}, err
	}
	if !info.Mode().IsRegular() {
		return scm.FileContent{}, fmt.Errorf("working copy path is not a regular file: %q", path)
	}
	data, err := os.ReadFile(full)
	return scm.FileContent{Content: string(data), Exists: err == nil}, err
}

// fileAt reads path at an immutable commit. A path absent from that commit is
// a missing endpoint, not an error.
func (r repository) fileAt(ctx context.Context, rev, path string) (scm.FileContent, error) {
	if rev == "" || path == "" {
		return scm.FileContent{}, nil
	}
	out, err := r.read(ctx, "file", "show", "-r", rev, "-T", "", "--", fileset(path))
	if err != nil {
		var cmdErr *CommandError
		if errors.As(err, &cmdErr) && strings.Contains(cmdErr.Stderr, "No such path") {
			return scm.FileContent{}, nil
		}
		return scm.FileContent{}, err
	}
	return scm.FileContent{Content: string(out), Exists: true}, nil
}

// diffRange compares against one explicit parent, so patch hunks line up with
// the Before endpoint even when the revision is a merge.
func diffRange(parent, rev string) []string {
	if parent == "" {
		return []string{"diff", "--git", "-r", rev}
	}
	return []string{"diff", "--git", "--from", parent, "--to", rev}
}
