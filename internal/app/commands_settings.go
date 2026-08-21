package app

import (
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
	a.EditorGroup.BracketPairColorization = s.Editor.BracketPairColorization
	a.EditorGroup.UndoDeleteCursorStart = s.Editor.UndoDeleteCursorStart
	a.EditorGroup.ApplyUndoDeleteCursorStart(s.Editor.UndoDeleteCursorStart)

	if a.EditorGroup.Editor != nil {
		a.EditorGroup.Editor.TabSize = s.Editor.TabSize
		a.EditorGroup.Editor.LineNumbers = s.Editor.LineNumbers
		a.EditorGroup.Editor.GutterStyle = s.Editor.GutterStyle
		a.EditorGroup.Editor.AutoDedent = s.Editor.IsAutoDedentEnabled()
		a.EditorGroup.Editor.AutoIndent = s.Editor.IsAutoIndentEnabled()
		a.EditorGroup.Editor.WordWrap = s.Editor.WordWrap
		a.EditorGroup.Editor.BracketPairColorization = s.Editor.BracketPairColorization
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
	if a.Sidebar != nil {
		a.Sidebar.SetPanelOrder(s.Sidebar.PanelOrder)
	}

	// LoadTheme also accepts the empty built-in-default ID used by both theme
	// pickers, so every surfaced theme follows one resolve-and-apply path.
	var themeBorders *term.BorderSet
	if a.Screen != nil {
		if theme, err := config.LoadTheme(s.Theme); err == nil {
			a.Screen.SetStyleMap(BuildStyleMap(theme))
			*a.Palette = BuildTerminalPalette(theme)
			borders := BuildBorderSet(theme.Borders)
			*a.Borders = borders
			themeBorders = &borders
			a.Renderer.Clear()
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
