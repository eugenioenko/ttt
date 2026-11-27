package app

import (
	"context"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/workspace"
)

type quickOpenState struct {
	palette *ui.SelectDialogWidget
	task    *asynctask.Task[[]ui.PaletteFile]
}

func (a *App) prepareQuickOpen(palette *ui.SelectDialogWidget) {
	state := &quickOpenState{
		palette: palette,
		task:    NewTask[[]ui.PaletteFile](a, asynctask.TaskOptions{}),
	}
	a.quickOpen = state
	palette.OnRequestFiles = func() {
		roots := a.Workspace.Paths()
		visibility := workspace.FileVisibility{
			ShowHidden:     a.Settings.QuickOpen.ShowHidden,
			ShowGitIgnored: a.Settings.QuickOpen.ShowGitIgnored,
		}
		state.task.Schedule(func(ctx context.Context) ([]ui.PaletteFile, error) {
			files := workspace.ListFiles(ctx, roots, visibility)
			items := make([]ui.PaletteFile, len(files))
			for i, file := range files {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				items[i] = ui.PaletteFile{Rel: file.Rel, Abs: file.Abs}
			}
			return items, ctx.Err()
		}, func(files []ui.PaletteFile, err error) {
			if err == nil {
				palette.SetFiles(files)
			}
		})
	}
}
