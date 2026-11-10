package app

import (
	"path/filepath"
	"slices"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/plugin"
)

func editorTerminalKey(id string) string { return "terminal:" + id }

func editorTerminalTitle(spec TerminalSpec) string {
	if spec.Label != "" {
		return spec.Label
	}
	if spec.Shell != "" {
		return filepath.Base(spec.Shell)
	}
	return i18n.Text("panel.terminal")
}

func (a *App) SpawnEditorTerminal() {
	a.SpawnTerminalSpec(TerminalSpec{InEditor: true})
}

func (a *App) SendNextTerminalKey() {
	if tw := a.focusedTerminalWidget(); tw != nil && !a.Root.HasOverlay() {
		a.Root.SendNextTerminalKey(tw)
	}
}

func (a *App) ToggleTerminalShortcutForwarding() {
	a.Root.SetForwardShortcuts(!a.Root.ForwardShortcuts)
	if a.Root.ForwardShortcuts {
		a.StatusNotify(i18n.Text("status.terminalShortcutsForwarded"))
	} else {
		a.StatusNotify(i18n.Text("status.tttShortcutsPrioritized"))
	}
}

func (a *App) openEditorTerminal(entry TerminalTab, spec TerminalSpec) {
	id := entry.ID
	entry.Tab.OnClose = func() { a.releaseEditorTerminal(id) }
	a.EditorTerminals = append(a.EditorTerminals, entry)
	a.EditorGroup.OpenPluginTab(editorTerminalKey(id), editorTerminalTitle(spec), entry.Tab)
	a.Root.SetFocus(a.EditorGroup)
}

func (a *App) releaseEditorTerminal(id string) bool {
	idx := slices.IndexFunc(a.EditorTerminals, func(tt TerminalTab) bool { return tt.ID == id })
	if idx < 0 {
		return false
	}
	a.EditorTerminals[idx].Term.Close()
	a.forgetAppTerminal(id)
	a.EditorTerminals = slices.Delete(a.EditorTerminals, idx, idx+1)
	return true
}

// closeEditorTerminal closes the editor tab, which releases the terminal
// through the tab's OnClose.
func (a *App) closeEditorTerminal(id string) bool {
	if !slices.ContainsFunc(a.EditorTerminals, func(tt TerminalTab) bool { return tt.ID == id }) {
		return false
	}
	if !a.EditorGroup.ClosePluginTab(editorTerminalKey(id)) {
		a.releaseEditorTerminal(id)
	}
	return true
}

func (a *App) focusEditorTerminal(id string) bool {
	if !a.EditorGroup.FocusPluginTab(editorTerminalKey(id)) {
		return false
	}
	a.Root.SetFocus(a.EditorGroup)
	return true
}

// resizeEditorTerminals sizes every editor terminal, shown or not, to the
// group's content area: all tabs share it, and a hidden tab's own rect is
// stale.
func (a *App) resizeEditorTerminals() {
	area := a.EditorGroup.ContentRect()
	for _, tt := range a.EditorTerminals {
		tt.Term.AckUpdate()
		if area.W <= 0 || area.H <= 0 {
			continue
		}
		tt.Tab.SetRect(area)
		if c := tt.Tab.ContentRect(); c.W > 0 && c.H > 0 {
			tt.Term.Resize(c.W, c.H)
		}
	}
}

func (tt TerminalTab) location() string {
	if tt.Tab != nil {
		return plugin.TerminalLocationEditor
	}
	return plugin.TerminalLocationPanel
}

func (a *App) allTerminals() []TerminalTab {
	return slices.Concat(a.Terminals, a.EditorTerminals)
}

func (a *App) terminalByID(id string) *TerminalTab {
	for _, list := range [][]TerminalTab{a.Terminals, a.EditorTerminals} {
		for i := range list {
			if list[i].ID == id {
				return &list[i]
			}
		}
	}
	return nil
}
