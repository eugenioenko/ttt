package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type workingTreeFileKind uint8

const (
	workingTreeFileMissing workingTreeFileKind = iota
	workingTreeFileRegular
	workingTreeFileSymlink
)

type workingTreePathKind string

const (
	workingTreePathSymlinkComponent workingTreePathKind = "symlink component"
	workingTreePathDirectory        workingTreePathKind = "directory"
	workingTreePathFIFO             workingTreePathKind = "FIFO"
	workingTreePathDevice           workingTreePathKind = "device"
	workingTreePathSocket           workingTreePathKind = "socket"
	workingTreePathSpecial          workingTreePathKind = "special file"
	workingTreePathUnverified       workingTreePathKind = "unverified path"
)

type workingTreePathError struct {
	Path string
	Kind workingTreePathKind
	Err  error
}

func (e *workingTreePathError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("unsupported working tree %s %q: %v", e.Kind, e.Path, e.Err)
	}
	return fmt.Sprintf("unsupported working tree %s %q", e.Kind, e.Path)
}

func (e *workingTreePathError) Unwrap() error { return e.Err }

type workingTreeFile struct {
	Content []byte
	Exists  bool
	Kind    workingTreeFileKind
}

func readWorkingTreeFileContext(ctx context.Context, root, path string) (workingTreeFile, error) {
	if err := ctx.Err(); err != nil {
		return workingTreeFile{}, err
	}
	clean := filepath.Clean(path)
	if path == "" || clean == "." || filepath.IsAbs(path) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return workingTreeFile{}, &workingTreePathError{Path: path, Kind: workingTreePathUnverified, Err: errors.New("path escapes repository root")}
	}
	return readWorkingTreeFilePlatform(ctx, root, clean)
}

func readWorkingTreeContent(path string) ([]byte, bool, error) {
	file, err := readWorkingTreeFileContext(context.Background(), filepath.Dir(path), filepath.Base(path))
	return file.Content, file.Exists, err
}
