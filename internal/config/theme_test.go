package config

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config/themes"
)

func TestDefaultTheme(t *testing.T) {
	th := DefaultTheme()
	if th.Tabs.Active.Fg != "#ffffff" {
		t.Fatalf("expected ActiveTab.Fg '#ffffff', got '%s'", th.Tabs.Active.Fg)
	}
	if th.Tabs.Active.Bold != true {
		t.Fatal("expected ActiveTab.Bold true")
	}
	if th.Border.Fg != "#555555" {
		t.Fatalf("expected Border.Fg '#555555', got '%s'", th.Border.Fg)
	}
}

func TestThemePartialJSON(t *testing.T) {
	th := DefaultTheme()
	json.Unmarshal([]byte(`{"statusBar": {"fg": "yellow", "bg": "#ff0000"}}`), &th)
	th.ResolveColors()

	if th.StatusBar.Fg != "yellow" {
		t.Fatalf("expected StatusBar.Fg 'yellow', got '%s'", th.StatusBar.Fg)
	}
	if th.StatusBar.Bg != "#ff0000" {
		t.Fatalf("expected StatusBar.Bg '#ff0000', got '%s'", th.StatusBar.Bg)
	}
	if th.Tabs.Active.Fg != "#ffffff" {
		t.Fatalf("ActiveTab.Fg should still be '#ffffff', got '%s'", th.Tabs.Active.Fg)
	}
}

func TestThemeHexColors(t *testing.T) {
	th := ThemeConfig{}
	json.Unmarshal([]byte(`{"editor": {"lineNumber": {"fg": "#808080"}}}`), &th)

	if th.Editor.LineNumber.Fg != "#808080" {
		t.Fatalf("expected '#808080', got '%s'", th.Editor.LineNumber.Fg)
	}
}

func TestBundledThemesLoad(t *testing.T) {
	entries, err := themes.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read embedded themes: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one bundled theme")
	}

	for _, e := range entries {
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			data, err := themes.FS.ReadFile(name)
			if err != nil {
				t.Fatalf("failed to read %s: %v", name, err)
			}
			th := DefaultTheme()
			if err := json.Unmarshal(data, &th); err != nil {
				t.Fatalf("failed to parse %s: %v", name, err)
			}
			th.ResolveColors()

			var explicit ThemeConfig
			if err := json.Unmarshal(data, &explicit); err != nil {
				t.Fatalf("failed to inspect %s: %v", name, err)
			}
			if explicit.Diff.Collapsed.Fg == "" || explicit.Diff.Collapsed.Bg == "" {
				t.Errorf("%s: diff.collapsed must define both fg and bg", name)
			}
			if !explicit.Diff.Collapsed.Bold {
				t.Errorf("%s: diff.collapsed must deliberately enable bold", name)
			}
			if explicit.CommitMessage.Fg == "" || explicit.CommitMessage.Bg == "" {
				t.Errorf("%s: commitMessage must define both fg and bg", name)
			}
			if strings.HasPrefix(name, "high-contrast-") && !explicit.CommitMessage.Bold {
				t.Errorf("%s: high-contrast commitMessage must use a strong bold treatment", name)
			}

			// After resolving, verify critical fields are non-empty
			if th.Default.Fg == "" {
				t.Errorf("%s: Default.Fg is empty after resolve", name)
			}
		})
	}
}

func TestBundledDiffFamilyTextContrast(t *testing.T) {
	const minimumContrast = 4.5
	entries, err := themes.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("read bundled themes: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		data, err := themes.FS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var theme ThemeConfig
		if err := json.Unmarshal(data, &theme); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		styles := []struct {
			name  string
			style StyleDef
		}{
			{name: "commitMessage", style: theme.CommitMessage},
			{name: "diff.collapsed", style: theme.Diff.Collapsed},
		}
		for _, candidate := range styles {
			styleName, style := candidate.name, candidate.style
			t.Run(name+"/"+styleName, func(t *testing.T) {
				ratio, err := styleContrastRatio(style)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("%s %s contrast: %.2f:1", name, styleName, ratio)
				if ratio < minimumContrast {
					t.Errorf("contrast %.2f:1 is below %.1f:1 (fg %s, bg %s)", ratio, minimumContrast, style.Fg, style.Bg)
				}
			})
		}
	}
}

func styleContrastRatio(style StyleDef) (float64, error) {
	foreground, err := relativeLuminance(style.Fg)
	if err != nil {
		return 0, fmt.Errorf("foreground %q: %w", style.Fg, err)
	}
	background, err := relativeLuminance(style.Bg)
	if err != nil {
		return 0, fmt.Errorf("background %q: %w", style.Bg, err)
	}
	lighter, darker := max(foreground, background), min(foreground, background)
	return (lighter + 0.05) / (darker + 0.05), nil
}

func relativeLuminance(color string) (float64, error) {
	if len(color) != 7 || color[0] != '#' {
		return 0, fmt.Errorf("want #RRGGBB")
	}
	value, err := strconv.ParseUint(color[1:], 16, 24)
	if err != nil {
		return 0, fmt.Errorf("want #RRGGBB: %w", err)
	}
	linear := func(channel uint64) float64 {
		srgb := float64(channel) / 255
		if srgb <= 0.04045 {
			return srgb / 12.92
		}
		return math.Pow((srgb+0.055)/1.055, 2.4)
	}
	red := linear((value >> 16) & 0xff)
	green := linear((value >> 8) & 0xff)
	blue := linear(value & 0xff)
	return 0.2126*red + 0.7152*green + 0.0722*blue, nil
}

func TestResolveColors(t *testing.T) {
	th := DefaultTheme()
	// Clear fields that ResolveColors should fill
	th.Diff.Added.Bg = ""
	th.Diff.Deleted.Bg = ""
	th.Diff.Modified.Bg = ""
	th.Success.Fg = ""
	th.Danger.Fg = ""
	th.Warning.Fg = ""
	th.Input.Item.Bg = ""
	th.Input.Item.Fg = ""
	th.Input.Placeholder.Fg = ""

	th.ResolveColors()

	if th.Diff.Added.Bg == "" {
		t.Error("expected Diff.Added.Bg to be filled by ResolveColors")
	}
	if th.Diff.Deleted.Bg == "" {
		t.Error("expected Diff.Deleted.Bg to be filled by ResolveColors")
	}
	if th.Diff.Modified.Bg == "" {
		t.Error("expected Diff.Modified.Bg to be filled by ResolveColors")
	}
	if th.Success.Fg == "" {
		t.Error("expected Success.Fg to be filled by ResolveColors")
	}
	if th.Danger.Fg == "" {
		t.Error("expected Danger.Fg to be filled by ResolveColors")
	}
	if th.Warning.Fg == "" {
		t.Error("expected Warning.Fg to be filled by ResolveColors")
	}
	if th.Input.Item.Bg == "" {
		t.Error("expected Input.Item.Bg to be filled by ResolveColors")
	}
	if th.Input.Item.Fg == "" {
		t.Error("expected Input.Item.Fg to be filled by ResolveColors")
	}
	if th.Input.Placeholder.Fg == "" {
		t.Error("expected Input.Placeholder.Fg to be filled by ResolveColors")
	}
	if !th.Hover.Bold.Bold {
		t.Error("expected Hover.Bold.Bold to be true after ResolveColors")
	}
}

func TestResolveColorsPreservesExisting(t *testing.T) {
	th := DefaultTheme()
	th.Success.Fg = "#custom"
	th.Danger.Fg = "#custom2"
	th.Diff.Added.Bg = "#custom3"

	th.ResolveColors()

	if th.Success.Fg != "#custom" {
		t.Errorf("expected Success.Fg to remain '#custom', got %q", th.Success.Fg)
	}
	if th.Danger.Fg != "#custom2" {
		t.Errorf("expected Danger.Fg to remain '#custom2', got %q", th.Danger.Fg)
	}
	if th.Diff.Added.Bg != "#custom3" {
		t.Errorf("expected Diff.Added.Bg to remain '#custom3', got %q", th.Diff.Added.Bg)
	}
}

func TestDefaultTerminalColors(t *testing.T) {
	tc := DefaultTerminalColors()

	if tc.Black == "" {
		t.Error("expected Black to be set")
	}
	if tc.White == "" {
		t.Error("expected White to be set")
	}
	if tc.Red == "" {
		t.Error("expected Red to be set")
	}
	if tc.BrightWhite == "" {
		t.Error("expected BrightWhite to be set")
	}
}

func TestTerminalColorsANSIPalette(t *testing.T) {
	tc := DefaultTerminalColors()
	palette := tc.ANSIPalette()

	if len(palette) != 16 {
		t.Fatalf("expected 16 colors in palette, got %d", len(palette))
	}

	// Verify palette order: ANSI 0-7 are normal colors, 8-15 are bright
	if palette[0] != tc.Black {
		t.Errorf("palette[0] should be Black, got %q", palette[0])
	}
	if palette[1] != tc.Red {
		t.Errorf("palette[1] should be Red, got %q", palette[1])
	}
	if palette[7] != tc.White {
		t.Errorf("palette[7] should be White, got %q", palette[7])
	}
	if palette[8] != tc.BrightBlack {
		t.Errorf("palette[8] should be BrightBlack, got %q", palette[8])
	}
	if palette[15] != tc.BrightWhite {
		t.Errorf("palette[15] should be BrightWhite, got %q", palette[15])
	}

	// All palette entries should be non-empty
	for i, c := range palette {
		if c == "" {
			t.Errorf("palette[%d] is empty", i)
		}
	}
}

func TestTerminalColorsColorByName(t *testing.T) {
	tc := DefaultTerminalColors()

	tests := []struct {
		name string
		want string
	}{
		{"black", tc.Black},
		{"red", tc.Red},
		{"green", tc.Green},
		{"yellow", tc.Yellow},
		{"blue", tc.Blue},
		{"magenta", tc.Magenta},
		{"cyan", tc.Cyan},
		{"white", tc.White},
		{"brightBlack", tc.BrightBlack},
		{"brightRed", tc.BrightRed},
		{"brightGreen", tc.BrightGreen},
		{"brightYellow", tc.BrightYellow},
		{"brightBlue", tc.BrightBlue},
		{"brightMagenta", tc.BrightMagenta},
		{"brightCyan", tc.BrightCyan},
		{"brightWhite", tc.BrightWhite},
	}

	for _, tt := range tests {
		got := tc.ColorByName(tt.name)
		if got != tt.want {
			t.Errorf("ColorByName(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestTerminalColorsColorByNameUnknown(t *testing.T) {
	tc := DefaultTerminalColors()
	got := tc.ColorByName("nonexistent")
	if got != "" {
		t.Errorf("expected empty string for unknown color name, got %q", got)
	}
}

func TestThemeNameFromFile(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"dark.json", "dark"},
		{"one-dark.json", "one-dark"},
		{"theme.txt", ""},
		{"nojson", ""},
		{".json", ""},
	}
	for _, tt := range tests {
		got := themeNameFromFile(tt.input)
		if got != tt.want {
			t.Errorf("themeNameFromFile(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDefaultThemeBorders(t *testing.T) {
	th := DefaultTheme()
	if th.Borders.Horizontal != "─" {
		t.Errorf("expected Borders.Horizontal '─', got %q", th.Borders.Horizontal)
	}
	if th.Borders.Vertical != "│" {
		t.Errorf("expected Borders.Vertical '│', got %q", th.Borders.Vertical)
	}
	if th.Borders.TopLeft != "╭" {
		t.Errorf("expected Borders.TopLeft '╭', got %q", th.Borders.TopLeft)
	}
}
