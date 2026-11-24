package delve

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func terminalScript(child *exec.Cmd, dir string, configuredEnv map[string]string) (string, error) {
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	var script strings.Builder
	script.WriteString("#!/bin/sh\ncd " + quote(child.Dir) + " || exit 1\nexec /usr/bin/env")
	for _, entry := range child.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if _, configured := configuredEnv[name]; configured {
			script.WriteString(" " + quote(entry))
		}
	}
	script.WriteString(" " + quote(child.Path))
	for _, arg := range child.Args[1:] {
		script.WriteString(" " + quote(arg))
	}
	path := filepath.Join(dir, "launch.command")
	return path, os.WriteFile(path, []byte(script.String()+"\n"), 0700)
}

func externalCommand(child *exec.Cmd, dir string, configuredEnv map[string]string) (*exec.Cmd, bool, error) {
	if runtime.GOOS == "darwin" {
		path, err := terminalScript(child, dir, configuredEnv)
		if err != nil {
			return nil, false, err
		}
		const script = `on run argv
tell application "Terminal" to do script (quoted form of (item 1 of argv))
end run`
		cmd := exec.Command("osascript", "-e", script, "--", path)
		return cmd, true, nil
	}
	commands := [][]string{{"gnome-terminal", "--window", "--wait", "--"}, {"konsole", "--separate", "-e"}, {"kitty"}, {"alacritty", "-e"}, {"xterm", "-e"}, {"x-terminal-emulator", "-e"}}
	if runtime.GOOS == "windows" {
		commands = [][]string{{"wt.exe", "-w", "new", "new-tab"}}
	}
	for _, prefix := range commands {
		if exe, err := exec.LookPath(prefix[0]); err == nil {
			cmd := exec.Command(exe, append(prefix[1:], append([]string{child.Path}, child.Args[1:]...)...)...)
			cmd.Dir = child.Dir
			cmd.Env = child.Env
			return cmd, prefix[0] == "x-terminal-emulator" || prefix[0] == "wt.exe" || prefix[0] == "kitty", nil
		}
	}
	return nil, false, fmt.Errorf("no supported external terminal emulator found on PATH")
}
