package main

import (
	"errors"
	"os"
	"os/exec"
)

func restartProcess(path string, args, env []string) (int, error) {
	cmd := exec.Command(path)
	cmd.Args, cmd.Env = args, env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 0, err
}
