package workspace

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const walkDirMaxFiles = 100000

type File struct {
	Rel string
	Abs string
}

type FileVisibility struct {
	ShowHidden     bool
	ShowGitIgnored bool
}

func ListFiles(workDirs []string, visibility FileVisibility) []File {
	var files []File
	for _, workDir := range workDirs {
		prefix := ""
		if len(workDirs) > 1 {
			prefix = filepath.Base(workDir) + string(filepath.Separator)
		}
		files = append(files, listDirFiles(workDir, prefix, visibility)...)
	}
	return files
}

func listDirFiles(workDir, prefix string, visibility FileVisibility) []File {
	args := []string{"--files", "--null", "--glob", "!.git"}
	if visibility.ShowHidden {
		args = append(args, "--hidden")
	}
	if visibility.ShowGitIgnored {
		args = append(args, "--no-ignore-vcs")
	}
	if paths, ok := fileCommand(workDir, "rg", args...); ok {
		return visibleFiles(workDir, prefix, paths, visibility)
	}
	args = []string{"ls-files", "-z", "--cached", "--others", "--deduplicate"}
	if !visibility.ShowGitIgnored {
		args = append(args, "--exclude-standard")
	}
	if paths, ok := fileCommand(workDir, "git", args...); ok {
		return visibleFiles(workDir, prefix, paths, visibility)
	}
	return listFilesWalkDir(workDir, prefix, visibility)
}

func fileCommand(workDir, name string, args ...string) ([]string, bool) {
	cmd := exec.Command(name, args...)
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		// ripgrep returns 1 for an empty file list; falling back would bypass its ignore rules.
		if name != "rg" || !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return nil, false
		}
	}
	return strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00"), true
}

func visiblePath(rel string, visibility FileVisibility) bool {
	for part := range strings.SplitSeq(filepath.ToSlash(rel), "/") {
		if part == ".git" || !visibility.ShowHidden && strings.HasPrefix(part, ".") {
			return false
		}
	}
	return rel != ""
}

func visibleFiles(workDir, prefix string, paths []string, visibility FileVisibility) []File {
	var files []File
	for _, rel := range paths {
		if visiblePath(rel, visibility) {
			files = append(files, File{Rel: prefix + rel, Abs: filepath.Join(workDir, rel)})
		}
	}
	return files
}

func listFilesWalkDir(workDir, prefix string, visibility FileVisibility) []File {
	ignored := make(map[string]bool)
	if !visibility.ShowGitIgnored {
		paths, _ := fileCommand(workDir, "git", "ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--directory")
		for _, path := range paths {
			ignored[filepath.Clean(path)] = true
		}
	}
	var files []File
	_ = filepath.WalkDir(workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || path == workDir {
			return nil
		}
		if d.IsDir() && !visibility.ShowGitIgnored && (d.Name() == "node_modules" || d.Name() == "__pycache__") {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(workDir, path)
		if err != nil {
			return nil
		}
		if !visiblePath(rel, visibility) || ignored[rel] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		const special = os.ModeNamedPipe | os.ModeSocket | os.ModeDevice | os.ModeCharDevice | os.ModeIrregular
		if d.Type()&special != 0 {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(path)
			if err != nil || info.IsDir() || info.Mode()&special != 0 {
				return nil
			}
		}
		files = append(files, File{Rel: prefix + rel, Abs: path})
		if len(files) >= walkDirMaxFiles {
			return filepath.SkipAll
		}
		return nil
	})
	return files
}
