package app

import (
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// runPiped copies cmd's output with its own readers instead of exec's: exec's
// WaitDelay closes the pipes on a timer, dropping output a slow writer (a
// plugin streaming on the main loop) has not taken yet. See pipeDrain for the
// bound that replaces it.
func runPiped(cmd *exec.Cmd, stdout, stderr io.Writer) error {
	outR, outW, err := os.Pipe()
	if err != nil {
		return err
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		outR.Close()
		outW.Close()
		return err
	}
	cmd.Stdout, cmd.Stderr = outW, errW
	startErr := cmd.Start()
	outW.Close()
	errW.Close()
	if startErr != nil {
		outR.Close()
		errR.Close()
		return startErr
	}

	drains := []*pipeDrain{{r: outR, w: stdout}, {r: errR, w: stderr}}
	var wg sync.WaitGroup
	for _, d := range drains {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer d.r.Close()
			d.run()
		}()
	}
	waitErr := cmd.Wait()
	for _, d := range drains {
		d.childExited()
	}
	wg.Wait()
	return waitErr
}

// pipeDrain reads a pipe for at most pluginExecWaitDelay after the child
// exits, not counting time spent blocked in w: a slow writer still gets what
// the child left in the pipe, but a grandchild that keeps writing cannot hold
// the result open.
type pipeDrain struct {
	r *os.File
	w io.Writer

	mu       sync.Mutex
	exitedAt time.Time
	deadline time.Time
}

func (d *pipeDrain) childExited() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.exitedAt = time.Now()
	d.deadline = d.exitedAt.Add(pluginExecWaitDelay)
	if d.r.SetReadDeadline(d.deadline) != nil {
		r := d.r
		time.AfterFunc(pluginExecWaitDelay, func() { r.Close() })
	}
}

func (d *pipeDrain) run() {
	buf := make([]byte, 32*1024)
	for {
		n, err := d.r.Read(buf)
		if n > 0 {
			start := time.Now()
			_, _ = d.w.Write(buf[:n])
			d.creditWrite(start)
		}
		if err != nil {
			return
		}
	}
}

func (d *pipeDrain) creditWrite(start time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.exitedAt.IsZero() {
		return
	}
	if d.exitedAt.After(start) {
		start = d.exitedAt
	}
	d.deadline = d.deadline.Add(time.Since(start))
	_ = d.r.SetReadDeadline(d.deadline)
}
