package terminal

import (
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/aymanbagabas/go-pty"
	"github.com/eugenioenko/vt10x"
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
	vt         vt10x.Terminal
	pt         pty.Pty
	cmd        *pty.Cmd
	cols, rows int
	done       chan struct{}
	started    bool
	closed     bool
	exited     bool
	rawTail    []byte
	OnUpdate   func()
	OnExit     func()
}

func New(shell string, cols, rows, scrollbackMax int, env []string, dir string) (*Terminal, error) {
	if shell == "" {
		shell = defaultShell()
	}
	if scrollbackMax <= 0 {
		scrollbackMax = 1000
	}

	t := &Terminal{
		cols: cols,
		rows: rows,
		done: make(chan struct{}),
	}

	t.vt = vt10x.New(vt10x.WithSize(cols, rows), vt10x.WithScrollback(scrollbackMax))

	pt, err := pty.New()
	if err != nil {
		return nil, err
	}

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

	pt.Resize(cols, rows)

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
	go t.readLoop()
}

func (t *Terminal) readLoop() {
	defer close(t.done)
	buf := make([]byte, 4096)
	for {
		n, err := t.pt.Read(buf)
		if n > 0 {
			t.mu.Lock()
			t.vt.Write(buf[:n])
			t.appendRawTail(buf[:n])
			t.mu.Unlock()
			if t.OnUpdate != nil {
				t.OnUpdate()
			}
		}
		if err != nil {
			t.mu.Lock()
			t.exited = true
			t.mu.Unlock()
			if t.OnExit != nil {
				t.OnExit()
			}
			return
		}
	}
}

func (t *Terminal) WriteString(s string) {
	io.WriteString(t.pt, s)
}

func (t *Terminal) Resize(cols, rows int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cols = cols
	t.rows = rows
	t.vt.Resize(cols, rows)
	t.pt.Resize(cols, rows)
}

func (t *Terminal) Snapshot(fn func(view vt10x.View)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fn(t.vt)
}

func (t *Terminal) CursorPos() (x, y int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	c := t.vt.Cursor()
	return c.X, c.Y
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

	t.pt.Close()
	if t.cmd.Process != nil {
		t.cmd.Process.Kill()
		t.cmd.Wait()
	}
	if started {
		<-t.done
	}
}

func (t *Terminal) ScrollbackLen() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.vt.ScrollbackLen()
}

func (t *Terminal) Mode() vt10x.ModeFlag {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.vt.Mode()
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

// RawTail returns the most recent bytes read from the PTY, unparsed by vt10x.
func (t *Terminal) RawTail() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]byte, len(t.rawTail))
	copy(out, t.rawTail)
	return out
}
