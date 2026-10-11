//go:build !windows

package terminal

import (
	"os"
	"os/signal"
	"syscall"
)

type shellSignal = syscall.Signal

const (
	sigHangup = syscall.SIGHUP
	sigKill   = syscall.SIGKILL
)

// go-pty starts the shell with Setsid, so its pid is also its process group id.
// An interactive shell forwards SIGHUP to its jobs, which live in their own groups.
func signalProcessGroup(p *os.Process, sig shellSignal) {
	if err := syscall.Kill(-p.Pid, sig); err != nil {
		_ = p.Signal(sig)
	}
}

var discardedHangups = make(chan os.Signal, 1)

// A parent that ignores SIGHUP (nohup, some launchers, CI runners) passes that
// on to the shell and its jobs, so hanging up the pty would leave the jobs
// running. Go resets a signal to its default in children only while it handles
// the signal itself, so handle SIGHUP just for the shell's fork.
func defaultHangupForChild() (restore func()) {
	if !signal.Ignored(syscall.SIGHUP) {
		return func() {}
	}
	signal.Notify(discardedHangups, syscall.SIGHUP)
	return func() { signal.Ignore(syscall.SIGHUP) }
}
