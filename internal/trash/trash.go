package trash

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func Move(path string) error {
	if path == "" || strings.IndexByte(path, 0) >= 0 {
		return &os.PathError{Op: "trash", Path: path, Err: os.ErrInvalid}
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		_, err = os.Lstat(abs)
	}
	if err == nil {
		err = move(abs)
	}
	if err != nil {
		return &os.PathError{Op: "trash", Path: path, Err: err}
	}
	return nil
}

func run(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if detail := strings.TrimSpace(string(output)); detail != "" {
			return nil, fmt.Errorf("%s: %w: %s", name, err, detail)
		}
	}
	return output, err
}
