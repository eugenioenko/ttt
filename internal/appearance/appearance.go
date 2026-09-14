// Package appearance detects the host terminal's light/dark appearance and
// resolves which ttt theme to use. The parsing and resolution here are pure
// logic (no tty I/O) so they stay unit-testable; the one tty query lives in
// query.go and must run before tcell.Init, because tcell swallows OSC and
// color-scheme DSR replies once its input loop owns the tty.
package appearance

import (
	"strconv"
	"strings"
)

type Appearance int

const (
	Unknown Appearance = iota
	Dark
	Light
)

func (a Appearance) String() string {
	switch a {
	case Light:
		return "light"
	case Dark:
		return "dark"
	default:
		return "unknown"
	}
}

// InferredAppearance maps an RGB background to an appearance using the same
// luminance weights herdr uses (r*299+g*587+b*114, light at >= 128000).
func InferredAppearance(r, g, b uint8) Appearance {
	lum := uint32(r)*299 + uint32(g)*587 + uint32(b)*114
	if lum >= 128_000 {
		return Light
	}
	return Dark
}

// ParseCOLORFGBG parses the COLORFGBG env value ("fg;default;bg"). Only the
// last field (background index 0-15) carries the signal. 0-6 and 8 are dark
// backgrounds, 7 and 15 light ones; anything else (bright hues) is honestly
// ambiguous without the palette, so it reports Unknown.
func ParseCOLORFGBG(s string) Appearance {
	fields := strings.Split(s, ";")
	bg, err := strconv.Atoi(strings.TrimSpace(fields[len(fields)-1]))
	if err != nil || bg < 0 || bg > 15 {
		return Unknown
	}
	switch bg {
	case 0, 1, 2, 3, 4, 5, 6, 8:
		return Dark
	case 7, 15:
		return Light
	default:
		return Unknown
	}
}

// ParseColorSchemeReport parses a Kitty color-scheme report,
// CSI ? 997 ; 1 n (dark) or CSI ? 997 ; 2 n (light), which the terminal sends
// in reply to CSI ? 996 n or spontaneously under mode 2031. Spaces are
// ignored; the value must be a lone digit, so longer values like 997;10
// never classify.
func ParseColorSchemeReport(resp string) Appearance {
	compact := strings.ReplaceAll(resp, " ", "")
	i := strings.Index(compact, "?997;")
	if i < 0 {
		return Unknown
	}
	rest := compact[i+5:]
	if len(rest) == 0 || (rest[0] != '1' && rest[0] != '2') {
		return Unknown
	}
	if len(rest) > 1 && rest[1] != 'n' && rest[1] != '\x1b' && rest[1] != '\x07' {
		return Unknown
	}
	if rest[0] == '1' {
		return Dark
	}
	return Light
}

// ParseOSC11Response parses an OSC 11 background query reply
// (ESC ] 11 ; <spec> ST, ST = ESC \ or BEL) into an appearance via luminance.
// It accepts rgb:R/G/B with 1-4 hex digits per channel and #RRGGBB / #RGB.
func ParseOSC11Response(resp string) Appearance {
	i := strings.Index(resp, "]11;")
	if i < 0 {
		return Unknown
	}
	spec := strings.TrimSpace(resp[i+4:])
	spec = strings.TrimSuffix(spec, "\x1b\\")
	spec = strings.TrimSuffix(spec, "\x07")
	spec = strings.TrimSpace(spec)
	r, g, b, ok := parseColorSpec(spec)
	if !ok {
		return Unknown
	}
	return InferredAppearance(r, g, b)
}

func parseColorSpec(spec string) (uint8, uint8, uint8, bool) {
	if rest, ok := strings.CutPrefix(spec, "rgb:"); ok {
		parts := strings.Split(rest, "/")
		if len(parts) != 3 {
			return 0, 0, 0, false
		}
		var v [3]uint8
		for i, p := range parts {
			n, ok := scaleHex(p)
			if !ok {
				return 0, 0, 0, false
			}
			v[i] = n
		}
		return v[0], v[1], v[2], true
	}
	if rest, ok := strings.CutPrefix(spec, "#"); ok {
		switch len(rest) {
		case 6, 12:
			n := len(rest) / 3
			var v [3]uint8
			for i := range 3 {
				sc, ok := scaleHex(rest[i*n : (i+1)*n])
				if !ok {
					return 0, 0, 0, false
				}
				v[i] = sc
			}
			return v[0], v[1], v[2], true
		case 3:
			var v [3]uint8
			for i := range 3 {
				sc, ok := scaleHex(rest[i : i+1])
				if !ok {
					return 0, 0, 0, false
				}
				v[i] = sc
			}
			return v[0], v[1], v[2], true
		}
	}
	return 0, 0, 0, false
}

// scaleHex scales 1-4 hex digits to 8 bits (rgb: spec allows up to 16 bits
// per channel; "f" and "ffff" both mean full intensity).
func scaleHex(s string) (uint8, bool) {
	if len(s) < 1 || len(s) > 4 {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return 0, false
	}
	max := uint64(1)<<(4*len(s)) - 1
	return uint8((n * 255) / max), true
}

var (
	lightSibling = map[string]string{
		"default-dark":       "default-light",
		"solarized-dark":     "solarized-light",
		"high-contrast-dark": "high-contrast-light",
		"rainy-day-dark":     "rainy-day",
	}
	darkSibling = map[string]string{
		"default-light":       "default-dark",
		"solarized-light":     "solarized-dark",
		"high-contrast-light": "high-contrast-dark",
		"rainy-day":           "rainy-day-dark",
	}
)

// ResolveThemeName picks the theme for an appearance. Explicit light/dark
// names win; otherwise the other side's known built-in sibling is used, and
// Unknown falls back to dark to preserve the current default behavior.
func ResolveThemeName(a Appearance, light, dark string) string {
	if a == Light {
		if light != "" {
			return light
		}
		if sib, ok := lightSibling[dark]; ok {
			return sib
		}
		return "default-light"
	}
	if dark != "" {
		return dark
	}
	if sib, ok := darkSibling[light]; ok {
		return sib
	}
	return "default-dark"
}
