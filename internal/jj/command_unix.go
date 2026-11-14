//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package jj

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// SIGTERM rather than SIGKILL: any jj command may snapshot the working copy,
// and an orderly exit releases its working-copy lock.
func commandContext(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = time.Second
	return cmd
}
