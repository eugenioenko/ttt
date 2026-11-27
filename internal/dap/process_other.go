//go:build !unix

package dap

import "os/exec"

func isolateAdapter(cmd *exec.Cmd) {}
func killAdapter(cmd *exec.Cmd)    { _ = cmd.Process.Kill() }
