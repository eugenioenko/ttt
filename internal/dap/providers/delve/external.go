package delve

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/eugenioenko/ttt/internal/dap"
)

func externalCommand(dlv, address string) (*exec.Cmd, error) {
	commands := [][]string{{"x-terminal-emulator", "-e"}, {"gnome-terminal", "--window", "--wait", "--"}, {"konsole", "--separate", "-e"}, {"kitty"}, {"alacritty", "-e"}, {"xterm", "-e"}}
	if runtime.GOOS == "windows" {
		commands = [][]string{{"wt.exe", "-w", "new", "new-tab"}}
	}
	for _, prefix := range commands {
		if exe, err := exec.LookPath(prefix[0]); err == nil {
			return exec.Command(exe, append(prefix[1:], dlv, "dap", "--client-addr", address)...), nil
		}
	}
	return nil, fmt.Errorf("no supported external terminal emulator found on PATH")
}

func openExternal(ctx context.Context, t dap.Target, onEvent func(dap.Event)) (*dap.Session, error) {
	dlv, err := exec.LookPath("dlv")
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "ttt-dap-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	socket := filepath.Join(dir, "debug.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	defer func() { _ = l.Close() }()
	cmd, err := externalCommand(dlv, "unix:"+socket)
	if err != nil {
		return nil, err
	}
	cmd.Dir = t.Folder
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	exited := make(chan struct{})
	var launchErr error
	go func() {
		launchErr = cmd.Wait()
		close(exited)
		if launchErr != nil {
			_ = l.Close()
		}
	}()
	stop := context.AfterFunc(ctx, func() { _ = l.Close() })
	conn, err := l.Accept()
	stop()
	if err != nil {
		select {
		case <-exited:
			if launchErr != nil {
				return nil, fmt.Errorf("external terminal: %w", launchErr)
			}
		default:
			_ = cmd.Process.Kill()
			<-exited
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	return dap.SocketSession(conn, func() {
		select {
		case <-exited:
		case <-time.After(2 * time.Second):
			_ = cmd.Process.Kill()
			<-exited
		}
	}, onEvent, dap.SessionOptions{}), nil
}
