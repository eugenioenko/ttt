package app

import (
	"github.com/eugenioenko/ttt/internal/ui"
)

// ShowCommandLine opens the framed command line above the status bar as a modal
// overlay and moves focus to it. Any of the callbacks may be nil.
//
// Calling it while a command line is already open replaces it rather than
// stacking a second one, so the saved focus never gets lost behind two layers.
// If some other overlay (dialog, palette, menu) is up, the call is a no-op —
// the usual HasOverlay guard.
func (a *App) ShowCommandLine(prefix string, onChange, onSubmit func(string), onCancel func()) *ui.CommandLineWidget {
	if a.commandLine != nil {
		a.HideCommandLine()
	} else if a.Root.HasOverlay() {
		return nil
	}

	w := ui.NewCommandLineWidget(prefix)
	w.Input().ControlModifiers = &a.Root.ControlModifiers
	w.Borders = a.Borders
	w.OnChange = onChange
	w.OnSubmit = func(text string) {
		a.HideCommandLine()
		if onSubmit != nil {
			onSubmit(text)
		}
	}
	w.OnCancel = func() {
		a.HideCommandLine()
		if onCancel != nil {
			onCancel()
		}
	}

	a.commandLinePrevFocus = a.Root.Focused
	a.commandLine = w
	a.Root.PushOverlay(ui.Overlay{Widget: w, Modal: true})
	a.Root.SetFocus(w)
	return w
}

// HideCommandLine closes the command line and restores the focus that was
// active when it opened.
func (a *App) HideCommandLine() {
	w := a.commandLine
	if w == nil {
		return
	}
	prev := a.commandLinePrevFocus
	a.commandLine = nil
	a.commandLinePrevFocus = nil

	a.Root.RemoveOverlay(w)
	if prev != nil {
		a.Root.SetFocus(prev)
	} else {
		a.FocusEditor()
	}
}

func (a *App) CommandLineActive() bool { return a.commandLine != nil }

func (a *App) CommandLineText() string {
	if a.commandLine == nil {
		return ""
	}
	return a.commandLine.Text()
}

// SetCommandLineText replaces the current text. It is a no-op when no command
// line is open.
func (a *App) SetCommandLineText(text string) {
	if a.commandLine == nil {
		return
	}
	a.commandLine.SetText(text)
}
