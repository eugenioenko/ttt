package app

import (
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// runPiped copies cmd's output with its own readers instead of exec's: exec's
// WaitDelay closes the pipes on a timer, dropping output a slow writer (a
// plugin streaming on the main loop) has not taken yet. Here only a pipe left
// idle for pluginExecWaitDelay after the child ends is abandoned.
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

	var exited atomic.Bool
	var wg sync.WaitGroup
	readers := []*os.File{outR, errR}
	for i, w := range []io.Writer{stdout, stderr} {
		r := readers[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer r.Close()
			drainPipe(r, w, &exited)
		}()
	}
	waitErr := cmd.Wait()
	exited.Store(true)
	for _, r := range readers {
		if r.SetReadDeadline(time.Now().Add(pluginExecWaitDelay)) != nil {
			time.AfterFunc(pluginExecWaitDelay, func() { r.Close() })
		}
	}
	wg.Wait()
	return waitErr
}

func drainPipe(r *os.File, w io.Writer, exited *atomic.Bool) {
	buf := make([]byte, 32*1024)
	for {
		if exited.Load() {
			_ = r.SetReadDeadline(time.Now().Add(pluginExecWaitDelay))
		}
		n, err := r.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}
