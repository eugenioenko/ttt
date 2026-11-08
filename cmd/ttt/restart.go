package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func restart() (int, error) {
	path, err := exec.LookPath(os.Args[0])
	if err != nil {
		path, err = os.Executable()
	}
	if err != nil {
		return 1, err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return 1, err
	}
	return restartProcess(path, os.Args, os.Environ())
}
