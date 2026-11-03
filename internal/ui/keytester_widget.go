package ui

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type KeyTesterWidget struct {
	BaseWidget
	Borders       *term.BorderSet
	OnDismiss     func()
	LookupBinding func(combo string) string
	lines         []string
	chordFirst    string
}

func NewKeyTesterWidget() *KeyTesterWidget {
	return &KeyTesterWidget{
		lines: []string{"Press any key combination..."},
	}
}

func (k *KeyTesterWidget) Focusable() bool { return true }

func (k *KeyTesterWidget) Render(surface Surface) {
	sw, sh := surface.Size()

	boxW := 50
	if boxW > sw-4 {
		boxW = sw - 4
	}
	boxH := 10
	boxX := (sw - boxW) / 2
	boxY := (sh - boxH) / 2
	if boxY < 1 {
		boxY = 1
	}

	b := term.DoubleBorderSet()
	if k.Borders != nil {
		b = *k.Borders
	}
	bs := term.StyleBorder

	surface.ClearRect(boxX, boxY, boxW, boxH, term.StylePaletteItem)
	surface.DrawBorder(boxX, boxY, boxW, boxH, b, bs)

	title := "Keyboard Tester"
	surface.DrawText(boxX+2, boxY, title, boxX+boxW-2, bs)

	pad := 1
	contentW := boxW - 2 - pad*2
	for i, line := range k.lines {
		y := boxY + 1 + pad + i
		if y >= boxY+boxH-2 {
			break
		}
		style := term.StyleDefault
		if i == 0 {
			style = term.StylePaletteSelected
		}
		surface.DrawText(boxX+1+pad, y, line, boxX+1+pad+contentW, style)
	}

	closeLabel := " Close (Esc) "
	closeStyle := term.StyleMuted
	closeX := boxX + boxW - 2 - pad - len([]rune(closeLabel))
	surface.DrawText(closeX, boxY+boxH-2, closeLabel, 0, closeStyle)
}

func (k *KeyTesterWidget) HandleEvent(ev tcell.Event) EventResult {
	if _, ok := ev.(*tcell.EventMouse); ok {
		return EventConsumed
	}

	kev, ok := ev.(*tcell.EventKey)
	if !ok {
		return EventConsumed
	}

	if kev.Key() == tcell.KeyEscape {
		if k.OnDismiss != nil {
			k.OnDismiss()
		}
		return EventConsumed
	}

	combo := k.describeKey(kev)

	if k.chordFirst != "" {
		full := k.chordFirst + " " + combo
		k.lines = k.makeLines(full, kev)
		k.chordFirst = ""
	} else {
		k.lines = k.makeLines(combo, kev)
		k.chordFirst = combo
	}

	return EventConsumed
}

func (k *KeyTesterWidget) describeKey(kev *tcell.EventKey) string {
	var parts []string

	mod := kev.Modifiers()
	if mod&tcell.ModCtrl != 0 {
		parts = append(parts, "ctrl")
	}
	if mod&tcell.ModShift != 0 {
		parts = append(parts, "shift")
	}
	if mod&tcell.ModAlt != 0 {
		parts = append(parts, "alt")
	}
	if mod&tcell.ModMeta != 0 {
		parts = append(parts, "super")
	}

	key := kev.Key()
	if name := specialKeyName(key); name != "" {
		parts = append(parts, name)
	} else if key == tcell.KeyRune {
		r := term.KeyRune(kev)
		if r == ' ' {
			parts = append(parts, "space")
		} else {
			parts = append(parts, string(r))
		}
	} else if key >= tcell.KeyCtrlA && key <= tcell.KeyCtrlZ {
		ch := 'a' + rune(key-tcell.KeyCtrlA)
		parts = append(parts, string(ch))
	} else {
		parts = append(parts, fmt.Sprintf("0x%x", int(key)))
	}

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "+"
		}
		result += p
	}
	return result
}

func (k *KeyTesterWidget) makeLines(combo string, kev *tcell.EventKey) []string {
	lines := []string{combo}
	lines = append(lines, "")

	bound := ""
	if k.LookupBinding != nil {
		bound = k.LookupBinding(combo)
	}
	if bound != "" {
		lines = append(lines, fmt.Sprintf("Bound to: %s", bound))
	} else {
		lines = append(lines, "Bound to: (none)")
	}

	lines = append(lines, fmt.Sprintf("Key: %d  Rune: %q  Mod: %d",
		kev.Key(), string(term.KeyRune(kev)), kev.Modifiers()))
	lines = append(lines, "")
	lines = append(lines, "Press next key for chord, or new combo")
	return lines
}
