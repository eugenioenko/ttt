// Package fileicons maps file and folder names to Nerd Font glyphs. Icons carry
// a hue family rather than a color, so callers resolve them through the theme.
package fileicons

//go:generate go run ./internal/gen -out icons_gen.go

import "strings"

type Color uint8

const (
	ColorDefault Color = iota
	ColorRed
	ColorYellow
	ColorGreen
	ColorCyan
	ColorBlue
	ColorMagenta
)

type Icon struct {
	Glyph string
	Color Color
}

var (
	defaultFile = Icon{Glyph: "\uf4a5", Color: ColorDefault}
	folder      = Icon{Glyph: "\ue5ff", Color: ColorBlue}
	folderOpen  = Icon{Glyph: "\ue5fe", Color: ColorBlue}
)

// ForFile takes a base name. An exact name match wins over the longest dotted
// suffix, so "app.test.ts" resolves through "test.ts" before "ts".
func ForFile(name string) Icon {
	lower := strings.ToLower(name)
	if icon, ok := byFilename[lower]; ok {
		return icon
	}
	for rest := lower; ; {
		dot := strings.IndexByte(rest, '.')
		if dot < 0 {
			break
		}
		rest = rest[dot+1:]
		if icon, ok := byExtension[rest]; ok {
			return icon
		}
	}
	return defaultFile
}

func ForFolder(expanded bool) Icon {
	if expanded {
		return folderOpen
	}
	return folder
}
