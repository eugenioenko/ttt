//go:build windows

package config

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func settingsLongPathName(path string) (*uint16, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(absPath, `\\?\`) {
		if strings.HasPrefix(absPath, `\\`) {
			absPath = `\\?\UNC\` + strings.TrimPrefix(absPath, `\\`)
		} else {
			absPath = `\\?\` + absPath
		}
	}
	return windows.UTF16PtrFromString(absPath)
}
