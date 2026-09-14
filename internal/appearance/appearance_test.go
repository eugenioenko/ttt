package appearance

import (
	"runtime"
	"testing"
)

func TestInferredAppearanceBoundary(t *testing.T) {
	if got := InferredAppearance(0, 0, 0); got != Dark {
		t.Errorf("black = %v, want dark", got)
	}
	if got := InferredAppearance(255, 255, 255); got != Light {
		t.Errorf("white = %v, want light", got)
	}
	// herdr weights: 0x1f*299+0x1f*587+0x1f*114 = 31000 -> dark.
	if got := InferredAppearance(0x1f, 0x1f, 0x1f); got != Dark {
		t.Errorf("0x1f gray = %v, want dark", got)
	}
}

func TestParseCOLORFGBG(t *testing.T) {
	cases := []struct {
		in   string
		want Appearance
	}{
		{"0;default;15", Light},
		{"15;default;0", Dark},
		{"15;8", Dark},
		{"0;7", Light},
		{"", Unknown},
		{"0;default;default", Unknown},
		{"0;default;99", Unknown},
	}
	for _, c := range cases {
		if got := ParseCOLORFGBG(c.in); got != c.want {
			t.Errorf("ParseCOLORFGBG(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseColorSchemeReport(t *testing.T) {
	if got := ParseColorSchemeReport("\x1b[?997;1n"); got != Dark {
		t.Errorf("997;1 = %v, want dark", got)
	}
	if got := ParseColorSchemeReport("\x1b[?997;2n"); got != Light {
		t.Errorf("997;2 = %v, want light", got)
	}
	if got := ParseColorSchemeReport("\x1b[?997;0n"); got != Unknown {
		t.Errorf("997;0 = %v, want unknown", got)
	}
	if got := ParseColorSchemeReport("noise"); got != Unknown {
		t.Errorf("noise = %v, want unknown", got)
	}
}

func TestParseOSC11Response(t *testing.T) {
	cases := []struct {
		in   string
		want Appearance
	}{
		{"\x1b]11;rgb:0000/0000/0000\x1b\\", Dark},
		{"\x1b]11;rgb:ffff/ffff/ffff\x1b\\", Light},
		{"\x1b]11;rgb:ff/ff/ff\x07", Light},
		{"\x1b]11;#ffffff\x1b\\", Light},
		{"\x1b]11;#000\x1b\\", Dark},
		{"\x1b]11;rgb:1e1e/2b2b/3a3a\x1b\\", Dark},
		{"unrelated", Unknown},
		{"\x1b]11;nonsense\x1b\\", Unknown},
	}
	for _, c := range cases {
		if got := ParseOSC11Response(c.in); got != c.want {
			t.Errorf("ParseOSC11Response(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestScaleHexWidth(t *testing.T) {
	// "f" and "ffff" are both full intensity in rgb: specs.
	full1, ok1 := scaleHex("f")
	full4, ok4 := scaleHex("ffff")
	if !ok1 || !ok4 || full1 != 255 || full4 != 255 {
		t.Errorf("scaleHex width handling broken: %v %v %v %v", full1, ok1, full4, ok4)
	}
	if _, ok := scaleHex("zz"); ok {
		t.Error("scaleHex accepted non-hex")
	}
}

func TestResolveThemeName(t *testing.T) {
	if got := ResolveThemeName(Light, "", ""); got != "default-light" {
		t.Errorf("light defaults = %q", got)
	}
	if got := ResolveThemeName(Dark, "", ""); got != "default-dark" {
		t.Errorf("dark defaults = %q", got)
	}
	if got := ResolveThemeName(Unknown, "", ""); got != "default-dark" {
		t.Errorf("unknown preserves dark default = %q", got)
	}
	if got := ResolveThemeName(Light, "", "solarized-dark"); got != "solarized-light" {
		t.Errorf("sibling fallback = %q", got)
	}
	if got := ResolveThemeName(Dark, "solarized-light", ""); got != "solarized-dark" {
		t.Errorf("sibling fallback = %q", got)
	}
	if got := ResolveThemeName(Light, "my-light", "my-dark"); got != "my-light" {
		t.Errorf("explicit light = %q", got)
	}
	if got := ResolveThemeName(Dark, "my-light", "my-dark"); got != "my-dark" {
		t.Errorf("explicit dark = %q", got)
	}
	if got := ResolveThemeName(Light, "", "dracula"); got != "default-light" {
		t.Errorf("no sibling falls back to default-light = %q", got)
	}
}

func TestDetectLivePrefersOSOverSpawnEnv(t *testing.T) {
	// Regression test for stale-COLORFGBG shadowing: the live signal must
	// not move when only the spawn-frozen env changes.
	t.Setenv("COLORFGBG", "15;default;0")
	a := DetectLive()
	t.Setenv("COLORFGBG", "0;default;15")
	b := DetectLive()
	if runtime.GOOS == "darwin" {
		if a == Unknown || b == Unknown {
			t.Fatalf("DetectLive = %v/%v on darwin, want a known appearance", a, b)
		}
		if a != b {
			t.Errorf("DetectLive follows COLORFGBG (%v vs %v), want the OS signal", a, b)
		}
		return
	}
	if b != Light {
		t.Errorf("DetectLive off-darwin = %v, want COLORFGBG light", b)
	}
}
func TestDetectFromEnv(t *testing.T) {
	env := func(k string) string {
		if k == "COLORFGBG" {
			return "0;default;15"
		}
		return ""
	}
	if got, src := detectFromEnv(env); got != Light || src != "env" {
		t.Errorf("detectFromEnv = (%v, %q), want (light, env)", got, src)
	}
}
