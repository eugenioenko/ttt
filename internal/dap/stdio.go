package dap

import (
	"context"
	"os/exec"
	"time"
)

func StdioProcess(ctx context.Context, binary string, args []string, folder string, onEvent func(Event)) (*Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := exec.Command(binary, args...)
	cmd.Dir = folder
	cmd.Stderr = adapterStderr(onEvent)
	cmd.WaitDelay = time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	cleanup := func() { _ = cmd.Process.Kill(); _ = cmd.Wait(); _ = stdout.Close() }
	return newSession(stdout, stdin, stdin, cleanup, onEvent), nil
}
