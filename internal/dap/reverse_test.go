package dap

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReverseForegroundExitPreservesDiagnostic(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("requires sh")
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var output strings.Builder
	_, err = ReverseProcess(ctx, filepath.Join(dir, "s"), dir, func(string) (*exec.Cmd, bool, error) {
		return exec.Command(shell, "-c", "echo launch-diagnostic >&2"), false, nil
	}, func(e Event) { output.Write(e.Body) }, SessionOptions{})
	if err == nil || !strings.Contains(err.Error(), "exited before connecting") || !strings.Contains(output.String(), "launch-diagnostic") {
		t.Fatalf("error=%v output=%s", err, output.String())
	}
}
