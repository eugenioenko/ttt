package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const walkDirMaxFiles = 100000

func listWorkspaceFiles(workDirs []string) []paletteFile {
	var files []paletteFile
	multiRoot := len(workDirs) > 1
	_, hasRg := exec.LookPath("rg")
	_, hasGit := exec.LookPath("git")
	for _, workDir := range workDirs {
		prefix := ""
		if multiRoot {
			prefix = filepath.Base(workDir) + string(filepath.Separator)
		}
		if hasRg == nil {
			if f, ok := listFilesCmdLines(workDir, prefix, "rg", "--files"); ok {
				files = append(files, f...)
				continue
			}
		}
		if hasGit == nil {
			if f, ok := listFilesCmdLines(workDir, prefix, "git", "ls-files", "--cached", "--others", "--exclude-standard"); ok {
				files = append(files, f...)
				continue
			}
		}
		files = append(files, listFilesWalkDir(workDir, prefix)...)
	}
	return files
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
