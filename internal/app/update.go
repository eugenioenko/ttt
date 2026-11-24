package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/update"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type UpdateService interface {
	Latest(context.Context) (update.Release, error)
	Apply(context.Context, update.Release, update.Method) error
}

type updateDialogState struct {
	dialog   *widgets.DialogWidget
	adapter  *ui.WidgetAdapter
	task     *asynctask.Task[update.Release]
	service  UpdateService
	method   update.Method
	release  update.Release
	updating bool
}

type updateDialogWidget struct {
	*widgets.DialogWidget
	message *widgets.VStackWidget
	notes   *widgets.BoxWidget
}

func (d *updateDialogWidget) Render(surface widgets.Surface) {
	if d.notes != nil {
		w, h := surface.Size()
		width := max(1, min(d.BoxWidth, w-4)-4)
		d.notes.FixedHeight = max(1, min(12, h-10-d.message.HeightForWidth(width)))
	}
	d.DialogWidget.Render(surface)
}

func (a *App) CheckForUpdates() {
	if a.Root.HasModalOverlay() {
		return
	}
	s := &updateDialogState{dialog: widgets.NewDialogWidget(76), service: a.Updates}
	if s.service == nil {
		s.service = update.Service{}
	}
	s.dialog.Title = i18n.Text("update.checkForUpdates")
	s.dialog.Borders = *a.Borders
	s.dialog.OnDismiss = a.closeUpdateDialog
	s.adapter = ui.NewWidgetAdapter(&updateDialogWidget{DialogWidget: s.dialog})
	s.task = NewTask[update.Release](a, asynctask.TaskOptions{})
	a.updateDialog = s
	a.showUpdateState(i18n.Text("update.checking"), "", []widgets.DialogButton{a.updateCloseButton(i18n.Text("general.cancelButton"))})
	a.ShowDialog(s.adapter)
	a.checkLatestUpdate()
}

func (a *App) closeUpdateDialog() {
	s := a.updateDialog
	if s == nil || s.updating {
		return
	}
	a.releaseTask(s.task)
	a.Root.RemoveOverlay(s.adapter)
	a.updateDialog = nil
	a.FocusEditor()
}

func (a *App) updateCloseButton(label string) widgets.DialogButton {
	return widgets.DialogButton{Label: label, Handler: a.closeUpdateDialog}
}

func (a *App) showUpdateState(message, notes string, buttons []widgets.DialogButton) {
	s := a.updateDialog
	md := widgets.NewMarkdownWidget()
	md.SetContent(notes)
	scroll := widgets.NewScrollViewWidget(md)
	md.SetScrollParent(scroll)
	view := s.adapter.Inner().(*updateDialogWidget)
	view.message = widgets.NewVStackWidget()
	view.message.Gap = 1
	for _, paragraph := range strings.Split(message, "\n") {
		view.message.Children = append(view.message.Children, widgets.NewParagraphWidget(paragraph))
	}
	view.notes = nil
	content := widgets.NewVStackWidget(view.message)
	if notes != "" {
		box := widgets.NewBoxWidget(widgets.BoxModel{})
		box.Child = scroll
		box.FixedHeight = max(1, min(12, a.Root.Height-15))
		view.notes = box
		content.Children = append(content.Children, box)
		content.Gap = 1
	}
	s.dialog.SetContent(content)
	s.dialog.Buttons = buttons
	s.dialog.Build()
	s.adapter.RebuildFocus()
}

func (a *App) checkLatestUpdate() {
	s := a.updateDialog
	a.showUpdateState(i18n.Text("update.checking"), "", []widgets.DialogButton{a.updateCloseButton(i18n.Text("general.cancelButton"))})
	version, method := a.Version, a.UpdateMethod
	var detected update.Method
	if method != nil {
		detected = *method
	}
	s.task.Schedule(func(ctx context.Context) (update.Release, error) {
		if method == nil {
			var err error
			detected, err = update.CurrentMethod(version)
			if err != nil {
				return update.Release{}, err
			}
		}
		return s.service.Latest(ctx)
	}, func(release update.Release, err error) {
		if a.updateDialog != s {
			return
		}
		if err != nil {
			a.showUpdateCheckError(err)
			return
		}
		s.method = detected
		current := version
		if update.IsDevelopmentVersion(version) {
			s.method.Kind = update.Dev
			current = "dev"
		}
		cmp, err := update.CompareVersions(current, release.Tag)
		if err != nil {
			a.showUpdateCheckError(err)
			return
		}
		if cmp >= 0 {
			a.showUpdateState(i18n.Format("update.latest", i18n.Args{"version": displayVersion(version)}), "", []widgets.DialogButton{a.updateCloseButton(i18n.Text("general.closeButton"))})
			return
		}
		s.release = release
		message := i18n.Format("update.available", i18n.Args{"latest": displayVersion(release.Tag), "current": displayVersion(version)})
		buttons := []widgets.DialogButton{a.updateCloseButton(i18n.Text("general.closeButton"))}
		switch s.method.Kind {
		case update.Dev:
			message += "\n" + i18n.Text("update.devBuild")
		case update.Nix:
			message += "\n" + i18n.Text("update.nix")
		default:
			buttons = []widgets.DialogButton{a.updateCloseButton(i18n.Text("general.cancelButton")), {Label: i18n.Text("update.updateButton"), Handler: a.applyUpdate}}
		}
		a.showUpdateState(message, release.Notes, buttons)
	})
}

func (a *App) showUpdateCheckError(err error) {
	a.showUpdateState(err.Error(), "", []widgets.DialogButton{
		{Label: i18n.Text("update.retryButton"), Handler: a.checkLatestUpdate},
		a.updateCloseButton(i18n.Text("general.closeButton")),
	})
}

func (a *App) applyUpdate() {
	s := a.updateDialog
	if s.updating {
		return
	}
	s.updating = true
	a.showUpdateState(i18n.Text("update.updating"), "", []widgets.DialogButton{{Label: i18n.Text("general.cancelButton")}, {Label: i18n.Text("update.updateButton")}})
	for _, child := range s.dialog.WidgetChildren() {
		if footer, ok := child.(*widgets.HStackWidget); ok {
			for _, button := range footer.Children {
				button.(*widgets.ButtonWidget).Disabled = true
			}
		}
	}
	release, method := s.release, s.method
	s.task.Schedule(func(ctx context.Context) (update.Release, error) {
		return release, s.service.Apply(ctx, release, method)
	}, func(_ update.Release, err error) {
		s.updating = false
		if err != nil {
			message := err.Error()
			if errors.Is(err, update.ErrNotWritable) {
				message = i18n.Text("update.rerunInstall") + "\n" + message
			}
			a.showUpdateState(i18n.Text("update.failed"), message, []widgets.DialogButton{a.updateCloseButton(i18n.Text("general.closeButton"))})
			return
		}
		a.showUpdateState(i18n.Format("update.success", i18n.Args{"version": displayVersion(release.Tag)}), "", []widgets.DialogButton{
			{Label: i18n.Text("update.restartButton"), Handler: a.restartAfterUpdate},
			a.updateCloseButton(i18n.Text("general.closeButton")),
		})
	})
}

func (a *App) restartAfterUpdate() {
	method := a.updateDialog.method.Kind
	a.closeUpdateDialog()
	a.quitWithAction(func() {
		a.RestartRequested = true
		a.RestartMethod = method
		a.forceQuit()
	})
}

func displayVersion(version string) string {
	if version == "dev" {
		return version
	}
	return "v" + strings.TrimPrefix(version, "v")
}

func (a *App) CheckForUpdatesOnStartup(now time.Time) {
	if a.Settings == nil || !a.Settings.Update.StartupCheckDue(a.startupUpdateLastCheck(), now) {
		return
	}
	if a.Root != nil && a.Root.HasModalOverlay() {
		return
	}
	a.rememberStartupUpdateCheck(now)

	service := a.Updates
	if service == nil {
		service = update.Service{}
	}
	if a.updateStartupTask == nil {
		a.updateStartupTask = NewTask[update.Release](a, asynctask.TaskOptions{})
	}
	version, method := a.Version, a.UpdateMethod
	var detected update.Method
	if method != nil {
		detected = *method
	}
	a.updateStartupTask.Schedule(func(ctx context.Context) (update.Release, error) {
		if method == nil {
			var err error
			detected, err = update.CurrentMethod(version)
			if err != nil {
				return update.Release{}, err
			}
		}
		return service.Latest(ctx)
	}, func(release update.Release, err error) {
		if err != nil || a.updateDialog != nil {
			return
		}
		if a.Root != nil && a.Root.HasModalOverlay() {
			return
		}
		current := version
		if update.IsDevelopmentVersion(version) {
			detected.Kind = update.Dev
			current = "dev"
		}
		cmp, cmpErr := update.CompareVersions(current, release.Tag)
		if cmpErr != nil || cmp >= 0 {
			return
		}
		a.showStartupUpdate(release, detected)
	})
}

func (a *App) startupUpdateLastCheck() time.Time {
	if t := config.LoadState().LastUpdateCheckTime(); !t.IsZero() {
		return t
	}
	return a.State.LastUpdateCheckTime()
}

func (a *App) rememberStartupUpdateCheck(now time.Time) {
	disk := config.LoadState()
	disk.SetLastUpdateCheckTime(now)
	if err := config.SaveState(disk); err != nil {
		return
	}
	a.State.SetLastUpdateCheckTime(now)
}

func (a *App) showStartupUpdate(release update.Release, method update.Method) {
	if a.Root == nil || a.Root.HasModalOverlay() {
		return
	}
	s := &updateDialogState{dialog: widgets.NewDialogWidget(76), service: a.Updates, method: method, release: release}
	if s.service == nil {
		s.service = update.Service{}
	}
	s.dialog.Title = i18n.Text("update.checkForUpdates")
	s.dialog.Borders = *a.Borders
	s.dialog.OnDismiss = a.closeUpdateDialog
	s.adapter = ui.NewWidgetAdapter(&updateDialogWidget{DialogWidget: s.dialog})
	if a.updateStartupTask != nil {
		s.task = a.updateStartupTask
	} else {
		s.task = NewTask[update.Release](a, asynctask.TaskOptions{})
	}
	a.updateDialog = s
	message := i18n.Format("update.available", i18n.Args{"latest": displayVersion(release.Tag), "current": displayVersion(a.Version)})
	buttons := []widgets.DialogButton{a.updateCloseButton(i18n.Text("general.closeButton"))}
	switch s.method.Kind {
	case update.Dev:
		message += "\n" + i18n.Text("update.devBuild")
	case update.Nix:
		message += "\n" + i18n.Text("update.nix")
	default:
		buttons = []widgets.DialogButton{a.updateCloseButton(i18n.Text("general.cancelButton")), {Label: i18n.Text("update.updateButton"), Handler: a.applyUpdate}}
	}
	a.showUpdateState(message, release.Notes, buttons)
	a.ShowDialog(s.adapter)
}
