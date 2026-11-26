package lsp

import (
	"os"
	"path/filepath"
	"strings"
)

// FindRoot returns the nearest directory at or above startDir that contains
// a marker, trying markers in priority order: the first marker found anywhere
// up the tree wins over a later marker in a nearer directory, so ["go.work",
// "go.mod"] roots a multi-module workspace at its go.work. A marker with a
// glob metacharacter matches names in each directory (for example "*.sln").
func FindRoot(startDir string, markers []string) string {
	if startDir == "" {
		return ""
	}
	start, err := filepath.Abs(startDir)
	if err != nil {
		return ""
	}
	for _, marker := range markers {
		if marker == "" {
			continue
		}
		for dir := start; ; {
			if containsMarker(dir, marker) {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return ""
}

func containsMarker(dir, marker string) bool {
	if !strings.ContainsAny(marker, `*?[\`) {
		_, err := os.Lstat(filepath.Join(dir, marker))
		return err == nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if matched, _ := filepath.Match(marker, entry.Name()); matched {
			return true
		}
	}
	return false
}
