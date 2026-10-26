package plugin

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/textwidth"
	lua "github.com/yuin/gopher-lua"
)

func setupTextModule(L *lua.LState) {
	L.PreloadModule("ttt.text", func(L *lua.LState) int {
		mod := L.NewTable()
		L.SetField(mod, "width", L.NewFunction(textWidth))
		L.SetField(mod, "truncate", L.NewFunction(textTruncate))
		L.SetField(mod, "wrap", L.NewFunction(textWrap))
		L.Push(mod)
		return 1
	})
}

func textWidth(L *lua.LState) int {
	L.Push(lua.LNumber(textwidth.String(L.CheckString(1))))
	return 1
}

func textTruncate(L *lua.LState) int {
	s := L.CheckString(1)
	width := L.CheckInt(2)
	if width < 0 {
		L.ArgError(2, "width must not be negative")
	}
	L.Push(lua.LString(truncateToWidth(s, width, L.OptString(3, ""))))
	return 1
}

func textWrap(L *lua.LState) int {
	s := L.CheckString(1)
	width := L.CheckInt(2)
	if width < 1 {
		L.ArgError(2, "width must be at least 1")
	}
	tbl := L.NewTable()
	for _, line := range wrapToWidth(s, width) {
		tbl.Append(lua.LString(line))
	}
	L.Push(tbl)
	return 1
}

func truncateToWidth(s string, width int, tail string) string {
	if textwidth.String(s) <= width {
		return s
	}
	tailW := textwidth.String(tail)
	if tailW > width {
		tail, tailW = "", 0
	}
	runes := []rune(s)
	end := fitRunes(runes, width-tailW)
	return string(runes[:end]) + tail
}

// fitRunes returns how many leading runes fit in width columns.
func fitRunes(runes []rune, width int) int {
	w := 0
	for i, r := range runes {
		w += textwidth.Rune(r)
		if w > width {
			return i
		}
	}
	return len(runes)
}

func wrapToWidth(s string, width int) []string {
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		runes := []rune(para)
		for {
			end := max(fitRunes(runes, width), 1)
			if end >= len(runes) {
				lines = append(lines, string(runes))
				break
			}
			brk := -1
			for i := end; i > 0; i-- {
				if runes[i] == ' ' {
					brk = i
					break
				}
			}
			if brk > 0 {
				lines = append(lines, string(runes[:brk]))
				runes = runes[brk+1:]
				if len(runes) == 0 {
					break
				}
			} else {
				lines = append(lines, string(runes[:end]))
				runes = runes[end:]
			}
		}
	}
	return lines
}
