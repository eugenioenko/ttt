package trash

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

func Move(path string) error {
	return MoveContext(context.Background(), path)
}

func MoveContext(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if path == "" || strings.IndexByte(path, 0) >= 0 {
		return &os.PathError{Op: "trash", Path: path, Err: os.ErrInvalid}
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		_, err = os.Lstat(abs)
	}
	if err == nil {
		err = move(ctx, abs)
	}
	if err != nil {
		return &os.PathError{Op: "trash", Path: path, Err: err}
	}
	return nil
}
