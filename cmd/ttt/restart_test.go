package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/update"
)

func TestRestartPath(t *testing.T) {
	dir := t.TempDir()
	name := "ttt"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	stable := filepath.Join(dir, name)
	old := filepath.Join(dir, "old-"+name)
	for _, path := range []string{stable, old} {
		if err := os.WriteFile(path, []byte("executable"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	target, err := captureRestartTarget(old)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []update.Kind{update.Homebrew, update.Scoop, update.Binary, update.GoInstall} {
		want := old
		if method == update.Homebrew || method == update.Scoop {
			want = stable
		}
		path, err := target.path(method)
		if err != nil || path != want {
			t.Fatalf("method %v: %q, %v; want %q", method, path, err, want)
		}
	}
}

func TestRestartTargetSurvivesReplacementAndRelativePATH(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("PATH", "bin")
	if err := os.Mkdir("bin", 0o755); err != nil {
		t.Fatal(err)
	}
	name := "ttt"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, "bin", name)
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	target, err := captureRestartTarget(name)
	if err != nil || target.launch != path {
		t.Fatalf("captured target = %+v, %v", target, err)
	}
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path + ".old"); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	for _, method := range []update.Kind{update.Binary, update.Homebrew, update.Scoop} {
		resolved, err := target.path(method)
		if err != nil || resolved != path {
			t.Fatalf("restart target = %q, %v", resolved, err)
		}
	}
}

func TestRestartTargetPreservesLaunchSymlink(t *testing.T) {
	dir := t.TempDir()
	old, latest, link := filepath.Join(dir, "old"), filepath.Join(dir, "latest"), filepath.Join(dir, "ttt")
	if runtime.GOOS == "windows" {
		old, latest, link = old+".exe", latest+".exe", link+".exe"
	}
	for _, path := range []string{old, latest} {
		if err := os.WriteFile(path, []byte("executable"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(old, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	target, err := captureRestartTarget(link)
	if err != nil || target.launch != link {
		t.Fatalf("launch symlink was resolved early: %+v, %v", target, err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(latest, link); err != nil {
		t.Fatal(err)
	}
	resolved, err := target.path(update.Binary)
	if err != nil || resolved != latest {
		t.Fatalf("updated symlink target = %q, %v", resolved, err)
	}
}

func TestRestartAfterReplacingRunningExecutable(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "ttt"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(bin, name)
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, "-test.run=^TestRestartReplacementHelper$")
	cmd.Args[0] = name
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH=bin", "TTT_REPLACE_STAGE=replace")
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 || !strings.Contains(string(output), "restarted replaced executable") {
		t.Fatalf("replacement restart: %v, %q", err, output)
	}
}

func TestRestartReplacementHelper(t *testing.T) {
	stage := os.Getenv("TTT_REPLACE_STAGE")
	if stage == "" {
		return
	}
	if stage == "restarted" {
		fmt.Println("restarted replaced executable")
		os.Exit(23)
	}
	target, err := captureRestartTarget(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target.launch)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(target.launch, target.launch+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target.launch, data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(target.launch + ".old"); err != nil && runtime.GOOS != "windows" {
		t.Fatal(err)
	}
	os.Setenv("TTT_REPLACE_STAGE", "restarted")
	code, err := target.restart(update.Binary)
	if err != nil {
		t.Fatal(err)
	}
	os.Exit(code)
}

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
