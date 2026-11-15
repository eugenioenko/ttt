//go:build !windows

package config

import "os"

func readSettingsFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}
