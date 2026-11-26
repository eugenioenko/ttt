package app

import (
	"fmt"
	"log/slog"
	"os"
	"slices"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v3"
)

type TerminalTab struct {
	ID     string
	Label  string
	Term   *terminal.Terminal
	Widget *ui.TerminalWidget
	// Tab is set when the terminal lives in an editor tab instead of the panel.
	Tab *ui.TerminalTabWidget
}

// TerminalSpec describes the program one terminal tab runs. The zero value
// inherits the global terminal settings and the workspace directory.
type TerminalSpec struct {
	Shell string
	Args  []string
	// Env are extra KEY=VALUE entries for the child (the process environment is
	// inherited; these are appended).
	Env []string
	Cwd string
	// Label is a short badge for the strip ("pi", "lg") and the tab record, or
	// the editor tab's title.
	Label    string
	InEditor bool
	// KeepOpen leaves the tab, with the program's last output, after it exits.
	KeepOpen bool
	// Nil uses the global padding; a pointer to zero disables padding.
	PaddingX *int
}

// SpawnTerminal opens a terminal tab running the configured shell.
func (a *App) SpawnTerminal() {
	_, _ = a.SpawnTerminalSpec(TerminalSpec{})
}

// SpawnTerminalSpec opens a terminal tab for a specific program. A caller that
// names a Shell supplies its own Args (empty = run the program bare); an empty
// Shell inherits both from the global terminal settings. Startup errors are
// reported in the status bar before being returned.
func (a *App) SpawnTerminalSpec(spec TerminalSpec) (string, error) {
	tw := ui.NewTerminalWidget(nil, a.Palette)
	paddingX := a.Settings.Terminal.PaddingX
	if spec.PaddingX != nil {
		paddingX = max(*spec.PaddingX, 0)
		tw.PaddingX = &paddingX
	}
	var tab *ui.TerminalTabWidget
	var content ui.Rect
	if spec.InEditor {
		tab = ui.NewTerminalTabWidget(tw)
		tab.PaddingX = paddingX
		tab.SetRect(a.EditorGroup.ContentRect())
		content = tab.ContentRect()
	} else {
		r := a.ContentSplit.BottomRect()
		content = a.TerminalPanel.ContentRectFor(tw, ui.Rect{W: r.W, H: r.H - 2}, false)
	}
	cols, rows := content.W, content.H
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	if spec.Shell == "" {
		spec.Shell = a.Settings.Terminal.Shell
		spec.Args = a.Settings.Terminal.Args
		if a.Settings.Terminal.ShellIntegration {
			spec.Shell, spec.Args = terminal.IntegrateShell(spec.Shell, spec.Args)
		}
	}
	if spec.Cwd == "" {
		spec.Cwd = a.Workspace.Primary()
	} else if err := checkTerminalDir(spec.Cwd); err != nil {
		a.StatusError(i18n.Format("status.failedToOpenTerminal", i18n.Args{"error": err.Error()}))
		return "", err
	}
	env := slices.Concat(spec.Env, []string{forwardSocketEnv + "=" + a.forwardSocket})
	t, err := terminal.New(spec.Shell, cols, rows, a.Settings.Terminal.Scrollback, env, spec.Cwd, spec.Args...)
	if err != nil {
		slog.Error("terminal.New", "err", err)
		a.StatusError(i18n.Format("status.failedToOpenTerminal", i18n.Args{"error": err.Error()}))
		return "", err
	}

	tw.Term = t
	tw.WorkDir = spec.Cwd
	tw.CtrlVPastes = a.Settings.Terminal.CtrlVPastes
	tw.NativeLinks = a.Settings.Links.Native
	tw.OnOpenURL = OpenURL
	tw.SuperShortcuts = a.Root.ControlModifiers&tcell.ModMeta != 0
	tw.OnOpenFile = a.openFileLink

	panelID := fmt.Sprintf("terminal-%d", a.TerminalSeq)
	a.TerminalSeq++
	entry := TerminalTab{ID: panelID, Label: spec.Label, Term: t, Widget: tw, Tab: tab}
	if tab != nil {
		a.openEditorTerminal(entry, spec)
	} else {
		a.Terminals = append(a.Terminals, entry)
		a.TerminalPanel.AddTerminalLabeled(tw, spec.Label)
		a.BottomPanel.SetActivePanel("terminal")

		if !a.ContentSplit.ShowBottom {
			a.ContentSplit.ShowBottom = true
		}
		a.Root.SetFocus(a.TerminalPanel)
	}

	t.OnUpdate = func() {
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
	t.OnExit = func() {
		if spec.KeepOpen {
			_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
			return
		}
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(panelID))
	}
	t.Run()
	return panelID, nil
}

// checkTerminalDir rejects an explicit cwd that is missing or not a directory;
// terminal.New would otherwise fall back to the process directory.
func checkTerminalDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("terminal cwd: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("terminal cwd %s: not a directory", dir)
	}
	return nil
}

// ---- terminal handles for plugins (the ttt.terminal Lua module) ----

// SendToTerminal writes text into a tab's PTY. Actions run on the main thread,
// so no extra synchronisation is needed.
func (a *App) SendToTerminal(id, text string) error {
	if tt := a.terminalByID(id); tt != nil {
		tt.Term.WriteString(text)
		return nil
	}
	return fmt.Errorf("no terminal %q", id)
}

// TerminalTail returns the last maxBytes of a tab's raw PTY output.
func (a *App) TerminalTail(id string, maxBytes int) (string, error) {
	if tt := a.terminalByID(id); tt != nil {
		raw := tt.Term.RawTail()
		if maxBytes > 0 && len(raw) > maxBytes {
			raw = raw[len(raw)-maxBytes:]
		}
		return string(raw), nil
	}
	return "", fmt.Errorf("no terminal %q", id)
}

// FocusTerminal reveals the terminal's tab and focuses it.
func (a *App) FocusTerminal(id string) error {
	if a.focusEditorTerminal(id) {
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
		return nil
	}
	idx := -1
	for i, tt := range a.Terminals {
		if tt.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("no terminal %q", id)
	}
	if !a.ContentSplit.ShowBottom {
		a.ContentSplit.ShowBottom = true
	}
	a.TerminalPanel.SetActive(idx)
	a.BottomPanel.SetActivePanel("terminal")
	a.Root.SetFocus(a.TerminalPanel)
	_ = a.Screen.PostEvent(tcell.NewEventInterrupt(nil))
	return nil
}

// TerminalScreen returns the visible screen of one tab as plain text.
func (a *App) TerminalScreen(id string) (string, error) {
	if tt := a.terminalByID(id); tt != nil {
		return tt.Term.ScreenText(), nil
	}
	return "", fmt.Errorf("terminal %q not found", id)
}

// ListTerminals reports the open tabs to a plugin.
func (a *App) ListTerminals() []plugin.TerminalInfo {
	terms := a.allTerminals()
	out := make([]plugin.TerminalInfo, 0, len(terms))
	for _, tt := range terms {
		cols, rows := tt.Term.Size()
		out = append(out, plugin.TerminalInfo{ID: tt.ID, Label: tt.Label, Cols: cols, Rows: rows, Location: tt.location()})
	}
	return out
}

func (a *App) CloseTerminal(panelID string) {
	if a.closeEditorTerminal(panelID) {
		return
	}
	idx := -1
	for i, tt := range a.Terminals {
		if tt.ID == panelID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	a.Terminals[idx].Term.Close()
	a.forgetAppTerminal(panelID)
	a.Terminals = append(a.Terminals[:idx], a.Terminals[idx+1:]...)
	a.TerminalPanel.RemoveTerminal(idx)

	if a.TerminalPanel.Count() == 0 {
		a.hideBottomPanelIfTerminalActive()
	} else {
		a.Root.SetFocus(a.TerminalPanel)
	}
}

func (a *App) CloseActiveTerminal() {
	idx := a.TerminalPanel.ActiveIndex()
	if idx < 0 || idx >= len(a.Terminals) {
		return
	}
	a.CloseTerminal(a.Terminals[idx].ID)
}

// hideBottomPanelIfTerminalActive collapses the bottom panel once the last
// terminal is gone, so `exit` in the shell closes the panel instead of leaving
// a dead one on screen. The panel also hosts Problems, References and Output,
// so it only collapses when the terminal is the tab actually being shown.
func (a *App) hideBottomPanelIfTerminalActive() {
	if a.BottomPanel != nil && a.BottomPanel.ActivePanel != "terminal" {
		a.FocusEditor()
		return
	}
	a.HideBottomPanel()
}

func (a *App) CloseAllTerminals() {
	terms := a.Terminals
	a.Terminals = nil
	for i := len(terms) - 1; i >= 0; i-- {
		a.TerminalPanel.RemoveTerminal(i)
	}
	for _, tt := range terms {
		tt.Term.Close()
		a.forgetAppTerminal(tt.ID)
	}
	a.hideBottomPanelIfTerminalActive()
}

func (a *App) focusedTerminalWidget() *ui.TerminalWidget {
	if a.Root == nil {
		return nil
	}
	if tp, ok := a.Root.Focused.(*ui.TerminalPanelWidget); ok {
		if tw, ok := tp.ActiveWidget().(*ui.TerminalWidget); ok {
			return tw
		}
	} else if tw, ok := a.Root.Focused.(*ui.TerminalWidget); ok {
		return tw
	} else if g, ok := a.Root.Focused.(*ui.EditorGroupWidget); ok {
		if tab := g.ActiveTerminalTab(); tab != nil {
			return tab.Terminal
		}
	}
	return nil
}

func resizeTerminals(app *App) {
	app.resizeEditorTerminals()
	if !app.ContentSplit.ShowBottom {
		return
	}
	r := app.ContentSplit.BottomRect()
	for _, tt := range app.Terminals {
		tt.Term.AckUpdate()
		content := app.TerminalPanel.ContentRectFor(tt.Widget, ui.Rect{W: r.W, H: r.H - 2}, tt.Widget.ScrollbarVisible())
		if content.W > 0 && content.H > 0 {
			tt.Term.Resize(content.W, content.H)
		}
	}
}
