package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const walkDirMaxFiles = 100000

func listWorkspaceFiles(workDirs []string) []paletteFile {
	var files []paletteFile
	multiRoot := len(workDirs) > 1
	for _, workDir := range workDirs {
		prefix := ""
		if multiRoot {
			prefix = filepath.Base(workDir) + string(filepath.Separator)
		}
		files = append(files, listDirFiles(workDir, prefix)...)
	}
	return files
}

func listDirFiles(workDir, prefix string) []paletteFile {
	if _, err := exec.LookPath("rg"); err == nil {
		if files, ok := listFilesCmdLines(workDir, prefix, "rg", "--files"); ok {
			return files
		}
	}
	if _, err := exec.LookPath("git"); err == nil {
		if files, ok := listFilesCmdLines(workDir, prefix, "git", "ls-files", "--cached", "--others", "--exclude-standard"); ok {
			return files
		}
	}
	return listFilesWalkDir(workDir, prefix)
}

func listFilesCmdLines(workDir, prefix string, name string, args ...string) ([]paletteFile, bool) {
	cmd := exec.Command(name, args...)
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	var files []paletteFile
	for rel := range strings.SplitSeq(string(out), "\n") {
		if rel == "" {
			continue
		}
		files = append(files, paletteFile{
			Rel: prefix + rel,
			Abs: filepath.Join(workDir, rel),
		})
	}
	return files, true
}

func listFilesWalkDir(workDir, prefix string) []paletteFile {
	var files []paletteFile
	filepath.WalkDir(workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "node_modules" || name == ".cache" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(workDir, path)
		if err != nil {
			return nil
		}
		files = append(files, paletteFile{Rel: prefix + rel, Abs: path})
		if len(files) >= walkDirMaxFiles {
			return filepath.SkipAll
		}
		return nil
	})
	return files
}

func fileDetail(f string) string {
	dir := filepath.Dir(f)
	if dir == "." {
		return ""
	}
	return dir
}

func fuzzyFilterFiles(files []paletteFile, query string, maxResults int) []PaletteItem {
	if query == "" {
		items := make([]PaletteItem, 0, min(len(files), maxResults))
		for _, f := range files {
			items = append(items, PaletteItem{
				Label:  filepath.Base(f.Rel),
				Detail: fileDetail(f.Rel),
				ID:     f.Abs,
			})
			if len(items) >= maxResults {
				break
			}
		}
		return items
	}

	type scored struct {
		item  PaletteItem
		score int
	}
	var matches []scored
	for _, f := range files {
		if ok, score := fuzzyMatch(query, f.Rel); ok {
			matches = append(matches, scored{
				item: PaletteItem{
					Label:  filepath.Base(f.Rel),
					Detail: fileDetail(f.Rel),
					ID:     f.Abs,
				},
				score: score,
			})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})
	items := make([]PaletteItem, 0, min(len(matches), maxResults))
	for _, m := range matches {
		items = append(items, m.item)
		if len(items) >= maxResults {
			break
		}
	}
	return items
}
