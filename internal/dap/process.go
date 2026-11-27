package dap

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func (a Adapter) Process(folder string) (*exec.Cmd, error) {
	if len(a.Command) == 0 || a.Command[0] == "" {
		return nil, fmt.Errorf("adapter command is empty")
	}
	expand := strings.NewReplacer("${workspaceFolder}", folder).Replace
	argv := make([]string, len(a.Command))
	for i, s := range a.Command {
		argv[i] = expand(s)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = expand(a.Cwd)
	if !filepath.IsAbs(cmd.Dir) {
		cmd.Dir = filepath.Join(folder, cmd.Dir)
	}
	cmd.Env = cmd.Environ()
	for k, v := range a.Env {
		cmd.Env = append(cmd.Env, k+"="+expand(v))
	}
	return cmd, nil
}
