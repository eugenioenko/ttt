package app

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/plugin"
)

type slowWriter struct {
	buf   bytes.Buffer
	delay time.Duration
}

func (w *slowWriter) Write(p []byte) (int, error) {
	time.Sleep(w.delay)
	return w.buf.Write(p)
}

func TestPluginSystemAPI_ExecSlowWriterGetsAllOutput(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	out := &slowWriter{delay: pluginExecWaitDelay + 500*time.Millisecond}
	var errOut bytes.Buffer
	_, _, code, err := NewPluginSystemAPI().Exec(context.Background(), "sh", []string{"-c", "echo one; echo two >&2"},
		plugin.ExecOptions{Stdout: out, Stderr: &errOut})
	if err != nil || code != 0 {
		t.Fatalf("exec: err=%v code=%d", err, code)
	}
	if out.buf.String() != "one\n" || errOut.String() != "two\n" {
		t.Errorf("stdout %q, stderr %q", out.buf.String(), errOut.String())
	}
}

func TestPluginSystemAPI_ExecGrandchildHoldingPipeDoesNotHang(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	start := time.Now()
	stdout, _, code, err := NewPluginSystemAPI().Exec(context.Background(), "sh", []string{"-c", "sleep 5 & echo hi"}, plugin.ExecOptions{})
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("exec waited %s for a grandchild holding stdout", elapsed)
	}
	if err != nil || code != 0 || stdout != "hi\n" {
		t.Errorf("stdout %q, code %d, err %v", stdout, code, err)
	}
}
