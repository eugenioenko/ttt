package delve

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestTerminalScriptPreservesConfiguredEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX shell")
	}
	dir := t.TempDir()
	child := exec.Command("sh", "-c", `printf '%s' "$DAP_TEST_VALUE/$1"`, "sh", "quoted'argument")
	child.Dir = dir
	child.Env = append(child.Environ(), "DAP_TEST_VALUE=a'b $HOME")
	path, err := terminalScript(child, dir, map[string]string{"DAP_TEST_VALUE": ""})
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("sh", path).CombinedOutput()
	if err != nil || string(out) != "a'b $HOME/quoted'argument" {
		t.Fatalf("got %q, %v", out, err)
	}
}
