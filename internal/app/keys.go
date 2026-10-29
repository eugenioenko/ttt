package app

import (
	"github.com/eugenioenko/ttt/internal/config"

	"github.com/gdamore/tcell/v3"
)

var canonicalKeyToTcell = map[string]tcell.Key{
	"Escape":    tcell.KeyEscape,
	"Enter":     tcell.KeyEnter,
	"Tab":       tcell.KeyTab,
	"Backspace": tcell.KeyBackspace,
	"Delete":    tcell.KeyDelete,
	"Insert":    tcell.KeyInsert,
	"Up":        tcell.KeyUp,
	"Down":      tcell.KeyDown,
	"Left":      tcell.KeyLeft,
	"Right":     tcell.KeyRight,
	"Home":      tcell.KeyHome,
	"End":       tcell.KeyEnd,
	"PgUp":      tcell.KeyPgUp,
	"PgDn":      tcell.KeyPgDn,
	"Space":     tcell.KeyRune,
	"F1":        tcell.KeyF1,
	"F2":        tcell.KeyF2,
	"F3":        tcell.KeyF3,
	"F4":        tcell.KeyF4,
	"F5":        tcell.KeyF5,
	"F6":        tcell.KeyF6,
	"F7":        tcell.KeyF7,
	"F8":        tcell.KeyF8,
	"F9":        tcell.KeyF9,
	"F10":       tcell.KeyF10,
	"F11":       tcell.KeyF11,
	"F12":       tcell.KeyF12,
}

// keyEventStr converts a comboToTcell rune to the string form that tcell v3's
// NewEventKey expects: empty for non-rune keys (ch == 0).
func keyEventStr(ch rune) string {
	if ch == 0 {
		return ""
	}
	return string(ch)
}

func comboToTcell(combo config.KeyCombo) (tcell.Key, tcell.ModMask, rune) {
	var mod tcell.ModMask
	if combo.Ctrl {
		mod |= tcell.ModCtrl
	}
	if combo.Alt {
		mod |= tcell.ModAlt
	}
	if combo.Shift {
		mod |= tcell.ModShift
	}
	if combo.Super {
		mod |= tcell.ModMeta
	}

	// Ctrl+non-letter printables (space, backtick, slash) are registered in
	// their canonical control-key form (KeyNUL for ctrl+space/ctrl+backtick,
	// KeyUS for ctrl+/) with ModCtrl kept. Incoming events are folded to the
	// same form by foldCtrlEvent in internal/ui/root.go regardless of whether
	// the terminal used the legacy byte encoding or the kitty keyboard
	// protocol. tcell v3 removed the KeyCtrlSpace/KeyCtrlUnderscore aliases;
	// KeyNUL (0x00) and KeyUS (0x1F) are the same values.
	if combo.KeyName != "" {
		if combo.KeyName == "Space" {
			if combo.Ctrl {
				return tcell.KeyNUL, mod, 0
			}
			return tcell.KeyRune, mod, ' '
		}
		if combo.KeyName == "Backtick" {
			if combo.Ctrl {
				return tcell.KeyNUL, mod, 0
			}
			return tcell.KeyRune, mod, '`'
		}
		key := canonicalKeyToTcell[combo.KeyName]
		return key, mod, 0
	}

	if combo.Ctrl && !combo.Super && combo.Rune >= 'a' && combo.Rune <= 'z' {
		key := tcell.KeyCtrlA + tcell.Key(combo.Rune-'a')
		return key, mod, 0
	}

	if combo.Ctrl && combo.Rune == '/' {
		return tcell.KeyUS, mod, 0
	}

	return tcell.KeyRune, mod, combo.Rune
}
