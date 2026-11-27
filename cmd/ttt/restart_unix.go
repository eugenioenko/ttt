//go:build !windows

package main

import "syscall"

func restartProcess(path string, args, env []string) (int, error) {
	return 1, syscall.Exec(path, args, env)
}
