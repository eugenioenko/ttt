package dap

import (
	"os/exec"
	"time"
)

// StartStdio owns the command and its pipes. Callers may set Dir, Env and Stderr
// before starting the adapter.
func StartStdio(cmd *exec.Cmd, onEvent func(Event)) (*Session, error) {
	if cmd.WaitDelay == 0 {
		cmd.WaitDelay = time.Second
	}
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
	return newSession(stdout, stdin, func() {
		_ = stdin.Close()
		_ = stdout.Close()
		// Give a disconnected adapter time to exit, then reap it even if it ignores EOF.
		kill := time.AfterFunc(time.Second, func() { _ = cmd.Process.Kill() })
		_ = cmd.Wait()
		kill.Stop()
	}, onEvent), nil
}
