//go:build windows

package terminal

import "os"

type shellSignal int

const (
	sigHangup shellSignal = iota
	sigKill
)

func signalProcessGroup(p *os.Process, _ shellSignal) {
	_ = p.Kill()
}

func defaultHangupForChild() (restore func()) { return func() {} }
