//go:build windows

package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		moveErr := windows.MoveFileEx(oldName, newName, 0)
		if moveErr == nil {
			return nil
		}
		if _, statErr := os.Lstat(newPath); statErr != nil {
			return moveErr
		}
	} else if err != nil {
		return err
	}
	return replaceExistingSettingsFile(oldPath, newPath, oldName, newName)
}

func replaceExistingSettingsFile(tempPath, newPath string, oldName, newName *uint16) error {
	backupPath, err := uniqueSettingsBackupPath(filepath.Dir(newPath), filepath.Base(newPath))
	if err != nil {
		return err
	}
	backupName, err := windows.UTF16PtrFromString(backupPath)
	if err != nil {
		return err
	}
	// ReplaceFileW preserves the destination's identity and ACL. The unique
	// backup lets us restore the old settings if Windows reports a partial move.
	result, _, callErr := replaceSettingsFileProc.Call(
		uintptr(unsafe.Pointer(newName)),
		uintptr(unsafe.Pointer(oldName)),
		uintptr(unsafe.Pointer(backupName)), 0, 0, 0,
	)
	runtime.KeepAlive(oldName)
	runtime.KeepAlive(newName)
	runtime.KeepAlive(backupName)
	if result != 0 {
		if err := os.Remove(backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("settings saved but could not remove backup %s: %w", backupPath, err)
		}
		return nil
	}
	if callErr == nil || callErr == syscall.Errno(0) {
		callErr = windows.ERROR_GEN_FAILURE
	}
	return recoverSettingsReplacement(tempPath, newPath, backupPath, callErr)
}

func uniqueSettingsBackupPath(dir, base string) (string, error) {
	for range 100 {
		var suffix [12]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return "", err
		}
		path := filepath.Join(dir, "."+base+"-"+hex.EncodeToString(suffix[:])+".bak")
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			return path, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("could not allocate a temporary settings backup path")
}

func recoverSettingsReplacement(tempPath, destinationPath, backupPath string, replaceErr error) error {
	if _, err := os.Lstat(destinationPath); err == nil {
		if _, backupErr := os.Lstat(backupPath); backupErr == nil {
			return fmt.Errorf("settings replacement failed; original retained at %s: %w", backupPath, replaceErr)
		}
		return replaceErr
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("settings replacement failed; could not inspect destination: %w", errors.Join(replaceErr, err))
	}
	if _, err := os.Lstat(backupPath); err == nil {
		backupName, pathErr := windows.UTF16PtrFromString(backupPath)
		if pathErr != nil {
			return fmt.Errorf("settings replacement failed; original retained at %s: %w", backupPath, errors.Join(replaceErr, pathErr))
		}
		destinationName, pathErr := windows.UTF16PtrFromString(destinationPath)
		if pathErr != nil {
			return fmt.Errorf("settings replacement failed; original retained at %s: %w", backupPath, errors.Join(replaceErr, pathErr))
		}
		if restoreErr := windows.MoveFileEx(backupName, destinationName, 0); restoreErr != nil {
			return fmt.Errorf("settings replacement failed; original retained at %s because restore failed: %w", backupPath, errors.Join(replaceErr, restoreErr))
		}
		return replaceErr
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("settings replacement failed; original may be retained at %s: %w", backupPath, errors.Join(replaceErr, err))
	}
	if _, err := os.Lstat(tempPath); err == nil {
		tempName, pathErr := windows.UTF16PtrFromString(tempPath)
		if pathErr == nil {
			destinationName, pathErr := windows.UTF16PtrFromString(destinationPath)
			if pathErr == nil {
				restoreErr := windows.MoveFileEx(tempName, destinationName, 0)
				if restoreErr == nil {
					return nil
				}
				pathErr = restoreErr
			}
		}
		return &retainedSettingsTempError{path: tempPath, err: errors.Join(replaceErr, pathErr)}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("settings replacement failed; could not inspect staged data: %w", errors.Join(replaceErr, err))
	}
	return replaceErr
}
