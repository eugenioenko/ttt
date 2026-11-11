package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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

func ListFiles(ctx context.Context, workDirs []string, visibility FileVisibility) []File {
	var files []File
	for _, workDir := range workDirs {
		if ctx.Err() != nil {
			return nil
		}
		prefix := ""
		if len(workDirs) > 1 {
			prefix = filepath.Base(workDir) + string(filepath.Separator)
		}
		files = append(files, listDirFiles(ctx, workDir, prefix, visibility)...)
	}
	if ctx.Err() != nil {
		return nil
	}
	return files
}

func listDirFiles(ctx context.Context, workDir, prefix string, visibility FileVisibility) []File {
	args := []string{"--files", "--null", "--glob", "!.git"}
	if visibility.ShowHidden {
		args = append(args, "--hidden")
	}
	if visibility.ShowGitIgnored {
		args = append(args, "--no-ignore-vcs")
	}
	if paths, ok := fileCommand(ctx, workDir, "rg", args...); ok {
		return visibleFiles(ctx, workDir, prefix, paths, visibility)
	}
	args = []string{"ls-files", "-z", "--cached", "--others", "--deduplicate"}
	if !visibility.ShowGitIgnored {
		args = append(args, "--exclude-standard")
	}
	if paths, ok := fileCommand(ctx, workDir, "git", args...); ok {
		return visibleFiles(ctx, workDir, prefix, paths, visibility)
	}
	return listFilesWalkDir(ctx, workDir, prefix, visibility)
}

func fileCommand(ctx context.Context, workDir, name string, args ...string) ([]string, bool) {
	if ctx.Err() != nil {
		return nil, false
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = time.Second
	cmd.Dir = workDir
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, false
	}
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

func visibleFiles(ctx context.Context, workDir, prefix string, paths []string, visibility FileVisibility) []File {
	var files []File
	for _, rel := range paths {
		if ctx.Err() != nil {
			return nil
		}
		if visiblePath(rel, visibility) {
			files = append(files, File{Rel: prefix + rel, Abs: filepath.Join(workDir, rel)})
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	return files
}

func listFilesWalkDir(ctx context.Context, workDir, prefix string, visibility FileVisibility) []File {
	if ctx.Err() != nil {
		return nil
	}
	ignored := make(map[string]bool)
	if !visibility.ShowGitIgnored {
		paths, _ := fileCommand(ctx, workDir, "git", "ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--directory")
		for _, path := range paths {
			if ctx.Err() != nil {
				return nil
			}
			ignored[filepath.Clean(path)] = true
		}
	}
	var files []File
	_ = filepath.WalkDir(workDir, func(path string, d os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
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
	if ctx.Err() != nil {
		return nil
	}
	return files
}
