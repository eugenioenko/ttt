package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type KeyCombo struct {
	KeyName string
	Rune    rune
	Ctrl    bool
	Alt     bool
	Shift   bool
}

type KeyBinding struct {
	Key     string     `json:"key"`
	Command string     `json:"command"`
	Steps   []KeyCombo `json:"-"`
}

func (kb *KeyBinding) IsChord() bool {
	return len(kb.Steps) > 1
}

// ForceKeyCommands are checked even when a raw key consumer (e.g. terminal) has focus.
var ForceKeyCommands = map[string]bool{
	"terminal.toggle":     true,
	"terminal.fullscreen": true,
	"editor.quit":         true,
	"command.palette":     true,
	"file.quickOpen":      true,
	"sidebar.toggle":      true,
}

func ParseKeyBindings(bindings []KeyBinding) error {
	var firstErr error
	for i := range bindings {
		steps, err := ParseKeyString(bindings[i].Key)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("keybinding %q: %w", bindings[i].Key, err)
			}
			continue
		}
		bindings[i].Steps = steps
	}
	return firstErr
}

func ParseKeyString(s string) ([]KeyCombo, error) {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty key string")
	}
	steps := make([]KeyCombo, 0, len(parts))
	for _, part := range parts {
		combo, err := parseSingleKey(strings.ToLower(strings.TrimSpace(part)))
		if err != nil {
			return nil, err
		}
		steps = append(steps, combo)
	}
	return steps, nil
}

func parseSingleKey(s string) (KeyCombo, error) {
	combo := KeyCombo{}
	tokens := strings.Split(s, "+")

	for i, token := range tokens {
		if i == len(tokens)-1 {
			if name, ok := specialKeyNames[token]; ok {
				combo.KeyName = name
			} else if runes := []rune(token); len(runes) == 1 {
				combo.Rune = runes[0]
			} else {
				return combo, fmt.Errorf("unknown key %q", token)
			}
		} else {
			switch token {
			case "ctrl":
				combo.Ctrl = true
			case "alt":
				combo.Alt = true
			case "shift":
				combo.Shift = true
			default:
				return combo, fmt.Errorf("unknown modifier %q", token)
			}
		}
	}
	return combo, nil
}

var specialKeyNames = map[string]string{
	"escape":    "Escape",
	"esc":       "Escape",
	"enter":     "Enter",
	"return":    "Enter",
	"tab":       "Tab",
	"backspace": "Backspace",
	"delete":    "Delete",
	"insert":    "Insert",
	"up":        "Up",
	"down":      "Down",
	"left":      "Left",
	"right":     "Right",
	"home":      "Home",
	"end":       "End",
	"pgup":      "PgUp",
	"pgdn":      "PgDn",
	"pageup":    "PgUp",
	"pagedown":  "PgDn",
	"space":     "Space",
	"`":         "Backtick",
	"backtick":  "Backtick",
	"f1":        "F1",
	"f2":        "F2",
	"f3":        "F3",
	"f4":        "F4",
	"f5":        "F5",
	"f6":        "F6",
	"f7":        "F7",
	"f8":        "F8",
	"f9":        "F9",
	"f10":       "F10",
	"f11":       "F11",
	"f12":       "F12",
}

func LoadKeybindings(data []byte) ([]KeyBinding, error) {
	// Try dict format: {"command": "key" | ["key1", "key2"] | "" (cleared)}
	var dict map[string]json.RawMessage
	if err := json.Unmarshal(data, &dict); err == nil {
		var bindings []KeyBinding
		for cmd, raw := range dict {
			var single string
			if json.Unmarshal(raw, &single) == nil {
				bindings = append(bindings, KeyBinding{Key: single, Command: cmd})
				continue
			}
			var multi []string
			if json.Unmarshal(raw, &multi) == nil {
				if len(multi) == 0 {
					bindings = append(bindings, KeyBinding{Key: "", Command: cmd})
				}
				for _, k := range multi {
					bindings = append(bindings, KeyBinding{Key: k, Command: cmd})
				}
			}
		}
		return bindings, nil
	}
	// Fall back to legacy array format: [{"key": "...", "command": "..."}]
	var arr []KeyBinding
	if err := json.Unmarshal(data, &arr); err != nil {
		return nil, err
	}
	return arr, nil
}

func MergeKeybindings(defaults, overrides []KeyBinding) []KeyBinding {
	overridden := make(map[string]bool)
	for _, kb := range overrides {
		overridden[kb.Command] = true
	}
	merged := make([]KeyBinding, 0, len(defaults))
	for _, kb := range overrides {
		if kb.Key != "" {
			merged = append(merged, kb)
		}
	}
	for _, kb := range defaults {
		if !overridden[kb.Command] {
			merged = append(merged, kb)
		}
	}
	return merged
}

func SaveKeybindings(bindings []KeyBinding) error {
	defaults := DefaultKeybindings()
	defaultMap := make(map[string][]string)
	for _, kb := range defaults {
		defaultMap[kb.Command] = append(defaultMap[kb.Command], kb.Key)
	}

	currentMap := make(map[string][]string)
	for _, kb := range bindings {
		currentMap[kb.Command] = append(currentMap[kb.Command], kb.Key)
	}

	out := make(map[string]any)
	var order []string

	// Find overrides and clears
	for cmd, keys := range currentMap {
		defKeys := defaultMap[cmd]
		if !stringSliceEqual(keys, defKeys) {
			order = append(order, cmd)
			if len(keys) == 1 {
				out[cmd] = keys[0]
			} else {
				out[cmd] = keys
			}
		}
	}
	// Find cleared defaults (in defaults but not in current)
	for cmd := range defaultMap {
		if _, exists := currentMap[cmd]; !exists {
			order = append(order, cmd)
			out[cmd] = ""
		}
	}

	if len(out) == 0 {
		path := ConfigFilePath("keybindings.json")
		os.Remove(path)
		return nil
	}

	sort.Strings(order)
	data, err := marshalOrderedMap(out, order)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := ConfigFilePath("keybindings.json")
	return os.WriteFile(path, data, 0644)
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func marshalOrderedMap(m map[string]any, order []string) ([]byte, error) {
	var buf strings.Builder
	buf.WriteString("{\n")
	for i, key := range order {
		val, ok := m[key]
		if !ok {
			continue
		}
		keyJSON, _ := json.Marshal(key)
		valJSON, err := json.Marshal(val)
		if err != nil {
			return nil, err
		}
		buf.WriteString("  ")
		buf.Write(keyJSON)
		buf.WriteString(": ")
		buf.Write(valJSON)
		if i < len(order)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("}")
	return []byte(buf.String()), nil
}

func DefaultKeybindings() []KeyBinding {
	return []KeyBinding{
		{Key: "ctrl+b", Command: "sidebar.toggle"},
		{Key: "ctrl+f", Command: "search.find"},
		{Key: "ctrl+r", Command: "search.replace"},
		{Key: "ctrl+g", Command: "editor.goToLine"},
		{Key: "ctrl+p", Command: "command.palette"},
		{Key: "ctrl+k p", Command: "file.quickOpen"},
		{Key: "ctrl+o", Command: "workspace.openFolder"},
		{Key: "ctrl+n", Command: "file.new"},
		{Key: "ctrl+s", Command: "file.save"},
		{Key: "ctrl+z", Command: "editor.undo"},
		{Key: "ctrl+y", Command: "editor.redo"},
		{Key: "ctrl+q", Command: "editor.quit"},
		{Key: "ctrl+0", Command: "sidebar.focus"},
		{Key: "ctrl+pgdn", Command: "tab.next"},
		{Key: "ctrl+pgup", Command: "tab.prev"},
		{Key: "ctrl+w", Command: "tab.close"},
		{Key: "ctrl+a", Command: "editor.selectAll"},
		{Key: "ctrl+c", Command: "editor.copy"},
		{Key: "ctrl+x", Command: "editor.cut"},
		{Key: "ctrl+v", Command: "editor.paste"},
		{Key: "f3", Command: "search.findNext"},
		{Key: "shift+f3", Command: "search.findPrev"},
		{Key: "ctrl+k e", Command: "sidebar.explorer"},
		{Key: "ctrl+k f", Command: "sidebar.search"},
		{Key: "ctrl+k r", Command: "sidebar.searchReplace"},
		{Key: "ctrl+k c", Command: "sidebar.changes"},
		{Key: "ctrl+k t", Command: "terminal.new"},
		{Key: "ctrl+k s", Command: "file.saveAs"},
		{Key: "ctrl+u", Command: "editor.autocomplete"},
		{Key: "ctrl+k i", Command: "editor.hover"},
		{Key: "f2", Command: "editor.rename"},
		{Key: "f12", Command: "editor.goToDefinition"},
		{Key: "shift+f12", Command: "editor.goToImplementation"},
		{Key: "ctrl+l i", Command: "editor.goToImplementation"},
		{Key: "ctrl+l f", Command: "editor.formatDocument"},
		{Key: "ctrl+l s", Command: "editor.formatSelection"},
		{Key: "ctrl+l o", Command: "editor.organizeImports"},
		{Key: "ctrl+l x", Command: "editor.fixAll"},
		{Key: "ctrl+l r", Command: "editor.findReferences"},
		{Key: "ctrl+l t", Command: "editor.goToTypeDefinition"},
		{Key: "ctrl+d", Command: "multicursor.selectNext"},
		{Key: "ctrl+k l", Command: "multicursor.selectAll"},
		{Key: "ctrl+k u", Command: "multicursor.undoCursor"},
		{Key: "alt+up", Command: "editor.moveLineUp"},
		{Key: "alt+down", Command: "editor.moveLineDown"},
		{Key: "alt+shift+up", Command: "panel.taller"},
		{Key: "alt+shift+down", Command: "panel.shorter"},
		{Key: "alt+shift+left", Command: "sidebar.narrower"},
		{Key: "alt+shift+right", Command: "sidebar.wider"},
		{Key: "ctrl+k k", Command: "editor.deleteLine"},
		{Key: "ctrl+/", Command: "editor.toggleComment"},
		{Key: "ctrl+enter", Command: "editor.insertLineBelow"},
		{Key: "ctrl+left", Command: "editor.moveWordLeft"},
		{Key: "ctrl+right", Command: "editor.moveWordRight"},
		{Key: "ctrl+shift+left", Command: "editor.selectWordLeft"},
		{Key: "ctrl+shift+right", Command: "editor.selectWordRight"},
		{Key: "alt+backspace", Command: "editor.deleteWordLeft"},
		{Key: "alt+delete", Command: "editor.deleteWordRight"},
		{Key: "ctrl+delete", Command: "editor.deleteWordRight"},
		{Key: "ctrl+k o", Command: "editor.sortLinesAsc"},
		{Key: "ctrl+k m", Command: "editor.goToMatchingBracket"},
		{Key: "ctrl+k [", Command: "fold.toggle"},
		{Key: "ctrl+k 0", Command: "fold.collapseAll"},
		{Key: "ctrl+k 9", Command: "fold.expandAll"},
		{Key: "ctrl+k b", Command: "panel.toggle"},
		{Key: "ctrl+k j", Command: "editor.joinLines"},
		{Key: "ctrl+k y", Command: "view.keybindings"},
		{Key: "ctrl+k g", Command: "pr.comments"},
		{Key: "ctrl+t", Command: "terminal.toggle"},
		{Key: "alt+t", Command: "terminal.fullscreen"},
		{Key: "f10", Command: "menu.file"},
		{Key: "alt+f", Command: "menu.file"},
		{Key: "alt+e", Command: "menu.edit"},
		{Key: "alt+s", Command: "menu.selection"},
		{Key: "alt+v", Command: "menu.view"},
		{Key: "alt+o", Command: "menu.options"},
		{Key: "alt+h", Command: "menu.help"},
	}
}
