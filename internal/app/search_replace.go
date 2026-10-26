package app

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/search"
	"github.com/eugenioenko/ttt/internal/ui"
)

type searchReplaceState struct {
	task    *asynctask.Task[map[string][]search.Match]
	request *ui.SearchReplaceRequest
	scan    func(context.Context, search.Options, []string) (map[string][]search.Match, error)
	dialog  ui.Widget
}

func (a *App) cancelSearchReplace() {
	if a.searchReplace.dialog != nil {
		a.Root.RemoveOverlay(a.searchReplace.dialog)
		if a.Root.Focused == a.searchReplace.dialog {
			a.FocusEditor()
		}
		a.searchReplace.dialog = nil
	}
	if a.searchReplace.task != nil {
		a.searchReplace.task.Stop()
	}
	a.searchReplace.request = nil
	a.Search.SetReplacementPending(false)
}

func (a *App) searchReplaceCurrent(req *ui.SearchReplaceRequest) bool {
	return !a.tasks.closed && a.searchReplace.request == req && a.Search.ReplacementCurrent(*req)
}

func (a *App) startSearchReplace(req ui.SearchReplaceRequest) {
	a.cancelSearchReplace()
	if a.tasks.closed || !a.Search.ReplacementCurrent(req) {
		return
	}
	a.searchReplace.request = &req
	if a.searchReplace.task == nil {
		a.searchReplace.task = NewTask[map[string][]search.Match](a, asynctask.TaskOptions{})
	}
	scan := a.searchReplace.scan
	if scan == nil {
		scan = search.FindAllMatches
	}
	a.Search.SetReplacementPending(true)
	a.searchReplace.task.Schedule(func(ctx context.Context) (map[string][]search.Match, error) {
		if req.Options.Query == "" {
			return nil, ctx.Err()
		}
		found, err := scan(ctx, req.Options, req.Paths)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return found, err
	}, func(found map[string][]search.Match, err error) {
		if !a.searchReplaceCurrent(&req) {
			a.cancelSearchReplace()
			return
		}
		a.Search.SetReplacementPending(false)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			a.cancelSearchReplace()
			return
		}
		matches := make(map[string][]ui.SearchMatch)
		for _, path := range req.Paths {
			ms := found[filepath.Clean(path)]
			// Preserve panel-only callers with no query. A successful empty
			// rescan stays authoritative even when stale LineText is empty.
			if err != nil || req.Options.Query == "" {
				ms = req.Fallback[path]
			}
			if len(ms) > 0 {
				matches[path] = ms
			}
		}
		if len(matches) == 0 {
			a.StatusNotify("No matches to replace")
			return
		}
		opts := ui.SearchOptions{CaseSensitive: req.Options.CaseSensitive, UseRegex: req.Options.UseRegex}
		if req.AllFiles {
			a.confirmSearchReplaceAll(matches, req.Replacement, opts, func() bool { return a.searchReplaceCurrent(&req) })
			a.searchReplace.dialog = a.Root.TopOverlayWidget()
		} else {
			path := req.Paths[0]
			a.ApplySearchReplace(path, matches[path], req.Replacement, opts)
		}
	})
}
