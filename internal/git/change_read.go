package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
			result.Conflicted = true
			return result, nil
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
	} else {
		result.Before = index
		path := filepath.Join(r.root, change.Path)
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			err = nil
		case err != nil:
			return result, err
		case info.Mode()&os.ModeSymlink != 0:
			result.After.Content, err = os.Readlink(path)
			result.After.Exists = err == nil
		case info.Mode().IsRegular():
			var data []byte
			data, err = os.ReadFile(path)
			result.After = scm.FileContent{Content: string(data), Exists: err == nil}
		case info.IsDir() && len(entries) != 0 && entries[0].Mode == "160000":
			workingGitlink = true
			result.After = index
		default:
			return result, errors.ErrUnsupported
		}
		if err != nil {
			return result, err
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
