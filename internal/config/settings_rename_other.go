//go:build !windows

package config

import "os"

func renameSettingsFile(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
