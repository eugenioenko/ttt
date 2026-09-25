package ui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

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
		{"left", tcell.KeyLeft, tcell.ModNone, "\x1b[D"},
		{"ctrl+left", tcell.KeyLeft, tcell.ModCtrl, "\x1b[1;5D"},
		{"shift+up", tcell.KeyUp, tcell.ModShift, "\x1b[1;2A"},
		{"ctrl+shift+right", tcell.KeyRight, tcell.ModCtrl | tcell.ModShift, "\x1b[1;6C"},
		{"ctrl+delete", tcell.KeyDelete, tcell.ModCtrl, "\x1b[3;5~"},
		{"backspace", tcell.KeyBackspace2, tcell.ModNone, "\x7f"},
		{"alt+backspace", tcell.KeyBackspace2, tcell.ModAlt, "\x1b\x7f"},
		{"ctrl+backspace", tcell.KeyBackspace2, tcell.ModCtrl, "\x7f"},
		{"shift+tab", tcell.KeyBacktab, tcell.ModShift, "\x1b[Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keyToVT(tcell.NewEventKey(tt.key, "", tt.mod))
			if got != tt.want {
				t.Errorf("keyToVT(%s) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
