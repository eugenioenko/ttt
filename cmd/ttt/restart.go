package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/update"
)

func restart(method update.Kind) (int, error) {
	current, err := os.Executable()
	if err != nil {
		return 1, err
	}
	path, err := restartPath(method, os.Args[0], current)
	if err != nil {
		return 1, err
	}
	return restartProcess(path, os.Args, os.Environ())
}

func restartPath(method update.Kind, original, current string) (string, error) {
	if method == update.Homebrew || method == update.Scoop {
		if path, err := exec.LookPath("ttt"); err == nil {
			return filepath.EvalSymlinks(path)
		}
	}
	path, err := exec.LookPath(original)
	if err != nil {
		path = current
	}
	return filepath.EvalSymlinks(path)
}
