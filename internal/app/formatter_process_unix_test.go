//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package app

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestFormatterProcessCancellationStopsChildren(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("test formatter requires sh")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", "sleep 3 & wait")
	configureFormatterProcess(cmd)
	var output bytes.Buffer
	cmd.Stdout = &output
	start := time.Now()
	if err := cmd.Run(); err == nil {
		t.Fatal("formatter was not canceled")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("formatter child held stdout open for %s after cancellation", elapsed)
	}
	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("formatter failed before the deadline: %v", ctx.Err())
	}
}
