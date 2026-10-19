// Package icons resolves named UI glyphs to a user override, their Nerd Font
// form or their plain form.
package icons

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/textwidth"
)

type Name string

const (
	Branch Name = "branch"
	Commit Name = "commit"
	Lock   Name = "lock"
	Pinned Name = "pinned"

	ChevronRight Name = "chevron-right"
	ChevronDown  Name = "chevron-down"
	ChevronUp    Name = "chevron-up"
	Submenu      Name = "submenu"
	ArrowLeft    Name = "arrow-left"
	ArrowRight   Name = "arrow-right"
	Close        Name = "close"
	Dirty        Name = "dirty"
	More         Name = "more"
	Add          Name = "add"
	Remove       Name = "remove"
	Discard      Name = "discard"
	Replace      Name = "replace"

	Function  Name = "function"
	Class     Name = "class"
	Interface Name = "interface"
	Module    Name = "module"
	Field     Name = "field"
	Constant  Name = "constant"
	Variable  Name = "variable"
	String    Name = "string"
	Keyword   Name = "keyword"
	Snippet   Name = "snippet"
	Symbol    Name = "symbol"
)

type glyphs struct {
	Plain string
	Nerd  string
}

var table = map[Name]glyphs{
	Branch: {Plain: "⎇", Nerd: "\uf418"},
	Commit: {Plain: "●", Nerd: ""},
	Lock:   {Plain: "⚿", Nerd: ""},
	Pinned: {Plain: "♦", Nerd: ""},

	ChevronRight: {Plain: "▸", Nerd: ""},
	ChevronDown:  {Plain: "▾", Nerd: ""},
	ChevronUp:    {Plain: "▴", Nerd: ""},
	Submenu:      {Plain: "›", Nerd: ""},
	ArrowLeft:    {Plain: "◂", Nerd: ""},
	ArrowRight:   {Plain: "▸", Nerd: ""},
	Close:        {Plain: "✕", Nerd: ""},
	Dirty:        {Plain: "●", Nerd: ""},
	More:         {Plain: "⋮", Nerd: "⋮"},
	Add:          {Plain: "+", Nerd: "+"},
	Remove:       {Plain: "−", Nerd: "−"},
	Discard:      {Plain: "✕", Nerd: "✕"},
	Replace:      {Plain: "⟳", Nerd: ""},

	Function:  {Plain: "ƒ", Nerd: ""},
	Class:     {Plain: "◆", Nerd: ""},
	Interface: {Plain: "◇", Nerd: ""},
	Module:    {Plain: "▤", Nerd: ""},
	Field:     {Plain: "▪", Nerd: ""},
	Constant:  {Plain: "●", Nerd: ""},
	Variable:  {Plain: "●", Nerd: ""},
	String:    {Plain: "§", Nerd: ""},
	Keyword:   {Plain: "■", Nerd: ""},
	Snippet:   {Plain: "■", Nerd: ""},
	Symbol:    {Plain: "•", Nerd: ""},
}

var (
	activeMode string
	overrides  = map[Name]string{}
)

// Configure sets the icon mode and user overrides used by Rune, and the
// overrides used by Get. An override must be one single-width rune for a known
// icon; each one that is not is ignored and reported in the returned warnings.
func Configure(mode string, userOverrides map[string]string) (warnings []string) {
	activeMode = mode
	overrides = map[Name]string{}
	for key, value := range userOverrides {
		name := Name(key)
		if _, ok := table[name]; !ok {
			warnings = append(warnings, fmt.Sprintf("unknown icon %q in appearance.overrides", key))
			continue
		}
		if r := []rune(value); len(r) != 1 || textwidth.Rune(r[0]) != 1 {
			warnings = append(warnings, fmt.Sprintf("icon override %q must be one single-width character, got %q", key, value))
			continue
		}
		overrides[name] = value
	}
	return warnings
}

func Get(mode string, name Name) string {
	if o, ok := overrides[name]; ok {
		return o
	}
	g := table[name]
	if mode == config.IconsNerdFont {
		return g.Nerd
	}
	return g.Plain
}

// Rune returns name's glyph for the configured icon mode.
func Rune(name Name) rune {
	return []rune(Get(activeMode, name))[0]
}

func IsNerd(mode string) bool { return mode == config.IconsNerdFont }
