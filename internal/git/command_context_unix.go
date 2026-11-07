//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func gitCommandContext(ctx context.Context, args ...string) *exec.Cmd {
	return gitCommandWithSignal(ctx, syscall.SIGKILL, args...)
}

func gitMutationCommandContext(ctx context.Context, args ...string) *exec.Cmd {
	// Let Git remove index.lock before WaitDelay escalates to a forced kill.
	return gitCommandWithSignal(ctx, syscall.SIGTERM, args...)
}

func gitCommandWithSignal(ctx context.Context, signal syscall.Signal, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, signal)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = time.Second
	return cmd
}
