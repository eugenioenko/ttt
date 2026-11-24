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
	isolateAdapter(cmd)
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
	cleanup := func() {
		exited := make(chan struct{})
		go func() { _ = cmd.Wait(); close(exited) }()
		select {
		case <-exited:
		case <-time.After(time.Second):
			killAdapter(cmd)
			<-exited
		}
		_ = stdout.Close()
	}
	return newSession(stdout, stdin, stdin, cleanup, onEvent), nil
}
