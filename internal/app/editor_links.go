package app

import (
	"context"
	"slices"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) resolveEditorFileLinks(paths []string, dir string, apply func(map[string]string)) {
	if a.fileLinks == nil {
		// A blocked Stat cannot be cancelled; keep one batch running and only the latest queued viewport.
		a.fileLinks = NewTask[map[string]string](a, asynctask.TaskOptions{KeepRunning: true})
	}
	paths = slices.Clone(paths)
	resolve := a.resolveFileLink
	if resolve == nil {
		resolve = ui.ResolveFilePath
	}
	a.fileLinks.Schedule(func(ctx context.Context) (map[string]string, error) {
		resolved := make(map[string]string, len(paths))
		for _, path := range paths {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			resolved[path] = resolve(path, dir)
		}
		return resolved, nil
	}, func(resolved map[string]string, err error) {
		apply(resolved)
	})
}
