//go:build !windows

package terminal

import (
	"errors"
	"os"

	"github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/unix"
)

type pollablePty struct {
	pty.UnixPty
	master *os.File
	closed bool
}

func (p *pollablePty) Read(b []byte) (int, error)  { return p.master.Read(b) }
func (p *pollablePty) Write(b []byte) (int, error) { return p.master.Write(b) }
func (p *pollablePty) Master() *os.File            { return p.master }
func (p *pollablePty) Fd() uintptr                 { return p.master.Fd() }

func (p *pollablePty) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	return errors.Join(p.master.Close(), p.UnixPty.Close())
}

func makePtyPollable(pt pty.Pty) (pty.Pty, error) {
	unixPty, ok := pt.(pty.UnixPty)
	if !ok {
		return nil, pty.ErrUnsupported
	}
	// Darwin and BSD create an unpollable os.File. Construct a new one after
	// setting O_NONBLOCK so Close interrupts Read on every supported Unix.
	dupFD := -1
	var fdErr error
	controlErr := unixPty.Control(func(fd uintptr) {
		dupFD, fdErr = unix.FcntlInt(fd, unix.F_DUPFD_CLOEXEC, 0)
		if fdErr == nil {
			fdErr = unix.SetNonblock(dupFD, true)
		}
	})
	if err := errors.Join(controlErr, fdErr); err != nil {
		if dupFD >= 0 {
			_ = unix.Close(dupFD)
		}
		return nil, err
	}
	return &pollablePty{UnixPty: unixPty, master: os.NewFile(uintptr(dupFD), unixPty.Master().Name())}, nil
}
