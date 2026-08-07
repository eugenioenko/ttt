package ui

import (
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
		var paths []string
		for _, entry := range entries {
			paths = append(paths, filepath.Join(dirPath, entry.Name()))
		}
		ignored = git.IgnoredFiles(gitRoot, paths)
	}

	var dirs, files []DirEntry

	for _, entry := range entries {
		name := entry.Name()
		if !settings.ShowHidden && strings.HasPrefix(name, ".") {
			continue
		}
		childPath := filepath.Join(dirPath, name)
		isIgnored := ignored[childPath]
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
