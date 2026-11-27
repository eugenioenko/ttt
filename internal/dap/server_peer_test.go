package dap

import (
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestServerVerifiesAcceptedConnection(t *testing.T) {
	if os.Getenv("TTT_TEST_ACCEPT_DAP") != "" {
		listener, err := net.Listen("tcp4", os.Args[len(os.Args)-1])
		if err != nil {
			os.Exit(2)
		}
		conn, err := listener.Accept()
		if err != nil {
			os.Exit(3)
		}
		_, _ = io.Copy(io.Discard, conn)
		os.Exit(0)
	}
	if runtime.GOOS != "windows" {
		if _, err := exec.LookPath("lsof"); err != nil {
			t.Skip("requires lsof")
		}
	}
	a := Adapter{Transport: "server", Command: []string{os.Args[0], "-test.run=^TestServerVerifiesAcceptedConnection$", "--", "127.0.0.1:${port}"}, Env: map[string]string{"TTT_TEST_ACCEPT_DAP": "1"}}
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	session, err := a.Open(ctx, Target{Folder: t.TempDir()}, nil, nil, SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	session.Close()
}
