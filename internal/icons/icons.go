// Package icons resolves named UI glyphs to their Nerd Font or plain form.
package icons

import "github.com/eugenioenko/ttt/internal/config"

type Name string

const (
	Branch Name = "branch"
	Commit Name = "commit"
)

type glyphs struct {
	Plain string
	Nerd  string
}

var table = map[Name]glyphs{
	Branch: {Plain: "⎇", Nerd: ""},
	Commit: {Plain: "●", Nerd: ""},
}

func Get(mode string, name Name) string {
	g := table[name]
	if mode == config.IconsNerdFont {
		return g.Nerd
	}
	return g.Plain
}
