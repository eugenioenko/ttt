//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package app

import "os/exec"

func configureFormatterProcess(cmd *exec.Cmd) {}
