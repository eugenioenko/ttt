package app

import (
	"github.com/eugenioenko/ttt/internal/appearance"
	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
)

func configuredDiffMode(mode string) ui.DiffMode {
	if mode == config.DiffModeUnified {
		return ui.DiffModeUnified
	}
	return ui.DiffModeSplit
}

func configuredDiffContext(contextMode string) ui.DiffContextMode {
	if contextMode == config.DiffContextFull {
		return ui.DiffContextFullFile
	}
	return ui.DiffContextChangesOnly
}

func (a *App) ReloadSettings() {
	s := config.LoadSettings()
	a.ApplySettings(s)
	a.StatusNotify("Settings reloaded")
}

// ApplySettings is the single live-apply path: anything that can take effect
// without a restart belongs here, so every caller produces identical results.
func (a *App) ApplySettings(s config.Settings) {
	// Side effects below are keyed off what actually changed: ApplySettings runs
	// on every option toggle, and refetching the git gutter or rebuilding bracket
	// colors each time would be wasted work. The baseline is the last applied
	// value, not *a.Settings — callers such as the Options toggles mutate that
	// before calling in, so reading it here would always compare s against itself.
	prev := a.appliedSettings
	a.appliedSettings = s
	*a.Settings = s

	// Apply editor settings to the editor group and active editor
	a.EditorGroup.TabSize = s.Editor.TabSize
	a.EditorGroup.InsertSpaces = s.Editor.InsertSpaces
	a.EditorGroup.LineNumbers = s.Editor.LineNumbers
	a.EditorGroup.GutterStyle = s.Editor.GutterStyle
	a.EditorGroup.InsertFinalNewline = s.Editor.InsertFinalNewline
	a.EditorGroup.ShowTrailingNewline = s.Editor.IsShowTrailingNewlineEnabled()
	a.EditorGroup.TrimTrailingWhitespace = s.Editor.TrimTrailingWhitespace
	a.EditorGroup.WordWrap = s.Editor.WordWrap
	a.EditorGroup.SetDiffDefaults(configuredDiffMode(s.Editor.DiffMode), configuredDiffContext(s.Editor.DiffContext), s.Editor.DiffWordWrap)
	a.EditorGroup.SetDiffHighContrast(s.Editor.DiffHighContrast)
	a.EditorGroup.SetDiffCollapsedEmphasis(s.Editor.DiffCollapsedEmphasis)
	a.EditorGroup.BracketPairColorization = s.Editor.BracketPairColorization
	a.EditorGroup.UndoDeleteCursorStart = s.Editor.UndoDeleteCursorStart
	a.EditorGroup.ApplyUndoDeleteCursorStart(s.Editor.UndoDeleteCursorStart)
	a.EditorGroup.SetImageProtocol(s.Image.Protocol)
	if a.Sidebar != nil {
		a.Sidebar.SetPanelOrder(s.Sidebar.PanelOrder)
	}

	if a.EditorGroup.Editor != nil {
		a.EditorGroup.Editor.TabSize = s.Editor.TabSize
		a.EditorGroup.Editor.LineNumbers = s.Editor.LineNumbers
		a.EditorGroup.Editor.GutterStyle = s.Editor.GutterStyle
		a.EditorGroup.Editor.AutoDedent = s.Editor.IsAutoDedentEnabled()
		a.EditorGroup.Editor.AutoIndent = s.Editor.IsAutoIndentEnabled()
		a.EditorGroup.Editor.WordWrap = s.Editor.WordWrap
		a.EditorGroup.Editor.BracketPairColorization = s.Editor.BracketPairColorization
		a.EditorGroup.Editor.IndentGuides = s.Editor.IndentGuides
		if s.Editor.BracketPairColorization != prev.Editor.BracketPairColorization {
			a.EditorGroup.Editor.InvalidateBracketColors()
		}
	}

	a.applyMenuBarVisibility(s.Editor.IsMenuBarVisible())

	// Apply cursor style
	if a.Screen != nil {
		a.Screen.SetCursorStyle(term.ParseCursorStyle(s.Editor.CursorStyle))
	}

	// Apply search debounce
	a.Search.Debounce.DelayMs = s.Search.Debounce

	if s.Editor.IsGitGutterEnabled() != prev.Editor.IsGitGutterEnabled() {
		if s.Editor.IsGitGutterEnabled() {
			a.RequestGitGutterForActiveFile()
		} else if a.EditorGroup.Editor != nil {
			a.EditorGroup.Editor.LineChanges = nil
		}
	}

	if a.Explorer != nil && a.Explorer.Settings != s.Explorer {
		a.Explorer.Settings = s.Explorer
		a.Explorer.Reload()
	}
	if a.Changes != nil {
		a.Changes.SetFileView(s.Git.FileView)
	}

	// An empty theme name means the built-in default, and must still be applied —
	// otherwise switching back to it leaves the previous theme's colors on screen.
	// Poll and focus-subscription follow mode transitions only: re-arming on
	// every apply would reset the 5s backstop under frequent toggles.
	enteringAuto := s.Theme == "auto" && prev.Theme != "auto"
	leavingAuto := s.Theme != "auto" && prev.Theme == "auto"
	if enteringAuto {
		a.StartAutoThemePoll()
	} else if leavingAuto {
		a.DisarmAutoThemePoll()
	}
	// Theme resolution runs only when a theme-relevant key changed: ApplySettings
	// fires on every option toggle, and each would otherwise spawn a ~300ms
	// `defaults` subprocess while in auto mode. (Switching back to "" is
	// covered by the Theme comparison, preserving the must-still-apply rule.)
	themeRelevant := s.Theme != prev.Theme ||
		s.ThemeLight != prev.ThemeLight || s.ThemeDark != prev.ThemeDark ||
		s.Editor.TransparentBackground != prev.Editor.TransparentBackground
	var themeBorders *term.BorderSet
	if a.Screen != nil && themeRelevant {
		// Focus reporting is a terminal mode, not a setting: entering auto at
		// runtime must subscribe, leaving it must unsubscribe.
		if enteringAuto {
			a.Screen.EnableFocusReporting()
		} else if leavingAuto {
			a.Screen.DisableFocusReporting()
		}
		theme, ok := config.DefaultTheme(), s.Theme == ""
		if !ok {
			if s.Theme == "auto" {
				// Side names changed: drop the loaded-theme history so the
				// Unknown branch below cannot re-apply the stale choice.
				if s.ThemeLight != prev.ThemeLight || s.ThemeDark != prev.ThemeDark {
					a.lastAutoTheme = ""
				}
				// No tty query can run while tcell owns it, so live-apply
				// resolves from the live signal (DetectLive, not the
				// spawn-frozen COLORFGBG).
				if ap := appearance.DetectLive(); ap != appearance.Unknown {
					var name string
					theme, name, ok = a.resolveAutoTheme(ap)
					if ok {
						a.autoAppearance = ap
						a.lastAutoTheme = name
					}
				} else if a.lastAutoTheme != "" {
					// Detection failed but a previous auto theme is known:
					// re-apply it so unrelated toggles (e.g. transparent
					// background) still take effect instead of stalling.
					loaded, err := config.LoadTheme(a.lastAutoTheme)
					theme, ok = loaded, err == nil
				} else {
					// No signal and no history (first apply in an
					// undetectable environment): fall back to the built-in
					// default so the explicit apply still takes effect,
					// matching the Unknown-keeps-default startup rule.
					theme, ok = config.DefaultTheme(), true
				}
			} else {
				loaded, err := config.LoadTheme(s.Theme)
				theme, ok = loaded, err == nil
			}
		}
		if ok {
			a.Screen.SetStyleMap(BuildStyleMap(theme, WithTransparentBackground(s.Editor.TransparentBackground)))
			*a.Palette = BuildTerminalPalette(theme, WithTransparentBackground(s.Editor.TransparentBackground))
			borders := BuildBorderSet(theme.Borders)
			*a.Borders = borders
			themeBorders = &borders
			a.Renderer.Clear()
			a.invalidateImageLayer()
		}
	}

	// Overrides what the theme resolved, so it must run last and unconditionally.
	// Passing the borders just built avoids reloading the theme from disk.
	a.applyBorderStyle(themeBorders)
}

func registerSettingsCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID: "settings.reload", Title: "Reload Settings",
		Handler: app.ReloadSettings,
	})
}
