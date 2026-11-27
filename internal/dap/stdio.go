package dap

import (
	"errors"
	"os"
	"os/exec"
	"time"
)

// StartStdio owns the command and its pipes. Callers may set Dir, Env and Stderr
// before starting the adapter.
func StartStdio(cmd *exec.Cmd, onEvent func(Event)) (*Session, error) {
	if cmd.Stdout != nil {
		return nil, errors.New("DAP adapter stdout already set")
	}
	if cmd.WaitDelay == 0 {
		cmd.WaitDelay = time.Second
	}
	// Wait must reap the adapter even if a descendant inherits stdout. Owning
	// this pipe lets the reader drain the final frame after Wait returns.
	stdout, childOut, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer childOut.Close()
	cmd.Stdout = childOut
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
	exited := make(chan struct{})
	s := newSession(stdout, stdin, func() {
		_ = stdin.Close()
		_ = stdout.Close()
		select {
		case <-exited:
		case <-time.After(time.Second):
			_ = cmd.Process.Kill()
			<-exited
		}
	}, onEvent)
	go func() {
		_ = cmd.Wait()
		close(exited)
		select {
		case <-s.Done():
		case <-time.After(time.Second):
			_ = stdout.Close()
		}
	}()
	return s, nil
}
