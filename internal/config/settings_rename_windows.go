//go:build windows

package config

import "golang.org/x/sys/windows"

func renameSettingsFile(oldPath, newPath string) error {
	oldName, err := settingsLongPathName(oldPath)
	if err != nil {
		return err
	}
	newName, err := settingsLongPathName(newPath)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(oldName, newName, windows.MOVEFILE_REPLACE_EXISTING)
}
