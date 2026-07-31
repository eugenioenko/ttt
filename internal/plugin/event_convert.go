package plugin

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
	lua "github.com/yuin/gopher-lua"
)

func eventToLua(L *lua.LState, ev tcell.Event) *lua.LTable {
	switch e := ev.(type) {
	case *tcell.EventKey:
		return keyEventToLua(L, e)
	case *tcell.EventMouse:
		return mouseEventToLua(L, e)
	}
	return nil
}

// tcell.KeyNames has no entry for the two control characters foldCtrlEvent
// canonicalises onto, so both would reach plugins as "unknown" and be
// indistinguishable. Names match the ttt keybinding spelling (ctrl+space,
// ctrl+/) so a plugin keymap round-trips.
func keyName(k tcell.Key) string {
	if name := tcell.KeyNames[k]; name != "" {
		return name
	}
	switch k {
	case tcell.KeyNUL:
		return "Ctrl-Space"
	case tcell.KeyUS:
		return "Ctrl-/"
	}
	return "unknown"
}

func keyEventToLua(L *lua.LState, e *tcell.EventKey) *lua.LTable {
	tbl := L.NewTable()
	L.SetField(tbl, "type", lua.LString("key"))

	if e.Key() == tcell.KeyRune {
		L.SetField(tbl, "key", lua.LString(string(term.KeyRune(e))))
		L.SetField(tbl, "rune", lua.LString(string(term.KeyRune(e))))
	} else {
		L.SetField(tbl, "key", lua.LString(keyName(e.Key())))
	}

	var mods []string
	if e.Modifiers()&tcell.ModCtrl != 0 {
		mods = append(mods, "ctrl")
	}
	if e.Modifiers()&tcell.ModAlt != 0 {
		mods = append(mods, "alt")
	}
	if e.Modifiers()&tcell.ModShift != 0 {
		mods = append(mods, "shift")
	}
	if len(mods) > 0 {
		L.SetField(tbl, "mod", lua.LString(strings.Join(mods, "+")))
	}

	return tbl
}

func mouseEventToLua(L *lua.LState, e *tcell.EventMouse) *lua.LTable {
	tbl := L.NewTable()
	L.SetField(tbl, "type", lua.LString("mouse"))

	x, y := e.Position()
	L.SetField(tbl, "x", lua.LNumber(x))
	L.SetField(tbl, "y", lua.LNumber(y))

	button := "none"
	switch {
	case e.Buttons()&tcell.Button1 != 0:
		button = "left"
	case e.Buttons()&tcell.Button2 != 0:
		button = "right"
	case e.Buttons()&tcell.Button3 != 0:
		button = "middle"
	case e.Buttons()&tcell.WheelUp != 0:
		button = "wheel_up"
	case e.Buttons()&tcell.WheelDown != 0:
		button = "wheel_down"
	}
	L.SetField(tbl, "button", lua.LString(button))

	return tbl
}
