package app

import (
	"github.com/eugenioenko/ttt/internal/widgets"
)

const menuBarToggleCommand = "menubar.toggle"

// applyMenuBarVisibility adds or removes the menu bar from the root stack.
// VStackWidget reads a child Height() of 0 as "grow", so a hidden bar cannot
// simply collapse itself — it has to leave the stack entirely.
func (a *App) applyMenuBarVisibility(visible bool) {
	if a.RootBox == nil || a.MenuBar == nil {
		return
	}
	a.MenuBar.Visible = visible
	if visible {
		a.RootBox.Children = []widgets.Widget{a.MenuBar, a.SplitPanel, a.StatusBar}
		return
	}
	a.MenuBar.Selected = -1
	a.RootBox.Children = []widgets.Widget{a.SplitPanel, a.StatusBar}
	if a.Root != nil && a.Root.Focused == a.MenuBar {
		a.Root.SetFocus(a.EditorGroup)
	}
}

// menuBarRestoreHint names both ways back to a hidden menu bar. The shortcut is
// read from the registry so a rebound key is reported correctly.
func (a *App) menuBarRestoreHint() string {
	msg := "Menu bar hidden: run \"View: Toggle Menu Bar\" from the command palette to show it"
	if a.Reg != nil {
		if cmd, ok := a.Reg.Get(menuBarToggleCommand); ok && cmd.Shortcut != "" {
			msg = "Menu bar hidden: press " + cmd.Shortcut + " to show it"
		}
	}
	return msg
}

func (a *App) ToggleMenuBar() {
	visible := !a.Settings.Editor.IsMenuBarVisible()
	a.Settings.Editor.MenuBar = boolPtr(visible)
	a.SaveAndApplySettings()
	if !visible {
		a.StatusNotify(a.menuBarRestoreHint())
	}
}
