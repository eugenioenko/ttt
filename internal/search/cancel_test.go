package search

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestScanCancellationProcess(t *testing.T) {
	if address := os.Getenv("TTT_TEST_SCAN_ADDRESS"); address != "" {
		conn, err := net.Dial("tcp", address)
		if err != nil {
			os.Exit(2)
		}
		defer conn.Close()
		_, _ = conn.Write([]byte{1})
		_, _ = io.Copy(io.Discard, conn)
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestScanCancellationProcess$")
	cmd.Env = append(os.Environ(), "TTT_TEST_SCAN_ADDRESS="+listener.Addr().String())
	done := make(chan error, 1)
	go func() { _, err := scanCommand(ctx, cmd); done <- err }()
	if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(conn, make([]byte, 1)); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || cmd.ProcessState == nil {
			t.Fatalf("cancelled process was not reaped: %v, %v", err, cmd.ProcessState)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel left scan subprocess running")
	}
}

func TestFindAllMatchesAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := FindAllMatches(ctx, Options{Query: "foo"}, []string{"unused"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel = %v", err)
	}
}
