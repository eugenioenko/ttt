package term

import (
	"bytes"
	"strconv"
)

const (
	vkOemSlash      = 191
	win32CtrlMask   = 0x0c
	win32AltMask    = 0x03
	win32CtrlSlashC = "31"
)

// fixWin32CtrlSlash fills in 0x1F for win32-input-mode ctrl+/ reports.
// Windows Terminal sends them as VK_OEM_2 with no character, which tcell drops.
// 0x1F is what the terminal sends for ctrl+/ without win32-input-mode.
// A report split across reads passes through unchanged.
func fixWin32CtrlSlash(in []byte) []byte {
	var out []byte
	last := 0
	for i := 0; i+1 < len(in); i++ {
		if in[i] != '\x1b' || in[i+1] != '[' {
			continue
		}
		end := i + 2
		for end < len(in) && (in[end] >= '0' && in[end] <= '9' || in[end] == ';') {
			end++
		}
		if end >= len(in) || in[end] != '_' {
			continue
		}
		params := bytes.Split(in[i+2:end], []byte(";"))
		if !isWin32CtrlSlash(params) {
			continue
		}
		params[2] = []byte(win32CtrlSlashC)
		out = append(out, in[last:i+2]...)
		out = append(out, bytes.Join(params, []byte(";"))...)
		last = end
		i = end
	}
	if out == nil {
		return in
	}
	return append(out, in[last:]...)
}

// isWin32CtrlSlash reports whether params (Vk;Sc;Uc;Kd;Cs;Rc) describe
// ctrl+/ without a character. Ctrl with Alt is AltGr and is left alone.
func isWin32CtrlSlash(params [][]byte) bool {
	if len(params) < 5 {
		return false
	}
	vk, _ := strconv.Atoi(string(params[0]))
	uc, _ := strconv.Atoi(string(params[2]))
	cs, _ := strconv.Atoi(string(params[4]))
	return vk == vkOemSlash && uc == 0 && cs&win32CtrlMask != 0 && cs&win32AltMask == 0
}
