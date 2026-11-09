// Package textwidth measures how many terminal columns text occupies.
//
// It is the single source of truth for display width in ttt. Nothing else may
// compute width by hand: a rune count is a buffer index, not a screen width,
// and two disagreeing measurements are what makes fullwidth characters render
// on top of each other.
package textwidth

import (
	"os"
	"strings"

	"github.com/clipperhouse/displaywidth"
)

// options mirrors tcell's own width configuration (its internal
// widthutil.Options). tcell measures every cell it draws with displaywidth
// using these options and advances the terminal cursor by that width, so ttt
// must measure with identical settings — where the two disagree, our layout and
// what the terminal actually shows drift apart.
var options = eastAsianOptions(os.Getenv("RUNEWIDTH_EASTASIAN"))

func eastAsianOptions(env string) displaywidth.Options {
	switch strings.ToLower(env) {
	case "1", "true", "yes":
		return displaywidth.Options{EastAsianWidth: true}
	}
	return displaywidth.Options{}
}

// Rune returns the number of terminal columns r occupies: 2 for East Asian wide
// and fullwidth runes, 1 for everything else.
//
// Zero-width runes (control characters, combining marks) report 1 rather than
// 0: rune-based surfaces never fold a rune into the preceding cell, so a 0
// here would stall column advance and overwrite the previous cell.
func Rune(r rune) int {
	if options.Rune(r) > 1 {
		return 2
	}
	return 1
}

// String returns the number of terminal columns s occupies.
//
// It sums Rune for surfaces that draw rune by rune. Surfaces that store complete
// graphemes in their cells must use GraphemeWidth instead.
func String(s string) int {
	w := 0
	for _, r := range s {
		w += Rune(r)
	}
	return w
}

// Runes returns the number of terminal columns runes occupy.
func Runes(runes []rune) int {
	w := 0
	for _, r := range runes {
		w += Rune(r)
	}
	return w
}

// Graphemes is for surfaces that store a complete grapheme in each cell.
// Rune, String and Runes retain the editor's per-rune layout contract.
func Graphemes(s string) displaywidth.Graphemes[string] {
	return options.StringGraphemes(s)
}

func GraphemeWidth(s string) int {
	w := 0
	clusters := Graphemes(s)
	for clusters.Next() {
		w += max(1, clusters.Width())
	}
	return w
}
