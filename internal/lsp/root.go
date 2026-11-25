package lsp

import (
	"os"
	"path/filepath"
)

// FindRoot walks up from startDir and returns the first directory containing
// one of markers. Markers are file or directory names; a marker containing a
// glob metacharacter matches names in each directory (for example "*.sln").
func FindRoot(startDir string, markers []string) string {
	if startDir == "" || len(markers) == 0 {
		return ""
	}
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return ""
	}
	for {
		if containsMarker(dir, markers) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func containsMarker(dir string, markers []string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if MatchesMarker(entry.Name(), markers) {
			return true
		}
	}
	return false
}

// MatchesMarker reports whether name matches any of markers with the same
// glob-or-exact rules FindRoot applies to directory entries.
func MatchesMarker(name string, markers []string) bool {
	for _, marker := range markers {
		if marker == "" {
			continue
		}
		if matched, _ := filepath.Match(marker, name); matched {
			return true
		}
	}
	return false
}
