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
	for pid, want := range map[int]bool{os.Getpid(): true, os.Getppid(): false} {
		got, err := ownsInspectorListener(context.Background(), pid, listener.Addr().(*net.TCPAddr).Port, listener.Addr().String())
		if err != nil || got != want {
			t.Fatalf("pid %d: got %v, want %v: %v", pid, got, want, err)
		}
	}
}
