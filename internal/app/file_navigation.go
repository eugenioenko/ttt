package app

import ()

func (a *App) openFileLink(path string, line, col int) {
	if err := a.EditorGroup.OpenFile(path); err != nil {
		a.StatusWarn(err.Error())
		return
	}
	if line > 0 {
		a.EditorGroup.GoToLineCol(line, col)
	}
	a.FocusEditor()
}

func (a *App) navigateToLocation(file string, line, _ int) {
	if err := a.EditorGroup.OpenFile(file); err != nil {
		return
	}
	a.EditorGroup.GoToLine(line + 1)
	a.Root.SetFocus(a.EditorGroup)
}
