package e2e

import (
	"reflect"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/appearance"
	"github.com/eugenioenko/ttt/internal/config"
)

func mustLoadTheme(t *testing.T, name string) config.ThemeConfig {
	t.Helper()
	theme, err := config.LoadTheme(name)
	if err != nil {
		t.Fatalf("LoadTheme(%s): %v", name, err)
	}
	return theme
}

func TestAutoThemeApplyAppearanceFlipsStyleMap(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	h.app.Settings.Theme = "auto"
	h.app.Settings.ThemeLight = "default-light"
	h.app.Settings.ThemeDark = "default-dark"
	h.app.SetAutoTheme(appearance.Dark)

	before := h.app.Screen.GetStyleMap()
	if !h.app.ApplyAppearance(appearance.Light) {
		t.Fatal("ApplyAppearance(light) reported no change")
	}
	after := h.app.Screen.GetStyleMap()
	if reflect.DeepEqual(before, after) {
		t.Fatal("style map unchanged after light switch")
	}
	if want := app.BuildStyleMap(mustLoadTheme(t, "default-light")); !reflect.DeepEqual(after, want) {
		t.Error("style map does not match default-light after switch")
	}
	if h.app.ApplyAppearance(appearance.Light) {
		t.Error("second ApplyAppearance(light) should be a no-op")
	}
	if !h.app.ApplyAppearance(appearance.Dark) {
		t.Fatal("ApplyAppearance(dark) should switch back")
	}
	h.app.Settings.Theme = "default-dark"
	if h.app.ApplyAppearance(appearance.Light) {
		t.Error("manual theme pick should disable auto switching")
	}
}

func TestAutoThemeInvalidSideThemeFallsBackToBuiltin(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	h.app.Settings.Theme = "auto"
	h.app.Settings.ThemeLight = "no-such-theme"
	h.app.Settings.ThemeDark = "default-dark"
	h.app.SetAutoTheme(appearance.Dark)

	if !h.app.ApplyAppearance(appearance.Light) {
		t.Fatal("ApplyAppearance(light) should fall back instead of stalling")
	}
	if want := app.BuildStyleMap(mustLoadTheme(t, "default-light")); !reflect.DeepEqual(h.app.Screen.GetStyleMap(), want) {
		t.Error("broken light side theme did not fall back to default-light")
	}
}

func TestAutoThemeSettingsViewAppliesLive(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	h.exec("settings.openUI")
	clickRowControl(t, h, "Appearance", "Appearance")

	if !rowHas(h, "Light theme", "Automatic") {
		t.Fatalf("Light theme row missing:\n%s", h.screenText())
	}
	if !rowHas(h, "Dark theme", "Automatic") {
		t.Fatalf("Dark theme row missing:\n%s", h.screenText())
	}

	// Open the Theme popup and pick Auto. "Automatic" also contains "Auto",
	// so match the popup item by exclusion.
	clickRowControl(t, h, "Theme", "Default")
	picked := false
	for y, line := range strings.Split(h.screenText(), "\n") {
		if strings.Contains(line, "Auto") && !strings.Contains(line, "Automatic") {
			if x := strings.Index(line, "Auto"); x >= 0 {
				h.click(x+1, y)
				picked = true
				break
			}
		}
	}
	if !picked {
		t.Fatalf("Auto missing from theme list:\n%s", h.screenText())
	}

	t.Setenv("COLORFGBG", "0;default;15")
	h.exec("settings.apply")
	if h.app.Settings.Theme != "auto" {
		t.Errorf("theme = %q, want auto", h.app.Settings.Theme)
	}
	// Live-apply follows the live signal (OS on macOS), not the
	// spawn-frozen COLORFGBG, so resolve the expectation the same way.
	// The want value must be one of the two built-in defaults here since
	// no explicit side themes are configured; that pins the wiring without
	// letting a wrong-but-consistent signal pass silently.
	want := appearance.ResolveThemeName(appearance.DetectLive(), "", "")
	if want != "default-light" && want != "default-dark" {
		t.Fatalf("test setup: want = %q, expected a built-in default", want)
	}
	if wantMap := app.BuildStyleMap(mustLoadTheme(t, want)); !reflect.DeepEqual(h.app.Screen.GetStyleMap(), wantMap) {
		t.Errorf("applying auto theme did not live-apply %s", want)
	}
	if !strings.Contains(h.screenText(), "Settings applied") {
		t.Errorf("expected apply confirmation:\n%s", h.screenText())
	}
}
