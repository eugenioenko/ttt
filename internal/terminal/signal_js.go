//go:build js && wasm

package terminal

import "os"

type shellSignal int

const (
	sigHangup shellSignal = iota
	sigKill
)

func signalProcessGroup(_ *os.Process, _ shellSignal) {}

func defaultHangupForChild() func() { return func() {} }
