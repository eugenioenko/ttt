//go:build windows

package config

import (
	"io"
	"os"
	"sync"

	"golang.org/x/sys/windows"
)

// MoveFileEx can reject replacement while our readers hold the destination,
// even with FILE_SHARE_DELETE. Keep those handles out of the rename window.
var settingsFileMu sync.RWMutex

func readSettingsFileBytes(path string) ([]byte, error) {
	settingsFileMu.RLock()
	defer settingsFileMu.RUnlock()

	name, err := settingsLongPathName(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return nil, &os.PathError{Op: "open", Path: path, Err: windows.ERROR_INVALID_HANDLE}
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, &os.PathError{Op: "read", Path: path, Err: err}
	}
	return data, nil
}
