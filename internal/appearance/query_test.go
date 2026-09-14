package appearance

import (
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	pty "github.com/aymanbagabas/go-pty"
)

// fakeTerminal runs a responder on the pty master: it waits for the query
// bytes, then writes reply. Seen returns the raw query bytes received.
type fakeTerminal struct {
	path string
	mu   sync.Mutex
	seen []byte
}

func newFakeTerminal(t *testing.T, reply string) *fakeTerminal {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("pty-based test is unix-only")
	}
	pt, err := pty.New()
	if err != nil {
		t.Fatalf("pty.New: %v", err)
	}
	t.Cleanup(func() { pt.Close() })
	up, ok := pt.(pty.UnixPty)
	if !ok {
		t.Fatal("not a unix pty")
	}
	ft := &fakeTerminal{path: pt.Name()}
	go func() {
		tmp := make([]byte, 256)
		for {
			n, err := up.Master().Read(tmp)
			if n > 0 {
				ft.mu.Lock()
				ft.seen = append(ft.seen, tmp[:n]...)
				ready := strings.Contains(string(ft.seen), "?996n")
				ft.mu.Unlock()
				if ready {
					_, _ = up.Master().Write([]byte(reply))
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return ft
}

func (f *fakeTerminal) Seen() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return string(f.seen)
}

func TestQueryTerminalOSC11Dark(t *testing.T) {
	ft := newFakeTerminal(t, "\x1b]11;rgb:0000/0000/0000\x1b\\")
	if got := queryTTY(ft.path, osc11Query+dslQuery, 2*time.Second); got != Dark {
		t.Errorf("query = %v, want dark", got)
	}
}

func TestQueryTerminalSchemeReportWins(t *testing.T) {
	// Authoritative 997 light beats a dark OSC 11 background in the same
	// burst, matching herdr's report-first behavior.
	ft := newFakeTerminal(t, "\x1b]11;rgb:0000/0000/0000\x1b\\"+"\x1b[?997;2n")
	if got := queryTTY(ft.path, osc11Query+dslQuery, 2*time.Second); got != Light {
		t.Errorf("query = %v, want light", got)
	}
}

func TestQueryTerminalTimeout(t *testing.T) {
	ft := newFakeTerminal(t, "")
	if got := queryTTY(ft.path, osc11Query+dslQuery, 100*time.Millisecond); got != Unknown {
		t.Errorf("silent terminal = %v, want unknown", got)
	}
}

func TestQueryTerminalMissingTTY(t *testing.T) {
	if got := queryTTY("/dev/ttt-no-such-tty", osc11Query+dslQuery, 100*time.Millisecond); got != Unknown {
		t.Errorf("missing tty = %v, want unknown", got)
	}
}

func TestAppearanceQuerySkipsOSC11InHerdrPane(t *testing.T) {
	t.Setenv("HERDR_PANE_ID", "pane-1")
	if q := appearanceQuery(); strings.Contains(q, "]11;") || !strings.Contains(q, "?996n") {
		t.Errorf("herdr query = %q, want 996 only (OSC 11 is stubbed white there)", q)
	}
}

func TestAppearanceQueryFullOutsideHerdr(t *testing.T) {
	t.Setenv("HERDR_PANE_ID", "")
	if q := appearanceQuery(); !strings.Contains(q, "]11;") || !strings.Contains(q, "?996n") {
		t.Errorf("query = %q, want OSC 11 + 996", q)
	}
}

func TestQueryTerminalOnHerdrPaneHonors997(t *testing.T) {
	t.Setenv("HERDR_PANE_ID", "pane-1")
	ft := newFakeTerminal(t, "\x1b[?997;1n")
	if got := queryTerminalOn(ft.path, 2*time.Second); got != Dark {
		t.Errorf("query = %v, want dark", got)
	}
	if seen := ft.Seen(); strings.Contains(seen, "]11;") {
		t.Errorf("sent OSC 11 inside herdr pane: %q", seen)
	}
}

func TestDetectStartupReportsSource(t *testing.T) {
	t.Setenv("COLORFGBG", "15;default;0")
	t.Setenv("HERDR_PANE_ID", "")
	if got, src := DetectStartup(false); got != Dark || src != "env" {
		t.Errorf("DetectStartup = (%v, %q), want (dark, env)", got, src)
	}
}
