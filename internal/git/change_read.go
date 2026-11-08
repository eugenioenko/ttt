package git

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/eugenioenko/ttt/internal/scm"
)

var _ scm.ChangeReader = repository{}

func (r repository) ReadChange(ctx context.Context, change scm.Change) (scm.ChangeDiff, error) {
	result := scm.ChangeDiff{}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if change.Group != "" && change.Group != scm.WorkingTree && change.Group != scm.Staged {
		return result, errors.ErrUnsupported
	}
	indexPath := change.Path
	if change.Group != scm.Staged && change.OldPath != "" {
		indexPath = change.OldPath
	}
	entries, err := IndexEntriesContext(ctx, r.root, indexPath)
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		if entry.Stage != 0 {
			return r.readConflictChange(ctx, change, entries)
		}
	}
	index := scm.FileContent{}
	if len(entries) != 0 {
		index, err = readChangeContent(entries[0].Mode, entries[0].Object, func() (string, error) {
			return ShowIndexFileContext(ctx, r.root, indexPath)
		})
		if err != nil {
			return result, err
		}
	}
	revision := ""
	workingGitlink := false
	if change.Group == scm.Staged {
		revision, err = RevisionIdentityContext(ctx, r.root)
		if err != nil {
			return result, err
		}
		oldPath := change.Path
		if change.OldPath != "" {
			oldPath = change.OldPath
		}
		entry, exists, err := TreeEntryContext(ctx, r.root, revision, oldPath)
		if err != nil {
			return result, err
		}
		if exists {
			result.Before, err = readChangeContent(entry.Mode, entry.Object, func() (string, error) {
				return ShowFileContext(ctx, r.root, oldPath, revision)
			})
			if err != nil {
				return result, err
			}
		}
		result.After = index
	} else if change.Kind == scm.Deleted {
		// A deleted endpoint can have an ancestor replaced by a file.
		result.Before = index
	} else {
		result.Before = index
		working, readErr := readWorkingTreeFileContext(ctx, r.root, change.Path)
		if readErr != nil {
			var pathErr *workingTreePathError
			if errors.As(readErr, &pathErr) && pathErr.Kind == workingTreePathDirectory && len(entries) != 0 && entries[0].Mode == "160000" {
				workingGitlink = true
				result.After = index
			} else {
				return result, readErr
			}
		} else {
			if !working.Exists {
				return result, fmt.Errorf("working tree path disappeared: %q", change.Path)
			}
			result.After = scm.FileContent{Content: string(working.Content), Exists: working.Exists}
		}
	}
	flags := []string{"--no-ext-diff", "--no-textconv", "--submodule=short", "-M"}
	paths := []string{change.Path}
	if change.OldPath != "" && change.OldPath != change.Path {
		paths = []string{change.OldPath, change.Path}
	}
	if change.Group == scm.Staged {
		flags = append(flags, "--cached")
		if revision != "" {
			flags = append(flags, revision)
		}
	}
	if change.Kind == scm.Untracked && change.Group != scm.Staged {
		result.Patch, err = diffFileFromEmptyContext(ctx, r.root, filepath.Join(r.root, change.Path))
	} else {
		result.Patch, err = diffPaths(ctx, r.root, flags, paths...)
	}
	if ctx.Err() != nil {
		return scm.ChangeDiff{}, ctx.Err()
	}
	if workingGitlink {
		// The patch represents the submodule revision and dirty suffix without
		// reading a directory or accidentally discovering its parent repository.
		for _, line := range strings.Split(result.Patch, "\n") {
			if strings.HasPrefix(line, "+Subproject commit ") {
				result.After.Content = strings.TrimPrefix(line, "+") + "\n"
				break
			}
		}
	}
	return result, err
}

func readChangeContent(mode, object string, read func() (string, error)) (scm.FileContent, error) {
	if mode == "160000" {
		return scm.FileContent{Content: "Subproject commit " + object + "\n", Exists: true}, nil
	}
	content, err := read()
	return scm.FileContent{Content: content, Exists: err == nil}, err
}

func (r repository) readConflictChange(ctx context.Context, change scm.Change, entries []IndexEntry) (scm.ChangeDiff, error) {
	result := scm.ChangeDiff{Conflicted: true, ConflictDetail: change.Detail}
	for _, entry := range entries {
		result.ConflictSources = append(result.ConflictSources, strconv.Itoa(entry.Stage))
	}
	entry, ok := preferredConflictEntry(entries)
	if !ok {
		return result, fmt.Errorf("unmerged path has no conflict endpoint")
	}
	var err error
	result.Before, err = readChangeContent(entry.Mode, entry.Object, func() (string, error) {
		data, err := ShowIndexFileBytesContext(ctx, r.root, change.Path, entry.Stage)
		return string(data), err
	})
	if err != nil {
		return result, err
	}
	working, err := readWorkingTreeFileContext(ctx, r.root, change.Path)
	if err != nil {
		var pathErr *workingTreePathError
		if errors.As(err, &pathErr) && pathErr.Kind == workingTreePathDirectory && entry.Mode == "160000" {
			result.After = scm.FileContent{Exists: true}
			return result, nil
		}
		return result, err
	}
	result.After = scm.FileContent{Content: string(working.Content), Exists: working.Exists}
	return result, ctx.Err()
}
