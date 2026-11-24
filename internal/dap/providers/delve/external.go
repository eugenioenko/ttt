package delve

import (
	"fmt"
	"os/exec"
	"runtime"
)

func externalCommand(child *exec.Cmd) (*exec.Cmd, bool, error) {
	if runtime.GOOS == "darwin" {
		const script = `on run argv
set launchCommand to "cd " & quoted form of (item 1 of argv) & " && exec " & quoted form of (item 2 of argv)
repeat with arg in items 3 thru -1 of argv
set launchCommand to launchCommand & " " & quoted form of arg
end repeat
tell application "Terminal" to do script launchCommand
end run`
		cmd := exec.Command("osascript", append([]string{"-e", script, "--", child.Dir, child.Path}, child.Args[1:]...)...)
		cmd.Dir = child.Dir
		cmd.Env = child.Env
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
