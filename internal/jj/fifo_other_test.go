//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package jj

import "errors"

func mkfifo(string) error { return errors.New("no FIFOs on this platform") }
