//go:build !windows

package config

import "os"

func preserveSettingsFileAccess(_, _ string) error { return nil }

func readSettingsFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func replaceSettingsFile(from, to string, rename func(string, string) error) error {
	return rename(from, to)
}

func renameSettingsFile(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
