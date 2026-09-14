package appearance

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/term"
)

// Queries sent before tcell.Init. OSC 11 is answered by Kitty, Ghostty,
// WezTerm, iTerm2, Konsole and most others; CSI ? 996 n is answered by newer
// Kitty/Ghostty with CSI ? 997 ; 1/2 n. Sending both covers old and new.
const (
	osc11Query = "\x1b]11;?\x1b\\"
	dslQuery   = "\x1b[?996n"
)

// QueryTerminal asks the host terminal for its background directly on
// /dev/tty. It must run before tcell.Init: once tcell's input loop owns the
// tty, OSC and color-scheme replies are swallowed before the app can see
// them. Bounded by timeout; never blocks startup past it.
func QueryTerminal(timeout time.Duration) Appearance {
	if underMux() {
		return Unknown
	}
	return queryTerminalOn("/dev/tty", timeout)
}

func queryTerminalOn(path string, timeout time.Duration) Appearance {
	return queryTTY(path, appearanceQuery(), timeout)
}

// appearanceQuery builds the query bytes. Inside a Herdr pane the OSC 11 leg
// is skipped: Herdr stubs it with constant white (herdr#714), which would
// poison luminance inference to always-light. The 996 leg stays: Herdr
// answers 997 with the effective pane appearance, and the reader prefers it.
func appearanceQuery() string {
	if os.Getenv("HERDR_PANE_ID") != "" {
		return dslQuery
	}
	return osc11Query + dslQuery
}

func queryTTY(path, query string, timeout time.Duration) Appearance {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return Unknown
	}
	defer f.Close()
	fd := int(f.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return Unknown
	}
	defer term.Restore(fd, oldState)
	if _, err := f.WriteString(query); err != nil {
		return Unknown
	}
	found := make(chan Appearance, 1)
	go func() {
		var buf []byte
		tmp := make([]byte, 256)
		for {
			n, err := f.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
				s := string(buf)
				// An authoritative 997 report wins whenever it shares the
				// accumulated buffer, even with an OSC 11 background: both
				// replies are emitted back-to-back, so same-burst arrival
				// is the realistic case. (Read deadlines are unreliable on
				// tty fds, so no grace window is attempted: settling on a
				// lone OSC 11 answer is correct for pre-997 terminals.)
				if a := ParseColorSchemeReport(s); a != Unknown {
					found <- a
					return
				}
				if a := ParseOSC11Response(s); a != Unknown {
					found <- a
					return
				}
				if len(buf) > 4096 {
					break
				}
			}
			if err != nil {
				break
			}
		}
		found <- Unknown
	}()
	select {
	case a := <-found:
		return a
	case <-time.After(timeout):
		return Unknown
	}
}

// darwinAppearance reads the system appearance. Ghostty's `theme = auto` and
// similar terminal settings track this, so it is the right live fallback when
// the tty cannot be queried while tcell runs. An absent key means light mode;
// any other failure is honestly Unknown, as is a surprising value.
func darwinAppearance() Appearance {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "defaults", "read", "-g", "AppleInterfaceStyle").CombinedOutput()
	if err != nil {
		if strings.Contains(strings.ToLower(string(out)), "does not exist") {
			return Light
		}
		return Unknown
	}
	if strings.Contains(strings.ToLower(string(out)), "dark") {
		return Dark
	}
	return Unknown
}
