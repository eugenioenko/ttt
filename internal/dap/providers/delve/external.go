package delve

import (
	"fmt"
	"os/exec"
	"runtime"
)

func externalCommand(folder, dlv, address string) (*exec.Cmd, bool, error) {
	if runtime.GOOS == "darwin" {
		const script = `on run argv
set launchCommand to "cd " & quoted form of (item 1 of argv) & " && exec " & quoted form of (item 2 of argv) & " dap --client-addr " & quoted form of (item 3 of argv)
tell application "Terminal" to do script launchCommand
end run`
		return exec.Command("osascript", "-e", script, "--", folder, dlv, address), true, nil
	}
	commands := [][]string{{"gnome-terminal", "--window", "--wait", "--"}, {"konsole", "--separate", "-e"}, {"kitty"}, {"alacritty", "-e"}, {"xterm", "-e"}, {"x-terminal-emulator", "-e"}}
	if runtime.GOOS == "windows" {
		commands = [][]string{{"wt.exe", "-w", "new", "new-tab"}}
	}
	for _, prefix := range commands {
		if exe, err := exec.LookPath(prefix[0]); err == nil {
			return exec.Command(exe, append(prefix[1:], dlv, "dap", "--client-addr", address)...), prefix[0] == "x-terminal-emulator" || prefix[0] == "wt.exe" || prefix[0] == "kitty", nil
		}
	}
	return nil, false, fmt.Errorf("no supported external terminal emulator found on PATH")
}
