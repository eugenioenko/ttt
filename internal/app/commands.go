package app

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/ui"
)

func RegisterCommands(app *App) {
	registerViewCommands(app)
	registerEditorCommands(app)
	registerSearchCommands(app)
	registerPaletteCommands(app)
	registerExplorerCommands(app)
	registerGitCommands(app)
	registerWorkspaceCommands(app)
	registerPRCommands(app)
	registerReviewCommands(app)
	registerHelpCommands(app)
	registerOptionsCommands(app)
	registerSettingsCommands(app)
	registerWidgetCallbacks(app)
	RegisterEscapeDismissers(app)
}

func (a *App) RebindKeys() {
	a.Root.ClearKeys()
	a.Reg.ClearAllShortcuts()
	config.ParseKeyBindings(a.Keybindings)
	BindKeys(a.Root, a.Reg, a.Keybindings)
}

func BindKeys(root *ui.Root, reg *command.Registry, keybindings []config.KeyBinding) {
	for _, kb := range keybindings {
		if len(kb.Steps) == 0 {
			continue
		}
		cmdID := kb.Command
		reg.SetShortcut(cmdID, FormatKeyBinding(kb.Key))
		if kb.IsChord() {
			steps := make([]ui.GlobalKeyBinding, len(kb.Steps))
			for i, step := range kb.Steps {
				key, mod, rn := comboToTcell(step)
				steps[i] = ui.GlobalKeyBinding{Key: key, Mod: mod, Rune: rn}
			}
			root.AddChordKey(steps, func() {
				reg.Execute(cmdID)
			})
		} else {
			key, mod, rn := comboToTcell(kb.Steps[0])
			handler := func() { reg.Execute(cmdID) }
			root.AddGlobalKey(key, mod, rn, handler)
			if config.ForceKeyCommands[cmdID] {
				root.AddForceKey(key, mod, rn, handler)
			}
		}
	}
}

func FormatKeyBinding(key string) string {
	parts := strings.Fields(key)
	for i, part := range parts {
		tokens := strings.Split(part, "+")
		for j, t := range tokens {
			if t == "backtick" {
				tokens[j] = "`"
			} else if len(t) > 0 {
				tokens[j] = strings.ToUpper(t[:1]) + t[1:]
			}
		}
		parts[i] = strings.Join(tokens, "+")
	}
	return strings.Join(parts, " ")
}

func RegisterEscapeDismissers(app *App) {
	app.Root.EscapeDismissers = []func() bool{
		func() bool {
			if app.IsAutocompleteActive() {
				app.DismissAutocomplete()
				return true
			}
			return false
		},
		func() bool {
			if app.EditorGroup.SignatureHelp != nil {
				app.DismissSignatureHelp()
				return true
			}
			return false
		},
		func() bool {
			if app.EditorGroup.Hover != nil {
				app.DismissHover()
				return true
			}
			return false
		},
		func() bool {
			if app.EditorGroup.IsMultiCursorActive() {
				app.EditorGroup.CollapseMultiCursor()
				return true
			}
			return false
		},
	}
	app.Root.EscapeFallback = func() {
		app.FocusEditor()
	}
}
