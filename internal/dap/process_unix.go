//go:build unix

package dap

import (
	"os/exec"
	"syscall"
)

func isolateAdapter(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func killAdapter(cmd *exec.Cmd)    { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
