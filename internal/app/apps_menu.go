package app

import (
	"slices"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) ShowAppMenu(name string, x, y int) {
	openContextMenu(a, []ui.ContextMenuItem{{Label: i18n.Text("apps.remove"), Command: "apps.remove:" + name}}, x, y)
}

func (a *App) RemoveApp(name string) {
	s := *a.Settings
	s.Applications = slices.DeleteFunc(slices.Clone(s.Applications), func(app config.Application) bool { return strings.TrimSpace(app.Name) == name })
	if len(s.Applications) == len(a.Settings.Applications) {
		return
	}
	if err := config.SaveSettings(s); err != nil {
		a.StatusError(i18n.Format("apps.removeFailed", i18n.Args{"name": name, "error": err.Error()}))
		return
	}
	a.ApplySettings(s)
	a.StatusNotify(i18n.Format("apps.removed", i18n.Args{"name": name}))
}
