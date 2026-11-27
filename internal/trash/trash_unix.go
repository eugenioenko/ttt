//go:build (linux && !android) || freebsd || openbsd || netbsd || dragonfly

package trash

import (
	"context"
	"fmt"
	"os/exec"
)

func move(ctx context.Context, path string) error {
	gio, err := exec.LookPath("gio")
	if err != nil {
		return fmt.Errorf("system Trash requires gio (GLib): %w", err)
	}
	_, err = run(ctx, gio, "trash", "--", path)
	return err
}
