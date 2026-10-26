package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/gdamore/tcell/v3"
)

func TestKeyToVTCursorModes(t *testing.T) {
	keys := []struct {
		name        string
		key         tcell.Key
		normal      string
		application string
		suffix      string
	}{
		{"up", tcell.KeyUp, "\x1b[A", "\x1bOA", "A"},
		{"down", tcell.KeyDown, "\x1b[B", "\x1bOB", "B"},
		{"right", tcell.KeyRight, "\x1b[C", "\x1bOC", "C"},
		{"left", tcell.KeyLeft, "\x1b[D", "\x1bOD", "D"},
		{"home", tcell.KeyHome, "\x1b[H", "\x1bOH", "H"},
		{"end", tcell.KeyEnd, "\x1b[F", "\x1bOF", "F"},
	}
	mods := []struct {
		name  string
		mod   tcell.ModMask
		param int
	}{
		{"none", tcell.ModNone, 1},
		{"shift", tcell.ModShift, 2},
		{"alt", tcell.ModAlt, 3},
		{"alt+shift", tcell.ModAlt | tcell.ModShift, 4},
		{"ctrl", tcell.ModCtrl, 5},
		{"ctrl+shift", tcell.ModCtrl | tcell.ModShift, 6},
		{"ctrl+alt", tcell.ModCtrl | tcell.ModAlt, 7},
		{"ctrl+alt+shift", tcell.ModCtrl | tcell.ModAlt | tcell.ModShift, 8},
	}
	for _, app := range []bool{false, true} {
		for _, key := range keys {
			for _, mod := range mods {
				t.Run(fmt.Sprintf("application=%t/%s/%s", app, key.name, mod.name), func(t *testing.T) {
					want := key.normal
					if app {
						want = key.application
					}
					if mod.mod != tcell.ModNone {
						want = fmt.Sprintf("\x1b[1;%d%s", mod.param, key.suffix)
					}
					if got := keyToVT(tcell.NewEventKey(key.key, "", mod.mod), app); got != want {
						t.Errorf("keyToVT() = %q, want %q", got, want)
					}
				})
			}
		}
	}
}

func TestTerminalWidget_ApplicationCursorKeys(t *testing.T) {
	term, err := terminal.New("/bin/sh", 80, 24, 0, nil, "", "-c",
		"stty raw -echo; printf cursor-keys-ready; exec cat")
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	updated := newUpdateChan(term)
	term.Run()
	waitFor(t, updated, func() bool {
		return strings.Contains(string(term.RawTail()), "cursor-keys-ready")
	})

	tw := NewTerminalWidget(term, nil)
	for _, tt := range []struct {
		name string
		mode string
		app  bool
		want string
	}{
		{"normal", "\x1b[?1l", false, "\x1b[A\x1b[B\x1b[C\x1b[D\x1b[H\x1b[F\x1b[1;5H"},
		{"application", "\x1b[?1h", true, "\x1bOA\x1bOB\x1bOC\x1bOD\x1bOH\x1bOF\x1b[1;5H"},
		{"reset", "\x1b[?1l", false, "\x1b[A\x1b[B\x1b[C\x1b[D\x1b[H\x1b[F\x1b[1;5H"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := len(term.RawTail())
			term.WriteString(tt.mode)
			waitFor(t, updated, func() bool {
				return len(term.RawTail()) >= before+len(tt.mode) &&
					term.DecPrivateModes().ApplicationCursorKeys == tt.app
			})

			before = len(term.RawTail())
			for _, key := range []tcell.Key{
				tcell.KeyUp, tcell.KeyDown, tcell.KeyRight, tcell.KeyLeft, tcell.KeyHome, tcell.KeyEnd,
			} {
				if got := tw.HandleEvent(tcell.NewEventKey(key, "", tcell.ModNone)); got != EventConsumed {
					t.Fatalf("HandleEvent(%v) = %v, want EventConsumed", key, got)
				}
			}
			if got := tw.HandleEvent(tcell.NewEventKey(tcell.KeyHome, "", tcell.ModCtrl)); got != EventConsumed {
				t.Fatalf("HandleEvent(Ctrl+Home) = %v, want EventConsumed", got)
			}
			waitFor(t, updated, func() bool {
				return len(term.RawTail()) >= before+len(tt.want)
			})
			if got := string(term.RawTail()[before:]); got != tt.want {
				t.Errorf("PTY received %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKeyToVTModifiers(t *testing.T) {
	tests := []struct {
		name string
		key  tcell.Key
		mod  tcell.ModMask
		want string
	}{
		{"enter", tcell.KeyEnter, tcell.ModNone, "\r"},
		{"shift+enter", tcell.KeyEnter, tcell.ModShift, "\x1b\r"},
		{"alt+enter", tcell.KeyEnter, tcell.ModAlt, "\x1b\r"},
		{"ctrl+enter", tcell.KeyEnter, tcell.ModCtrl, "\r"},
		{"ctrl+left", tcell.KeyLeft, tcell.ModCtrl, "\x1b[1;5D"},
		{"shift+up", tcell.KeyUp, tcell.ModShift, "\x1b[1;2A"},
		{"ctrl+shift+right", tcell.KeyRight, tcell.ModCtrl | tcell.ModShift, "\x1b[1;6C"},
		{"ctrl+delete", tcell.KeyDelete, tcell.ModCtrl, "\x1b[3;5~"},
		{"backspace", tcell.KeyBackspace2, tcell.ModNone, "\x7f"},
		{"alt+backspace", tcell.KeyBackspace2, tcell.ModAlt, "\x1b\x7f"},
		{"ctrl+backspace", tcell.KeyBackspace2, tcell.ModCtrl, "\x7f"},
		{"shift+tab", tcell.KeyBacktab, tcell.ModShift, "\x1b[Z"},
		{"page up", tcell.KeyPgUp, tcell.ModNone, "\x1b[5~"},
		{"page down", tcell.KeyPgDn, tcell.ModNone, "\x1b[6~"},
		{"insert", tcell.KeyInsert, tcell.ModNone, "\x1b[2~"},
		{"delete", tcell.KeyDelete, tcell.ModNone, "\x1b[3~"},
		{"F1", tcell.KeyF1, tcell.ModNone, "\x1bOP"},
	}

	for _, app := range []bool{false, true} {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("application=%t/%s", app, tt.name), func(t *testing.T) {
				got := keyToVT(tcell.NewEventKey(tt.key, "", tt.mod), app)
				if got != tt.want {
					t.Errorf("keyToVT(%s) = %q, want %q", tt.name, got, tt.want)
				}
			})
		}
	}
}
