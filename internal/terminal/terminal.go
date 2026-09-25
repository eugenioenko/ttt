package terminal

import (
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/aymanbagabas/go-pty"
	"github.com/gitpod-io/xterm-go"
)

const (
	AttrReverse   int16 = 1
	AttrUnderline int16 = 2
	AttrBold      int16 = 4
	AttrItalic    int16 = 16
	AttrBlink     int16 = 32
)

const rawTailMax = 64 * 1024

type Terminal struct {
	mu         sync.Mutex
	term       *xterm.Terminal
	pt         pty.Pty
	cmd        *pty.Cmd
	cols, rows int
	done       chan struct{}
	waited     chan struct{}
	started    bool
	closed     bool
	exited     bool
	rawTail    []byte
	// promptMarker tracks the row of the last OSC 133 prompt start while
	// that prompt is still being edited; promptCol is the column it starts at.
	promptMarker *xterm.Marker
	promptCol    int
	OnUpdate     func()
	OnExit       func()

	updatePending atomic.Bool
}

func New(shell string, cols, rows, scrollbackMax int, env []string, dir string) (*Terminal, error) {
	if shell == "" {
		shell = defaultShell()
	}
	if scrollbackMax <= 0 {
		scrollbackMax = 1000
	}

	t := &Terminal{
		cols:   cols,
		rows:   rows,
		done:   make(chan struct{}),
		waited: make(chan struct{}),
	}

	pt, err := pty.New()
	if err != nil {
		return nil, err
	}

	t.term = xterm.New(
		xterm.WithCols(cols),
		xterm.WithRows(rows),
		xterm.WithScrollback(scrollbackMax),
	)
	t.watchPromptMarks()
	t.term.OnData(func(s string) {
		io.WriteString(pt, s)
	})

	cmd := pt.Command(shell)
	// Verify dir exists before setting it — chaos monkey and random commands can
	// delete the workspace dir, causing Start to fail with "no such file or directory"
	if dir != "" {
		if _, err := os.Stat(dir); err == nil {
			cmd.Dir = dir
		}
	}
	cmd.Env = append(os.Environ(), env...)
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")

	if err := pt.Resize(cols, rows); err != nil {
		pt.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		pt.Close()
		return nil, err
	}
	t.pt = pt
	t.cmd = cmd

	return t, nil
}

func defaultShell() string {
	if runtime.GOOS == "windows" {
		if comspec := os.Getenv("COMSPEC"); comspec != "" {
			return comspec
		}
		return "powershell.exe"
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

func (t *Terminal) Run() {
	t.mu.Lock()
	t.started = true
	t.mu.Unlock()
	go t.waitLoop()
	go t.readLoop()
}

// waitLoop reaps the child and is what makes shell exit observable.
//
// Reading the master until it errors does not work here: go-pty assigns the
// slave to the child's stdio and keeps its own copy of that fd open in this
// process, so the master never reports EOF when the child dies and readLoop
// blocks forever. Waiting on the process is the only reliable signal, and
// closing the pty afterwards is what releases readLoop.
func (t *Terminal) waitLoop() {
	defer close(t.waited)
	if t.cmd == nil {
		return
	}
	t.cmd.Wait()

	t.mu.Lock()
	t.exited = true
	closing := t.closed
	t.pt.Close()
	t.mu.Unlock()

	// Close() already tears the tab down; firing OnExit there would close it twice.
	if !closing && t.OnExit != nil {
		t.OnExit()
	}
}

func (t *Terminal) readLoop() {
	defer close(t.done)
	buf := make([]byte, 4096)
	for {
		n, err := t.pt.Read(buf)
		if n > 0 {
			t.mu.Lock()
			t.term.Write(buf[:n])
			t.appendRawTail(buf[:n])
			t.mu.Unlock()
			if t.OnUpdate != nil && t.updatePending.CompareAndSwap(false, true) {
				t.OnUpdate()
			}
		}
		if err != nil {
			t.mu.Lock()
			t.exited = true
			t.mu.Unlock()
			return
		}
	}
}

func (t *Terminal) WriteString(s string) {
	io.WriteString(t.pt, s)
}

func (t *Terminal) Resize(cols, rows int) {
	// xterm clamps below these itself; matching it here keeps t.cols/t.rows in
	// step with the emulator, which trimForReflow relies on to size its estimate.
	cols = max(cols, xterm.MinimumCols)
	rows = max(rows, xterm.MinimumRows)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.resizeEmulator(cols, rows) {
		t.pt.Resize(cols, rows)
	}
}

// resizeEmulator reports whether the size changed. An unchanged size must not
// clear the prompt: the pty sends no SIGWINCH for it, so nothing redraws it.
func (t *Terminal) resizeEmulator(cols, rows int) bool {
	if cols == t.cols && rows == t.rows {
		return false
	}
	// ConPTY repaints the screen itself on resize; the blanking is only for
	// Unix ptys, where the shell's SIGWINCH redraw is what brings it back.
	if runtime.GOOS != "windows" {
		t.clearPromptForRedraw()
	}
	trimForReflow(t.term.NormalBuffer(), t.cols, t.rows, cols, rows)
	t.cols = cols
	t.rows = rows
	t.term.Resize(cols, rows)
	return true
}

// AckUpdate must run before a frame reads the emulator so later output re-arms OnUpdate.
func (t *Terminal) AckUpdate() {
	t.updatePending.Store(false)
}

func (t *Terminal) Snapshot(fn func(term *xterm.Terminal)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fn(t.term)
}

func (t *Terminal) CursorPos() (x, y int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.term.CursorX(), t.term.CursorY()
}

func (t *Terminal) Size() (cols, rows int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cols, t.rows
}

func (t *Terminal) Close() {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	started := t.started
	t.mu.Unlock()

	t.mu.Lock()
	t.pt.Close()
	t.mu.Unlock()
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}
	if started {
		<-t.waited
		<-t.done
	}
}

func (t *Terminal) ScrollbackLen() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.term.Buffer().YBase
}

func (t *Terminal) DecPrivateModes() xterm.DecPrivateModes {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.term.DecPrivateModes()
}

// appendRawTail must be called with t.mu held.
func (t *Terminal) appendRawTail(b []byte) {
	if len(b) >= rawTailMax {
		t.rawTail = append(t.rawTail[:0], b[len(b)-rawTailMax:]...)
		return
	}
	if excess := len(t.rawTail) + len(b) - rawTailMax; excess > 0 {
		copy(t.rawTail, t.rawTail[excess:])
		t.rawTail = t.rawTail[:len(t.rawTail)-excess]
	}
	t.rawTail = append(t.rawTail, b...)
}

// RawTail returns the most recent bytes read from the PTY, unparsed by xterm.
func (t *Terminal) RawTail() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]byte, len(t.rawTail))
	copy(out, t.rawTail)
	return out
}
