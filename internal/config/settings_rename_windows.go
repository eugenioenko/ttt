//go:build windows

package config

import (
	"errors"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var replaceSettingsFileProc = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

func renameSettingsFile(oldPath, newPath string) error {
	oldName, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return err
	}
	newName, err := windows.UTF16PtrFromString(newPath)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(newPath); errors.Is(err, os.ErrNotExist) {
		return windows.MoveFileEx(oldName, newName, 0)
	} else if err != nil {
		return err
	}

	// ReplaceFileW replaces an existing destination in one operation, unlike
	// MoveFileEx's remove-then-move behavior on Windows.
	result, _, callErr := replaceSettingsFileProc.Call(
		uintptr(unsafe.Pointer(newName)),
		uintptr(unsafe.Pointer(oldName)),
		0, 0, 0, 0,
	)
	runtime.KeepAlive(oldName)
	runtime.KeepAlive(newName)
	if result != 0 {
		return nil
	}
	if errno, ok := callErr.(syscall.Errno); ok && errno != 0 {
		return errno
	}
	return windows.ERROR_GEN_FAILURE
}
