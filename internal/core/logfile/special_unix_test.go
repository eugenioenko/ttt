//go:build unix

package logfile

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestOpenRejectsFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if f, err := Open(path, nil); err == nil {
		f.Close()
		t.Fatal("opened a FIFO as a log")
	}
}
