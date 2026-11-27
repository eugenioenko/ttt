package dap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"time"
)

func ReverseProcess(ctx context.Context, socket, folder string, command func(string) (*exec.Cmd, bool, error), onEvent func(Event), options SessionOptions) (*Session, error) {
	l, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	defer func() { _ = l.Close() }()
	cmd, handoff, err := command("unix:" + socket)
	if err != nil {
		return nil, err
	}
	if cmd.Dir == "" {
		cmd.Dir = folder
	}
	if cmd.Stderr == nil {
		cmd.Stderr = adapterStderr(onEvent)
	}
	cmd.WaitDelay = time.Second
	isolateAdapter(cmd)
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	exited := make(chan struct{})
	var launchErr error
	go func() {
		launchErr = cmd.Wait()
		if handoff && errors.Is(launchErr, exec.ErrWaitDelay) {
			launchErr = nil
		}
		close(exited)
		if launchErr != nil || !handoff {
			_ = l.Close()
		}
	}()
	stop := context.AfterFunc(ctx, func() { _ = l.Close() })
	conn, err := l.Accept()
	stop()
	if err != nil {
		select {
		case <-exited:
			if launchErr != nil {
				return nil, fmt.Errorf("external terminal: %w", launchErr)
			}
			if handoff && ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("adapter launcher exited before connecting")
		default:
			killAdapter(cmd)
			<-exited
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	return SocketSession(conn, func() {
		select {
		case <-exited:
		case <-time.After(2 * time.Second):
			killAdapter(cmd)
			<-exited
		}
	}, onEvent, options), nil
}
