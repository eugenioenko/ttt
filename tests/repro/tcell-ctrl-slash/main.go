package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
)

type tracingTty struct {
	tcell.Tty
	trace *log.Logger
}

func (t tracingTty) Read(p []byte) (int, error) {
	n, err := t.Tty.Read(p)
	if n > 0 {
		t.trace.Printf("read %q", p[:n])
	}
	return n, err
}

func run(replay bool) error {
	var tty tcell.Tty
	var mock vt.MockTerm
	var opts []tcell.TerminfoScreenOption
	if replay {
		mock = vt.NewMockTerm()
		tty = mock
		opts = []tcell.TerminfoScreenOption{tcell.OptKeyboardProtocol(tcell.Win32Keyboard), tcell.OptNegotiation(false)}
	} else {
		var err error
		if tty, err = tcell.NewDevTty(); err != nil {
			return err
		}
	}

	trace := log.New(io.Discard, "", 0)
	screen, err := tcell.NewTerminfoScreenFromTty(tracingTty{tty, trace}, opts...)
	if err == nil {
		err = screen.Init()
	}
	if err != nil {
		_ = tty.Close()
		return err
	}
	var transcript bytes.Buffer
	defer func() {
		screen.Fini()
		fmt.Print(transcript.String())
	}()
	trace.SetOutput(&transcript)
	trace.Printf("win32-input-mode=%t", screen.KeyboardProtocol() == tcell.Win32Keyboard)

	if replay {
		const slash = "\x1b[191;53;47;1;0;1_"
		const ctrlSlash = "\x1b[191;53;0;1;8;1_"
		const q = "\x1b[81;16;113;1;0;1_"
		mock.SendRaw([]byte(slash + ctrlSlash + q))
	} else {
		screen.PutStr(0, 0, "Press /, then left Ctrl+/, then q to print the trace.")
		screen.Show()
	}

	for event := range screen.EventQ() {
		switch ev := event.(type) {
		case *tcell.EventKey:
			trace.Printf("key %s str=%q", ev.Name(), ev.Str())
			if ev.Str() == "q" && ev.Modifiers() == tcell.ModNone {
				return nil
			}
		case *tcell.EventError:
			return ev
		}
	}
	return nil
}

func main() {
	replay := flag.Bool("replay", false, "feed the reported Win32 input between two plain key presses")
	flag.Parse()
	if err := run(*replay); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
