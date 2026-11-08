package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/update"
)

type restartTarget struct{ launch, stable string }

func captureRestartTarget(original string) (restartTarget, error) {
	launch, err := absoluteLauncher(original)
	if err != nil {
		launch, err = os.Executable()
		if err != nil {
			return restartTarget{}, err
		}
		launch, err = filepath.Abs(launch)
		if err != nil {
			return restartTarget{}, err
		}
	}
	stable, err := exec.LookPath("ttt")
	if err == nil {
		stable, err = filepath.Abs(stable)
	}
	if err != nil {
		stable = ""
	}
	return restartTarget{launch: launch, stable: stable}, nil
}

func absoluteLauncher(command string) (string, error) {
	path, err := exec.LookPath(command)
	// The parent already launched this command; retain its relative PATH entry.
	if err != nil && !errors.Is(err, exec.ErrDot) {
		return "", err
	}
	return filepath.Abs(path)
}

func (t restartTarget) restart(method update.Kind) (int, error) {
	path, err := t.path(method)
	if err != nil {
		return 1, err
	}
	return restartProcess(path, os.Args, os.Environ())
}

func (t restartTarget) path(method update.Kind) (string, error) {
	path := t.launch
	if (method == update.Homebrew || method == update.Scoop) && t.stable != "" {
		path = t.stable
	}
	return filepath.EvalSymlinks(path)
}
