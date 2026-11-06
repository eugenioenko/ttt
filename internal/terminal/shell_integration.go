package terminal

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed shellintegration/bash.sh
var bashIntegration []byte

// IntegrateShell returns the program and arguments that start shell with ttt's
// OSC 133 prompt marks. Shells that mark prompts natively (fish), shells
// without an integration, and explicit arguments are returned unchanged.
func IntegrateShell(shell string, args []string) (string, []string) {
	if shell == "" {
		shell = defaultShell()
	}
	if len(args) > 0 || filepath.Base(shell) != "bash" {
		return shell, args
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return shell, args
	}
	path, err := writeIntegration(filepath.Join(dir, "ttt", "shell-integration"), "bash-integration.sh", bashIntegration)
	if err != nil {
		return shell, args
	}
	return shell, []string{"--rcfile", path}
}

func writeIntegration(dir, name string, script []byte) (string, error) {
	path := filepath.Join(dir, name)
	if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, script) {
		return path, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	// Another ttt process may be starting a shell from the same file.
	tmp, err := os.CreateTemp(dir, name+".*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(script); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return path, os.Rename(tmp.Name(), path)
}
