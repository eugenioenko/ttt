package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Roslyn does not load a project from rootUri: it keeps files covered by no
// project as loose files, with only file-local symbols, until the client asks
// it to open a solution or project.
const maxRoslynProjects = 500

var skippedProjectDirs = map[string]bool{
	".git":         true,
	".vs":          true,
	"bin":          true,
	"node_modules": true,
	"obj":          true,
}

type roslynSolutionParams struct {
	Solution string `json:"solution"`
}

type roslynProjectsParams struct {
	Projects []string `json:"projects"`
}

func isRoslynServer(serverKey string, command []string) bool {
	if strings.Contains(strings.ToLower(serverKey), "roslyn") {
		return true
	}
	for _, arg := range command {
		name := strings.ToLower(filepath.Base(arg))
		if strings.Contains(name, "roslyn") || strings.Contains(name, "microsoft.codeanalysis.languageserver") {
			return true
		}
	}
	return false
}

func (c *Client) OpenRoslynWorkspace(root string) {
	solution, projects := FindRoslynWorkspace(root)
	switch {
	case solution != "":
		if err := c.notify("solution/open", roslynSolutionParams{Solution: FileURI(solution)}); err != nil {
			c.log("error", "solution/open: "+err.Error())
			return
		}
		c.log("info", "loading solution "+solution)
	case len(projects) > 0:
		uris := make([]string, len(projects))
		for i, project := range projects {
			uris[i] = FileURI(project)
		}
		if err := c.notify("project/open", roslynProjectsParams{Projects: uris}); err != nil {
			c.log("error", "project/open: "+err.Error())
			return
		}
		c.log("info", fmt.Sprintf("loading %d project(s)", len(projects)))
	default:
		c.log("info", "no .sln or .csproj found under "+root)
	}
}

// FindRoslynWorkspace prefers a solution next to root or in one of its
// ancestors, then a single solution discovered below it, then every project.
func FindRoslynWorkspace(root string) (solution string, projects []string) {
	if root == "" {
		return "", nil
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", nil
	}
	if solution := solutionAtOrAbove(abs); solution != "" {
		return solution, nil
	}
	if solutions := findProjectFiles(abs, isSolutionFile, 2); len(solutions) == 1 {
		return solutions[0], nil
	}
	return "", findProjectFiles(abs, isProjectFile, maxRoslynProjects)
}

func solutionAtOrAbove(dir string) string {
	for {
		solutions := directProjectFiles(dir, isSolutionFile)
		if len(solutions) == 1 {
			return solutions[0]
		}
		if len(solutions) > 1 {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func directProjectFiles(dir string, match func(string) bool) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var found []string
	for _, entry := range entries {
		if !entry.IsDir() && match(entry.Name()) {
			found = append(found, filepath.Join(dir, entry.Name()))
		}
	}
	return found
}

func findProjectFiles(root string, match func(string) bool, limit int) []string {
	var found []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && skippedProjectDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if match(entry.Name()) {
			found = append(found, path)
			if len(found) >= limit {
				return filepath.SkipAll
			}
		}
		return nil
	})
	sort.Strings(found)
	return found
}

func isSolutionFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".sln") || strings.HasSuffix(lower, ".slnx")
}

func isProjectFile(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".csproj")
}
