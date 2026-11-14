package e2e

import (
	"os"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestThemePickerAppearance(t *testing.T) {
	for _, tc := range []struct {
		name        string
		theme       string
		borderStyle string
		preview     term.BorderSet
	}{
		{"built-in default", "", "default", term.DoubleBorderSet()},
		{"named theme", "default-dark", "default", term.DoubleBorderSet()},
		{"theme alias", "default-dark", "theme", term.DoubleBorderSet()},
		{"rounded override", "default-dark", "rounded", term.RoundedBorderSet()},
		{"ASCII override", "default-dark", "ascii", term.AsciiBorderSet()},
	} {
		for _, confirm := range []bool{false, true} {
			action := "cancel"
			if confirm {
				action = "confirm"
			}
			t.Run(tc.name+"/"+action, func(t *testing.T) {
				h := newTestHarness(t, 120, 40)
				h.app.Settings.Theme = tc.theme
				h.app.Settings.Editor.BorderStyle = tc.borderStyle
				if !h.app.SaveAndApplySettings() {
					t.Fatal("could not save initial settings")
				}
				h.redraw()
				originalBorders := *h.app.Borders
				originalStyles := h.app.Screen.GetStyleMap()
				originalTokens := highlight.CurrentTokenTheme()
				originalPalette := *h.app.Palette
				settingsPath := config.ConfigFilePath("settings.json")
				originalSettings, err := os.ReadFile(settingsPath)
				if err != nil {
					t.Fatal(err)
				}

				h.exec("theme.switch")
				for _, r := range "turbo" {
					h.pressRune(r)
				}
				if got := *h.app.Borders; got != tc.preview {
					t.Errorf("preview borders = %v, want %v", got, tc.preview)
				}
				previewStyles := h.app.Screen.GetStyleMap()
				previewPalette := *h.app.Palette
				if previewStyles == originalStyles {
					t.Error("preview did not change theme colors")
				}
				if h.app.Settings.Theme != tc.theme {
					t.Error("preview changed the selected theme setting")
				}

				if confirm {
					h.pressKey(tcell.KeyEnter, tcell.ModNone)
					if *h.app.Borders != tc.preview || h.app.Screen.GetStyleMap() != previewStyles || *h.app.Palette != previewPalette {
						t.Error("confirm did not retain the previewed appearance")
					}
					if h.app.Settings.Theme != "turbo-vision" {
						t.Error("confirm did not select turbo-vision")
					}
					saved, err := config.ReadSettingsFile(settingsPath)
					if err != nil || saved.Theme != "turbo-vision" || saved.Editor.BorderStyle != tc.borderStyle {
						t.Fatalf("confirmed settings = %+v, error = %v", saved, err)
					}
				} else {
					h.pressKey(tcell.KeyEscape, tcell.ModNone)
					if *h.app.Borders != originalBorders {
						t.Errorf("cancel borders = %v, want %v", *h.app.Borders, originalBorders)
					}
					if h.app.Screen.GetStyleMap() != originalStyles || highlight.CurrentTokenTheme() != originalTokens || *h.app.Palette != originalPalette {
						t.Error("cancel did not restore the original theme colors")
					}
					if h.app.Settings.Theme != tc.theme {
						t.Error("cancel changed the selected theme setting")
					}
					unchanged, err := os.ReadFile(settingsPath)
					if err != nil || string(unchanged) != string(originalSettings) {
						t.Fatalf("cancel changed saved settings: %v", err)
					}
				}
				if h.app.Root.HasOverlay() {
					t.Error("theme picker remained open")
				}
			})
		}
	}
}
