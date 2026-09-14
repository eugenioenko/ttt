package app

import (
	"log/slog"
	"runtime"
	"time"

	"github.com/eugenioenko/ttt/internal/appearance"
	"github.com/eugenioenko/ttt/internal/config"

	"github.com/gdamore/tcell/v3"
)

// autoThemePollInterval is the backstop behind focus-gained re-checks: some
// hosts (multiplexers) swallow focus reports, so a cheap OS/env poll keeps
// auto mode live there too. Ticks that change nothing skip the redraw.
const autoThemePollInterval = 5 * time.Second

type autoThemeTick struct{ Gen uint64 }

// autoThemeAppearance carries an off-loop detection result back to the event
// loop. DetectLive shells to `defaults` on darwin (up to 300ms), so focus
// and poll handlers request it asynchronously instead of blocking the loop.
type autoThemeAppearance struct{ Ap appearance.Appearance }

// autoThemeLiveSupported reports whether the live signal can observe change.
// Off darwin DetectLive only re-reads the spawn-frozen COLORFGBG, which can
// never change, so focus re-checks and the backstop poll are darwin-only.
// (Linux portal support lifts this when it lands.)
func autoThemeLiveSupported() bool { return runtime.GOOS == "darwin" }

// requestAutoThemeCheck resolves the live appearance off the event loop and
// posts the result back for ApplyAppearance. At most one check is ever in
// flight (loop-thread-confined flag): concurrent triggers coalesce, which
// also rules out stale-after-newer application without a sequence number.
func (a *App) requestAutoThemeCheck() {
	screen := a.Screen
	if screen == nil || a.autoThemeCheckInflight {
		return
	}
	a.autoThemeCheckInflight = true
	go func() {
		ap := appearance.DetectLive()
		screen.PostEvent(tcell.NewEventInterrupt(&autoThemeAppearance{Ap: ap}))
	}()
}

func (a *App) applyThemeConfig(theme config.ThemeConfig) {
	a.Screen.SetStyleMap(BuildStyleMap(theme, WithTransparentBackground(a.Settings.Editor.TransparentBackground)))
	*a.Palette = BuildTerminalPalette(theme, WithTransparentBackground(a.Settings.Editor.TransparentBackground))
	*a.Borders = BuildBorderSet(theme.Borders)
	a.ApplyBorderStyle()
	a.Renderer.Clear()
	a.invalidateImageLayer()
}

// IsAutoTheme reports whether live theme switching is active.
func (a *App) IsAutoTheme() bool {
	return a.Settings != nil && a.Settings.Theme == "auto"
}

// SetAutoTheme records the startup appearance (and its resolved theme name,
// backing the lastAutoTheme fallback) so the first live re-check is a no-op
// when nothing changed.
func (a *App) SetAutoTheme(ap appearance.Appearance) {
	a.autoAppearance = ap
	if ap != appearance.Unknown && a.Settings != nil {
		a.lastAutoTheme = appearance.ResolveThemeName(ap, a.Settings.ThemeLight, a.Settings.ThemeDark)
	}
}

// resolveAutoTheme maps an appearance to a loaded theme honoring the
// configured light/dark names.
func (a *App) resolveAutoTheme(ap appearance.Appearance) (config.ThemeConfig, string, bool) {
	name := appearance.ResolveThemeName(ap, a.Settings.ThemeLight, a.Settings.ThemeDark)
	theme, err := config.LoadTheme(name)
	if err != nil {
		slog.Debug("auto theme resolve", "appearance", ap.String(), "theme", name, "error", err)
		return config.ThemeConfig{}, "", false
	}
	return theme, name, true
}

// StartAutoThemePoll begins the backstop poll; call once when auto mode is
// entered (startup or settings apply). DisarmAutoThemePoll stops it.
func (a *App) StartAutoThemePoll() {
	a.armAutoThemePoll()
}

// DisarmAutoThemePoll stops the backstop poll and invalidates in-flight ticks.
func (a *App) DisarmAutoThemePoll() {
	a.autoThemeGen++
	if a.autoThemeTimer != nil {
		a.autoThemeTimer.Stop()
		a.autoThemeTimer = nil
	}
}

func (a *App) armAutoThemePoll() {
	if !a.IsAutoTheme() || a.Screen == nil {
		return
	}
	if !autoThemeLiveSupported() {
		return
	}
	a.autoThemeGen++
	gen := a.autoThemeGen
	if a.autoThemeTimer != nil {
		a.autoThemeTimer.Stop()
	}
	screen := a.Screen
	a.autoThemeTimer = time.AfterFunc(autoThemePollInterval, func() {
		screen.PostEvent(tcell.NewEventInterrupt(&autoThemeTick{Gen: gen}))
	})
}

// ApplyAppearance switches to the theme for ap. It is a no-op unless auto
// mode is on (`"theme": "auto"`), ap is known, and ap differs from current.
// Manual theme picks write Settings.Theme and turn auto off by construction.
func (a *App) ApplyAppearance(ap appearance.Appearance) bool {
	if !a.IsAutoTheme() {
		return false
	}
	if ap == appearance.Unknown || ap == a.autoAppearance {
		slog.Debug("auto theme check", "appearance", ap.String(), "current", a.autoAppearance.String(), "changed", false)
		return false
	}
	theme, name, ok := a.resolveAutoTheme(ap)
	if !ok {
		return false
	}
	a.autoAppearance = ap
	a.lastAutoTheme = name
	a.applyThemeConfig(theme)
	slog.Info("auto theme applied", "appearance", ap.String(), "theme", name)
	return true
}
