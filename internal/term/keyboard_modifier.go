package term

import (
	"github.com/gdamore/tcell/v3"
)

// Normalize a local shortcut check without changing the original event.
func ControlKeyEvent(ev *tcell.EventKey, modifiers *tcell.ModMask) *tcell.EventKey {
	allowed := controlModifiers(modifiers)
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
				ch := KeyRune(ev)
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

func controlModifiers(modifiers *tcell.ModMask) tcell.ModMask {
	if modifiers == nil || *modifiers == 0 {
		return tcell.ModCtrl
	}
	return *modifiers
}

func ControlModifierHeld(mod tcell.ModMask, modifiers *tcell.ModMask) bool {
	held := mod & (tcell.ModCtrl | tcell.ModMeta)
	return (held == tcell.ModCtrl || held == tcell.ModMeta) && controlModifiers(modifiers)&held != 0
}

func ControlKeyMatches(ev *tcell.EventKey, key tcell.Key, modifiers *tcell.ModMask) bool {
	normalized := ControlKeyEvent(ev, modifiers)
	return normalized != nil && normalized.Key() == key
}
