//go:build !windows

package app

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func forwardDirOK(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Getuid()) && info.Mode().Perm()&0077 == 0
}

func forwardProcessDead(name string) bool {
	name = strings.TrimSuffix(name, ".sock")
	pid, err := strconv.ParseInt(name[strings.LastIndexByte(name, '-')+1:], 36, 32)
	return err == nil && pid > 0 && errors.Is(syscall.Kill(int(pid), 0), syscall.ESRCH)
}
