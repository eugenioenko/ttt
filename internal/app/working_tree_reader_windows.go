//go:build windows

package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func readWorkingTreeFilePlatform(ctx context.Context, root, path string) (workingTreeFile, error) {
	if err := ctx.Err(); err != nil {
		return workingTreeFile{}, err
	}
	rootPath, err := verifiedWindowsRootPath(root)
	if err != nil {
		return workingTreeFile{}, err
	}
	var handles []windows.Handle
	defer func() {
		for i := len(handles) - 1; i >= 0; i-- {
			_ = windows.CloseHandle(handles[i])
		}
	}()

	rootHandle, rootInfo, err := openWindowsWorkingTreePath(rootPath, windows.FILE_READ_ATTRIBUTES)
	if err != nil {
		return workingTreeFile{}, err
	}
	handles = append(handles, rootHandle)
	if rootInfo.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return workingTreeFile{}, &workingTreePathError{Path: root, Kind: workingTreePathSymlinkComponent}
	}
	if rootInfo.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return workingTreeFile{}, &workingTreePathError{Path: root, Kind: workingTreePathDirectory}
	}

	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	current := rootPath
	for _, part := range parts[:len(parts)-1] {
		if err := ctx.Err(); err != nil {
			return workingTreeFile{}, err
		}
		current = filepath.Join(current, part)
		handle, info, openErr := openWindowsWorkingTreePath(current, windows.FILE_READ_ATTRIBUTES)
		if openErr != nil {
			return workingTreeFile{}, classifyWindowsWorkingTreeOpenError(filepath.Join(root, path), openErr)
		}
		handles = append(handles, handle)
		if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return workingTreeFile{}, &workingTreePathError{Path: filepath.Join(root, path), Kind: workingTreePathSymlinkComponent}
		}
		if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
			return workingTreeFile{}, &workingTreePathError{Path: filepath.Join(root, path), Kind: workingTreePathDirectory}
		}
	}

	fullPath := filepath.Join(rootPath, path)
	handle, info, err := openWindowsWorkingTreePath(fullPath, windows.FILE_READ_ATTRIBUTES)
	if err != nil {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			return workingTreeFile{Kind: workingTreeFileMissing}, nil
		}
		return workingTreeFile{}, classifyWindowsWorkingTreeOpenError(filepath.Join(root, path), err)
	}
	handles = append(handles, handle)
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		target, readErr := os.Readlink(fullPath)
		if readErr != nil {
			return workingTreeFile{}, &workingTreePathError{Path: filepath.Join(root, path), Kind: workingTreePathSymlinkComponent, Err: readErr}
		}
		return workingTreeFile{Content: []byte(target), Exists: true, Kind: workingTreeFileSymlink}, nil
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return workingTreeFile{}, &workingTreePathError{Path: filepath.Join(root, path), Kind: workingTreePathDirectory}
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DEVICE != 0 {
		return workingTreeFile{}, &workingTreePathError{Path: filepath.Join(root, path), Kind: workingTreePathDevice}
	}
	fileType, _ := windows.GetFileType(handle)
	if fileType != windows.FILE_TYPE_DISK {
		kind := workingTreePathSpecial
		if fileType == windows.FILE_TYPE_PIPE {
			kind = workingTreePathFIFO
		}
		return workingTreeFile{}, &workingTreePathError{Path: filepath.Join(root, path), Kind: kind}
	}
	readHandle, _, err := openWindowsWorkingTreePath(fullPath, windows.FILE_GENERIC_READ)
	if err != nil {
		return workingTreeFile{}, err
	}
	handles = append(handles, readHandle)
	content, err := readWindowsRegularFileContext(ctx, readHandle)
	if err != nil {
		return workingTreeFile{}, err
	}
	return workingTreeFile{Content: content, Exists: true, Kind: workingTreeFileRegular}, nil
}

func verifiedWindowsRootPath(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", &workingTreePathError{Path: root, Kind: workingTreePathUnverified, Err: err}
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", &workingTreePathError{Path: root, Kind: workingTreePathUnverified, Err: err}
	}
	return filepath.Join(parent, filepath.Base(abs)), nil
}

func openWindowsWorkingTreePath(path string, access uint32) (windows.Handle, windows.ByHandleFileInformation, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, windows.ByHandleFileInformation{}, err
	}
	handle, err := windows.CreateFile(
		pathPtr,
		access,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return windows.InvalidHandle, windows.ByHandleFileInformation{}, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		_ = windows.CloseHandle(handle)
		return windows.InvalidHandle, windows.ByHandleFileInformation{}, err
	}
	return handle, info, nil
}

func classifyWindowsWorkingTreeOpenError(path string, err error) error {
	kind := workingTreePathUnverified
	if errors.Is(err, windows.ERROR_CANT_ACCESS_FILE) {
		kind = workingTreePathSymlinkComponent
	}
	return &workingTreePathError{Path: path, Kind: kind, Err: err}
}

func readWindowsRegularFileContext(ctx context.Context, handle windows.Handle) ([]byte, error) {
	var content bytes.Buffer
	buf := make([]byte, workingTreeReadChunk)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var read uint32
		err := windows.ReadFile(handle, buf, &read, nil)
		if read > 0 {
			_, _ = content.Write(buf[:read])
		}
		if errors.Is(err, io.EOF) || (err == nil && read == 0) {
			return content.Bytes(), nil
		}
		if err != nil {
			return nil, err
		}
	}
}
