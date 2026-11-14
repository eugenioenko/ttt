//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package jj

import "syscall"

func mkfifo(path string) error { return syscall.Mkfifo(path, 0o644) }
