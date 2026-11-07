//go:build (linux && !android) || freebsd || openbsd || netbsd || dragonfly

package trash

import (
	"fmt"
	"os/exec"
)

func move(path string) error {
	gio, err := exec.LookPath("gio")
	if err != nil {
		return fmt.Errorf("system Trash requires gio (GLib): %w", err)
	}
	_, err = run(gio, "trash", "--", path)
	return err
}
