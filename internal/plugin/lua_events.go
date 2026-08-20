package plugin

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

var fileEvents = map[string]bool{
	"file.open":  true,
	"file.close": true,
	"file.save":  true,
}

var editorEvents = map[string]bool{
	"editor.change": true,
	"cursor.change": true,
	"tab.change":    true,
}

var keyEvents = map[string]bool{
	"key.press": true,
}

var bookmarkEvents = map[string]bool{
	"bookmark.change": true,
}

func setupEventsModule(L *lua.LState, p *Plugin) {
	loader := func(L *lua.LState) int {
		mod := L.NewTable()

		L.SetField(mod, "on", L.NewFunction(eventsOn(p)))

		L.Push(mod)
		return 1
	}

	L.PreloadModule("ttt.events", loader)
}

func eventsOn(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		eventName := L.CheckString(1)
		callback := L.CheckFunction(2)

		if fileEvents[eventName] {
			if err := p.Granted.Check("events.file"); err != nil {
				L.ArgError(1, err.Error())
				return 0
			}
		} else if editorEvents[eventName] {
			if err := p.Granted.Check("events.editor"); err != nil {
				L.ArgError(1, err.Error())
				return 0
			}
		} else if keyEvents[eventName] {
			if err := p.Granted.Check("keybindings"); err != nil {
				L.ArgError(1, err.Error())
				return 0
			}
		} else if bookmarkEvents[eventName] {
			if err := p.Granted.Check("editor.bookmarks"); err != nil {
				L.ArgError(1, err.Error())
				return 0
			}
		} else {
			L.ArgError(1, fmt.Sprintf("unknown event: %s", eventName))
			return 0
		}

		p.EventListeners[eventName] = append(p.EventListeners[eventName], callback)
		return 0
	}
}
