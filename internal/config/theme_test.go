package config

import (
	"encoding/json"
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

			// After resolving, verify critical fields are non-empty
			if th.Default.Fg == "" {
				t.Errorf("%s: Default.Fg is empty after resolve", name)
			}
			if th.Diff.Collapsed.Fg == "" || th.Diff.Collapsed.Bg == "" {
				t.Errorf("%s: collapsed diff style did not inherit defaults: %+v", name, th.Diff.Collapsed)
			}
		})
	}
}

func TestResolveColors(t *testing.T) {
	th := DefaultTheme()
	// Clear fields that ResolveColors should fill
	th.Diff.Added.Bg = ""
	th.Diff.Deleted.Bg = ""
	th.Diff.Modified.Bg = ""
	th.Diff.Collapsed.Fg = ""
	th.Diff.Collapsed.Bg = ""
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
	if th.Diff.Collapsed.Fg == "" || th.Diff.Collapsed.Bg == "" {
		t.Errorf("expected Diff.Collapsed colors to be filled by ResolveColors, got %+v", th.Diff.Collapsed)
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
	th.Diff.Collapsed = StyleDef{Fg: "#custom4", Bg: "#custom5", Bold: true}

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
	if th.Diff.Collapsed.Fg != "#custom4" || th.Diff.Collapsed.Bg != "#custom5" || !th.Diff.Collapsed.Bold {
		t.Errorf("expected explicit Diff.Collapsed to remain unchanged, got %+v", th.Diff.Collapsed)
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
