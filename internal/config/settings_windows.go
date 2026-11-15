//go:build windows

package config

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

func preserveSettingsFileAccess(source, destination string) error {
	sd, err := windows.GetNamedSecurityInfo(source, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil || sd == nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	control, _, err := sd.Control()
	if err != nil {
		return err
	}
	securityInfo := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION)
	if control&windows.SE_DACL_PROTECTED != 0 {
		securityInfo |= windows.PROTECTED_DACL_SECURITY_INFORMATION
	} else {
		securityInfo |= windows.UNPROTECTED_DACL_SECURITY_INFORMATION
	}
	return windows.SetNamedSecurityInfo(destination, windows.SE_FILE_OBJECT, securityInfo, nil, nil, dacl, nil)
}

func readSettingsFileBytes(path string) ([]byte, error) {
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

func replaceSettingsFile(from, to string, rename func(string, string) error) error {
	for attempt := 0; ; attempt++ {
		err := rename(from, to)
		if err == nil || !isWindowsRenameRetryable(err) || attempt == 19 {
			return err
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func isWindowsRenameRetryable(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
