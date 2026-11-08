//go:build !linux && !darwin && !freebsd && !windows

package git

import (
	"context"
	"path/filepath"
)

func readWorkingTreeFilePlatform(_ context.Context, root, path string) (workingTreeFile, error) {
	return workingTreeFile{}, &workingTreePathError{Path: filepath.Join(root, path), Kind: workingTreePathUnverified}
}
