//go:build windows

package terminal

import "github.com/aymanbagabas/go-pty"

func makePtyPollable(pt pty.Pty) (pty.Pty, error) { return pt, nil }
