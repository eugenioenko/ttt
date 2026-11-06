package ui

import (
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

// Only binding matching and recording use the translated event. Plugins and
// raw key consumers must receive the terminal's original modifiers.
func controlKeyEvent(ev *tcell.EventKey, allowed tcell.ModMask) *tcell.EventKey {
	if allowed == 0 {
		allowed = tcell.ModCtrl
	}
	mods := ev.Modifiers()
	switch mods & (tcell.ModCtrl | tcell.ModMeta) {
	case tcell.ModNone:
		return ev
	case tcell.ModCtrl:
		if allowed&tcell.ModCtrl != 0 {
			return ev
		}
	case tcell.ModMeta:
		if allowed&tcell.ModMeta != 0 {
			key, text := ev.Key(), ev.Str()
			if key == tcell.KeyRune {
				ch := term.KeyRune(ev)
				if ch >= 'A' && ch <= 'Z' {
					ch += 'a' - 'A'
				}
				if ch >= 'a' && ch <= 'z' {
					key, text = tcell.KeyCtrlA+tcell.Key(ch-'a'), ""
				}
			}
			return tcell.NewEventKey(key, text, mods&^tcell.ModMeta|tcell.ModCtrl)
		}
	}
	return nil
}
