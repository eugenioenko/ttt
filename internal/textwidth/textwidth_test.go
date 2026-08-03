package textwidth

import (
	"testing"

	"github.com/clipperhouse/displaywidth"
)

func TestRuneWidth(t *testing.T) {
	cases := []struct {
		name string
		r    rune
		want int
	}{
		{"ascii letter", 'a', 1},
		{"ascii digit", '7', 1},
		{"space", ' ', 1},
		{"cyrillic is multi-byte but narrow", 'п', 1},
		{"greek", 'λ', 1},
		{"box drawing", '─', 1},
		{"ellipsis", '…', 1},
		{"korean syllable", '가', 2},
		{"chinese han", '你', 2},
		{"japanese hiragana", 'こ', 2},
		{"japanese katakana", 'ア', 2},
		{"fullwidth latin", 'Ａ', 2},
		{"halfwidth katakana", 'ｱ', 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Rune(tc.r); got != tc.want {
				t.Errorf("Rune(%q) = %d, want %d", tc.r, got, tc.want)
			}
		})
	}
}

// Zero-width runes must still advance one column: ttt draws one cell per rune,
// so a 0 would stall the layout and overwrite the previous cell.
func TestRuneWidthNeverZero(t *testing.T) {
	for _, r := range []rune{'\x00', '\x01', '\x7f', '́', '​'} {
		if got := Rune(r); got != 1 {
			t.Errorf("Rune(%U) = %d, want 1", r, got)
		}
	}
}

func TestStringWidth(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"hello", 5},
		{"привет", 6},
		{"가나다라마바사", 14},
		{"你好世界", 8},
		{"가a나b다", 8},
		{"hello가", 7},
	}
	for _, tc := range cases {
		if got := String(tc.in); got != tc.want {
			t.Errorf("String(%q) = %d, want %d", tc.in, got, tc.want)
		}
		if got := Runes([]rune(tc.in)); got != tc.want {
			t.Errorf("Runes(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// The RUNEWIDTH_EASTASIAN toggle must be read the same way tcell reads it
// (its internal widthutil.Options), or ambiguous-width runes get laid out at
// one width and drawn at another.
func TestEastAsianOptions(t *testing.T) {
	wide := displaywidth.Options{EastAsianWidth: true}
	narrow := displaywidth.Options{}

	for _, env := range []string{"1", "true", "yes", "TRUE", "Yes"} {
		if got := eastAsianOptions(env); got != wide {
			t.Errorf("eastAsianOptions(%q) = %+v, want east asian width", env, got)
		}
	}
	for _, env := range []string{"", "0", "false", "no", "maybe"} {
		if got := eastAsianOptions(env); got != narrow {
			t.Errorf("eastAsianOptions(%q) = %+v, want narrow", env, got)
		}
	}
}
