package dap

import (
	"context"
	"os/exec"
	"time"
)

func stdioProcess(ctx context.Context, cmd *exec.Cmd, onEvent func(Event), options SessionOptions) (*Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	isolateAdapter(cmd)
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
	options.openChild = func(ctx context.Context) (*Session, error) {
		child := exec.Command(cmd.Path, cmd.Args[1:]...)
		child.Dir = cmd.Dir
		child.Env = cmd.Env
		return stdioProcess(ctx, child, onEvent, options)
	}
	return makeSession(stdout, stdin, stdin, cleanup, onEvent, options), nil
}
