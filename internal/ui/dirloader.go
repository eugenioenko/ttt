package ui

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/git"
)

type DirEntry struct {
	Name       string
	Path       string
	IsDir      bool
	GitIgnored bool
}

func LoadDirEntries(dirPath string, settings config.ExplorerSettings) []DirEntry {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil
	}

	var ignored map[string]bool
	gitRoot := git.RepoRoot(dirPath)
	if gitRoot != "" {
		ignored = git.IgnoredFiles(gitRoot, entryPaths(dirPath, entries))
	}
	return filterDirEntries(dirPath, entries, settings, func(path string) bool { return ignored[path] })
}

// WalkDirEntries reads every directory LoadDirEntries would show below roots,
// keyed by directory path. Symlinked directories and .git are not descended.
// Ignore checks are batched per repository per tree level, so the number of
// git processes grows with depth rather than with the number of directories.
func WalkDirEntries(ctx context.Context, roots []string, settings config.ExplorerSettings) (map[string][]DirEntry, error) {
	type pendingDir struct {
		path     string
		repo     string
		inherits bool
		root     bool
	}
	type readDir struct {
		pendingDir
		entries []os.DirEntry
	}
	result := make(map[string][]DirEntry)
	level := make([]pendingDir, 0, len(roots))
	for _, root := range roots {
		level = append(level, pendingDir{path: root, root: true})
	}
	for len(level) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		read := make([]readDir, 0, len(level))
		batches := make(map[string][]string)
		for _, dir := range level {
			entries, err := os.ReadDir(dir.path)
			if err != nil {
				continue
			}
			if dir.root || hasGitEntry(entries) {
				dir.repo = git.RepoRootContext(ctx, dir.path)
				dir.inherits = false
			}
			if dir.repo != "" && !dir.inherits {
				batches[dir.repo] = append(batches[dir.repo], entryPaths(dir.path, entries)...)
			}
			read = append(read, readDir{pendingDir: dir, entries: entries})
		}
		ignored := make(map[string]bool)
		for repo, paths := range batches {
			for path := range git.IgnoredFilesContext(ctx, repo, paths) {
				ignored[path] = true
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		level = level[:0]
		for _, dir := range read {
			list := filterDirEntries(dir.path, dir.entries, settings, func(path string) bool {
				return dir.inherits || ignored[path]
			})
			result[dir.path] = list
			for _, de := range list {
				if !de.IsDir || de.Name == ".git" {
					continue
				}
				if info, err := os.Lstat(de.Path); err != nil || info.Mode()&os.ModeSymlink != 0 {
					continue
				}
				level = append(level, pendingDir{path: de.Path, repo: dir.repo, inherits: de.GitIgnored})
			}
		}
	}
	return result, nil
}

func hasGitEntry(entries []os.DirEntry) bool {
	for _, entry := range entries {
		if entry.Name() == ".git" {
			return true
		}
	}
	return false
}

func entryPaths(dirPath string, entries []os.DirEntry) []string {
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, filepath.Join(dirPath, entry.Name()))
	}
	return paths
}

func filterDirEntries(dirPath string, entries []os.DirEntry, settings config.ExplorerSettings, ignored func(string) bool) []DirEntry {
	var dirs, files []DirEntry

	for _, entry := range entries {
		name := entry.Name()
		if !settings.ShowHidden && strings.HasPrefix(name, ".") {
			continue
		}
		childPath := filepath.Join(dirPath, name)
		isIgnored := ignored(childPath)
		if !settings.ShowGitIgnored && isIgnored {
			continue
		}

		isDir := entry.IsDir()
		if entry.Type()&os.ModeSymlink != 0 {
			if info, err := os.Stat(childPath); err == nil {
				isDir = info.IsDir()
			}
		}

		de := DirEntry{
			Name:       name,
			Path:       childPath,
			IsDir:      isDir,
			GitIgnored: isIgnored,
		}

		if isDir {
			dirs = append(dirs, de)
		} else {
			files = append(files, de)
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	return append(dirs, files...)
}
