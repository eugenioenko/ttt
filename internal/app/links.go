package app

import (
	"os"
	"path/filepath"
	"strings"
)

func resolveFilePath(path, dir string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	if !filepath.IsAbs(path) {
		if dir == "" {
			return ""
		}
		path = filepath.Join(dir, path)
	}
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}
