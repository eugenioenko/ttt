// Package icons resolves named UI glyphs to their Nerd Font or plain form.
package icons

import "github.com/eugenioenko/ttt/internal/config"

type Name string

const (
	Branch Name = "branch"
	Commit Name = "commit"

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
	Branch: {Plain: "⎇", Nerd: ""},
	Commit: {Plain: "●", Nerd: "\uf417"},

	Function:  {Plain: "ƒ", Nerd: "\uea8c"},
	Class:     {Plain: "◆", Nerd: "\ueb5b"},
	Interface: {Plain: "◇", Nerd: "\ueb61"},
	Module:    {Plain: "▤", Nerd: "\uea8b"},
	Field:     {Plain: "▪", Nerd: "\ueb5f"},
	Constant:  {Plain: "●", Nerd: "\ueb5d"},
	Variable:  {Plain: "●", Nerd: "\uea88"},
	String:    {Plain: "§", Nerd: "\ueb8d"},
	Keyword:   {Plain: "■", Nerd: "\ueb62"},
	Snippet:   {Plain: "■", Nerd: "\ueb66"},
	Symbol:    {Plain: "•", Nerd: "\ueb63"},
}

func Get(mode string, name Name) string {
	g := table[name]
	if mode == config.IconsNerdFont {
		return g.Nerd
	}
	return g.Plain
}

func IsNerd(mode string) bool { return mode == config.IconsNerdFont }
