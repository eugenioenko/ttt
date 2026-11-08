package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestRestartProcess(t *testing.T) {
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cmd := exec.Command(path, "-test.run=^TestRestartHelper$", "--", "file with spaces.txt")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TTT_RESTART_STAGE=launch", "TTT_RESTART_DIR="+dir)
	cmd.Stdin = strings.NewReader("console input")
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 || !strings.Contains(string(output), "restarted stdout") || !strings.Contains(string(output), "restarted stderr") {
		t.Fatalf("restart: %v, output %q", err, output)
	}
	if _, err := restartProcess(dir+"/missing-ttt", []string{"missing-ttt"}, os.Environ()); err == nil {
		t.Fatal("missing executable succeeded")
	}
}

func TestRestartHelper(t *testing.T) {
	stage := os.Getenv("TTT_RESTART_STAGE")
	if stage == "" {
		return
	}
	if stage == "launch" {
		os.Setenv("TTT_RESTART_STAGE", "restarted")
		os.Setenv("TTT_RESTART_PID", strconv.Itoa(os.Getpid()))
		code, err := restartProcess(os.Args[0], os.Args, os.Environ())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(99)
		}
		os.Exit(code)
	}
	dir, _ := os.Getwd()
	input, _ := io.ReadAll(os.Stdin)
	if dir != os.Getenv("TTT_RESTART_DIR") || string(input) != "console input" || os.Args[len(os.Args)-1] != "file with spaces.txt" {
		fmt.Fprintf(os.Stderr, "lost context: %s %q %v", dir, input, os.Args)
		os.Exit(98)
	}
	if runtime.GOOS != "windows" && strconv.Itoa(os.Getpid()) != os.Getenv("TTT_RESTART_PID") {
		os.Exit(97)
	}
	fmt.Fprintln(os.Stdout, "restarted stdout")
	fmt.Fprintln(os.Stderr, "restarted stderr")
	os.Exit(23)
}
