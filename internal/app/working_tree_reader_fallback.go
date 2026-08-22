//go:build !linux && !darwin && !freebsd

package app

import (
	"context"
	"path/filepath"
)

func readWorkingTreeFilePlatform(_ context.Context, root, path string) (workingTreeFile, error) {
	return workingTreeFile{}, &workingTreePathError{Path: filepath.Join(root, path), Kind: workingTreePathUnverified}
}
