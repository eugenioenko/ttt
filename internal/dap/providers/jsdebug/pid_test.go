package jsdebug

import (
	"context"
	"net"
	"os"
	"os/exec"
	"runtime"
	"testing"
)

func TestPIDMustOwnInspectorPort(t *testing.T) {
	if runtime.GOOS != "windows" {
		if _, err := exec.LookPath("lsof"); err != nil {
			t.Skip("lsof unavailable")
		}
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if ws, err := inspectorWebsocket(context.Background(), os.Getppid(), listener.Addr().(*net.TCPAddr).Port); err == nil || ws != "" {
		t.Fatalf("accepted another process's port: %q", ws)
	}
}
