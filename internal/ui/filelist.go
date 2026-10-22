package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/git"
)

const walkDirMaxFiles = 100000

func listWorkspaceFiles(workDirs []string, opts config.QuickOpenSettings) []paletteFile {
	var files []paletteFile
	multiRoot := len(workDirs) > 1
	for _, workDir := range workDirs {
		prefix := ""
		if multiRoot {
			prefix = filepath.Base(workDir) + string(filepath.Separator)
		}
		files = append(files, listDirFiles(workDir, prefix, opts)...)
	}
	return files
}

func listDirFiles(workDir, prefix string, opts config.QuickOpenSettings) []paletteFile {
	if _, err := exec.LookPath("rg"); err == nil {
		// --hidden lists the files under a dot directory and --no-ignore the
		// gitignored ones. Each is asked for only when its setting is on, so
		// ripgrep keeps skipping the trees the settings hide. The .git
		// directory stays out, as it does in the walk below.
		args := []string{"--files"}
		if opts.ShowHidden {
			args = append(args, "--hidden")
		}
		if opts.ShowGitIgnored {
			args = append(args, "--no-ignore")
		}
		args = append(args, "--glob", "!.git")
		if files, ok := listFilesCmdLines(workDir, prefix, "rg", args...); ok {
			return files
		}
	}
	if _, err := exec.LookPath("git"); err == nil {
		// git lists the dotfiles whatever the setting says, so the hidden
		// filter still applies to its output; --exclude-standard is what
		// keeps the gitignored files out of it.
		args := []string{"ls-files", "--cached", "--others"}
		if !opts.ShowGitIgnored {
			args = append(args, "--exclude-standard")
		}
		if files, ok := listFilesCmdLines(workDir, prefix, "git", args...); ok {
			return filterHiddenFiles(files, workDir, opts.ShowHidden)
		}
	}
	files := listFilesWalkDir(workDir, prefix)
	if !opts.ShowGitIgnored {
		files = filterGitIgnoredFiles(files, workDir)
	}
	return filterHiddenFiles(files, workDir, opts.ShowHidden)
}

// filterHiddenFiles drops the dot-prefixed paths when the quick open hides
// them. A path is hidden when any of its components starts with a dot.
func filterHiddenFiles(files []paletteFile, workDir string, showHidden bool) []paletteFile {
	if showHidden {
		return files
	}
	kept := make([]paletteFile, 0, len(files))
	for _, f := range files {
		rel, err := filepath.Rel(workDir, f.Abs)
		if err != nil {
			kept = append(kept, f)
			continue
		}
		if isHiddenPath(rel) {
			continue
		}
		kept = append(kept, f)
	}
	return kept
}

// filterGitIgnoredFiles drops the files a .gitignore names. Without git there
// is no way to know which files those are, and the walk lists them.
func filterGitIgnoredFiles(files []paletteFile, workDir string) []paletteFile {
	gitRoot := git.RepoRoot(workDir)
	if gitRoot == "" {
		return files
	}
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Abs
	}
	ignored := git.IgnoredFiles(gitRoot, paths)
	kept := make([]paletteFile, 0, len(files))
	for _, f := range files {
		if ignored[f.Abs] {
			continue
		}
		kept = append(kept, f)
	}
	return kept
}

func isHiddenPath(rel string) bool {
	parts := strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == os.PathSeparator })
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
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
		if d.Type()&specialFileMask != 0 || d.Type()&os.ModeSymlink != 0 && isSpecialFile(path) {
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
	query = strings.Join(strings.Fields(query), "")
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
